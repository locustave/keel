package merge_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keel/internal/merge"
)

const projectManifest = `title: My Project
source: docs/TDD.md
planning_scope: full-product
phases:
- phase: 0
  name: Setup
  goal: Implement Setup.
  inputs:
  - setup.go
  tasks:
  - Setup
  exit_criteria:
  - go build ./...
  out_of_scope:
  - Deliverables belonging to other phases.
- phase: 1
  name: Core
  goal: Implement Core.
  inputs:
  - core.go
  tasks:
  - Core
  exit_criteria:
  - go test ./...
  out_of_scope:
  - Deliverables belonging to other phases.
`

const featureManifest = `title: My Feature
source: docs/features/my-feature/TDD.md
planning_scope: full-product
phases:
  - phase: 1
    name: Feature Alpha
    goal: Implement Feature Alpha.
    inputs:
    - alpha.go
    tasks:
    - Feature Alpha
    exit_criteria:
    - go test ./internal/alpha/...
    out_of_scope:
    - Deliverables belonging to other phases.
  - phase: 2
    name: Feature Beta
    goal: Implement Feature Beta.
    inputs:
    - beta.go
    tasks:
    - Feature Beta
    exit_criteria:
    - go test ./internal/beta/...
    out_of_scope:
    - Deliverables belonging to other phases.
`

func setupDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "BUILD_MANIFEST.yaml"), []byte(projectManifest), 0o644)
	featureDir := filepath.Join(dir, "docs", "features", "my-feature")
	os.MkdirAll(featureDir, 0o755)
	os.WriteFile(filepath.Join(featureDir, "BUILD_MANIFEST.yaml"), []byte(featureManifest), 0o644)
	return dir
}

func TestRun_dryRun(t *testing.T) {
	dir := setupDir(t)
	var out, errOut bytes.Buffer
	code := merge.Run(merge.Options{
		Slug:    "my-feature",
		Confirm: false,
		RepoDir: dir,
		Out:     &out,
		Err:     &errOut,
	})
	if code != 0 {
		t.Fatalf("expected 0, got %d\nerr: %s", code, errOut.String())
	}
	output := out.String()
	if !strings.Contains(output, "Dry run") {
		t.Errorf("expected 'Dry run' in output:\n%s", output)
	}
	if !strings.Contains(output, "Mapping") {
		t.Errorf("expected 'Mapping' in output:\n%s", output)
	}
	// Verify project manifest was NOT written (dry run)
	data, _ := os.ReadFile(filepath.Join(dir, "BUILD_MANIFEST.yaml"))
	if strings.Contains(string(data), "phase: 2") && !strings.Contains(string(data), "Feature Alpha") {
		// ok
	}
	if strings.Contains(string(data), "Feature Alpha") {
		t.Error("dry run should not write project manifest")
	}
}

func TestRun_confirm(t *testing.T) {
	dir := setupDir(t)
	var out, errOut bytes.Buffer
	code := merge.Run(merge.Options{
		Slug:    "my-feature",
		Confirm: true,
		RepoDir: dir,
		Out:     &out,
		Err:     &errOut,
	})
	if code != 0 {
		t.Fatalf("expected 0, got %d\nerr: %s\nout: %s", code, errOut.String(), out.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "BUILD_MANIFEST.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	// Feature phases should be renumbered from 2 (last project phase was 1, so start=2)
	if !strings.Contains(content, "phase: 2") {
		t.Errorf("expected phase 2 in merged manifest:\n%s", content)
	}
	if !strings.Contains(content, "phase: 3") {
		t.Errorf("expected phase 3 in merged manifest:\n%s", content)
	}
	if !strings.Contains(content, "Feature Alpha") {
		t.Errorf("expected Feature Alpha in merged manifest:\n%s", content)
	}
}

func TestRun_missingSlug(t *testing.T) {
	var out, errOut bytes.Buffer
	code := merge.Run(merge.Options{
		Slug: "",
		Out:  &out,
		Err:  &errOut,
	})
	if code != 2 {
		t.Errorf("expected exit 2 for missing slug, got %d", code)
	}
}

func TestRun_nonExistentSlug(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "BUILD_MANIFEST.yaml"), []byte(projectManifest), 0o644)
	var out, errOut bytes.Buffer
	code := merge.Run(merge.Options{
		Slug:    "does-not-exist",
		RepoDir: dir,
		Out:     &out,
		Err:     &errOut,
	})
	if code != 1 {
		t.Errorf("expected exit 1 for nonexistent slug, got %d", code)
	}
}

func TestRun_noProjectManifest(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := merge.Run(merge.Options{
		Slug:    "something",
		RepoDir: dir,
		Out:     &out,
		Err:     &errOut,
	})
	if code != 1 {
		t.Errorf("expected exit 1 when no project manifest, got %d", code)
	}
}

func TestRun_mappingNoOverlapCascade(t *testing.T) {
	// Regression: renumbering 1→3 and 2→4 must not cascade.
	// Feature has phases 1 and 2; project has phases 0,1,2 → start=3 → feature phases become 3,4.
	projText := `title: P
phases:
- phase: 0
  name: A
  goal: x.
  inputs:
  exit_criteria:
  out_of_scope:
- phase: 1
  name: B
  goal: x.
  inputs:
  exit_criteria:
  out_of_scope:
- phase: 2
  name: C
  goal: x.
  inputs:
  exit_criteria:
  out_of_scope:
`
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "BUILD_MANIFEST.yaml"), []byte(projText), 0o644)
	featureDir := filepath.Join(dir, "docs", "features", "feat")
	os.MkdirAll(featureDir, 0o755)
	os.WriteFile(filepath.Join(featureDir, "BUILD_MANIFEST.yaml"), []byte(featureManifest), 0o644)

	var out, errOut bytes.Buffer
	code := merge.Run(merge.Options{
		Slug:    "feat",
		Confirm: true,
		RepoDir: dir,
		Out:     &out,
		Err:     &errOut,
	})
	if code != 0 {
		t.Fatalf("expected 0, got %d\nerr: %s\nout: %s", code, errOut.String(), out.String())
	}
	data, _ := os.ReadFile(filepath.Join(dir, "BUILD_MANIFEST.yaml"))
	content := string(data)
	if !strings.Contains(content, "phase: 3") {
		t.Errorf("expected phase 3 in merged:\n%s", content)
	}
	if !strings.Contains(content, "phase: 4") {
		t.Errorf("expected phase 4 in merged:\n%s", content)
	}
	// Must not have duplicate or wrong phase numbers
	if strings.Count(content, "- phase: 3") != 1 {
		t.Errorf("expected exactly one phase 3:\n%s", content)
	}
	if strings.Count(content, "- phase: 4") != 1 {
		t.Errorf("expected exactly one phase 4:\n%s", content)
	}
}
