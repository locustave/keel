package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClosePassedPhase(t *testing.T) {
	repo := t.TempDir()
	setupRepo(t, repo)

	result, err := Close(CloseInput{
		Phase:    1,
		RepoPath: repo,
		Name:     "Core API",
		Agent:    "claude",
		Model:    "opus",
		Summary:  "Built the core API endpoints",
		Commands: []string{"go test ./...", "bash scripts/verify.sh"},
		FilesChanged: []string{"internal/api/handler.go", "internal/api/routes.go"},
		ExitCriteriaResults: []ExitCriterionResult{
			{Criterion: "all tests pass", Passed: true},
			{Criterion: "no lint errors", Passed: true},
		},
	})
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Verify gate file.
	gateData, err := os.ReadFile(filepath.Join(repo, result.GatePath))
	if err != nil {
		t.Fatalf("read gate: %v", err)
	}
	var gate GateRecord
	if err := json.Unmarshal(gateData, &gate); err != nil {
		t.Fatalf("unmarshal gate: %v", err)
	}
	if gate.Status != "passed" {
		t.Errorf("gate.Status = %q, want %q", gate.Status, "passed")
	}
	if gate.Phase != 1 {
		t.Errorf("gate.Phase = %d, want 1", gate.Phase)
	}
	if gate.Name != "Core API" {
		t.Errorf("gate.Name = %q, want %q", gate.Name, "Core API")
	}
	if gate.SelfVerification != "pass" {
		t.Errorf("gate.SelfVerification = %q, want %q", gate.SelfVerification, "pass")
	}
	if len(gate.ExitCriteriaResults) != 2 {
		t.Errorf("exit criteria count = %d, want 2", len(gate.ExitCriteriaResults))
	}
	if len(gate.FilesChanged) != 2 {
		t.Errorf("files changed count = %d, want 2", len(gate.FilesChanged))
	}

	// Verify build ledger.
	ledgerData, err := os.ReadFile(filepath.Join(repo, result.LedgerPath))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	ledger := string(ledgerData)
	if !strings.Contains(ledger, "# Phase 1 Build Ledger") {
		t.Error("ledger missing header")
	}
	if !strings.Contains(ledger, "passed") {
		t.Error("ledger missing status")
	}
	if !strings.Contains(ledger, "Core API") {
		t.Error("ledger missing phase name")
	}
	if !strings.Contains(ledger, "claude") {
		t.Error("ledger missing agent")
	}

	// Verify audit log.
	auditData, err := os.ReadFile(filepath.Join(repo, result.AuditLogPath))
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	auditLog := string(auditData)
	if !strings.Contains(auditLog, "Phase 1 Audit Log") {
		t.Error("audit log missing header")
	}
	if !strings.Contains(auditLog, "pass") {
		t.Error("audit log missing self-verification")
	}

	// Verify audit.jsonl.
	auditJSONL, err := os.ReadFile(filepath.Join(repo, ".agent", "audit.jsonl"))
	if err != nil {
		t.Fatalf("read audit.jsonl: %v", err)
	}
	if !strings.Contains(string(auditJSONL), "phase.passed") {
		t.Error("audit.jsonl missing phase.passed event")
	}

	// Verify run_log.jsonl.
	runLog, err := os.ReadFile(filepath.Join(repo, ".agent", "run_log.jsonl"))
	if err != nil {
		t.Fatalf("read run_log.jsonl: %v", err)
	}
	if !strings.Contains(string(runLog), "phase.closed") {
		t.Error("run_log.jsonl missing phase.closed event")
	}
}

func TestCloseFailedPhase(t *testing.T) {
	repo := t.TempDir()
	setupRepo(t, repo)

	result, err := Close(CloseInput{
		Phase:    2,
		RepoPath: repo,
		Name:     "Auth Layer",
		ExitCriteriaResults: []ExitCriterionResult{
			{Criterion: "all tests pass", Passed: true},
			{Criterion: "security audit", Passed: false, Detail: "missing CSRF protection"},
		},
	})
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Gate should be failed.
	if !strings.HasSuffix(result.GatePath, "failed.json") {
		t.Errorf("gate path = %q, want *.failed.json", result.GatePath)
	}

	gateData, err := os.ReadFile(filepath.Join(repo, result.GatePath))
	if err != nil {
		t.Fatalf("read gate: %v", err)
	}
	var gate GateRecord
	json.Unmarshal(gateData, &gate)
	if gate.Status != "failed" {
		t.Errorf("gate.Status = %q, want %q", gate.Status, "failed")
	}
	if gate.SelfVerification != "fail" {
		t.Errorf("gate.SelfVerification = %q, want %q", gate.SelfVerification, "fail")
	}
}

func TestCloseNoCriteria(t *testing.T) {
	repo := t.TempDir()
	setupRepo(t, repo)

	result, err := Close(CloseInput{
		Phase:    0,
		RepoPath: repo,
	})
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// With no criteria, status should be passed (vacuously true).
	gateData, _ := os.ReadFile(filepath.Join(repo, result.GatePath))
	var gate GateRecord
	json.Unmarshal(gateData, &gate)
	if gate.Status != "passed" {
		t.Errorf("gate.Status = %q, want %q", gate.Status, "passed")
	}
}

func TestReadPhaseName(t *testing.T) {
	repo := t.TempDir()
	manifest := `title: BUILD_MANIFEST.yaml
phases:
  - phase: 0
    name: Harness Bootstrap
  - phase: 1
    name: Core API
  - phase: 2
    name: Auth Layer
`
	os.WriteFile(filepath.Join(repo, "BUILD_MANIFEST.yaml"), []byte(manifest), 0o644)

	tests := []struct {
		phase int
		want  string
	}{
		{0, "Harness Bootstrap"},
		{1, "Core API"},
		{2, "Auth Layer"},
		{99, ""},
	}
	for _, tt := range tests {
		got := readPhaseName(repo, tt.phase)
		if got != tt.want {
			t.Errorf("readPhaseName(%d) = %q, want %q", tt.phase, got, tt.want)
		}
	}
}

// setupRepo creates the minimal directory structure needed for Close.
func setupRepo(t *testing.T, repo string) {
	t.Helper()
	for _, dir := range []string{
		".agent/phase_gates",
		".agent/snapshots",
		"docs/build-ledger",
		"docs/audit",
	} {
		os.MkdirAll(filepath.Join(repo, dir), 0o755)
	}
}
