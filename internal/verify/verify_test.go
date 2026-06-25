package verify_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keel/internal/verify"
)

// buildMinimalRepo creates the minimum structure needed to pass all checks.
func buildMinimalRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Required root files.
	required := []string{
		"agent-rules.md",
		"keel/README.md",
		"docs/PRD.md",
		"scripts/verify.sh",
		"scripts/verify_phase0.sh",
		"scripts/current_phase.sh",
		"scripts/merge_feature.py",
		"keel/hooks/preflight.sh",
		"keel/scripts/preflight_context.py",
		"keel/scripts/verify_repo.py",
		"keel/scripts/verify_phase0.py",
		"keel/scripts/validate_workflows.py",
		"keel/scripts/current_phase.py",
		"keel/scripts/merge_feature.py",
		"keel/skills.md",
	}
	for _, f := range required {
		writeFile(t, root, f, "# stub")
	}

	// Rule files.
	for _, r := range []string{
		"pre-flight.md", "phase-state.md", "allowed-paths.md", "gates.md",
		"retry-rollback.md", "audit-ledger.md", "drift-checkpoints.md", "stale-plan.md",
	} {
		writeFile(t, root, "keel/rules/"+r, "# stub")
	}

	// Command files.
	for _, c := range []string{
		"preflight.md", "keel-run.md", "verify.md", "new-feature.md", "approve-feature.md", "rollback-phase.md",
	} {
		writeFile(t, root, "keel/commands/"+c, "# stub")
	}

	// Minimal manifest with phases 0 and 1.
	writeFile(t, root, "BUILD_MANIFEST.yaml", "phases:\n- phase: 0\n- phase: 1\n")

	// Phase build files.
	for _, n := range []string{"0", "1"} {
		writeFile(t, root, "docs/phases/phase_"+n+".md", minimalPhaseTDD(n))
	}

	return root
}

func minimalPhaseTDD(n string) string {
	return "# Phase " + n + "\n\n" +
		"## Phase Summary\n\nGoal.\n\n" +
		"## Manifest Tasks\n\n- task\n\n" +
		"## Allowed Paths\n\n- `docs/audit/phase_" + n + ".log`\n\n" +
		"## Blocked Paths\n\n- `some/blocked`\n\n" +
		"## Tasks\n\nTasks.\n\n" +
		"## Verification Commands\n\n```\necho ok\n```\n\n" +
		"## Exit Criteria\n\n- passes\n\n" +
		"## Out of Scope\n\n- nothing\n"
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestPassesOnWellFormedRepo(t *testing.T) {
	root := buildMinimalRepo(t)
	if err := verify.Run(root, -1); err != nil {
		t.Fatalf("expected pass, got: %v", err)
	}
}

func TestFailsOnMissingRequiredFile(t *testing.T) {
	root := buildMinimalRepo(t)
	os.Remove(filepath.Join(root, "docs", "PRD.md"))
	err := verify.Run(root, -1)
	if err == nil {
		t.Fatal("expected failure for missing docs/PRD.md")
	}
	if !strings.Contains(err.Error(), "PRD.md") {
		t.Fatalf("expected error to mention PRD.md, got: %v", err)
	}
}

func TestFailsOnMissingRuleFile(t *testing.T) {
	root := buildMinimalRepo(t)
	os.Remove(filepath.Join(root, "keel", "rules", "gates.md"))
	err := verify.Run(root, -1)
	if err == nil {
		t.Fatal("expected failure for missing rule file")
	}
	if !strings.Contains(err.Error(), "gates.md") {
		t.Fatalf("expected error to mention gates.md, got: %v", err)
	}
}

func TestFailsOnNonContiguousPhases(t *testing.T) {
	root := buildMinimalRepo(t)
	// Write manifest with gap: phases 0 and 2 (missing 1).
	writeFile(t, root, "BUILD_MANIFEST.yaml", "phases:\n- phase: 0\n- phase: 2\n")
	writeFile(t, root, "docs/phases/phase_2.md", minimalPhaseTDD("2"))
	err := verify.Run(root, -1)
	if err == nil {
		t.Fatal("expected failure for non-contiguous phases")
	}
	if !strings.Contains(err.Error(), "contiguous") {
		t.Fatalf("expected error to mention contiguous, got: %v", err)
	}
}

func TestFailsOnMissingPhaseFile(t *testing.T) {
	root := buildMinimalRepo(t)
	os.Remove(filepath.Join(root, "docs", "phases", "phase_1.md"))
	err := verify.Run(root, -1)
	if err == nil {
		t.Fatal("expected failure for missing phase file")
	}
	if !strings.Contains(err.Error(), "phase_1.md") {
		t.Fatalf("expected error to mention phase_1.md, got: %v", err)
	}
}

func TestFailsOnMissingMarker(t *testing.T) {
	root := buildMinimalRepo(t)
	// Phase 0 missing Exit Criteria marker.
	content := minimalPhaseTDD("0")
	content = strings.ReplaceAll(content, "## Exit Criteria", "## Something Else")
	writeFile(t, root, "docs/phases/phase_0.md", content)
	err := verify.Run(root, -1)
	if err == nil {
		t.Fatal("expected failure for missing marker")
	}
	if !strings.Contains(err.Error(), "Exit Criteria") {
		t.Fatalf("expected error to mention Exit Criteria, got: %v", err)
	}
}

func TestAcceptsPhaseGoalMarker(t *testing.T) {
	root := buildMinimalRepo(t)
	// Replace ## Phase Summary with ## Phase Goal — should still pass.
	for _, n := range []string{"0", "1"} {
		content := strings.ReplaceAll(minimalPhaseTDD(n), "## Phase Summary", "## Phase Goal")
		writeFile(t, root, "docs/phases/phase_"+n+".md", content)
	}
	if err := verify.Run(root, -1); err != nil {
		t.Fatalf("expected pass with ## Phase Goal marker, got: %v", err)
	}
}

func TestFailsOnPassedGateWithMissingLedger(t *testing.T) {
	root := buildMinimalRepo(t)
	// Write a passed gate for phase 0 without ledger/audit files.
	gate := `{"phase":0,"name":"Test","status":"passed","exit_criteria_results":[{"criterion":"x","passed":true}]}`
	writeFile(t, root, ".agent/phase_gates/phase_0.gate.json", gate)
	err := verify.Run(root, -1)
	if err == nil {
		t.Fatal("expected failure for missing build ledger")
	}
	if !strings.Contains(err.Error(), "build ledger") {
		t.Fatalf("expected error to mention build ledger, got: %v", err)
	}
}

func TestPhaseGateCheck(t *testing.T) {
	root := buildMinimalRepo(t)
	// No gate file for phase 0 — --phase 0 should fail.
	err := verify.Run(root, 0)
	if err == nil {
		t.Fatal("expected failure for missing gate")
	}
	if !strings.Contains(err.Error(), "missing passed gate") {
		t.Fatalf("expected error to mention missing passed gate, got: %v", err)
	}
}
