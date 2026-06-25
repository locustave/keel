// Package planner derives build phases from a TDD ## Deliverables section
// using a deterministic, policy-driven algorithm.
//
// # Phase planning algorithm
//
//  1. Validate deliverables: unique IDs, required fields, no unknown depends_on.
//  2. Build a DAG from depends_on edges (dependency → dependent).
//  3. Compute topological layers via Kahn's algorithm; within each layer sort
//     deliverables by: isolation priority → critical-path depth (desc) →
//     risk score (desc) → id (asc) for stable, repeatable output.
//  4. Apply phase policy: hard-isolated types each get a solo phase; remaining
//     deliverables are packed greedily until any ceiling is exceeded.
//  5. Emit a richer phase manifest with allowed_files, risk_score, and
//     rollback_checkpoint_required alongside the backward-compatible fields.
//
// # Phase policy
//
// A ## Phase Policy fenced YAML block in the TDD overrides defaults.
// If absent, the built-in defaults are used (max_files=15, max_criteria=6,
// max_risk=8; isolation types: schema, migration, contract, auth,
// security-sensitive, cross-cutting-refactor, public-api-change, generated-code).
//
// # Backward compatibility
//
// Old deliverables using `files` continue to work. New deliverables may use
// `touches`; if both are present, `touches` takes priority.
package planner

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Phase policy
// ---------------------------------------------------------------------------

// PhasePolicy controls isolation, packing ceilings, and risk scoring.
type PhasePolicy struct {
	MaxFilesPerPhase     int
	MaxCriteriaPerPhase  int
	MaxRiskScorePerPhase int
	// IsolationTypes is the set of deliverable types that must have solo phases.
	IsolationTypes map[string]bool
	// IsolationTypesOrder determines sort priority (index 0 = highest priority).
	IsolationTypesOrder []string
	// RiskScores maps deliverable type → risk score integer.
	RiskScores map[string]int
}

// RiskScoreFor returns the risk score for a deliverable type (0 if unknown).
func (p *PhasePolicy) RiskScoreFor(typ string) int {
	return p.RiskScores[typ]
}

// DefaultPolicy returns the built-in phase policy.
func DefaultPolicy() PhasePolicy {
	return PhasePolicy{
		MaxFilesPerPhase:     15,
		MaxCriteriaPerPhase:  6,
		MaxRiskScorePerPhase: 8,
		IsolationTypesOrder: []string{
			"schema", "migration", "contract", "auth",
			"security-sensitive", "cross-cutting-refactor",
			"public-api-change", "generated-code",
		},
		IsolationTypes: map[string]bool{
			"schema":                 true,
			"migration":              true,
			"contract":               true,
			"auth":                   true,
			"security-sensitive":     true,
			"cross-cutting-refactor": true,
			"public-api-change":      true,
			"generated-code":         true,
		},
		RiskScores: map[string]int{
			"schema":                 5,
			"migration":              5,
			"auth":                   5,
			"security-sensitive":     5,
			"contract":               4,
			"cross-cutting-refactor": 4,
			"public-api-change":      4,
			"generated-code":         3,
			"api":                    2,
			"backend":                2,
			"integration":            2,
			"logic":                  2,
			"frontend":               1,
			"ui":                     1,
			"infra":                  1,
			"tests":                  1,
			"docs":                   1,
		},
	}
}

// isolationPriority returns the sort key for a type's isolation priority.
// Lower value = higher priority (appears first). Non-isolated types return
// a value higher than any isolation type index.
func (p *PhasePolicy) isolationPriority(typ string) int {
	for i, t := range p.IsolationTypesOrder {
		if t == typ {
			return i
		}
	}
	if p.IsolationTypes[typ] {
		return len(p.IsolationTypesOrder) // isolated but not in ordered list
	}
	return len(p.IsolationTypesOrder) + 1
}

// ---------------------------------------------------------------------------
// Parse phase policy from TDD
// ---------------------------------------------------------------------------

