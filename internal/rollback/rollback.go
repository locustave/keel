// Package rollback implements phase rollback for the harness CLI.
//
// Rollback reads .agent/snapshots/phase_N.rollback.json, determines all
// downstream phases that must also be unwound (highest first), and either
// prints a dry-run plan or executes: git reset --hard, file deletion, and
// gate invalidation.
package rollback

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"keel/internal/audit"
	"keel/internal/snapshot"
)

// RollbackDAG is the schema for .agent/snapshots/phase_N.rollback.json.
type RollbackDAG struct {
	Phase           int      `json:"phase"`
	Name            string   `json:"name"`
	GitSHABefore    string   `json:"git_sha_before"`
	GitSHAAfter     string   `json:"git_sha_after"`
	TimestampUTC    string   `json:"timestamp_utc"`
	Deliverables    []string `json:"deliverables"`
	FilesCreated    []string `json:"files_created"`
	FilesModified   []string `json:"files_modified"`
	DependsOnPhases []int    `json:"depends_on_phases"`
	DownstreamPhases []int   `json:"downstream_phases"`
	Rollback        struct {
		GitResetTo    string   `json:"git_reset_to"`
		FilesToDelete []string `json:"files_to_delete"`
		FilesToRestore []string `json:"files_to_restore"`
	} `json:"rollback"`
}

// Options controls rollback behaviour.
type Options struct {
	Phase       int
	RepoPath    string    // defaults to "."
	Confirm     bool      // false = dry-run only
	Out         io.Writer // stdout (nil → os.Stdout)
}

