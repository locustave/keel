// Package gate writes all phase-closing artifacts for the keel harness.
//
// When an agent finishes a build phase it calls `keel phase close N`.
// The Close function in this package writes:
//
//  1. .agent/phase_gates/phase_N.gate.json   — the canonical gate record
//  2. .agent/snapshots/phase_N.rollback.json — rollback DAG (derived from gate)
//  3. docs/build-ledger/phase_N_build.md     — human-readable build ledger
//  4. docs/audit/phase_N.log                 — human-readable audit log
//  5. .agent/audit.jsonl                     — append audit event
//  6. .agent/run_log.jsonl                   — append run log event
//  7. Session event (phase_completed)        — via caller
package gate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"keel/internal/audit"
	"keel/internal/rollback"
	"keel/internal/snapshot"
)

// ExitCriterionResult records whether a single exit criterion passed or failed.
type ExitCriterionResult struct {
	Criterion string `json:"criterion"`
	Passed    bool   `json:"passed"`
	Detail    string `json:"detail,omitempty"`
}

// GateRecord is the canonical schema for .agent/phase_gates/phase_N.gate.json.
type GateRecord struct {
	Phase              int                   `json:"phase"`
	Name               string                `json:"name"`
	Status             string                `json:"status"` // "passed" or "failed"
	TimestampUTC       string                `json:"timestamp_utc"`
	GitSHABefore       string                `json:"git_sha_before"`
	GitSHAAfter        string                `json:"git_sha_after"`
	FilesChanged       []string              `json:"files_changed"`
	FilesCreated       []string              `json:"files_created,omitempty"`
	FilesModified      []string              `json:"files_modified,omitempty"`
	Commands           []string              `json:"commands"`
	ExitCriteriaResults []ExitCriterionResult `json:"exit_criteria_results"`
	SelfVerification   string                `json:"self_verification"` // "pass" or "fail"
	Agent              string                `json:"agent,omitempty"`
	Model              string                `json:"model,omitempty"`
	Summary            string                `json:"summary,omitempty"`
}

// CloseInput is the data needed to close a phase.
type CloseInput struct {
	Phase    int
	RepoPath string

	// Optional overrides — if empty, derived automatically.
	Name         string
	GitSHABefore string
	GitSHAAfter  string
	Agent        string
	Model        string
	Summary      string
	Commands     []string
	FilesChanged []string

	ExitCriteriaResults []ExitCriterionResult
}

// CloseResult holds paths of artifacts written.
type CloseResult struct {
	GatePath    string
	RollbackPath string
	LedgerPath  string
	AuditLogPath string
}

