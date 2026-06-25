// Package bootstrap initialises a new project's harness file structure.
// It copies every deterministic artifact from the embedded template: harness
// rules, hooks, scripts, slash commands (.claude/.codex), stub manifest,
// phase-0 prompt, audit/ledger/gate stubs, and agent log files.
package bootstrap

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"keel/internal/assets"
	"keel/internal/session"
)

// Options controls bootstrap behaviour.
type Options struct {
	ForceTemplate bool
	Tracking      string    // "local" (default) or "off"
	Out           io.Writer // stdout (nil → os.Stdout)
}

// Run bootstraps the harness in repoPath. Returns an error on failure.
func Run(repoPath string, opts Options) error {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	repo, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("cannot resolve repo path: %w", err)
	}

	w := opts.Out
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)

	// Collect what we did for the summary.
	var dirsCreated []string
	var filesCopied []string
	var filesCreated []string
	var configActions []string

	// --- Directory creation ---
	dirs := []string{
		"docs/phases",
		"docs/audit",
		"docs/build-ledger",
		"docs/decisions",
		".agent/phase_gates",
		".agent/logs",
		".agent/snapshots",
	}
	for _, rel := range dirs {
		target := filepath.Join(repo, rel)
		isNew := !dirExists(target)
		if err := os.MkdirAll(target, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", rel, err)
		}
		if isNew {
			dirsCreated = append(dirsCreated, rel+"/")
		}
	}

	// --- Asset copy from embedded FS ---
	templateFS := assets.KeelTemplate()
	templateRoot, err := fs.Sub(templateFS, "assets/keel-template")
	if err != nil {
		return fmt.Errorf("cannot sub into keel-template: %w", err)
	}

	// harness/ — copy if force or not yet present
	if opts.ForceTemplate || !fileExists(filepath.Join(repo, "keel", "README.md")) {
		if err := copyTree(templateRoot, "keel", repo, "keel"); err != nil {
			return fmt.Errorf("copy keel: %w", err)
		}
		filesCopied = append(filesCopied, "keel/  (rules, commands, hooks, scripts)")
	}

	// scripts/ — copy if force or not yet present
	if opts.ForceTemplate || !fileExists(filepath.Join(repo, "scripts", "verify.sh")) {
		if err := copyTree(templateRoot, "scripts", repo, "scripts"); err != nil {
			return fmt.Errorf("copy scripts: %w", err)
		}
		filesCopied = append(filesCopied, "scripts/  (verify.sh, verify_phase0.sh)")
	}

	// .claude/ and .codex/ — always copy (now embedded via all: prefix)
	for _, adapter := range []string{".claude", ".codex"} {
		if dirExistsInFS(templateRoot, adapter) {
			if opts.ForceTemplate || !dirExists(filepath.Join(repo, adapter, "commands")) {
				if err := copyTree(templateRoot, adapter, repo, adapter); err != nil {
					return fmt.Errorf("copy %s: %w", adapter, err)
				}
				filesCopied = append(filesCopied, adapter+"/  (slash commands: /keel-run, /new-feature, /approve-feature)")
			}
		}
	}

	// docs/ tree from template (phase_0.md, etc.) — copy if force or not yet present
	if opts.ForceTemplate || !fileExists(filepath.Join(repo, "docs", "phases", "phase_0.md")) {
		if err := copyTree(templateRoot, "docs", repo, "docs"); err != nil {
			return fmt.Errorf("copy docs: %w", err)
		}
		filesCopied = append(filesCopied, "docs/phases/phase_0.md  (Phase 0 build file)")
	}

	// Root-level template files — copy if force or not yet present
	for _, name := range []string{"agent-rules.md", "AGENTS.md"} {
		dst := filepath.Join(repo, name)
		if opts.ForceTemplate || !fileExists(dst) {
			if err := copyFile(templateRoot, name, dst); err != nil {
				return fmt.Errorf("copy %s: %w", name, err)
			}
			filesCopied = append(filesCopied, name)
		}
	}

	// --- Manifest stub and phase_0 prompt ---
	if writeIfMissing(filepath.Join(repo, "BUILD_MANIFEST.yaml"), buildManifestStub) {
		filesCreated = append(filesCreated, "BUILD_MANIFEST.yaml  (phase manifest stub)")
	}
	if writeIfMissing(filepath.Join(repo, "docs", "phases", "phase_0.prompt.md"), phase0PromptContent) {
		filesCreated = append(filesCreated, "docs/phases/phase_0.prompt.md  (Phase 0 agent prompt)")
	}

	// --- Phase 0 audit / ledger / gate stubs ---
	if writeIfMissing(filepath.Join(repo, "docs", "audit", "phase_0.log"),
		"Phase 0 Audit Log — Controlled Execution Harness Bootstrap\nStatus: pending\n") {
		filesCreated = append(filesCreated, "docs/audit/phase_0.log")
	}
	if writeIfMissing(filepath.Join(repo, "docs", "build-ledger", "phase_0_build.md"),
		"# Phase 0 Build Ledger — Controlled Execution Harness Bootstrap\n\n## Status\npending\n") {
		filesCreated = append(filesCreated, "docs/build-ledger/phase_0_build.md")
	}
	if writeIfMissing(filepath.Join(repo, ".agent", "phase_gates", "phase_0.gate.json"),
		"{\n  \"phase\": 0,\n  \"name\": \"Controlled Execution Harness Bootstrap\",\n  \"status\": \"pending\",\n  \"timestamp_utc\": \"\",\n  \"files_changed\": [],\n  \"commands\": [],\n  \"exit_criteria_results\": []\n}\n") {
		filesCreated = append(filesCreated, ".agent/phase_gates/phase_0.gate.json  (pending)")
	}

	// --- Touch agent log files ---
	if touchFile(filepath.Join(repo, ".agent", "run_log.jsonl")) {
		filesCreated = append(filesCreated, ".agent/run_log.jsonl  (append-only run log)")
	}
	if touchFile(filepath.Join(repo, ".agent", "audit.jsonl")) {
		filesCreated = append(filesCreated, ".agent/audit.jsonl  (append-only audit log)")
	}

	// --- Session tracking: config + hooks ---
	trackingMode := opts.Tracking
	if trackingMode == "" {
		trackingMode = "local"
	}
	var trackingCfg session.TrackingConfig
	if trackingMode == "off" {
		trackingCfg = session.DisabledConfig()
	} else {
		trackingCfg = session.DefaultConfig()
	}
	if err := session.SaveConfig(repo, trackingCfg); err != nil {
		fmt.Fprintf(w, "Warning: could not write tracking config: %v\n", err)
	} else {
		configActions = append(configActions, "Session tracking: "+trackingMode)
	}
	if trackingCfg.IsEnabled() {
		installer := &session.HookInstaller{RepoPath: repo}
		if err := installer.Install(); err != nil {
			fmt.Fprintf(w, "Warning: could not install hooks: %v\n", err)
		} else {
			configActions = append(configActions, "Session hooks installed")
		}
		if err := installer.InstallClaudeSettings(); err != nil {
			fmt.Fprintf(w, "Warning: could not update .claude/settings.json: %v\n", err)
		} else {
			configActions = append(configActions, ".claude/settings.json updated with hook permissions")
		}
		if err := installer.InstallCodexSettings(); err != nil {
			fmt.Fprintf(w, "Warning: could not update .codex/setup.sh: %v\n", err)
		} else {
			configActions = append(configActions, ".codex/setup.sh updated for Codex integration")
		}
	}

	// --- Print summary ---
	printSummary(w, repo, now, dirsCreated, filesCopied, filesCreated, configActions)
	return nil
}

