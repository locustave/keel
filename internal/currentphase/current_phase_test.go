package currentphase_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keel/internal/currentphase"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func buildManifest(phases ...int) string {
	s := "phases:\n"
	for _, p := range phases {
		s += fmt.Sprintf("- phase: %d\n", p)
	}
	return s
}

func passedGate(phase int) string {
	return fmt.Sprintf(`{"phase":%d,"status":"passed","exit_criteria_results":[{"passed":true}]}`, phase)
}

func runCurrent(t *testing.T, root string) (string, int) {
	t.Helper()
	var buf bytes.Buffer
	code := currentphase.Run(root, &buf)
	return buf.String(), code
}

func TestNoGates_StatusReady(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "BUILD_MANIFEST.yaml", buildManifest(0, 1, 2))

	out, code := runCurrent(t, root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d. output: %s", code, out)
	}
	if !strings.Contains(out, "STATUS: ready") {
		t.Fatalf("expected STATUS: ready, got: %s", out)
	}
	if !strings.Contains(out, "Next phase    : 0") {
		t.Fatalf("expected Next phase 0, got: %s", out)
	}
}

func TestSomeGatesPassed_StatusReady(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "BUILD_MANIFEST.yaml", buildManifest(0, 1, 2))
	writeFile(t, root, ".agent/phase_gates/phase_0.gate.json", passedGate(0))
	writeFile(t, root, ".agent/phase_gates/phase_1.gate.json", passedGate(1))

	out, code := runCurrent(t, root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d. output: %s", code, out)
	}
	if !strings.Contains(out, "STATUS: ready") {
		t.Fatalf("expected STATUS: ready, got: %s", out)
	}
	if !strings.Contains(out, "Next phase    : 2") {
		t.Fatalf("expected Next phase 2, got: %s", out)
	}
}

func TestAllGatesPassed_StatusComplete(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "BUILD_MANIFEST.yaml", buildManifest(0, 1))
	writeFile(t, root, ".agent/phase_gates/phase_0.gate.json", passedGate(0))
	writeFile(t, root, ".agent/phase_gates/phase_1.gate.json", passedGate(1))

	out, code := runCurrent(t, root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d. output: %s", code, out)
	}
	if !strings.Contains(out, "STATUS: complete") {
		t.Fatalf("expected STATUS: complete, got: %s", out)
	}
}

func TestFailedGate_StatusBlocked(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "BUILD_MANIFEST.yaml", buildManifest(0, 1))
	writeFile(t, root, ".agent/phase_gates/phase_0.failed.json", `{"phase":0,"status":"failed"}`)

	out, code := runCurrent(t, root)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d. output: %s", code, out)
	}
	if !strings.Contains(out, "STATUS: blocked") {
		t.Fatalf("expected STATUS: blocked, got: %s", out)
	}
}

func TestMissingManifest_ReturnsError(t *testing.T) {
	root := t.TempDir()
	_, code := runCurrent(t, root)
	if code != 1 {
		t.Fatalf("expected exit 1 for missing manifest, got %d", code)
	}
}

func TestOutputContainsSTATUSField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "BUILD_MANIFEST.yaml", buildManifest(0))

	out, _ := runCurrent(t, root)
	if !strings.Contains(out, "STATUS:") {
		t.Fatalf("expected STATUS field in output, got: %s", out)
	}
}
