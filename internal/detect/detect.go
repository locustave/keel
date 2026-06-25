// Package detect scans a repository for tech stack markers and prompts the
// user to confirm the detected stack before keel init writes it to agent-rules.md.
package detect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Stack holds the five dimensions written to agent-rules.md ## Tech Stack.
type Stack struct {
	Language    string // Language / runtime
	DataStorage string // Data storage
	Frontend    string // Frontend / UI
	TestRunner  string // Test runner
	Deployment  string // Deployment target
}

// IsEmpty returns true when nothing was detected.
func (s Stack) IsEmpty() bool {
	return s.Language == "" && s.TestRunner == ""
}

// HasPlaceholders returns true when any dimension is still unset.
func (s Stack) HasPlaceholders() bool {
	return s.Language == "" || s.DataStorage == "" || s.Frontend == "" ||
		s.TestRunner == "" || s.Deployment == ""
}

// ---------------------------------------------------------------------------
// Scan
// ---------------------------------------------------------------------------

// Scan inspects repoPath for well-known marker files and returns a best-effort
// Stack. All markers are checked — multiple languages are combined.
func Scan(repoPath string) Stack {
	var langs, runners, frontends []string

	if exists(repoPath, "go.mod") {
		langs = append(langs, "Go")
		runners = append(runners, "go test")
	}
	if exists(repoPath, "Cargo.toml") {
		langs = append(langs, "Rust")
		runners = append(runners, "cargo test")
	}
	if exists(repoPath, "package.json") {
		langs = append(langs, nodeLanguage(repoPath))
		tr, fe := nodeStack(repoPath)
		runners = append(runners, tr)
		if fe != "" {
			frontends = append(frontends, fe)
		}
	}
	if exists(repoPath, "pyproject.toml") || exists(repoPath, "requirements.txt") || exists(repoPath, "setup.py") {
		langs = append(langs, "Python "+pythonPkgManager(repoPath))
		runners = append(runners, pythonTestRunner(repoPath))
	}
	if exists(repoPath, "Gemfile") {
		langs = append(langs, "Ruby")
		runners = append(runners, "RSpec")
	}
	if exists(repoPath, "pom.xml") {
		langs = append(langs, "Java")
		runners = append(runners, "JUnit / Maven")
	}
	if exists(repoPath, "build.gradle") || exists(repoPath, "build.gradle.kts") {
		langs = append(langs, "Java / Kotlin")
		runners = append(runners, "JUnit / Gradle")
	}
	if exists(repoPath, "Package.swift") {
		langs = append(langs, "Swift")
		runners = append(runners, "XCTest")
	}
	if exists(repoPath, "pubspec.yaml") {
		langs = append(langs, "Dart / Flutter")
		runners = append(runners, "flutter test")
	}
	if exists(repoPath, "mix.exs") {
		langs = append(langs, "Elixir")
		runners = append(runners, "ExUnit")
	}
	if globAny(repoPath, "*.csproj") {
		langs = append(langs, "C#")
		runners = append(runners, "dotnet test")
	}
	if exists(repoPath, "CMakeLists.txt") {
		langs = append(langs, "C/C++")
	}

	s := Stack{
		Language:   join(langs),
		TestRunner: join(dedupe(runners)),
		Frontend:   join(frontends),
	}

	if exists(repoPath, "docker-compose.yml") || exists(repoPath, "docker-compose.yaml") {
		s.Deployment = "Docker"
		s.DataStorage = inferStorageFromCompose(repoPath)
	}

	return s
}

func join(parts []string) string  { return strings.Join(parts, ", ") }