// printSummary outputs the structured post-init report.
func printSummary(w io.Writer, repo, timestamp string, dirsCreated, filesCopied, filesCreated, configActions []string) {
	const divider = "────────────────────────────────────────────────────────"

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  keel init complete\n")
	fmt.Fprintf(w, "  %s\n", divider)
	fmt.Fprintf(w, "  Project:   %s\n", repo)
	fmt.Fprintf(w, "  Timestamp: %s\n", timestamp)
	fmt.Fprintln(w)

	// Section 1: What was created
	fmt.Fprintf(w, "  CREATED\n")
	fmt.Fprintf(w, "  %s\n", divider)
	fmt.Fprintln(w)

	if len(dirsCreated) > 0 {
		fmt.Fprintf(w, "  Directories:\n")
		for _, d := range dirsCreated {
			fmt.Fprintf(w, "    %s\n", d)
		}
		fmt.Fprintln(w)
	}

	if len(filesCopied) > 0 {
		fmt.Fprintf(w, "  Template files:\n")
		for _, f := range filesCopied {
			fmt.Fprintf(w, "    %s\n", f)
		}
		fmt.Fprintln(w)
	}

	if len(filesCreated) > 0 {
		fmt.Fprintf(w, "  Stub files:\n")
		for _, f := range filesCreated {
			fmt.Fprintf(w, "    %s\n", f)
		}
		fmt.Fprintln(w)
	}

	if len(configActions) > 0 {
		fmt.Fprintf(w, "  Configuration:\n")
		for _, c := range configActions {
			fmt.Fprintf(w, "    %s\n", c)
		}
		fmt.Fprintln(w)
	}

	// Section 2: Project structure
	fmt.Fprintf(w, "  PROJECT STRUCTURE\n")
	fmt.Fprintf(w, "  %s\n", divider)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  keel/                     Governance rules, commands, hooks, scripts\n")
	fmt.Fprintf(w, "    rules/                  Phase rules the agent must follow\n")
	fmt.Fprintf(w, "    commands/               Slash commands (/keel-run, /new-feature, etc.)\n")
	fmt.Fprintf(w, "    hooks/                  Pre-flight checks run before each phase\n")
	fmt.Fprintf(w, "    scripts/                Verification and utility scripts\n")
	fmt.Fprintf(w, "  docs/\n")
	fmt.Fprintf(w, "    phases/                 Phase build files (one per phase)\n")
	fmt.Fprintf(w, "    audit/                  Human-readable audit logs\n")
	fmt.Fprintf(w, "    build-ledger/           Build ledger entries per phase\n")
	fmt.Fprintf(w, "    decisions/              Architecture decision records\n")
	fmt.Fprintf(w, "  .agent/                   Machine-readable state (gates, snapshots, logs)\n")
	fmt.Fprintf(w, "  scripts/                  Verification shell scripts\n")
	fmt.Fprintf(w, "  BUILD_MANIFEST.yaml       Phase manifest (generated by Phase 0)\n")
	fmt.Fprintf(w, "  agent-rules.md            Agent rules and tech stack\n")
	fmt.Fprintln(w)

	// Section 3: What happens next
	fmt.Fprintf(w, "  NEXT STEP: /keel-run 0\n")
	fmt.Fprintf(w, "  %s\n", divider)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Run this command in your AI agent (Claude Code, Codex, etc.):\n")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    /keel-run 0\n")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  This starts Phase 0 — the planning phase. The agent will:\n")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    1. Read docs/PRD.md (your product requirements)\n")
	fmt.Fprintf(w, "    2. Generate docs/TDD.md (technical design)\n")
	fmt.Fprintf(w, "    3. Build the full BUILD_MANIFEST.yaml with all phases\n")
	fmt.Fprintf(w, "    4. Generate docs/phases/phase_N.md for each phase\n")
	fmt.Fprintf(w, "    5. Run verification and close Phase 0\n")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Phase 0 does NOT write any product code.\n")
	fmt.Fprintf(w, "  It only creates the plan and governance files.\n")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  After Phase 0 completes, run /keel-run 1 to start building.\n")
	fmt.Fprintln(w)

	// Section 4: Available commands
	fmt.Fprintf(w, "  AVAILABLE COMMANDS\n")
	fmt.Fprintf(w, "  %s\n", divider)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Agent slash commands (run inside your AI agent):\n")
	fmt.Fprintf(w, "    /keel-run N              Execute phase N\n")
	fmt.Fprintf(w, "    /new-feature             Propose a new feature branch\n")
	fmt.Fprintf(w, "    /approve-feature         Approve and merge a feature\n")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  CLI commands (run in your terminal):\n")
	fmt.Fprintf(w, "    keel phase start N       Record phase start\n")
	fmt.Fprintf(w, "    keel phase close N       Close phase with all artifacts\n")
	fmt.Fprintf(w, "    keel phase failed N      Record phase failure\n")
	fmt.Fprintf(w, "    keel rollback N          Roll back phase N (dry-run)\n")
	fmt.Fprintf(w, "    keel rollback N --confirm  Execute rollback\n")
	fmt.Fprintf(w, "    keel verify              Run verification checks\n")
	fmt.Fprintf(w, "    keel plan-phases         Regenerate phase build files\n")
	fmt.Fprintln(w)
}

