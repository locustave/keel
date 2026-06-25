// Package verify ports harness/scripts/verify_repo.py to Go.
package verify

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var rootRequired = []string{
	"agent-rules.md",
	"keel/README.md",
	"BUILD_MANIFEST.yaml",
	"docs/PRD.md",
	"docs/phases/phase_0.md",
	"scripts/verify.sh",
	"scripts/verify_phase0.sh",
	"scripts/current_phase.sh",
	"scripts/merge_feature.py",
}

var ruleFiles = []string{
	"pre-flight.md",
	"phase-state.md",
	"allowed-paths.md",
	"gates.md",
	"retry-rollback.md",
	"audit-ledger.md",
	"drift-checkpoints.md",
	"stale-plan.md",
}

var commandFiles = []string{
	"preflight.md",
	"keel-run.md",
	"verify.md",
	"new-feature.md",
	"approve-feature.md",
	"rollback-phase.md",
}

var keelAssets = []string{
	"keel/hooks/preflight.sh",
	"keel/scripts/preflight_context.py",
	"keel/scripts/verify_repo.py",
	"keel/scripts/verify_phase0.py",
	"keel/scripts/validate_workflows.py",
	"keel/scripts/current_phase.py",
	"keel/scripts/merge_feature.py",
	"keel/skills.md",
}

// phaseRe matches "- phase: N" at the start of a line with no leading spaces.
var phaseRe = regexp.MustCompile(`(?m)^- phase: (\d+)$`)

// allowedBlockRe extracts backtick-quoted items from a named section block.
func extractListBlock(text, startSection, endSection string) map[string]bool {
	pattern := regexp.MustCompile(`(?s)## ` + regexp.QuoteMeta(startSection) + `\n\n(.*?)\n\n## ` + regexp.QuoteMeta(endSection))
	m := pattern.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	items := regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(m[1], -1)
	result := make(map[string]bool)
	for _, item := range items {
		result[item[1]] = true
	}
	return result
}

func manifestPhases(text string) []int {
	matches := phaseRe.FindAllStringSubmatch(text, -1)
	var phases []int
	for _, m := range matches {
		n, _ := strconv.Atoi(m[1])
		phases = append(phases, n)
	}
	return phases
}

func gateForPhase(root string, phase int) map[string]interface{} {
	path := filepath.Join(root, ".agent", "phase_gates", fmt.Sprintf("phase_%d.gate.json", phase))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var gate map[string]interface{}
	if err := json.Unmarshal(data, &gate); err != nil {
		return nil
	}
	return gate
}