// Close writes all phase-closing artifacts. Returns an error if any
// required artifact cannot be written.
func Close(in CloseInput) (*CloseResult, error) {
	repo := in.RepoPath
	if repo == "" {
		repo = "."
	}
	repo, err := filepath.Abs(repo)
	if err != nil {
		return nil, fmt.Errorf("resolve repo: %w", err)
	}

	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)

	// Derive name from manifest if not provided.
	name := in.Name
	if name == "" {
		name = readPhaseName(repo, in.Phase)
	}
	if name == "" {
		name = fmt.Sprintf("Phase %d", in.Phase)
	}

	// Derive git SHAs if not provided.
	gitBefore := in.GitSHABefore
	if gitBefore == "" {
		gitBefore = readSnapshotSHA(repo, in.Phase)
	}
	gitAfter := in.GitSHAAfter
	if gitAfter == "" {
		gitAfter = currentGitSHA(repo)
	}

	// Derive files changed using manifest diff (preferred) or git diff (fallback).
	var filesChanged, filesCreated, filesModified []string

	if len(in.FilesChanged) > 0 {
		filesChanged = in.FilesChanged
	} else {
		// Try manifest-based diff first.
		preManifest, preErr := snapshot.Read(repo, in.Phase, "pre")
		if preErr == nil {
			postManifest, postErr := snapshot.Capture(repo, in.Phase, "post")
			if postErr == nil {
				// Write post manifest for audit trail.
				snapshot.Write(repo, postManifest)

				diff := snapshot.Diff(preManifest, postManifest)
				filesCreated = diff.Created
				filesModified = diff.Modified
				// Combine for backward-compatible files_changed field.
				filesChanged = make([]string, 0, len(diff.Created)+len(diff.Modified))
				filesChanged = append(filesChanged, diff.Created...)
				filesChanged = append(filesChanged, diff.Modified...)
			}
		}
		// Fallback to git diff if no manifest available.
		if len(filesChanged) == 0 && gitBefore != "" && gitAfter != "" {
			filesChanged = gitDiffFiles(repo, gitBefore, gitAfter)
		}
	}

	// Determine pass/fail from exit criteria.
	status := "passed"
	selfVerification := "pass"
	for _, ec := range in.ExitCriteriaResults {
		if !ec.Passed {
			status = "failed"
			selfVerification = "fail"
			break
		}
	}

	gate := GateRecord{
		Phase:              in.Phase,
		Name:               name,
		Status:             status,
		TimestampUTC:       now,
		GitSHABefore:       gitBefore,
		GitSHAAfter:        gitAfter,
		FilesChanged:       filesChanged,
		FilesCreated:       filesCreated,
		FilesModified:      filesModified,
		Commands:           in.Commands,
		ExitCriteriaResults: in.ExitCriteriaResults,
		SelfVerification:   selfVerification,
		Agent:              in.Agent,
		Model:              in.Model,
		Summary:            in.Summary,
	}

	var result CloseResult

	// 1. Write gate JSON.
	gatePath, err := writeGate(repo, gate)
	if err != nil {
		return nil, fmt.Errorf("write gate: %w", err)
	}
	result.GatePath = gatePath

	// 2. Build and write rollback DAG.
	dag := buildRollbackDAG(gate)
	if wErr := rollback.WriteDAG(repo, dag); wErr == nil {
		result.RollbackPath = filepath.Join(".agent", "snapshots",
			fmt.Sprintf("phase_%d.rollback.json", in.Phase))
	}

	// 3. Write build ledger.
	ledgerPath, err := writeLedger(repo, gate)
	if err != nil {
		return nil, fmt.Errorf("write ledger: %w", err)
	}
	result.LedgerPath = ledgerPath

	// 4. Write human-readable audit log.
	auditLogPath, err := writeAuditLog(repo, gate)
	if err != nil {
		return nil, fmt.Errorf("write audit log: %w", err)
	}
	result.AuditLogPath = auditLogPath

	// 5. Append to .agent/audit.jsonl.
	phaseNum := in.Phase
	audit.Append(repo, audit.Record{
		EventType:    audit.EvtPhasePassed,
		TimestampUTC: now,
		Phase:        &phaseNum,
		Metadata: map[string]interface{}{
			"status":           status,
			"gate_path":        result.GatePath,
			"files_changed":    len(filesChanged),
			"self_verification": selfVerification,
		},
	})

	// 6. Append to .agent/run_log.jsonl.
	appendRunLog(repo, gate)

	return &result, nil
}

// ---------------------------------------------------------------------------
// Gate file
// ---------------------------------------------------------------------------