var rePolicyBlock = regexp.MustCompile(`(?s)## Phase Policy\s*\n+` + "```yaml\n" + `(.*?)` + "```")

type policyFileYAML struct {
	PhasePolicy struct {
		MaxFilesPerPhase     int            `yaml:"max_files_per_phase"`
		MaxCriteriaPerPhase  int            `yaml:"max_criteria_per_phase"`
		MaxRiskScorePerPhase int            `yaml:"max_risk_score_per_phase"`
		IsolationTypes       []string       `yaml:"isolation_types"`
		RiskScores           map[string]int `yaml:"risk_scores"`
	} `yaml:"phase_policy"`
}

// ParsePolicy extracts a phase_policy block from the TDD text.
// Returns the default policy if no ## Phase Policy section is found.
func ParsePolicy(tddText string) (PhasePolicy, error) {
	m := rePolicyBlock.FindStringSubmatch(tddText)
	if m == nil {
		return DefaultPolicy(), nil
	}
	var raw policyFileYAML
	if err := yaml.Unmarshal([]byte(m[1]), &raw); err != nil {
		return DefaultPolicy(), fmt.Errorf("cannot parse phase_policy block: %w", err)
	}
	p := raw.PhasePolicy
	policy := DefaultPolicy()
	if p.MaxFilesPerPhase > 0 {
		policy.MaxFilesPerPhase = p.MaxFilesPerPhase
	}
	if p.MaxCriteriaPerPhase > 0 {
		policy.MaxCriteriaPerPhase = p.MaxCriteriaPerPhase
	}
	if p.MaxRiskScorePerPhase > 0 {
		policy.MaxRiskScorePerPhase = p.MaxRiskScorePerPhase
	}
	if len(p.IsolationTypes) > 0 {
		policy.IsolationTypesOrder = p.IsolationTypes
		policy.IsolationTypes = make(map[string]bool, len(p.IsolationTypes))
		for _, t := range p.IsolationTypes {
			policy.IsolationTypes[t] = true
		}
	}
	if len(p.RiskScores) > 0 {
		for k, v := range p.RiskScores {
			policy.RiskScores[k] = v
		}
	}
	return policy, nil
}

// ---------------------------------------------------------------------------
// Deliverable
// ---------------------------------------------------------------------------

// Deliverable represents one item from the ## Deliverables YAML block.
type Deliverable struct {
	ID          string
	Type        string
	Description string
	Files       []string // backward-compatible
	Touches     []string // preferred over Files if non-empty
	DependsOn   []string
	VerifiedBy  []string
}

// AllFiles returns Touches if non-empty, otherwise Files.
func (d *Deliverable) AllFiles() []string {
	if len(d.Touches) > 0 {
		return d.Touches
	}
	return d.Files
}

// ---------------------------------------------------------------------------
// Phase
// ---------------------------------------------------------------------------

// Phase is a grouped set of deliverables assigned to a numbered phase.
type Phase struct {
	Number                     int    // gate file index (phase_N.gate.json)
	ID                         string // formatted id: phase-001
	Name                       string
	DependencyLayer            int
	IsolationReason            string // empty string if not isolated
	DeliverableIDs             []string
	Descriptions               []string
	AllowedFiles               []string
	ExitCriteria               []string
	RiskScore                  int
	RollbackCheckpointRequired bool
}

// ---------------------------------------------------------------------------
// Options
// ---------------------------------------------------------------------------

