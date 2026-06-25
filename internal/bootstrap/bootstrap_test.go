package bootstrap_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keel/internal/bootstrap"
)

// mkRepo creates a temp repo with the given relative files (as empty files).
func mkRepo(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, rel := range files {
		path := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte("# placeholder\n"), 0o644)
	}
	return dir
}

// ---------------------------------------------------------------------------
// Precondition tests
// ---------------------------------------------------------------------------

func TestRun_emptyRepo(t *testing.T) {
	repo := t.TempDir()
	var out bytes.Buffer
	err := bootstrap.Run(repo, bootstrap.Options{Out: &out})
	if err != nil {
		t.Fatalf("expected no error on empty repo, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Directory creation tests
// ---------------------------------------------------------------------------

func TestDirectories(t *testing.T) {
	repo := mkRepo(t, "docs/PRD.md", "docs/TDD.md", "DESIGN.md")
	var out bytes.Buffer
	if err := bootstrap.Run(repo, bootstrap.Options{Out: &out}); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	expectedDirs := []string{
		"docs/phases",
		"docs/audit",
		"docs/build-ledger",
		"docs/decisions",
		".agent/phase_gates",
		".agent/logs",
		".agent/snapshots",
	}
	for _, rel := range expectedDirs {
		path := filepath.Join(repo, rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", rel, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", rel)
		}
	}
}

// ---------------------------------------------------------------------------
// Asset copy tests
// ---------------------------------------------------------------------------

func TestAssetCopy(t *testing.T) {
	repo := mkRepo(t, "docs/PRD.md", "docs/TDD.md", "DESIGN.md")
	var out bytes.Buffer
	if err := bootstrap.Run(repo, bootstrap.Options{Out: &out}); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// harness/README.md should have been copied from the embedded template
	harnessREADME := filepath.Join(repo, "keel", "README.md")
	if _, err := os.Stat(harnessREADME); os.IsNotExist(err) {
		t.Errorf("expected harness/README.md to exist after bootstrap")
	}
}

func TestAssetCopy_noOverwriteWithoutForce(t *testing.T) {
	repo := mkRepo(t, "docs/PRD.md", "docs/TDD.md", "DESIGN.md")

	// Pre-create harness/README.md with sentinel content
	harnessDir := filepath.Join(repo, "keel")
	os.MkdirAll(harnessDir, 0o755)
	sentinel := "SENTINEL CONTENT DO NOT OVERWRITE"
	os.WriteFile(filepath.Join(harnessDir, "README.md"), []byte(sentinel), 0o644)

	var out bytes.Buffer
	if err := bootstrap.Run(repo, bootstrap.Options{Out: &out}); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(harnessDir, "README.md"))
	if string(data) != sentinel {
		t.Errorf("keel/README.md was overwritten without --force-template")
	}
}

func TestAssetCopy_forceOverwrites(t *testing.T) {
	repo := mkRepo(t, "docs/PRD.md", "docs/TDD.md", "DESIGN.md")

	// Pre-create harness/README.md with sentinel content
	harnessDir := filepath.Join(repo, "keel")
	os.MkdirAll(harnessDir, 0o755)
	sentinel := "SENTINEL CONTENT"
	os.WriteFile(filepath.Join(harnessDir, "README.md"), []byte(sentinel), 0o644)

	var out bytes.Buffer
	if err := bootstrap.Run(repo, bootstrap.Options{ForceTemplate: true, Out: &out}); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(harnessDir, "README.md"))
	if string(data) == sentinel {
		t.Errorf("keel/README.md was not overwritten with --force-template")
	}
}

// ---------------------------------------------------------------------------
// Manifest stub tests
// ---------------------------------------------------------------------------

func TestManifestStub(t *testing.T) {
	repo := mkRepo(t, "docs/PRD.md", "docs/TDD.md", "DESIGN.md")
	var out bytes.Buffer
	if err := bootstrap.Run(repo, bootstrap.Options{Out: &out}); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// BUILD_MANIFEST.yaml should be created
	manifestPath := filepath.Join(repo, "BUILD_MANIFEST.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("BUILD_MANIFEST.yaml not created: %v", err)
	}
	if !strings.Contains(string(data), "phase: 0") {
		t.Errorf("BUILD_MANIFEST.yaml should contain phase 0:\n%s", string(data))
	}

	// docs/phases/phase_0.prompt.md should be created
	promptPath := filepath.Join(repo, "docs", "phases", "phase_0.prompt.md")
	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("phase_0.prompt.md not created: %v", err)
	}
	if !strings.Contains(string(promptData), "Phase 0 Prompt") {
		t.Errorf("phase_0.prompt.md missing expected content:\n%s", string(promptData))
	}

	// .agent/run_log.jsonl and .agent/audit.jsonl should exist (may be empty)
	for _, rel := range []string{".agent/run_log.jsonl", ".agent/audit.jsonl"} {
		if _, err := os.Stat(filepath.Join(repo, rel)); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after bootstrap", rel)
		}
	}
}

func TestAdapterCommandsCopied(t *testing.T) {
	repo := mkRepo(t, "docs/PRD.md", "docs/TDD.md", "DESIGN.md")
	var out bytes.Buffer
	if err := bootstrap.Run(repo, bootstrap.Options{Out: &out}); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	for _, rel := range []string{
		".claude/commands/approve-feature.md",
		".codex/commands/approve-feature.md",
	} {
		if _, err := os.Stat(filepath.Join(repo, rel)); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after bootstrap", rel)
		}
	}
}

func TestPhase0Stubs(t *testing.T) {
	repo := mkRepo(t, "docs/PRD.md", "docs/TDD.md", "DESIGN.md")
	var out bytes.Buffer
	if err := bootstrap.Run(repo, bootstrap.Options{Out: &out}); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	for _, rel := range []string{
		"docs/audit/phase_0.log",
		"docs/build-ledger/phase_0_build.md",
		".agent/phase_gates/phase_0.gate.json",
	} {
		if _, err := os.Stat(filepath.Join(repo, rel)); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after bootstrap", rel)
		}
	}
}

func TestRootFilesCopied(t *testing.T) {
	repo := t.TempDir()
	var out bytes.Buffer
	if err := bootstrap.Run(repo, bootstrap.Options{Out: &out}); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	for _, rel := range []string{
		"agent-rules.md",
		"AGENTS.md",
		"docs/phases/phase_0.md",
		"keel/README.md",
	} {
		if _, err := os.Stat(filepath.Join(repo, rel)); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after bootstrap", rel)
		}
	}
}

func TestManifestStub_noOverwrite(t *testing.T) {
	repo := mkRepo(t, "docs/PRD.md", "docs/TDD.md", "DESIGN.md")

	// Pre-create BUILD_MANIFEST.yaml with custom content
	existing := "title: My Custom Project\n"
	os.WriteFile(filepath.Join(repo, "BUILD_MANIFEST.yaml"), []byte(existing), 0o644)

	var out bytes.Buffer
	if err := bootstrap.Run(repo, bootstrap.Options{Out: &out}); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(repo, "BUILD_MANIFEST.yaml"))
	if string(data) != existing {
		t.Errorf("BUILD_MANIFEST.yaml was overwritten (should not be): got:\n%s", string(data))
	}
}