func writeGate(repo string, gate GateRecord) (string, error) {
	dir := filepath.Join(repo, ".agent", "phase_gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	suffix := "gate.json"
	if gate.Status == "failed" {
		suffix = "failed.json"
	}
	rel := filepath.Join(".agent", "phase_gates", fmt.Sprintf("phase_%d.%s", gate.Phase, suffix))
	path := filepath.Join(repo, rel)

	data, err := json.MarshalIndent(gate, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return rel, nil
}

// ---------------------------------------------------------------------------
// Build ledger
// ---------------------------------------------------------------------------

func writeLedger(repo string, gate GateRecord) (string, error) {
	dir := filepath.Join(repo, "docs", "build-ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	rel := filepath.Join("docs", "build-ledger", fmt.Sprintf("phase_%d_build.md", gate.Phase))
	path := filepath.Join(repo, rel)

	var b strings.Builder
	fmt.Fprintf(&b, "# Phase %d Build Ledger — %s\n\n", gate.Phase, gate.Name)
	fmt.Fprintf(&b, "## Status\n%s\n\n", gate.Status)
	fmt.Fprintf(&b, "## Timestamp\n%s\n\n", gate.TimestampUTC)

	if gate.Agent != "" || gate.Model != "" {
		fmt.Fprintf(&b, "## Agent\n")
		if gate.Agent != "" {
			fmt.Fprintf(&b, "- Agent: %s\n", gate.Agent)
		}
		if gate.Model != "" {
			fmt.Fprintf(&b, "- Model: %s\n", gate.Model)
		}
		fmt.Fprintln(&b)
	}

	if gate.Summary != "" {
		fmt.Fprintf(&b, "## Summary\n%s\n\n", gate.Summary)
	}

	fmt.Fprintf(&b, "## Git\n- Before: %s\n- After: %s\n\n", gate.GitSHABefore, gate.GitSHAAfter)

	if len(gate.FilesChanged) > 0 {
		fmt.Fprintf(&b, "## Files Changed (%d)\n", len(gate.FilesChanged))
		for _, f := range gate.FilesChanged {
			fmt.Fprintf(&b, "- %s\n", f)
		}
		fmt.Fprintln(&b)
	}

	if len(gate.Commands) > 0 {
		fmt.Fprintf(&b, "## Commands\n")
		for _, c := range gate.Commands {
			fmt.Fprintf(&b, "- `%s`\n", c)
		}
		fmt.Fprintln(&b)
	}

	if len(gate.ExitCriteriaResults) > 0 {
		fmt.Fprintf(&b, "## Exit Criteria\n")
		for _, ec := range gate.ExitCriteriaResults {
			mark := "PASS"
			if !ec.Passed {
				mark = "FAIL"
			}
			fmt.Fprintf(&b, "- [%s] %s\n", mark, ec.Criterion)
		}
		fmt.Fprintln(&b)
	}

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return rel, nil
}

// ---------------------------------------------------------------------------
// Human-readable audit log
// ---------------------------------------------------------------------------

func writeAuditLog(repo string, gate GateRecord) (string, error) {
	dir := filepath.Join(repo, "docs", "audit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	rel := filepath.Join("docs", "audit", fmt.Sprintf("phase_%d.log", gate.Phase))
	path := filepath.Join(repo, rel)

	var b strings.Builder
	fmt.Fprintf(&b, "Phase %d Audit Log — %s\n", gate.Phase, gate.Name)
	fmt.Fprintf(&b, "Status: %s\n", gate.Status)
	fmt.Fprintf(&b, "Timestamp: %s\n", gate.TimestampUTC)

	if gate.Agent != "" {
		fmt.Fprintf(&b, "Agent: %s\n", gate.Agent)
	}
	if gate.Model != "" {
		fmt.Fprintf(&b, "Model: %s\n", gate.Model)
	}

	fmt.Fprintf(&b, "Git SHA before: %s\n", gate.GitSHABefore)
	fmt.Fprintf(&b, "Git SHA after: %s\n", gate.GitSHAAfter)
	fmt.Fprintf(&b, "Self-verification: %s\n", gate.SelfVerification)

	if len(gate.Commands) > 0 {
		fmt.Fprintf(&b, "\nVerification commands:\n")
		for _, c := range gate.Commands {
			fmt.Fprintf(&b, "  %s\n", c)
		}
	}

	if len(gate.ExitCriteriaResults) > 0 {
		fmt.Fprintf(&b, "\nExit criteria:\n")
		for _, ec := range gate.ExitCriteriaResults {
			mark := "PASS"
			if !ec.Passed {
				mark = "FAIL"
			}
			fmt.Fprintf(&b, "  [%s] %s\n", mark, ec.Criterion)
		}
	}

	if len(gate.FilesChanged) > 0 {
		fmt.Fprintf(&b, "\nFiles changed (%d):\n", len(gate.FilesChanged))
		for _, f := range gate.FilesChanged {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return rel, nil
}

// ---------------------------------------------------------------------------
// Run log (.agent/run_log.jsonl)
// ---------------------------------------------------------------------------

type runLogEntry struct {
	EventType    string `json:"event_type"`
	TimestampUTC string `json:"timestamp_utc"`
	Phase        int    `json:"phase"`
	Status       string `json:"status"`
	GatePath     string `json:"gate_path"`
}

func appendRunLog(repo string, gate GateRecord) {
	path := filepath.Join(repo, ".agent", "run_log.jsonl")
	entry := runLogEntry{
		EventType:    "phase.closed",
		TimestampUTC: gate.TimestampUTC,
		Phase:        gate.Phase,
		Status:       gate.Status,
		GatePath: filepath.Join(".agent", "phase_gates",
			fmt.Sprintf("phase_%d.gate.json", gate.Phase)),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}

// ---------------------------------------------------------------------------
// Helpers — read phase name from manifest, git operations
// ---------------------------------------------------------------------------

func readPhaseName(repo string, phase int) string {
	// Simple regex-based parse of BUILD_MANIFEST.yaml to get phase name.
	data, err := os.ReadFile(filepath.Join(repo, "BUILD_MANIFEST.yaml"))
	if err != nil {
		return ""
	}
	// Look for "- phase: N" followed by "name: ..."
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == fmt.Sprintf("- phase: %d", phase) || trimmed == fmt.Sprintf("phase: %d", phase) {
			// Scan forward for name
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				nameLine := strings.TrimSpace(lines[j])
				if strings.HasPrefix(nameLine, "name:") {
					return strings.TrimSpace(strings.TrimPrefix(nameLine, "name:"))
				}
			}
		}
	}
	return ""
}

func readSnapshotSHA(repo string, phase int) string {
	// Check if there's a pre-phase snapshot with the git SHA.
	// Convention: .agent/snapshots/phase_N.pre.sha
	path := filepath.Join(repo, ".agent", "snapshots", fmt.Sprintf("phase_%d.pre.sha", phase))
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func currentGitSHA(repo string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitDiffFiles(repo, before, after string) []string {
	cmd := exec.Command("git", "diff", "--name-only", before, after)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, l)
		}
	}
	return files
}

// ---------------------------------------------------------------------------
// Rollback DAG builder — uses created/modified distinction from gate
// ---------------------------------------------------------------------------

func buildRollbackDAG(gate GateRecord) rollback.RollbackDAG {
	gitResetTo := gate.GitSHABefore
	if gitResetTo == "" {
		gitResetTo = "unknown"
	}

	// Build deliverables from exit criteria.
	deliverables := make([]string, 0, len(gate.ExitCriteriaResults))
	for _, ec := range gate.ExitCriteriaResults {
		if ec.Criterion != "" {
			deliverables = append(deliverables, ec.Criterion)
		}
	}

	dag := rollback.RollbackDAG{
		Phase:        gate.Phase,
		Name:         gate.Name,
		GitSHABefore: gate.GitSHABefore,
		GitSHAAfter:  gate.GitSHAAfter,
		TimestampUTC: gate.TimestampUTC,
		Deliverables: deliverables,
		FilesCreated: gate.FilesCreated,
		FilesModified: gate.FilesModified,
	}

	// Rollback instructions: only delete created files.
	// Modified files are restored from git, not deleted.
	dag.Rollback.GitResetTo = gitResetTo
	dag.Rollback.FilesToDelete = gate.FilesCreated
	dag.Rollback.FilesToRestore = gate.FilesModified

	return dag
}
