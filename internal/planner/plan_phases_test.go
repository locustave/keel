package planner_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keel/internal/planner"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func defaultPolicy() planner.PhasePolicy { return planner.DefaultPolicy() }

func dmap(ds []planner.Deliverable) map[string]*planner.Deliverable {
	m := make(map[string]*planner.Deliverable, len(ds))
	for i := range ds {
		m[ds[i].ID] = &ds[i]
	}
	return m
}

func buildAndLayer(t *testing.T, ds []planner.Deliverable) (map[string][]string, [][]string) {
	t.Helper()
	adj, inDeg, err := planner.BuildDAG(ds)
	if err != nil {
		t.Fatalf("BuildDAG: %v", err)
	}
	layers, err := planner.TopologicalLayers(ds, adj, inDeg)
	if err != nil {
		t.Fatalf("TopologicalLayers: %v", err)
	}
	return adj, layers
}

func mustGroupPhases(t *testing.T, ds []planner.Deliverable) []planner.Phase {
	t.Helper()
	adj, layers := buildAndLayer(t, ds)
	return planner.GroupIntoPhases(layers, adj, dmap(ds), 0, defaultPolicy())
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func TestValidate_duplicateID(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "a", Type: "logic", Files: []string{"a.go"}, VerifiedBy: []string{"go test"}},
		{ID: "a", Type: "logic", Files: []string{"b.go"}, VerifiedBy: []string{"go test"}},
	}
	if err := planner.ValidateDeliverables(ds); err == nil {
		t.Fatal("expected error for duplicate id, got nil")
	}
}

func TestValidate_missingType(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "a", Files: []string{"a.go"}, VerifiedBy: []string{"go test"}},
	}
	if err := planner.ValidateDeliverables(ds); err == nil || !strings.Contains(err.Error(), "type") {
		t.Fatalf("expected error mentioning 'type', got: %v", err)
	}
}

func TestValidate_missingFiles(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "a", Type: "logic", VerifiedBy: []string{"go test"}},
	}
	if err := planner.ValidateDeliverables(ds); err == nil || !strings.Contains(err.Error(), "files") {
		t.Fatalf("expected error mentioning 'files', got: %v", err)
	}
}

func TestValidate_missingVerifiedBy(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "a", Type: "logic", Files: []string{"a.go"}},
	}
	if err := planner.ValidateDeliverables(ds); err == nil || !strings.Contains(err.Error(), "verified_by") {
		t.Fatalf("expected error mentioning 'verified_by', got: %v", err)
	}
}

func TestValidate_unknownDependency(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "a", Type: "logic", Files: []string{"a.go"}, VerifiedBy: []string{"t"}, DependsOn: []string{"nonexistent"}},
	}
	if err := planner.ValidateDeliverables(ds); err == nil || !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("expected error mentioning unknown dep, got: %v", err)
	}
}

func TestValidate_valid(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "a", Type: "logic", Files: []string{"a.go"}, VerifiedBy: []string{"t"}, DependsOn: []string{}},
		{ID: "b", Type: "api", Files: []string{"b.go"}, VerifiedBy: []string{"t"}, DependsOn: []string{"a"}},
	}
	if err := planner.ValidateDeliverables(ds); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ExtractDeliverables
// ---------------------------------------------------------------------------

func TestExtractDeliverables_missing(t *testing.T) {
	ds, err := planner.ExtractDeliverables("# No deliverables here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds != nil {
		t.Fatalf("expected nil, got %v", ds)
	}
}

func TestExtractDeliverables_valid(t *testing.T) {
	tdd := "## Deliverables\n\n" + "```yaml\n" +
		"- id: alpha\n  type: logic\n  description: Alpha\n  files:\n    - alpha.go\n  depends_on: []\n  verified_by:\n    - go test\n" +
		"```"
	ds, err := planner.ExtractDeliverables(tdd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ds) != 1 || ds[0].ID != "alpha" {
		t.Errorf("unexpected deliverables: %+v", ds)
	}
}

func TestExtractDeliverables_touchesField(t *testing.T) {
	tdd := "## Deliverables\n\n" + "```yaml\n" +
		"- id: gen\n  type: generated-code\n  description: Generated\n  touches:\n    - gen/api.go\n    - gen/types.go\n  depends_on: []\n  verified_by:\n    - go build ./gen/...\n" +
		"```"
	ds, err := planner.ExtractDeliverables(tdd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("expected 1 deliverable, got %d", len(ds))
	}
	if len(ds[0].Touches) != 2 {
		t.Errorf("expected 2 touches, got %v", ds[0].Touches)
	}
	if len(ds[0].Files) != 0 {
		t.Errorf("expected no files, got %v", ds[0].Files)
	}
	// AllFiles should return touches
	if files := ds[0].AllFiles(); len(files) != 2 || files[0] != "gen/api.go" {
		t.Errorf("AllFiles(): expected touches, got %v", files)
	}
}

