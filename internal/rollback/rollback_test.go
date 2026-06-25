package rollback_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keel/internal/rollback"
)

func makeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".agent", "snapshots"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".agent", "phase_gates"), 0o755)
	return dir
}

func writeDAG(t *testing.T, dir string, dag rollback.RollbackDAG) {
	t.Helper()
	if err := rollback.WriteDAG(dir, dag); err != nil {
		t.Fatalf("WriteDAG: %v", err)
	}
}

func writeGate(t *testing.T, dir string, phase int, status string) {
	t.Helper()
	gate := map[string]interface{}{
		"phase":  phase,
		"name":   "test phase",
		"status": status,
	}
	data, _ := json.Marshal(gate)
	path := filepath.Join(dir, ".agent", "phase_gates", fmt.Sprintf("phase_%d.gate.json", phase))
	os.WriteFile(path, data, 0o644)
}

func TestDryRun_noConfirm(t *testing.T) {
	repo := makeRepo(t)
	writeDAG(t, repo, rollback.RollbackDAG{
		Phase:        3,
		Name:         "api-handlers",
		GitSHABefore: "abc123",
		GitSHAAfter:  "def456",
	})

	var out bytes.Buffer
	code := rollback.Run(rollback.Options{
		Phase:    3,
		RepoPath: repo,
		Confirm:  false,
		Out:      &out,
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "Dry run") {
		t.Errorf("expected 'Dry run' in output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Phase 3") {
		t.Errorf("expected 'Phase 3' in output:\n%s", out.String())
	}
}

func TestDryRun_missingDAG(t *testing.T) {
	repo := makeRepo(t)
	var out bytes.Buffer
	code := rollback.Run(rollback.Options{
		Phase:    5,
		RepoPath: repo,
		Confirm:  false,
		Out:      &out,
	})
	if code == 0 {
		t.Fatal("expected non-zero exit when DAG is missing")
	}
}

func TestConfirm_invalidatesGate(t *testing.T) {
	repo := makeRepo(t)

	// Create a file that should be deleted on rollback
	target := filepath.Join(repo, "internal", "api", "handler.go")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("package api\n"), 0o644)

	dag := rollback.RollbackDAG{
		Phase:        3,
		Name:         "api-handlers",
		GitSHABefore: "abc123",
		GitSHAAfter:  "def456",
	}
	dag.Rollback.GitResetTo = "abc123"
	dag.Rollback.FilesToDelete = []string{"internal/api/handler.go"}

	writeDAG(t, repo, dag)
	writeGate(t, repo, 3, "passed")

	var out bytes.Buffer
	code := rollback.Run(rollback.Options{
		Phase:    3,
		RepoPath: repo,
		Confirm:  true,
		Out:      &out,
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput: %s", code, out.String())
	}

	// File should be deleted
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("expected handler.go to be deleted after rollback")
	}

	// Gate should be renamed to rolled_back
	rolledPath := filepath.Join(repo, ".agent", "phase_gates", "phase_3.rolled_back.json")
	if _, err := os.Stat(rolledPath); os.IsNotExist(err) {
		t.Error("expected phase_3.rolled_back.json to exist after rollback")
	}
	passedPath := filepath.Join(repo, ".agent", "phase_gates", "phase_3.gate.json")
	if _, err := os.Stat(passedPath); !os.IsNotExist(err) {
		t.Error("expected phase_3.gate.json to be removed after rollback")
	}

	// Audit log should be appended
	auditPath := filepath.Join(repo, ".agent", "audit.jsonl")
	data, _ := os.ReadFile(auditPath)
	if !strings.Contains(string(data), "rollback.executed") {
		t.Errorf("expected rollback.executed in audit log:\n%s", string(data))
	}
}

func TestConfirm_downstreamUnwoundFirst(t *testing.T) {
	repo := makeRepo(t)

	// Phase 3 — target
	writeDAG(t, repo, rollback.RollbackDAG{
		Phase: 3, Name: "api-handlers",
		GitSHABefore: "sha3before", GitSHAAfter: "sha3after",
	})
	writeGate(t, repo, 3, "passed")

	// Phase 4 — downstream
	writeDAG(t, repo, rollback.RollbackDAG{
		Phase: 4, Name: "server-entrypoint",
		GitSHABefore: "sha4before", GitSHAAfter: "sha4after",
	})
	writeGate(t, repo, 4, "passed")

	var out bytes.Buffer
	code := rollback.Run(rollback.Options{
		Phase:    3,
		RepoPath: repo,
		Confirm:  true,
		Out:      &out,
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput: %s", code, out.String())
	}

	// Both gates should be rolled back
	for _, phase := range []int{3, 4} {
		rolledPath := filepath.Join(repo, ".agent", "phase_gates", fmt.Sprintf("phase_%d.rolled_back.json", phase))
		if _, err := os.Stat(rolledPath); os.IsNotExist(err) {
			t.Errorf("expected phase_%d.rolled_back.json to exist", phase)
		}
	}
}

func writeSampleGate(t *testing.T, dir string, phase int) {
	t.Helper()
	gate := map[string]interface{}{
		"phase":         phase,
		"name":          "sample-phase",
		"status":        "passed",
		"timestamp_utc": "2026-06-10T00:00:00Z",
		"git_sha_before": "sha_before",
		"git_sha_after":  "sha_after",
		"files_changed": []string{"internal/foo/foo.go", "go.mod"},
		"exit_criteria_results": []map[string]interface{}{
			{"criterion": "go test ./... exits 0", "passed": true},
		},
	}
	data, _ := json.Marshal(gate)
	path := filepath.Join(dir, ".agent", "phase_gates", fmt.Sprintf("phase_%d.gate.json", phase))
	os.WriteFile(path, data, 0o644)
}

func TestDeriveFromGate_basic(t *testing.T) {
	repo := makeRepo(t)
	writeSampleGate(t, repo, 5)

	dag, err := rollback.DeriveFromGate(repo, 5)
	if err != nil {
		t.Fatalf("DeriveFromGate: %v", err)
	}

	if dag.Phase != 5 {
		t.Errorf("expected phase 5, got %d", dag.Phase)
	}
	if dag.Name != "sample-phase" {
		t.Errorf("expected name 'sample-phase', got %q", dag.Name)
	}
	if dag.GitSHABefore != "sha_before" {
		t.Errorf("expected git_sha_before 'sha_before', got %q", dag.GitSHABefore)
	}
	if dag.Rollback.GitResetTo != "sha_before" {
		t.Errorf("expected rollback.git_reset_to 'sha_before', got %q", dag.Rollback.GitResetTo)
	}
	if len(dag.FilesCreated) != 2 {
		t.Errorf("expected 2 files_created, got %d", len(dag.FilesCreated))
	}
	if len(dag.Rollback.FilesToDelete) != 2 {
		t.Errorf("expected 2 files_to_delete, got %d", len(dag.Rollback.FilesToDelete))
	}
	if len(dag.Deliverables) != 1 || dag.Deliverables[0] != "go test ./... exits 0" {
		t.Errorf("unexpected deliverables: %v", dag.Deliverables)
	}
}

func TestDeriveFromGate_missingGate(t *testing.T) {
	repo := makeRepo(t)
	_, err := rollback.DeriveFromGate(repo, 99)
	if err == nil {
		t.Fatal("expected error for missing gate, got nil")
	}
}

func TestDeriveFromGate_unknownGitSHA(t *testing.T) {
	repo := makeRepo(t)
	gate := map[string]interface{}{
		"phase":  6,
		"name":   "no-git",
		"status": "passed",
	}
	data, _ := json.Marshal(gate)
	path := filepath.Join(repo, ".agent", "phase_gates", "phase_6.gate.json")
	os.WriteFile(path, data, 0o644)

	dag, err := rollback.DeriveFromGate(repo, 6)
	if err != nil {
		t.Fatalf("DeriveFromGate: %v", err)
	}
	if dag.Rollback.GitResetTo != "unknown" {
		t.Errorf("expected 'unknown' git_reset_to, got %q", dag.Rollback.GitResetTo)
	}
}

func TestDeriveFromGate_writesDAG(t *testing.T) {
	repo := makeRepo(t)
	writeSampleGate(t, repo, 5)

	dag, err := rollback.DeriveFromGate(repo, 5)
	if err != nil {
		t.Fatalf("DeriveFromGate: %v", err)
	}
	if err := rollback.WriteDAG(repo, *dag); err != nil {
		t.Fatalf("WriteDAG: %v", err)
	}

	path := filepath.Join(repo, ".agent", "snapshots", "phase_5.rollback.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected phase_5.rollback.json to exist after WriteDAG")
	}
}

func TestWriteDAG_roundtrip(t *testing.T) {
	repo := makeRepo(t)
	dag := rollback.RollbackDAG{
		Phase:           2,
		Name:            "todo-repo",
		GitSHABefore:    "aaa",
		GitSHAAfter:     "bbb",
		TimestampUTC:    "2026-06-10T00:00:00Z",
		Deliverables:    []string{"todo-repo"},
		FilesCreated:    []string{"internal/todo/todo.go"},
		FilesModified:   []string{"go.mod"},
		DependsOnPhases: []int{1},
		DownstreamPhases: []int{3, 4},
	}
	dag.Rollback.GitResetTo = "aaa"
	dag.Rollback.FilesToDelete = []string{"internal/todo/todo.go"}
	dag.Rollback.FilesToRestore = []string{"go.mod"}

	if err := rollback.WriteDAG(repo, dag); err != nil {
		t.Fatalf("WriteDAG: %v", err)
	}

	path := filepath.Join(repo, ".agent", "snapshots", "phase_2.rollback.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("rollback DAG not written: %v", err)
	}
	var loaded rollback.RollbackDAG
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if loaded.Phase != 2 || loaded.Name != "todo-repo" {
		t.Errorf("unexpected values: phase=%d name=%s", loaded.Phase, loaded.Name)
	}
	if len(loaded.DownstreamPhases) != 2 {
		t.Errorf("expected 2 downstream phases, got %d", len(loaded.DownstreamPhases))
	}
}