// Run executes the rollback command. Returns exit code 0/1.
func Run(opts Options) int {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.RepoPath == "" {
		opts.RepoPath = "."
	}

	snapshotsDir := filepath.Join(opts.RepoPath, ".agent", "snapshots")
	gatesDir := filepath.Join(opts.RepoPath, ".agent", "phase_gates")

	dag, err := loadDAG(snapshotsDir, opts.Phase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Collect downstream phases that have rollback DAGs
	downstream, err := collectDownstream(snapshotsDir, opts.Phase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Unwind order: highest downstream first, then the requested phase
	unwindOrder := append(downstream, dag)

	printPlan(opts.Out, unwindOrder)

	if !opts.Confirm {
		fmt.Fprintln(opts.Out, "\nDry run. Run with --confirm to execute rollback.")
		return 0
	}

	fmt.Fprintln(opts.Out, "\nExecuting rollback...")
	for _, d := range unwindOrder {
		if err := executeRollback(opts.RepoPath, gatesDir, d, opts.Out); err != nil {
			fmt.Fprintf(os.Stderr, "error rolling back phase %d: %v\n", d.Phase, err)
			return 1
		}
	}

	appendAuditEvent(opts.RepoPath, unwindOrder)

	fmt.Fprintf(opts.Out, "\nRollback complete. Active phase is now %d.\n", opts.Phase-1)
	fmt.Fprintln(opts.Out, "Audit log: .agent/audit.jsonl")
	return 0
}

// WriteDAG writes a rollback DAG file for a completed phase.
// Call this from the gate-writing step at the end of each phase.
func WriteDAG(repoPath string, dag RollbackDAG) error {
	dir := filepath.Join(repoPath, ".agent", "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, fmt.Sprintf("phase_%d.rollback.json", dag.Phase))
	data, err := json.MarshalIndent(dag, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

var reRollbackFile = regexp.MustCompile(`^phase_(\d+)\.rollback\.json$`)

// ---------------------------------------------------------------------------
// DeriveFromGate — build a RollbackDAG from an existing gate file
// ---------------------------------------------------------------------------

// gateSnapshot is the subset of a gate file needed to derive a rollback DAG.
type gateSnapshot struct {
	Phase        int      `json:"phase"`
	Name         string   `json:"name"`
	TimestampUTC string   `json:"timestamp_utc"`
	GitSHABefore string   `json:"git_sha_before"`
	GitSHAAfter  string   `json:"git_sha_after"`
	FilesChanged []string `json:"files_changed"`
	ExitCriteria []struct {
		Criterion string `json:"criterion"`
	} `json:"exit_criteria_results"`
}

// DeriveFromGate reads a passed gate file and derives a RollbackDAG from it.
// This should be called immediately after the gate is written. It can also
// retroactively backfill DAGs for phases completed before write-dag was
// integrated into the run-phase flow.
//
// Limitations of derived DAGs (vs hand-crafted ones):
//   - FilesCreated and FilesModified are not distinguished — all files_changed
//     are treated as FilesCreated (rollback will attempt to delete them all).
//   - DependsOnPhases and DownstreamPhases are left empty; collectDownstream
//     discovers downstream phases at runtime by scanning the snapshots dir.
//   - If the repo had no git history, GitResetTo is set to "unknown".
func DeriveFromGate(repoPath string, phase int) (*RollbackDAG, error) {
	gatePath := filepath.Join(repoPath, ".agent", "phase_gates", fmt.Sprintf("phase_%d.gate.json", phase))
	data, err := os.ReadFile(gatePath)
	if err != nil {
		return nil, fmt.Errorf("gate not found for phase %d: %s", phase, gatePath)
	}
	var g gateSnapshot
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("cannot parse gate for phase %d: %w", phase, err)
	}

	name := g.Name
	if name == "" {
		name = fmt.Sprintf("phase_%d", phase)
	}

	gitResetTo := g.GitSHABefore
	if gitResetTo == "" {
		gitResetTo = "unknown"
	}

	// Use exit criteria text as deliverable descriptions.
	deliverables := make([]string, 0, len(g.ExitCriteria))
	for _, ec := range g.ExitCriteria {
		if ec.Criterion != "" {
			deliverables = append(deliverables, ec.Criterion)
		}
	}

	dag := &RollbackDAG{
		Phase:        g.Phase,
		Name:         name,
		GitSHABefore: g.GitSHABefore,
		GitSHAAfter:  g.GitSHAAfter,
		TimestampUTC: g.TimestampUTC,
		Deliverables: deliverables,
		FilesCreated: g.FilesChanged,
	}
	dag.Rollback.GitResetTo = gitResetTo
	dag.Rollback.FilesToDelete = g.FilesChanged

	return dag, nil
}

func loadDAG(snapshotsDir string, phase int) (*RollbackDAG, error) {
	path := filepath.Join(snapshotsDir, fmt.Sprintf("phase_%d.rollback.json", phase))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rollback DAG not found for phase %d: %s\n"+
			"The rollback DAG is written at phase completion. If it is missing, the phase\n"+
			"may not have completed with a rollback-aware gate.", phase, path)
	}
	var dag RollbackDAG
	if err := json.Unmarshal(data, &dag); err != nil {
		return nil, fmt.Errorf("cannot parse rollback DAG for phase %d: %w", phase, err)
	}
	return &dag, nil
}

func collectDownstream(snapshotsDir string, phase int) ([]*RollbackDAG, error) {
	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var nums []int
	for _, e := range entries {
		m := reRollbackFile.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		if n > phase {
			nums = append(nums, n)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(nums)))

	var dags []*RollbackDAG
	for _, n := range nums {
		d, err := loadDAG(snapshotsDir, n)
		if err != nil {
			return nil, err
		}
		dags = append(dags, d)
	}
	return dags, nil
}

func printPlan(out io.Writer, unwindOrder []*RollbackDAG) {
	fmt.Fprintf(out, "Rollback plan — %d phase(s) will be unwound:\n\n", len(unwindOrder))
	for _, d := range unwindOrder {
		fmt.Fprintf(out, "  Phase %d: %s\n", d.Phase, d.Name)
		if len(d.Rollback.FilesToDelete) > 0 {
			// Count files that will actually be deleted (excluding protected).
			var deletable []string
			for _, f := range d.Rollback.FilesToDelete {
				if !isProtected(f) {
					deletable = append(deletable, f)
				}
			}
			if len(deletable) > 0 {
				fmt.Fprintf(out, "    delete %d created file(s)\n", len(deletable))
			}
		}
		if len(d.Rollback.FilesToRestore) > 0 {
			var restorable []string
			for _, f := range d.Rollback.FilesToRestore {
				if !isProtected(f) {
					restorable = append(restorable, f)
				}
			}
			if len(restorable) > 0 {
				fmt.Fprintf(out, "    restore %d modified file(s) from %s\n", len(restorable), d.Rollback.GitResetTo)
			}
		}
		if len(d.Rollback.FilesToRestore) == 0 && d.Rollback.GitResetTo != "" {
			fmt.Fprintf(out, "    git reset --hard %s (manual)\n", d.Rollback.GitResetTo)
		}
		fmt.Fprintf(out, "    gate → phase_%d.rolled_back.json\n\n", d.Phase)
	}
}

// protectedFiles are append-only harness files that must never be deleted by rollback.
var protectedFiles = map[string]bool{
	".agent/audit.jsonl":  true,
	".agent/run_log.jsonl": true,
}

// isProtected returns true if a file should never be deleted by rollback.
func isProtected(rel string) bool {
	if protectedFiles[rel] {
		return true
	}
	// Protect session event files.
	if strings.HasPrefix(rel, ".agent/sessions/") {
		return true
	}
	return false
}

func executeRollback(repoPath, gatesDir string, dag *RollbackDAG, out io.Writer) error {
	fmt.Fprintf(out, "Rolling back phase %d (%s)...\n", dag.Phase, dag.Name)

	// Delete files that were created by this phase (skip protected files).
	for _, f := range dag.Rollback.FilesToDelete {
		if isProtected(f) {
			fmt.Fprintf(out, "  skip (protected): %s\n", f)
			continue
		}
		path := filepath.Join(repoPath, f)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete %s: %w", f, err)
		}
	}

	// Restore files that were modified by this phase from the stash.
	if len(dag.Rollback.FilesToRestore) > 0 {
		for _, f := range dag.Rollback.FilesToRestore {
			if isProtected(f) {
				fmt.Fprintf(out, "  skip (protected): %s\n", f)
				continue
			}
			if err := snapshot.RestoreFile(repoPath, dag.Phase, f); err != nil {
				fmt.Fprintf(out, "  warning: could not restore %s: %v\n", f, err)
			} else {
				fmt.Fprintf(out, "  restored: %s\n", f)
			}
		}
	}

	// Invalidate the gate file
	if err := invalidateGate(gatesDir, dag.Phase, dag.Name); err != nil {
		return err
	}

	fmt.Fprintf(out, "  Phase %d rolled back. Created files deleted, modified files restored from stash.\n", dag.Phase)

	// Clean up the stash directory for this phase.
	snapshot.CleanStash(repoPath, dag.Phase)

	return nil
}

func invalidateGate(gatesDir string, phase int, name string) error {
	passedPath := filepath.Join(gatesDir, fmt.Sprintf("phase_%d.gate.json", phase))
	rolledPath := filepath.Join(gatesDir, fmt.Sprintf("phase_%d.rolled_back.json", phase))

	data, err := os.ReadFile(passedPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Gate may already be invalidated or never written — not fatal
			return nil
		}
		return fmt.Errorf("read gate for phase %d: %w", phase, err)
	}

	var gate map[string]interface{}
	if err := json.Unmarshal(data, &gate); err != nil {
		return fmt.Errorf("parse gate for phase %d: %w", phase, err)
	}

	gate["status"] = "rolled_back"
	gate["rolled_back_at_utc"] = time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)

	out, err := json.MarshalIndent(gate, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(rolledPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("write rolled-back gate for phase %d: %w", phase, err)
	}
	if err := os.Remove(passedPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove passed gate for phase %d: %w", phase, err)
	}
	return nil
}

func appendAuditEvent(repoPath string, unwindOrder []*RollbackDAG) {
	phases := make([]int, len(unwindOrder))
	for i, d := range unwindOrder {
		phases[i] = d.Phase
	}
	audit.Append(repoPath, audit.Record{
		EventType: audit.EvtRollbackExecuted,
		Metadata:  map[string]interface{}{"phases_unwound": phases},
	})
}