func TestExtractDeliverables_filesCompatibility(t *testing.T) {
	// Old-style deliverable with only `files` (no touches) still works.
	tdd := "## Deliverables\n\n" + "```yaml\n" +
		"- id: old\n  type: logic\n  files:\n    - old.go\n  depends_on: []\n  verified_by:\n    - go test\n" +
		"```"
	ds, err := planner.ExtractDeliverables(tdd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ds) != 1 || ds[0].AllFiles()[0] != "old.go" {
		t.Errorf("unexpected: %+v", ds)
	}
}

// ---------------------------------------------------------------------------
// Cycle detection
// ---------------------------------------------------------------------------

func TestTopologicalLayers_cycle(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}
	adj, inDeg, err := planner.BuildDAG(ds)
	if err != nil {
		t.Fatalf("BuildDAG: %v", err)
	}
	_, err = planner.TopologicalLayers(ds, adj, inDeg)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle: %v", err)
	}
	if !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Errorf("error should name cycle nodes: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Flat project (no dependencies)
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_flatNoDepends(t *testing.T) {
	// Three independent deliverables with no deps — all in layer 0.
	// Combined files < 15, criteria < 6, risk < 8 → should all pack into one phase.
	ds := []planner.Deliverable{
		{ID: "a", Type: "logic", Files: []string{"a.go"}, VerifiedBy: []string{"go test ./a/..."}},
		{ID: "b", Type: "logic", Files: []string{"b.go"}, VerifiedBy: []string{"go test ./b/..."}},
		{ID: "c", Type: "logic", Files: []string{"c.go"}, VerifiedBy: []string{"go test ./c/..."}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 1 {
		t.Errorf("expected 1 phase for flat project, got %d: %v",
			len(phases), phaseIDs(phases))
	}
	if len(phases[0].DeliverableIDs) != 3 {
		t.Errorf("expected all 3 deliverables in phase 0, got %v", phases[0].DeliverableIDs)
	}
}

// ---------------------------------------------------------------------------
// Deep dependency chain
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_deepChain(t *testing.T) {
	// a → b → c → d — each must be in its own phase.
	ds := []planner.Deliverable{
		{ID: "a", Type: "logic", Files: []string{"a.go"}, VerifiedBy: []string{"t"}, DependsOn: []string{}},
		{ID: "b", Type: "logic", Files: []string{"b.go"}, VerifiedBy: []string{"t"}, DependsOn: []string{"a"}},
		{ID: "c", Type: "logic", Files: []string{"c.go"}, VerifiedBy: []string{"t"}, DependsOn: []string{"b"}},
		{ID: "d", Type: "logic", Files: []string{"d.go"}, VerifiedBy: []string{"t"}, DependsOn: []string{"c"}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 4 {
		t.Errorf("expected 4 phases for a chain of 4, got %d", len(phases))
	}
	// Verify order: a, b, c, d
	expected := []string{"a", "b", "c", "d"}
	for i, p := range phases {
		if len(p.DeliverableIDs) != 1 || p.DeliverableIDs[0] != expected[i] {
			t.Errorf("phase %d: expected [%s], got %v", i, expected[i], p.DeliverableIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// Schema isolation
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_schemaIsolated(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "s", Type: "schema", Files: []string{"schema.sql"}, VerifiedBy: []string{"test schema"}},
		{ID: "l", Type: "logic", Files: []string{"logic.go"}, VerifiedBy: []string{"go test"}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases (schema isolated), got %d", len(phases))
	}
	if phases[0].IsolationReason != "schema" {
		t.Errorf("expected schema isolation reason, got %q", phases[0].IsolationReason)
	}
	if phases[0].DeliverableIDs[0] != "s" {
		t.Errorf("expected schema first, got %v", phases[0].DeliverableIDs)
	}
}

// ---------------------------------------------------------------------------
// Auth isolation
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_authIsolated(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "mw", Type: "auth", Files: []string{"middleware.go"}, VerifiedBy: []string{"go test ./auth/..."}},
		{ID: "api", Type: "logic", Files: []string{"api.go"}, VerifiedBy: []string{"go test ./api/..."}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases (auth isolated), got %d", len(phases))
	}
	if phases[0].IsolationReason != "auth" {
		t.Errorf("expected auth isolation reason, got %q", phases[0].IsolationReason)
	}
}

// ---------------------------------------------------------------------------
// Contract isolation
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_contractIsolated(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "svc", Type: "contract", Files: []string{"openapi.yaml"}, VerifiedBy: []string{"validate-schema"}},
		{ID: "impl", Type: "logic", Files: []string{"impl.go"}, VerifiedBy: []string{"go test"}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 2 {
		t.Fatalf("expected 2 phases (contract isolated), got %d", len(phases))
	}
	if phases[0].IsolationReason != "contract" {
		t.Errorf("expected contract isolation, got %q", phases[0].IsolationReason)
	}
}

// ---------------------------------------------------------------------------
// File ceiling splitting
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_fileCeilingSplit(t *testing.T) {
	// Two deliverables each with 10 files — together they exceed MAX_FILES=15.
	files10 := make([]string, 10)
	for i := range files10 {
		files10[i] = "file.go"
	}
	ds := []planner.Deliverable{
		{ID: "big1", Type: "logic", Files: files10, VerifiedBy: []string{"t1"}},
		{ID: "big2", Type: "logic", Files: files10, VerifiedBy: []string{"t2"}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 2 {
		t.Errorf("expected 2 phases due to file ceiling, got %d", len(phases))
	}
}

// ---------------------------------------------------------------------------
// Criteria ceiling splitting
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_criteriaCeilingSplit(t *testing.T) {
	// Two deliverables each with 4 criteria — together exceed MAX_CRITERIA=6.
	crit4 := []string{"t1", "t2", "t3", "t4"}
	ds := []planner.Deliverable{
		{ID: "c1", Type: "logic", Files: []string{"c1.go"}, VerifiedBy: crit4},
		{ID: "c2", Type: "logic", Files: []string{"c2.go"}, VerifiedBy: crit4},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 2 {
		t.Errorf("expected 2 phases due to criteria ceiling, got %d", len(phases))
	}
}

// ---------------------------------------------------------------------------
// Risk budget splitting
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_riskBudgetSplit(t *testing.T) {
	// policy max_risk=8; schema(5) + logic(2) = 7 ≤ 8 → packed
	// but schema is isolated anyway; test with logic(2)+logic(2)+logic(2)+logic(2)+logic(2) = 10 > 8
	ds := []planner.Deliverable{
		{ID: "a", Type: "logic", Files: []string{"a.go"}, VerifiedBy: []string{"t"}},
		{ID: "b", Type: "logic", Files: []string{"b.go"}, VerifiedBy: []string{"t"}},
		{ID: "c", Type: "logic", Files: []string{"c.go"}, VerifiedBy: []string{"t"}},
		{ID: "d", Type: "logic", Files: []string{"d.go"}, VerifiedBy: []string{"t"}},
		{ID: "e", Type: "logic", Files: []string{"e.go"}, VerifiedBy: []string{"t"}},
	}
	// Each logic deliverable has risk=2. After 4 packed (risk=8), the 5th overflows.
	phases := mustGroupPhases(t, ds)
	if len(phases) < 2 {
		t.Errorf("expected at least 2 phases due to risk budget (5×logic risk=2), got %d", len(phases))
	}
	// First phase should have exactly 4 deliverables (4×2=8 = max_risk)
	if len(phases[0].DeliverableIDs) != 4 {
		t.Errorf("expected 4 deliverables in first phase (risk budget 8), got %d: %v",
			len(phases[0].DeliverableIDs), phases[0].DeliverableIDs)
	}
	if len(phases[1].DeliverableIDs) != 1 {
		t.Errorf("expected 1 deliverable in second phase, got %d", len(phases[1].DeliverableIDs))
	}
}

// ---------------------------------------------------------------------------
// Deterministic sort order
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_deterministicOrder(t *testing.T) {
	// Two independent deliverables with same type and no deps.
	// ID sort (ascending) must be the tiebreaker → "alpha" before "zulu".
	ds := []planner.Deliverable{
		{ID: "zulu", Type: "logic", Files: []string{"z.go"}, VerifiedBy: []string{"t"}},
		{ID: "alpha", Type: "logic", Files: []string{"a.go"}, VerifiedBy: []string{"t"}},
	}
	// Risk: 2+2=4 ≤ 8, files: 2 ≤ 15, criteria: 2 ≤ 6 → packed into one phase.
	phases := mustGroupPhases(t, ds)
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	if phases[0].DeliverableIDs[0] != "alpha" {
		t.Errorf("expected alpha first (id sort), got %v", phases[0].DeliverableIDs)
	}
}

func TestGroupIntoPhases_riskScorePriorityBeforeID(t *testing.T) {
	// "zapi" has type=api (risk=2), "aaaa" has type=logic (risk=2) — same risk, id wins.
	// But if we use different types: "zulu"=integration(risk=2) vs "alpha"=integration(risk=2) → id sort.
	// To test risk priority: one is api(2), one is infra(1) — api goes first (higher risk).
	ds := []planner.Deliverable{
		{ID: "z-infra", Type: "infra", Files: []string{"z.go"}, VerifiedBy: []string{"t"}},
		{ID: "a-api", Type: "api", Files: []string{"a.go"}, VerifiedBy: []string{"t"}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	// a-api (risk=2) should come before z-infra (risk=1) despite z coming after a alphabetically.
	if phases[0].DeliverableIDs[0] != "a-api" {
		t.Errorf("expected a-api first (higher risk score), got %v", phases[0].DeliverableIDs)
	}
}

// ---------------------------------------------------------------------------
// Richer phase manifest fields
// ---------------------------------------------------------------------------

func TestPhase_richerFields(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "db", Type: "schema", Files: []string{"schema.sql", "migrate.go"}, VerifiedBy: []string{"go test ./db/..."}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	p := phases[0]
	if p.ID == "" {
		t.Error("expected non-empty ID field")
	}
	if !strings.HasPrefix(p.ID, "phase-") {
		t.Errorf("expected ID to start with 'phase-', got %q", p.ID)
	}
	if p.DependencyLayer != 0 {
		t.Errorf("expected dependency_layer=0, got %d", p.DependencyLayer)
	}
	if p.IsolationReason != "schema" {
		t.Errorf("expected isolation_reason=schema, got %q", p.IsolationReason)
	}
	if p.RiskScore != 5 {
		t.Errorf("expected risk_score=5 for schema, got %d", p.RiskScore)
	}
	if !p.RollbackCheckpointRequired {
		t.Error("expected rollback_checkpoint_required=true")
	}
	if len(p.AllowedFiles) != 2 {
		t.Errorf("expected 2 allowed_files, got %v", p.AllowedFiles)
	}
}

// ---------------------------------------------------------------------------
// Phase policy parsing
// ---------------------------------------------------------------------------

func TestParsePolicy_default(t *testing.T) {
	p, err := planner.ParsePolicy("# No policy here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.MaxFilesPerPhase != 15 {
		t.Errorf("expected default max_files=15, got %d", p.MaxFilesPerPhase)
	}
	if !p.IsolationTypes["schema"] {
		t.Error("expected schema to be an isolation type")
	}
}

func TestParsePolicy_custom(t *testing.T) {
	tdd := "# TDD\n\n## Phase Policy\n\n```yaml\nphase_policy:\n  max_files_per_phase: 5\n  max_criteria_per_phase: 3\n  max_risk_score_per_phase: 6\n  isolation_types:\n    - schema\n    - auth\n  risk_scores:\n    schema: 5\n    auth: 4\n    logic: 1\n```\n"
	p, err := planner.ParsePolicy(tdd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.MaxFilesPerPhase != 5 {
		t.Errorf("expected max_files=5, got %d", p.MaxFilesPerPhase)
	}
	if p.MaxCriteriaPerPhase != 3 {
		t.Errorf("expected max_criteria=3, got %d", p.MaxCriteriaPerPhase)
	}
	if !p.IsolationTypes["auth"] {
		t.Error("expected auth to be isolation type after custom policy")
	}
	if p.RiskScores["logic"] != 1 {
		t.Errorf("expected logic risk=1, got %d", p.RiskScores["logic"])
	}
}

// ---------------------------------------------------------------------------
// Touches field (newer deliverables)
// ---------------------------------------------------------------------------

func TestGroupIntoPhases_touchesField(t *testing.T) {
	// Deliverable uses `touches` instead of `files`.
	ds := []planner.Deliverable{
		{ID: "gen", Type: "generated-code", Touches: []string{"gen/api.go", "gen/types.go"}, VerifiedBy: []string{"go build ./gen/..."}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	if len(phases[0].AllowedFiles) != 2 {
		t.Errorf("expected 2 allowed_files from touches, got %v", phases[0].AllowedFiles)
	}
}

func TestGroupIntoPhases_touchesPrefersOverFiles(t *testing.T) {
	// When both files and touches are set, touches takes priority.
	ds := []planner.Deliverable{
		{ID: "x", Type: "logic",
			Files:   []string{"ignored.go"},
			Touches: []string{"real.go"},
			VerifiedBy: []string{"go test"}},
	}
	phases := mustGroupPhases(t, ds)
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}
	if phases[0].AllowedFiles[0] != "real.go" {
		t.Errorf("expected touches to take priority, got allowed_files=%v", phases[0].AllowedFiles)
	}
}

// ---------------------------------------------------------------------------
// BuildDAG unknown dependency error
// ---------------------------------------------------------------------------

func TestBuildDAG_unknownDep(t *testing.T) {
	ds := []planner.Deliverable{
		{ID: "a", DependsOn: []string{"ghost"}},
	}
	_, _, err := planner.BuildDAG(ds)
	if err == nil {
		t.Fatal("expected error for unknown dep in BuildDAG")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error should name missing dep: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Full dry-run integration
// ---------------------------------------------------------------------------

func TestRun_missingTDD(t *testing.T) {
	code := planner.Run(planner.Options{
		TDDPath: "/tmp/nonexistent-tdd-file-12345.md",
		Out:     &bytes.Buffer{},
	})
	if code != 1 {
		t.Errorf("expected exit 1 for missing TDD, got %d", code)
	}
}

func TestRun_noDeliverablesSection(t *testing.T) {
	f, _ := os.CreateTemp("", "tdd-*.md")
	defer os.Remove(f.Name())
	f.WriteString("# TDD\n\nNo deliverables here.\n")
	f.Close()

	if code := planner.Run(planner.Options{TDDPath: f.Name(), Out: &bytes.Buffer{}}); code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
}

func TestRun_dryRun(t *testing.T) {
	tdd := "# TDD\n\n## Deliverables\n\n```yaml\n" +
		"- id: scaffold\n  type: logic\n  description: Go scaffold\n  files:\n    - go.mod\n    - main.go\n  depends_on: []\n  verified_by:\n    - go build ./...\n" +
		"- id: server\n  type: logic\n  description: HTTP server\n  files:\n    - server.go\n  depends_on: [scaffold]\n  verified_by:\n    - go test ./...\n" +
		"```"

	f, _ := os.CreateTemp("", "tdd-*.md")
	defer os.Remove(f.Name())
	f.WriteString(tdd)
	f.Close()

	var out bytes.Buffer
	code := planner.Run(planner.Options{TDDPath: f.Name(), Out: &out})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "Total phases: 2") {
		t.Errorf("expected 'Total phases: 2' in output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Dry run") {
		t.Errorf("expected 'Dry run' in output:\n%s", out.String())
	}
}

func TestRun_confirm(t *testing.T) {
	tdd := "# TDD\n\n## Deliverables\n\n```yaml\n" +
		"- id: only\n  type: logic\n  description: The only thing\n  files:\n    - only.go\n  depends_on: []\n  verified_by:\n    - go test ./...\n" +
		"```"

	dir := t.TempDir()
	tddPath := filepath.Join(dir, "TDD.md")
	os.WriteFile(tddPath, []byte(tdd), 0o644)

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	var out bytes.Buffer
	code := planner.Run(planner.Options{TDDPath: tddPath, Confirm: true, Title: "Test Project", Out: &out})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput: %s", code, out.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "BUILD_MANIFEST.yaml"))
	if err != nil {
		t.Fatalf("BUILD_MANIFEST.yaml not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "- phase: 0") {
		t.Errorf("expected '- phase: 0' in manifest:\n%s", content)
	}
	// New fields should be present
	if !strings.Contains(content, "risk_score:") {
		t.Errorf("expected 'risk_score:' in manifest:\n%s", content)
	}
	if !strings.Contains(content, "allowed_files:") {
		t.Errorf("expected 'allowed_files:' in manifest:\n%s", content)
	}
	if !strings.Contains(content, "rollback_checkpoint_required: true") {
		t.Errorf("expected 'rollback_checkpoint_required: true' in manifest:\n%s", content)
	}
}

func TestRun_validationError(t *testing.T) {
	// Missing type field should fail at validation.
	tdd := "# TDD\n\n## Deliverables\n\n```yaml\n" +
		"- id: bad\n  files:\n    - bad.go\n  depends_on: []\n  verified_by:\n    - go test\n" +
		"```"

	f, _ := os.CreateTemp("", "tdd-*.md")
	defer os.Remove(f.Name())
	f.WriteString(tdd)
	f.Close()

	if code := planner.Run(planner.Options{TDDPath: f.Name(), Out: &bytes.Buffer{}}); code != 1 {
		t.Errorf("expected exit 1 for validation error, got %d", code)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func phaseIDs(phases []planner.Phase) []string {
	ids := make([]string, len(phases))
	for i, p := range phases {
		ids[i] = p.ID
	}
	return ids
}