func dedupe(parts []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range parts {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Confirm
// ---------------------------------------------------------------------------

// Confirm shows the detected stack to the user and prompts for confirmation
// or corrections. If in is not a terminal (e.g. CI), it prints the detected
// stack and returns it unchanged.
func Confirm(stack Stack, in io.Reader, out io.Writer) Stack {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Detected tech stack:")
	fmt.Fprintln(out)
	row(out, "Language / runtime", stack.Language)
	row(out, "Data storage", stack.DataStorage)
	row(out, "Frontend / UI", stack.Frontend)
	row(out, "Test runner", stack.TestRunner)
	row(out, "Deployment target", stack.Deployment)
	fmt.Fprintln(out)

	if !isTerminal(in) {
		fmt.Fprintln(out, "  (non-interactive — using detected stack)")
		return stack
	}

	scanner := bufio.NewScanner(in)

	fmt.Fprint(out, "  Confirm? [Y/n/e(dit)]: ")
	scanner.Scan()
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))

	if answer == "n" || answer == "no" {
		// User wants to correct — prompt every field.
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  Enter corrections (press Enter to keep current value):")
		fmt.Fprintln(out)
		stack.Language = prompt(scanner, out, "  Language / runtime", stack.Language)
		stack.DataStorage = prompt(scanner, out, "  Data storage", stack.DataStorage)
		stack.Frontend = prompt(scanner, out, "  Frontend / UI", stack.Frontend)
		stack.TestRunner = prompt(scanner, out, "  Test runner", stack.TestRunner)
		stack.Deployment = prompt(scanner, out, "  Deployment target", stack.Deployment)
	} else if answer == "e" || answer == "edit" {
		// Edit mode — prompt every field.
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  Edit each field (press Enter to keep current value):")
		fmt.Fprintln(out)
		stack.Language = prompt(scanner, out, "  Language / runtime", stack.Language)
		stack.DataStorage = prompt(scanner, out, "  Data storage", stack.DataStorage)
		stack.Frontend = prompt(scanner, out, "  Frontend / UI", stack.Frontend)
		stack.TestRunner = prompt(scanner, out, "  Test runner", stack.TestRunner)
		stack.Deployment = prompt(scanner, out, "  Deployment target", stack.Deployment)
	} else if stack.HasPlaceholders() {
		// Confirmed but some fields are empty — prompt only the missing ones.
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  Some categories are empty — fill in or press Enter to skip:")
		fmt.Fprintln(out)
		if stack.Language == "" {
			stack.Language = prompt(scanner, out, "  Language / runtime", "")
		}
		if stack.DataStorage == "" {
			stack.DataStorage = prompt(scanner, out, "  Data storage", "")
		}
		if stack.Frontend == "" {
			stack.Frontend = prompt(scanner, out, "  Frontend / UI", "")
		}
		if stack.TestRunner == "" {
			stack.TestRunner = prompt(scanner, out, "  Test runner", "")
		}
		if stack.Deployment == "" {
			stack.Deployment = prompt(scanner, out, "  Deployment target", "")
		}
	} else {
		return stack
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "  Updated tech stack:")
	fmt.Fprintln(out)
	row(out, "Language / runtime", stack.Language)
	row(out, "Data storage", stack.DataStorage)
	row(out, "Frontend / UI", stack.Frontend)
	row(out, "Test runner", stack.TestRunner)
	row(out, "Deployment target", stack.Deployment)
	fmt.Fprintln(out)

	return stack
}

// ---------------------------------------------------------------------------
// ApplyToAgentRules rewrites the ## Tech Stack table in agent-rules.md.
// ---------------------------------------------------------------------------