// Options controls the planner's behaviour.
type Options struct {
	TDDPath       string
	Confirm       bool
	StartPhase    int
	ExistingGates string // directory to auto-detect start phase
	FeatureSlug   string
	Title         string
	Out           io.Writer // stdout for dry-run output (nil → os.Stdout)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// ValidateDeliverables checks that all deliverables have required fields,
// unique IDs, and valid depends_on references. Call before BuildDAG.
func ValidateDeliverables(deliverables []Deliverable) error {
	var errs []string

	// Pass 1: collect IDs, check uniqueness and required id field.
	ids := make(map[string]bool, len(deliverables))
	for _, d := range deliverables {
		if d.ID == "" {
			errs = append(errs, "a deliverable is missing required field: id")
			continue
		}
		if ids[d.ID] {
			errs = append(errs, fmt.Sprintf("duplicate deliverable id: %q", d.ID))
		}
		ids[d.ID] = true
	}

	// Pass 2: validate fields and cross-references.
	for _, d := range deliverables {
		if d.ID == "" {
			continue // already reported
		}
		if d.Type == "" {
			errs = append(errs, fmt.Sprintf("deliverable %q missing required field: type", d.ID))
		}
		if len(d.AllFiles()) == 0 {
			errs = append(errs, fmt.Sprintf("deliverable %q missing required field: files or touches", d.ID))
		}
		if len(d.VerifiedBy) == 0 {
			errs = append(errs, fmt.Sprintf("deliverable %q missing required field: verified_by", d.ID))
		}
		for _, dep := range d.DependsOn {
			if !ids[dep] {
				errs = append(errs, fmt.Sprintf("deliverable %q depends_on unknown id: %q", d.ID, dep))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("deliverable validation failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// YAML parser for ## Deliverables block
// ---------------------------------------------------------------------------

var (
	reNewDeliverable = regexp.MustCompile(`^- id:`)
	reEmptyList      = regexp.MustCompile(`^  (\w[\w-]*):\s*\[\]\s*$`)
	reInlineList     = regexp.MustCompile(`^  (\w[\w-]*):\s*\[(.+)\]\s*$`)
	reScalar         = regexp.MustCompile(`^  (\w[\w-]*):\s+([^\[].*)$`)
	reListHeader     = regexp.MustCompile(`^  (\w[\w-]*):\s*$`)
	reListItem       = regexp.MustCompile(`^    - (.+)$`)
)

// ParseDeliverablesYAML parses the minimal YAML deliverables format.
func ParseDeliverablesYAML(block string) []Deliverable {
	var deliverables []Deliverable
	var current *map[string]interface{}
	currentListKey := ""

	flushCurrent := func() {
		if current == nil {
			return
		}
		d := mapToDeliverable(*current)
		deliverables = append(deliverables, d)
		current = nil
		currentListKey = ""
	}

	for _, raw := range strings.Split(block, "\n") {
		line := strings.TrimRight(raw, "\r")

		if reNewDeliverable.MatchString(line) {
			flushCurrent()
			parts := strings.SplitN(line, "id:", 2)
			id := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			m := map[string]interface{}{"id": id}
			current = &m
			currentListKey = ""
			continue
		}
		if current == nil {
			continue
		}

		if m := reEmptyList.FindStringSubmatch(line); m != nil {
			(*current)[m[1]] = []string{}
			currentListKey = ""
			continue
		}
		if m := reInlineList.FindStringSubmatch(line); m != nil {
			key := m[1]
			var items []string
			for _, v := range strings.Split(m[2], ",") {
				v = strings.Trim(strings.TrimSpace(v), `"'`)
				if v != "" {
					items = append(items, v)
				}
			}
			(*current)[key] = items
			currentListKey = ""
			continue
		}
		if m := reScalar.FindStringSubmatch(line); m != nil {
			key := m[1]
			val := strings.Trim(strings.TrimSpace(m[2]), `"'`)
			(*current)[key] = val
			currentListKey = ""
			continue
		}
		if m := reListHeader.FindStringSubmatch(line); m != nil {
			currentListKey = m[1]
			(*current)[currentListKey] = []string{}
			continue
		}
		if m := reListItem.FindStringSubmatch(line); m != nil && currentListKey != "" {
			val := strings.Trim(strings.TrimSpace(m[1]), `"'`)
			existing := (*current)[currentListKey].([]string)
			(*current)[currentListKey] = append(existing, val)
			continue
		}
	}
	flushCurrent()
	return deliverables
}

func mapToDeliverable(m map[string]interface{}) Deliverable {
	d := Deliverable{}
	if v, ok := m["id"].(string); ok {
		d.ID = v
	}
	if v, ok := m["type"].(string); ok {
		d.Type = v
	}
	if v, ok := m["description"].(string); ok {
		d.Description = v
	}
	if v, ok := m["files"].([]string); ok {
		d.Files = v
	}
	if v, ok := m["touches"].([]string); ok {
		d.Touches = v
	}
	if v, ok := m["depends_on"].([]string); ok {
		d.DependsOn = v
	}
	if v, ok := m["verified_by"].([]string); ok {
		d.VerifiedBy = v
	}
	return d
}

var reDeliverablesBlock = regexp.MustCompile(`(?s)## Deliverables\s*\n+` + "```yaml\n" + `(.*?)` + "```")

// ExtractDeliverables finds and parses the ## Deliverables block.
// Returns nil if no block is found.
func ExtractDeliverables(tddText string) ([]Deliverable, error) {
	m := reDeliverablesBlock.FindStringSubmatch(tddText)
	if m == nil {
		return nil, nil
	}
	return ParseDeliverablesYAML(m[1]), nil
}

// ---------------------------------------------------------------------------
// DAG construction
// ---------------------------------------------------------------------------

// BuildDAG constructs adjacency list and in-degree map from deliverables.
// Edge direction: dependency → dependent (forward edges).
func BuildDAG(deliverables []Deliverable) (adj map[string][]string, inDegree map[string]int, err error) {
	ids := make(map[string]bool, len(deliverables))
	for _, d := range deliverables {
		ids[d.ID] = true
	}

	adj = make(map[string][]string)
	inDegree = make(map[string]int, len(deliverables))
	for _, d := range deliverables {
		inDegree[d.ID] = 0
	}

	for _, d := range deliverables {
		for _, dep := range d.DependsOn {
			if !ids[dep] {
				return nil, nil, fmt.Errorf("deliverable %q depends on unknown id %q", d.ID, dep)
			}
			adj[dep] = append(adj[dep], d.ID)
			inDegree[d.ID]++
		}
	}
	return adj, inDegree, nil
}

// ---------------------------------------------------------------------------
// Topological layers
// ---------------------------------------------------------------------------

// TopologicalLayers runs Kahn's algorithm and returns nodes grouped by
// dependency depth layer. Within each layer the order is stable (sorted by
// deliverable id) so downstream sort steps produce repeatable results.
func TopologicalLayers(deliverables []Deliverable, adj map[string][]string, inDegree map[string]int) ([][]string, error) {
	inDeg := make(map[string]int, len(inDegree))
	for k, v := range inDegree {
		inDeg[k] = v
	}

	// Seed queue with zero-in-degree nodes sorted by id for stable ordering.
	var queue []string
	for _, d := range deliverables {
		if inDeg[d.ID] == 0 {
			queue = append(queue, d.ID)
		}
	}
	sort.Strings(queue)

	var layers [][]string
	for len(queue) > 0 {
		layer := make([]string, len(queue))
		copy(layer, queue)
		layers = append(layers, layer)

		queue = nil
		var nextLayer []string
		for _, node := range layer {
			for _, neighbor := range adj[node] {
				inDeg[neighbor]--
				if inDeg[neighbor] == 0 {
					nextLayer = append(nextLayer, neighbor)
				}
			}
		}
		sort.Strings(nextLayer)
		queue = nextLayer
	}

	allLayered := make(map[string]bool)
	for _, layer := range layers {
		for _, n := range layer {
			allLayered[n] = true
		}
	}
	var cycleNodes []string
	for _, d := range deliverables {
		if !allLayered[d.ID] {
			cycleNodes = append(cycleNodes, d.ID)
		}
	}
	if len(cycleNodes) > 0 {
		sort.Strings(cycleNodes)
		return nil, fmt.Errorf("dependency cycle detected involving: %v", cycleNodes)
	}

	return layers, nil
}

// ---------------------------------------------------------------------------
// Critical path depth
// ---------------------------------------------------------------------------

// computeCriticalDepth returns for each node the length of the longest forward
// path from that node to any leaf. Leaf nodes = 1. Higher depth = more work
// depends on this node (it is more "blocking").
func computeCriticalDepth(adj map[string][]string, allIDs []string) map[string]int {
	depth := make(map[string]int, len(allIDs))
	var dfs func(id string) int
	dfs = func(id string) int {
		if d := depth[id]; d > 0 {
			return d
		}
		children := adj[id]
		if len(children) == 0 {
			depth[id] = 1
			return 1
		}
		max := 0
		for _, child := range children {
			if cd := dfs(child); cd > max {
				max = cd
			}
		}
		depth[id] = 1 + max
		return depth[id]
	}
	for _, id := range allIDs {
		dfs(id)
	}
	return depth
}

// ---------------------------------------------------------------------------
// Deterministic sort within a layer
// ---------------------------------------------------------------------------

// sortLayer returns the deliverables in a layer sorted by:
//  1. isolation priority (lower index in IsolationTypesOrder = first)
//  2. critical-path depth, descending
//  3. risk score, descending
//  4. deliverable id, ascending
func sortLayer(ids []string, dmap map[string]*Deliverable, critDepth map[string]int, policy *PhasePolicy) []string {
	type item struct {
		id      string
		isoPrio int
		depth   int
		risk    int
	}
	items := make([]item, len(ids))
	for i, id := range ids {
		d := dmap[id]
		items[i] = item{
			id:      id,
			isoPrio: policy.isolationPriority(d.Type),
			depth:   critDepth[id],
			risk:    policy.RiskScoreFor(d.Type),
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.isoPrio != b.isoPrio {
			return a.isoPrio < b.isoPrio
		}
		if a.depth != b.depth {
			return a.depth > b.depth // descending
		}
		if a.risk != b.risk {
			return a.risk > b.risk // descending
		}
		return a.id < b.id // ascending
	})
	result := make([]string, len(items))
	for i, it := range items {
		result[i] = it.id
	}
	return result
}

// ---------------------------------------------------------------------------
// Phase grouping
// ---------------------------------------------------------------------------

// GroupIntoPhases assigns deliverables to numbered phases using the policy.
// adj is the forward adjacency map from BuildDAG (needed for critical-path depth).
func GroupIntoPhases(layers [][]string, adj map[string][]string, dmap map[string]*Deliverable, startPhase int, policy PhasePolicy) []Phase {
	// Collect all IDs for critical depth computation.
	allIDs := make([]string, 0, len(dmap))
	for id := range dmap {
		allIDs = append(allIDs, id)
	}
	critDepth := computeCriticalDepth(adj, allIDs)

	var phases []Phase
	num := startPhase

	for layerIdx, layer := range layers {
		sorted := sortLayer(layer, dmap, critDepth, &policy)

		var packGroup []*Deliverable
		gFiles, gCriteria, gRisk := 0, 0, 0

		for _, nid := range sorted {
			d := dmap[nid]

			// Hard-isolated deliverables always get their own phase.
			if policy.IsolationTypes[d.Type] {
				// Flush any open pack group first.
				if len(packGroup) > 0 {
					phases = append(phases, makePhase(num, layerIdx, packGroup, "", &policy))
					num++
					packGroup = nil
					gFiles, gCriteria, gRisk = 0, 0, 0
				}
				phases = append(phases, makePhase(num, layerIdx, []*Deliverable{d}, d.Type, &policy))
				num++
				continue
			}

			// Non-isolated: try to pack into current group.
			df := len(d.AllFiles())
			dc := len(d.VerifiedBy)
			dr := policy.RiskScoreFor(d.Type)

			if len(packGroup) > 0 &&
				(gFiles+df > policy.MaxFilesPerPhase ||
					gCriteria+dc > policy.MaxCriteriaPerPhase ||
					gRisk+dr > policy.MaxRiskScorePerPhase) {
				phases = append(phases, makePhase(num, layerIdx, packGroup, "", &policy))
				num++
				packGroup = nil
				gFiles, gCriteria, gRisk = 0, 0, 0
			}

			packGroup = append(packGroup, d)
			gFiles += df
			gCriteria += dc
			gRisk += dr
		}

		if len(packGroup) > 0 {
			phases = append(phases, makePhase(num, layerIdx, packGroup, "", &policy))
			num++
			packGroup = nil
		}
	}
	return phases
}

func makePhase(num int, layer int, deliverables []*Deliverable, isolationReason string, policy *PhasePolicy) Phase {
	var ids, descs, files, criteria []string
	totalRisk := 0

	for _, d := range deliverables {
		ids = append(ids, d.ID)
		desc := d.Description
		if desc == "" {
			desc = d.ID
		}
		descs = append(descs, desc)
		files = append(files, d.AllFiles()...)
		criteria = append(criteria, d.VerifiedBy...)
		totalRisk += policy.RiskScoreFor(d.Type)
	}

	name := descs[0]
	if len(deliverables) > 1 {
		name = strings.Join(ids, "; ")
	}

	phaseID := fmt.Sprintf("phase-%03d", num+1) // 1-indexed, zero-padded

	return Phase{
		Number:                     num,
		ID:                         phaseID,
		Name:                       name,
		DependencyLayer:            layer,
		IsolationReason:            isolationReason,
		DeliverableIDs:             ids,
		Descriptions:               descs,
		AllowedFiles:               files,
		ExitCriteria:               criteria,
		RiskScore:                  totalRisk,
		RollbackCheckpointRequired: true,
	}
}

// ---------------------------------------------------------------------------
// Output
// ---------------------------------------------------------------------------

// FormatPlan returns the human-readable dry-run output.
func FormatPlan(phases []Phase, tddPath string, policy PhasePolicy) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Phase plan derived from: %s\n", tddPath)
	fmt.Fprintf(&sb, "Total phases: %d\n", len(phases))
	fmt.Fprintf(&sb, "Policy: max_files=%d, max_criteria=%d, max_risk=%d\n",
		policy.MaxFilesPerPhase, policy.MaxCriteriaPerPhase, policy.MaxRiskScorePerPhase)

	isoKeys := make([]string, len(policy.IsolationTypesOrder))
	copy(isoKeys, policy.IsolationTypesOrder)
	fmt.Fprintf(&sb, "Isolation types (solo phase): %s\n", strings.Join(isoKeys, ", "))
	fmt.Fprintf(&sb, "Sort order: isolation priority → critical-path depth ↓ → risk score ↓ → id ↑\n\n")

	for _, p := range phases {
		isoNote := ""
		if p.IsolationReason != "" {
			isoNote = fmt.Sprintf(" [isolated: %s]", p.IsolationReason)
		}
		fmt.Fprintf(&sb, "Phase %d [%s] — %s (layer %d)%s\n",
			p.Number, p.ID, p.Name, p.DependencyLayer, isoNote)
		fmt.Fprintf(&sb, "  Deliverables (%d): %s\n",
			len(p.DeliverableIDs), strings.Join(p.DeliverableIDs, ", "))

		filePreview := p.AllowedFiles
		suffix := ""
		if len(p.AllowedFiles) > 4 {
			filePreview = p.AllowedFiles[:4]
			suffix = fmt.Sprintf(" +%d more", len(p.AllowedFiles)-4)
		}
		fmt.Fprintf(&sb, "  Files (%d): %s%s\n",
			len(p.AllowedFiles), strings.Join(filePreview, ", "), suffix)

		critPreview := p.ExitCriteria
		csuffix := ""
		if len(p.ExitCriteria) > 3 {
			critPreview = p.ExitCriteria[:3]
			csuffix = fmt.Sprintf(" +%d more", len(p.ExitCriteria)-3)
		}
		fmt.Fprintf(&sb, "  Gates (%d): %s%s\n",
			len(p.ExitCriteria), strings.Join(critPreview, ", "), csuffix)
		fmt.Fprintf(&sb, "  Risk score: %d | rollback checkpoint: required\n\n", p.RiskScore)
	}
	return sb.String()
}

func renderPhaseBlocks(phases []Phase) string {
	var sb strings.Builder
	for _, p := range phases {
		isolationReason := "null"
		if p.IsolationReason != "" {
			isolationReason = p.IsolationReason
		}

		fmt.Fprintf(&sb, "- phase: %d\n", p.Number)
		fmt.Fprintf(&sb, "  id: %s\n", p.ID)
		fmt.Fprintf(&sb, "  name: %s\n", p.Name)
		fmt.Fprintf(&sb, "  dependency_layer: %d\n", p.DependencyLayer)
		fmt.Fprintf(&sb, "  isolation_reason: %s\n", isolationReason)
		fmt.Fprintf(&sb, "  risk_score: %d\n", p.RiskScore)
		fmt.Fprintf(&sb, "  rollback_checkpoint_required: true\n")

		fmt.Fprintf(&sb, "  deliverables:\n")
		for _, id := range p.DeliverableIDs {
			fmt.Fprintf(&sb, "  - %s\n", id)
		}

		fmt.Fprintf(&sb, "  allowed_files:\n")
		for _, f := range p.AllowedFiles {
			fmt.Fprintf(&sb, "  - %s\n", f)
		}
		fmt.Fprintf(&sb, "  blocked_files: []\n")

		fmt.Fprintf(&sb, "  goal: Implement %s.\n", strings.Join(p.Descriptions, "; "))
		fmt.Fprintf(&sb, "  tasks:\n")
		for _, desc := range p.Descriptions {
			fmt.Fprintf(&sb, "  - %s\n", desc)
		}

		fmt.Fprintf(&sb, "  exit_criteria:\n")
		if len(p.ExitCriteria) == 0 {
			fmt.Fprintf(&sb, "  - All deliverable files exist and unit tests pass.\n")
		} else {
			for _, c := range p.ExitCriteria {
				fmt.Fprintf(&sb, "  - %s\n", c)
			}
		}

		fmt.Fprintf(&sb, "  gates:\n")
		if len(p.ExitCriteria) == 0 {
			fmt.Fprintf(&sb, "  - All deliverable files exist and unit tests pass.\n")
		} else {
			for _, c := range p.ExitCriteria {
				fmt.Fprintf(&sb, "  - %s\n", c)
			}
		}

		fmt.Fprintf(&sb, "  out_of_scope:\n")
		fmt.Fprintf(&sb, "  - Deliverables belonging to other phases.\n")
	}
	return sb.String()
}

// RenderPhasesOnly returns only the phase blocks (no header), for appending.
func RenderPhasesOnly(phases []Phase) string {
	return renderPhaseBlocks(phases)
}

// RenderManifest returns a complete BUILD_MANIFEST.yaml string.
func RenderManifest(phases []Phase, title, tddPath string) string {
	header := fmt.Sprintf("title: %s\nsource: %s\nplanning_scope: full-product\nphases:\n", title, tddPath)
	return header + renderPhaseBlocks(phases) + "\n"
}

// ---------------------------------------------------------------------------
// Gate detection
// ---------------------------------------------------------------------------

var reGateFile = regexp.MustCompile(`^phase_(\d+)\.gate\.json$`)

// NextPhaseFromGates returns the lowest phase number not yet passed.
func NextPhaseFromGates(gatesDir string) int {
	entries, err := os.ReadDir(gatesDir)
	if err != nil {
		return 0
	}
	passed := make(map[int]bool)
	for _, e := range entries {
		m := reGateFile.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(gatesDir, e.Name()))
		if err != nil {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err != nil {
			continue
		}
		if obj["status"] == "passed" {
			n, err := strconv.Atoi(m[1])
			if err == nil {
				passed[n] = true
			}
		}
	}
	n := 0
	for passed[n] {
		n++
	}
	return n
}

// ---------------------------------------------------------------------------
// Run — main entry point
// ---------------------------------------------------------------------------

// Run is the main entry point. Returns exit code 0/1.
func Run(opts Options) int {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	tddPath := opts.TDDPath
	if tddPath == "" {
		tddPath = "docs/TDD.md"
	}

	data, err := os.ReadFile(tddPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: TDD file not found: %s\n", tddPath)
		return 1
	}
	tddText := string(data)

	// Parse phase policy (defaults if not found).
	policy, err := ParsePolicy(tddText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	deliverables, err := ExtractDeliverables(tddText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if deliverables == nil {
		fmt.Fprintf(os.Stderr, "error: No '## Deliverables' fenced YAML block found in TDD.\n\n"+
			"Add this section to your TDD:\n\n"+
			"## Deliverables\n\n"+
			"```yaml\n"+
			"- id: my-component\n"+
			"  type: logic\n"+
			"  description: What this delivers\n"+
			"  files:\n"+
			"    - path/to/file.go\n"+
			"  depends_on: []\n"+
			"  verified_by:\n"+
			"    - go test ./...\n"+
			"```\n\n"+
			"Valid types: schema, migration, contract, auth, security-sensitive,\n"+
			"  cross-cutting-refactor, public-api-change, generated-code,\n"+
			"  api, backend, integration, logic, frontend, ui, infra, tests, docs\n"+
			"Isolation types (always solo phases): schema, migration, contract, auth,\n"+
			"  security-sensitive, cross-cutting-refactor, public-api-change, generated-code\n")
		return 1
	}
	if len(deliverables) == 0 {
		fmt.Fprintf(os.Stderr, "error: Deliverables list is empty.\n")
		return 1
	}

	// Validate before planning.
	if err := ValidateDeliverables(deliverables); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	adj, inDegree, err := BuildDAG(deliverables)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	layers, err := TopologicalLayers(deliverables, adj, inDegree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Determine starting phase number.
	start := opts.StartPhase
	if opts.ExistingGates != "" {
		start = NextPhaseFromGates(opts.ExistingGates)
	}

	dmap := make(map[string]*Deliverable, len(deliverables))
	for i := range deliverables {
		dmap[deliverables[i].ID] = &deliverables[i]
	}

	phases := GroupIntoPhases(layers, adj, dmap, start, policy)

	fmt.Fprint(opts.Out, FormatPlan(phases, tddPath, policy))

	if !opts.Confirm {
		fmt.Fprintln(opts.Out, "Dry run. Run with --confirm to write BUILD_MANIFEST.yaml.")
		return 0
	}

	var outPath string
	if opts.FeatureSlug != "" {
		outPath = filepath.Join("docs", "features", opts.FeatureSlug, "BUILD_MANIFEST.yaml")
	} else {
		outPath = "BUILD_MANIFEST.yaml"
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	existing, readErr := os.ReadFile(outPath)
	if readErr == nil {
		block := RenderPhasesOnly(phases)
		merged := strings.TrimRight(string(existing), "\n") + "\n" + block + "\n"
		if err := os.WriteFile(outPath, []byte(merged), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(opts.Out, "Appended %d phases to existing %s.\n", len(phases), outPath)
	} else {
		title := opts.Title
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(tddPath), filepath.Ext(tddPath))
		}
		manifest := RenderManifest(phases, title, tddPath)
		if err := os.WriteFile(outPath, []byte(manifest), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(opts.Out, "Wrote %s (%d phases).\n", outPath, len(phases))
	}
	return 0
}