// copyTree walks srcDir within srcFS and writes each file under dstRootDir/dstSub.
func copyTree(srcFS fs.FS, srcDir string, dstRoot string, dstSub string) error {
	return fs.WalkDir(srcFS, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Skip Python cache and .DS_Store
		if strings.Contains(path, "__pycache__") || strings.HasSuffix(path, ".pyc") || d.Name() == ".DS_Store" {
			return nil
		}

		rel, _ := filepath.Rel(srcDir, path)
		dst := filepath.Join(dstRoot, dstSub, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}

		f, err := srcFS.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()

		if _, err := io.Copy(out, f); err != nil {
			return err
		}

		// Make shell scripts executable
		if strings.HasSuffix(dst, ".sh") {
			if info, err := out.Stat(); err == nil {
				os.Chmod(dst, info.Mode()|0o111)
			}
		}
		return nil
	})
}

// copyFile copies a single file from srcFS to dstPath.
func copyFile(srcFS fs.FS, srcName string, dstPath string) error {
	f, err := srcFS.Open(srcName)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, f)
	return err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func dirExistsInFS(fsys fs.FS, path string) bool {
	info, err := fs.Stat(fsys, path)
	return err == nil && info.IsDir()
}

func writeIfMissing(path string, content string) bool {
	if fileExists(path) {
		return false
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(content), 0o644)
	return true
}