// Run verifies the harness at repoPath. If phase >= 0, also verifies that phase's gate.
// Returns nil on success, error on any failure.
func Run(repoPath string, phase int) error {
	root, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("cannot resolve repo path: %w", err)
	}

	var failures []string

	// Required root files.
	for _, item := range rootRequired {
		if _, err := os.Stat(filepath.Join(root, item)); err != nil {
			failures = append(failures, "missing required path: "+item)
		}
	}

	// Harness rule files.
	for _, item := range ruleFiles {
		if _, err := os.Stat(filepath.Join(root, "keel", "rules", item)); err != nil {
			failures = append(failures, "missing keel rule: keel/rules/"+item)
		}
	}

	// Harness command files.
	for _, item := range commandFiles {
		if _, err := os.Stat(filepath.Join(root, "keel", "commands", item)); err != nil {
			failures = append(failures, "missing keel command: keel/commands/"+item)
		}
	}

	// Harness asset files.
	for _, item := range keelAssets {
		if _, err := os.Stat(filepath.Join(root, item)); err != nil {
			failures = append(failures, "missing keel asset: "+item)
		}
	}

	// Manifest phases contiguity.
	manifestPath := filepath.Join(root, "BUILD_MANIFEST.yaml")
	manifestText, _ := os.ReadFile(manifestPath)
	phases := manifestPhases(string(manifestText))
	if len(phases) == 0 {
		failures = append(failures, "manifest has no phases")
	} else {
		maxPhase := 0
		for _, p := range phases {
			if p > maxPhase {
				maxPhase = p
			}
		}
		expected := make([]int, maxPhase+1)
		for i := range expected {
			expected[i] = i
		}
		sort.Ints(phases)
		match := len(phases) == len(expected)
		if match {
			for i, p := range phases {
				if p != expected[i] {
					match = false
					break
				}
			}
		}
		if !match {
			failures = append(failures, fmt.Sprintf("manifest phases are not contiguous from 0: %v", phases))
		}
	}

	// Per-phase file checks.
	phaseDir := filepath.Join(root, "docs", "phases")
	// Accept both old format (## Phase Summary) and new format (## Phase Goal).
	titleMarkers := []string{"## Phase Summary", "## Phase Goal"}
	requiredMarkers := []string{
		"## Allowed Paths",
		"## Blocked Paths",
		"## Manifest Tasks",
		"## Tasks",
		"## Verification Commands",
		"## Exit Criteria",
		"## Out of Scope",
	}

	for _, p := range phases {
		tddPath := filepath.Join(phaseDir, fmt.Sprintf("phase_%d.md", p))
		data, err := os.ReadFile(tddPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("missing phase file: docs/phases/phase_%d.md", p))
			continue
		}
		text := string(data)

		// Check title marker (either format accepted).
		hasTitle := false
		for _, m := range titleMarkers {
			if strings.Contains(text, m) {
				hasTitle = true
				break
			}
		}
		if !hasTitle {
			failures = append(failures, fmt.Sprintf("phase %d missing marker ## Phase Summary or ## Phase Goal", p))
		}

		// Check remaining required markers.
		for _, marker := range requiredMarkers {
			if !strings.Contains(text, marker) {
				failures = append(failures, fmt.Sprintf("phase %d missing marker %s", p, marker))
			}
		}

		// Check allowed/blocked path overlap.
		allowed := extractListBlock(text, "Allowed Paths", "Blocked Paths")
		blocked := extractListBlock(text, "Blocked Paths", "Manifest Tasks")
		var overlap []string
		for k := range allowed {
			if blocked[k] {
				overlap = append(overlap, k)
			}
		}
		if len(overlap) > 0 {
			sort.Strings(overlap)
			failures = append(failures, fmt.Sprintf("phase %d allowed/blocked path overlap: %v", p, overlap))
		}
	}

	// Stale reference scan.
	scanPaths := []string{
		filepath.Join(root, "agent-rules.md"),
		filepath.Join(root, "keel"),
		filepath.Join(root, "docs", "phases"),
		filepath.Join(root, "BUILD_MANIFEST.yaml"),
	}
	for _, scanPath := range scanPaths {
		info, err := os.Stat(scanPath)
		if err != nil {
			continue
		}
		var files []string
		if info.IsDir() {
			_ = filepath.WalkDir(scanPath, func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					files = append(files, path)
				}
				return nil
			})
		} else {
			files = []string{scanPath}
		}
		for _, filePath := range files {
			base := filepath.Base(filePath)
			if strings.Contains(filePath, "__pycache__") || strings.HasSuffix(filePath, ".pyc") {
				continue
			}
			if base == "verify_repo.py" {
				continue
			}
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			text := string(data)
			rel, _ := filepath.Rel(root, filePath)
			if strings.Contains(text, "Agent Inbox") {
				failures = append(failures, "stale Agent Inbox reference: "+rel)
			}
			if strings.Contains(text, "HARNESS.md") && base != "README.md" {
				failures = append(failures, "stale HARNESS.md reference: "+rel)
			}
			if strings.Contains(text, "phase_prompts") && base != "README.md" {
				failures = append(failures, "stale phase_prompts reference: "+rel)
			}
			if strings.Contains(text, "harness/") && base != "README.md" {
				failures = append(failures, "stale harness/ reference: "+rel)
			}
		}
	}

	// Passed gates must have build ledger and audit log.
	ledgerDir := filepath.Join(root, "docs", "build-ledger")
	auditDir := filepath.Join(root, "docs", "audit")
	for _, p := range phases {
		gate := gateForPhase(root, p)
		if gate == nil {
			continue
		}
		status, _ := gate["status"].(string)
		if status != "passed" {
			continue
		}
		ledgerPath := filepath.Join(ledgerDir, fmt.Sprintf("phase_%d_build.md", p))
		if _, err := os.Stat(ledgerPath); err != nil {
			failures = append(failures, fmt.Sprintf("phase %d gate is passed but build ledger is missing: docs/build-ledger/phase_%d_build.md", p, p))
		}
		auditPath := filepath.Join(auditDir, fmt.Sprintf("phase_%d.log", p))
		if _, err := os.Stat(auditPath); err != nil {
			failures = append(failures, fmt.Sprintf("phase %d gate is passed but audit log is missing: docs/audit/phase_%d.log", p, p))
		}
		ecr, hasECR := gate["exit_criteria_results"]
		if !hasECR || ecr == nil {
			failures = append(failures, fmt.Sprintf("phase %d passed gate is missing exit_criteria_results", p))
		} else if ecrList, ok := ecr.([]interface{}); ok {
			for _, item := range ecrList {
				if m, ok := item.(map[string]interface{}); ok {
					if passed, _ := m["passed"].(bool); !passed {
						failures = append(failures, fmt.Sprintf("phase %d passed gate has failing exit criteria in exit_criteria_results", p))
						break
					}
				}
			}
		}
	}

	// Optional single-phase gate check.
	if phase >= 0 {
		gate := gateForPhase(root, phase)
		if gate == nil {
			failures = append(failures, fmt.Sprintf("missing passed gate for phase %d", phase))
		} else if status, _ := gate["status"].(string); status != "passed" {
			failures = append(failures, fmt.Sprintf("phase %d gate is not passed", phase))
		}
	}

	if len(failures) > 0 {
		msg := "Repository verification failed:\n"
		for _, f := range failures {
			msg += "- " + f + "\n"
		}
		return fmt.Errorf("%s", strings.TrimRight(msg, "\n"))
	}

	return nil
}
