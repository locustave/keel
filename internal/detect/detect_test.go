package detect_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keel/internal/detect"
)

func TestScan_Go(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.25\n"), 0o644)
	s := detect.Scan(dir)
	if s.Language != "Go" {
		t.Errorf("expected Language=Go, got %q", s.Language)
	}
	if s.TestRunner != "go test" {
		t.Errorf("expected TestRunner='go test', got %q", s.TestRunner)
	}
}

func TestScan_Rust(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"myapp\"\n"), 0o644)
	s := detect.Scan(dir)
	if s.Language != "Rust" {
		t.Errorf("expected Language=Rust, got %q", s.Language)
	}
}

func TestScan_NodeReact(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"react":"^18","jest":"^29"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644)
	s := detect.Scan(dir)
	if !strings.Contains(s.Language, "JavaScript") {
		t.Errorf("expected JavaScript language, got %q", s.Language)
	}
	if s.Frontend != "React" {
		t.Errorf("expected Frontend=React, got %q", s.Frontend)
	}
	if s.TestRunner != "jest" {
		t.Errorf("expected TestRunner=jest, got %q", s.TestRunner)
	}
}

func TestScan_NodeTypeScriptNextVitest(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644)
	pkg := `{"dependencies":{"next":"^14","vitest":"^1"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644)
	s := detect.Scan(dir)
	if !strings.Contains(s.Language, "TypeScript") {
		t.Errorf("expected TypeScript language, got %q", s.Language)
	}
	if s.Frontend != "Next.js" {
		t.Errorf("expected Frontend=Next.js, got %q", s.Frontend)
	}
	if s.TestRunner != "vitest" {
		t.Errorf("expected TestRunner=vitest, got %q", s.TestRunner)
	}
}

func TestScan_Python(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.pytest.ini_options]\n"), 0o644)
	s := detect.Scan(dir)
	if !strings.Contains(s.Language, "Python") {
		t.Errorf("expected Python language, got %q", s.Language)
	}
	if s.TestRunner != "pytest" {
		t.Errorf("expected TestRunner=pytest, got %q", s.TestRunner)
	}
}

func TestScan_Empty(t *testing.T) {
	dir := t.TempDir()
	s := detect.Scan(dir)
	if !s.IsEmpty() {
		t.Errorf("expected empty stack for empty dir, got %+v", s)
	}
}

func TestScan_DockerCompose(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.25\n"), 0o644)
	compose := "services:\n  db:\n    image: postgres:16\n"
	os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0o644)
	s := detect.Scan(dir)
	if s.DataStorage != "PostgreSQL" {
		t.Errorf("expected DataStorage=PostgreSQL, got %q", s.DataStorage)
	}
	if s.Deployment != "Docker" {
		t.Errorf("expected Deployment=Docker, got %q", s.Deployment)
	}
}

func TestConfirm_NonInteractive(t *testing.T) {
	// Non-terminal reader → auto-confirm.
	stack := detect.Stack{Language: "Go", TestRunner: "go test"}
	var out bytes.Buffer
	result := detect.Confirm(stack, strings.NewReader(""), &out)
	if result.Language != "Go" {
		t.Errorf("expected Language=Go, got %q", result.Language)
	}
}

func TestApplyToAgentRules(t *testing.T) {
	dir := t.TempDir()
	content := `## Tech Stack

| Dimension | Decision |
|-----------|----------|
| Language / runtime | — |
| Data storage | — |
| Frontend / UI | — |
| Test runner | — |
| Deployment target | — |
`
	os.WriteFile(filepath.Join(dir, "agent-rules.md"), []byte(content), 0o644)

	stack := detect.Stack{
		Language:    "Go",
		DataStorage: "PostgreSQL",
		Frontend:    "—",
		TestRunner:  "go test",
		Deployment:  "Docker",
	}
	if err := detect.ApplyToAgentRules(dir, stack); err != nil {
		t.Fatalf("ApplyToAgentRules: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "agent-rules.md"))
	updated := string(data)
	if !strings.Contains(updated, "| Language / runtime | Go |") {
		t.Error("expected Language/runtime=Go in agent-rules.md")
	}
	if !strings.Contains(updated, "| Test runner | go test |") {
		t.Error("expected Test runner=go test in agent-rules.md")
	}
	if strings.Contains(updated, "| Language / runtime | — |") {
		t.Error("placeholder should have been replaced")
	}
}