func touchFile(path string) bool {
	if fileExists(path) {
		return false
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte{}, 0o644)
	return true
}

// ---------------------------------------------------------------------------
// Stub file content (matches bootstrap_harness.py exactly)
// ---------------------------------------------------------------------------

const buildManifestStub = `title: BUILD_MANIFEST.yaml
source: BUILD_MANIFEST.yaml
planning_scope: keel-generation-bootstrap
phases:
  - phase: 0
    name: Controlled Execution Harness Bootstrap
    goal: Generate docs/TDD.md from the PRD, confirm the tech stack, produce the full build manifest, and generate phase build prompts — without implementing product code.
    inputs:
      - docs/PRD.md
      - docs/phases/phase_0.prompt.md
    outputs:
      - docs/TDD.md
      - BUILD_MANIFEST.yaml
      - docs/phases/phase_N.md (one per manifest phase)
    tasks:
      - Generate docs/TDD.md from docs/PRD.md using the tdd-builder procedure.
      - Confirm the tech stack and write it to agent-rules.md.
      - Generate the full product build manifest from docs/PRD.md and docs/TDD.md.
      - Generate one docs/phases/phase_N.md file per manifest phase.
      - Run harness-only verification.
      - Write audit, ledger, run-log, and gate records.
    exit_criteria:
      - bash keel/hooks/preflight.sh exits 0.
      - python3 keel/scripts/verify_repo.py . exits 0.
      - bash scripts/verify_phase0.sh exits 0.
    out_of_scope:
      - Product backend, web, runtime, database, compose, worker, MCP, UI, dependency install, product build, and product test files.
`

const phase0PromptContent = `# Phase 0 Prompt - Generate Controlled Execution Harness Only

Generate the keel governance for this repository.

Do not build the product. Do not scaffold backend code, web/frontend code, runtime code, compose services, database migrations, workers, MCP runtime code, or UI screens. This phase creates only the harness, rules, hooks, commands, skills, verification helpers, build manifest, execution plan, and phase prompts needed for later agents to build the product under phase control.

## Local Product Sources

Use this repository's sources of truth:

- docs/PRD.md (required — the starting point for Phase 0)
- DESIGN.md (optional, read if present)
- agent-rules.md or AGENTS.md, if present

## Required Outputs

Generate or update:

- docs/TDD.md (generated from docs/PRD.md via tdd-builder procedure)
- BUILD_MANIFEST.yaml
- keel/**
- scripts/verify.sh and scripts/verify_phase0.sh
- docs/phases/phase_0.md
- one docs/phases/phase_<n>.md file for every phase in BUILD_MANIFEST.yaml
- docs/audit/phase_0.log
- docs/build-ledger/phase_0_build.md
- docs/decisions/ when an ADR is required
- .agent/audit.jsonl, .agent/run_log.jsonl, and .agent/phase_gates/phase_0.gate.json

## Verification

Run only harness-generation verification:

` + "```bash" + `
bash keel/hooks/preflight.sh
python3 keel/scripts/verify_repo.py .
bash scripts/verify_phase0.sh
` + "```" + `

## Stop Conditions

Stop and ask the human if PRD/TDD are missing, requirements conflict, product files would need to be created, or dependency installs/product tests would be required.
`