func ApplyToAgentRules(repoPath string, stack Stack) error {
	path := filepath.Join(repoPath, "agent-rules.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	val := func(s string) string {
		if s == "" {
			return "—"
		}
		return s
	}

	replacement := fmt.Sprintf(
		"| Language / runtime | %s |\n| Data storage | %s |\n| Frontend / UI | %s |\n| Test runner | %s |\n| Deployment target | %s |",
		val(stack.Language), val(stack.DataStorage), val(stack.Frontend),
		val(stack.TestRunner), val(stack.Deployment),
	)

	original := "| Language / runtime | — |\n| Data storage | — |\n| Frontend / UI | — |\n| Test runner | — |\n| Deployment target | — |"
	updated := strings.ReplaceAll(string(data), original, replacement)

	if updated == string(data) {
		// Table not found in expected format — skip silently.
		return nil
	}

	return os.WriteFile(path, []byte(updated), 0o644)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func row(out io.Writer, label, value string) {
	if value == "" {
		value = "—"
	}
	fmt.Fprintf(out, "    %-22s %s\n", label, value)
}

func prompt(scanner *bufio.Scanner, out io.Writer, label, current string) string {
	if current == "" {
		fmt.Fprintf(out, "  %s: ", label)
	} else {
		fmt.Fprintf(out, "  %s [%s]: ", label, current)
	}
	scanner.Scan()
	text := strings.TrimSpace(scanner.Text())
	if text == "" {
		return current
	}
	return text
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func exists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

func globAny(root, pattern string) bool {
	matches, _ := filepath.Glob(filepath.Join(root, pattern))
	return len(matches) > 0
}

// ---------------------------------------------------------------------------
// Language-specific helpers
// ---------------------------------------------------------------------------

func nodeLanguage(root string) string {
	if exists(root, "tsconfig.json") {
		return "TypeScript / Node"
	}
	if globAny(root, "*.ts") || globAny(root, "src/*.ts") {
		return "TypeScript / Node"
	}
	return "JavaScript / Node"
}

type pkgJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func nodeStack(root string) (testRunner, frontend string) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return "jest", ""
	}
	var pkg pkgJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "jest", ""
	}

	all := map[string]bool{}
	for k := range pkg.Dependencies {
		all[k] = true
	}
	for k := range pkg.DevDependencies {
		all[k] = true
	}

	// Test runner
	switch {
	case all["vitest"]:
		testRunner = "vitest"
	case all["jest"] || all["@jest/core"]:
		testRunner = "jest"
	case all["mocha"]:
		testRunner = "mocha"
	default:
		testRunner = "jest"
	}

	// Frontend framework
	switch {
	case all["next"]:
		frontend = "Next.js"
	case all["nuxt"] || all["nuxt3"]:
		frontend = "Nuxt"
	case all["@remix-run/react"]:
		frontend = "Remix"
	case all["react"]:
		frontend = "React"
	case all["vue"]:
		frontend = "Vue"
	case all["svelte"]:
		frontend = "Svelte"
	case all["@angular/core"]:
		frontend = "Angular"
	case all["express"]:
		frontend = "Express (API)"
	case all["fastify"]:
		frontend = "Fastify (API)"
	case all["@nestjs/core"]:
		frontend = "NestJS (API)"
	}

	return testRunner, frontend
}

func pythonPkgManager(root string) string {
	if exists(root, "uv.lock") {
		return "(uv)"
	}
	data, _ := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	content := string(data)
	switch {
	case strings.Contains(content, "[tool.uv]"):
		return "(uv)"
	case strings.Contains(content, "[tool.poetry]"):
		return "(Poetry)"
	case exists(root, "Pipfile"):
		return "(Pipenv)"
	default:
		return "(pip)"
	}
}

func pythonTestRunner(root string) string {
	if exists(root, "pytest.ini") || exists(root, "conftest.py") {
		return "pytest"
	}
	data, _ := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if strings.Contains(string(data), "pytest") {
		return "pytest"
	}
	return "pytest"
}

func inferStorageFromCompose(root string) string {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		content := strings.ToLower(string(data))
		switch {
		case strings.Contains(content, "postgres"):
			return "PostgreSQL"
		case strings.Contains(content, "mysql") || strings.Contains(content, "mariadb"):
			return "MySQL"
		case strings.Contains(content, "mongo"):
			return "MongoDB"
		case strings.Contains(content, "redis"):
			return "Redis"
		case strings.Contains(content, "sqlite"):
			return "SQLite"
		}
	}
	return ""
}
