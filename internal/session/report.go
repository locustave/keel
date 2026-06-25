package session

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Session report
// ---------------------------------------------------------------------------

// SessionReport generates a human-readable session report from the event ledger.
func SessionReport(sessDir string, out io.Writer) error {
	// Load session meta.
	var meta SessionMeta
	if err := readJSON(filepath.Join(sessDir, "session.json"), &meta); err != nil {
		return fmt.Errorf("read session.json: %w", err)
	}

	// Load events.
	events, err := ReadEvents(filepath.Join(sessDir, "events.jsonl"))
	if err != nil {
		return err
	}

	state := ReduceState(meta.SessionID, events)
	metrics := ReduceMetrics(meta.SessionID, events)

	fmt.Fprintf(out, "Session Report\n")
	fmt.Fprintf(out, "==============\n")
	fmt.Fprintf(out, "Session ID     : %s\n", meta.SessionID)
	fmt.Fprintf(out, "Status         : %s\n", state.SessionLifecycle)
	fmt.Fprintf(out, "Created at     : %s\n", meta.CreatedAt)
	if meta.EndedAt != "" {
		fmt.Fprintf(out, "Ended at       : %s\n", meta.EndedAt)
		if d := sessionDuration(meta.CreatedAt, meta.EndedAt); d != "" {
			fmt.Fprintf(out, "Duration       : %s\n", d)
		}
	}
	fmt.Fprintf(out, "Session owner  : %s (%s)\n", meta.SessionOwner.Name, meta.SessionOwner.UserID)
	fmt.Fprintf(out, "Created by     : %s [%s]\n", meta.CreatedBy.ActorName, meta.CreatedBy.ActorType)
	if meta.Host.Hostname != "" {
		fmt.Fprintf(out, "Host           : %s\n", meta.Host.Hostname)
	}
	fmt.Fprintf(out, "Working dir    : %s\n", meta.Host.WorkingDirectory)
	fmt.Fprintf(out, "\n")

	fmt.Fprintf(out, "Current phase  : %s\n", orDash(state.CurrentPhase))
	fmt.Fprintf(out, "Activity       : %s\n", orDash(state.CurrentActivity))
	if state.ActiveImplementer.ImplementerName != nil {
		fmt.Fprintf(out, "Implementer    : %s [%s]\n",
			*state.ActiveImplementer.ImplementerName, state.ActiveImplementer.ImplementerType)
	}
	fmt.Fprintf(out, "\n")

	fmt.Fprintf(out, "Phases\n------\n")
	fmt.Fprintf(out, "  Completed    : %d\n", metrics.PhasesCompleted)
	fmt.Fprintf(out, "  Failed       : %d\n", metrics.PhasesFailed)
	fmt.Fprintf(out, "\n")

	fmt.Fprintf(out, "Activity\n--------\n")
	fmt.Fprintf(out, "  Tool calls   : %d (failures: %d)\n", metrics.ToolCalls, metrics.ToolFailures)
	fmt.Fprintf(out, "  File edits   : %d\n", metrics.FileEdits)
	fmt.Fprintf(out, "  Debug loops  : %d\n", metrics.DebugLoops)
	fmt.Fprintf(out, "  Tests passed : %d\n", metrics.TestsPassed)
	fmt.Fprintf(out, "  Tests failed : %d\n", metrics.TestsFailed)
	fmt.Fprintf(out, "\n")

	fmt.Fprintf(out, "Tokens\n------\n")
	fmt.Fprintf(out, "  Total        : %d\n", metrics.TotalTokens)
	fmt.Fprintf(out, "  Exact        : %d\n", metrics.ExactTokens)
	fmt.Fprintf(out, "  Estimated    : %d\n", metrics.EstimatedTokens)
	fmt.Fprintf(out, "  LLM calls    : %d\n", metrics.LLMCalls)
	if len(metrics.TokensByImplementer) > 0 {
		fmt.Fprintf(out, "  By implementer:\n")
		for _, kv := range sortedMap(metrics.TokensByImplementer) {
			fmt.Fprintf(out, "    %-30s %d\n", kv.key, kv.val)
		}
	}
	fmt.Fprintf(out, "\n")

	fmt.Fprintf(out, "Events by actor type\n--------------------\n")
	for _, kv := range sortedMap(metrics.EventsByActorType) {
		fmt.Fprintf(out, "  %-20s %d\n", kv.key, kv.val)
	}
	fmt.Fprintf(out, "\n")

	fmt.Fprintf(out, "Events by implementer\n---------------------\n")
	for _, kv := range sortedMap(metrics.EventsByImplementer) {
		fmt.Fprintf(out, "  %-30s %d\n", kv.key, kv.val)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Phase report
// ---------------------------------------------------------------------------

// PhaseReport generates a phase-scoped report from the event ledger.
func PhaseReport(sessDir, phaseID string, out io.Writer) error {
	events, err := ReadEvents(filepath.Join(sessDir, "events.jsonl"))
	if err != nil {
		return err
	}

	var phaseEvents []EventRecord
	for _, e := range events {
		if e.PhaseID == phaseID {
			phaseEvents = append(phaseEvents, e)
		}
	}

	tokens := ReducePhaseTokens(phaseID, events)
	phaseMetrics := ReduceMetrics(phaseID, phaseEvents)
	phaseState := ReduceState(phaseID, phaseEvents)

	startTime := ""
	endTime := ""
	implementerName := "unknown"
	startedBy := "unknown"

	for _, e := range phaseEvents {
		switch e.EventType {
		case EvtPhaseStarted:
			startTime = e.Timestamp
			startedBy = e.CommandUser.Name
			if e.Implementer.ImplementerName != nil {
				implementerName = *e.Implementer.ImplementerName
			}
		case EvtPhaseCompleted, EvtPhaseFailed:
			endTime = e.Timestamp
		}
	}

	status := phaseState.SessionLifecycle
	if status == "" {
		status = "unknown"
	}

	fmt.Fprintf(out, "Phase Report: %s\n", phaseID)
	fmt.Fprintf(out, "%s\n\n", strings.Repeat("=", 20+len(phaseID)))
	fmt.Fprintf(out, "Status         : %s\n", status)
	fmt.Fprintf(out, "Started at     : %s\n", orDash(startTime))
	fmt.Fprintf(out, "Completed at   : %s\n", orDash(endTime))
	if startTime != "" && endTime != "" {
		fmt.Fprintf(out, "Duration       : %s\n", sessionDuration(startTime, endTime))
	}
	fmt.Fprintf(out, "Implementer    : %s\n", implementerName)
	fmt.Fprintf(out, "Started by     : %s\n", startedBy)
	fmt.Fprintf(out, "\n")

	fmt.Fprintf(out, "Activity\n--------\n")
	fmt.Fprintf(out, "  Tool calls   : %d (failures: %d)\n", phaseMetrics.ToolCalls, phaseMetrics.ToolFailures)
	fmt.Fprintf(out, "  File edits   : %d\n", phaseMetrics.FileEdits)
	fmt.Fprintf(out, "  Debug loops  : %d\n", phaseMetrics.DebugLoops)
	fmt.Fprintf(out, "  Tests passed : %d\n", phaseMetrics.TestsPassed)
	fmt.Fprintf(out, "  Tests failed : %d\n", phaseMetrics.TestsFailed)
	fmt.Fprintf(out, "\n")

	fmt.Fprintf(out, "Tokens\n------\n")
	fmt.Fprintf(out, "  Total        : %d\n", tokens.TotalTokens)
	fmt.Fprintf(out, "  Exact        : %d\n", tokens.ExactTokens)
	fmt.Fprintf(out, "  Estimated    : %d\n", tokens.EstimatedTokens)
	fmt.Fprintf(out, "  LLM calls    : %d\n", tokens.LLMCalls)
	if len(tokens.TokensByImplementer) > 0 {
		fmt.Fprintf(out, "  By implementer:\n")
		for _, kv := range sortedMap(tokens.TokensByImplementer) {
			fmt.Fprintf(out, "    %-30s %d\n", kv.key, kv.val)
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Token reports
// ---------------------------------------------------------------------------

// TokenReport prints a token usage report grouped by the given dimension.
func TokenReport(sessDir, groupBy string, out io.Writer) error {
	events, err := ReadEvents(filepath.Join(sessDir, "events.jsonl"))
	if err != nil {
		return err
	}

	metrics := ReduceMetrics("", events)

	fmt.Fprintf(out, "Token Usage Report\n==================\n")
	fmt.Fprintf(out, "Total tokens   : %d\n", metrics.TotalTokens)
	fmt.Fprintf(out, "Exact tokens   : %d\n", metrics.ExactTokens)
	fmt.Fprintf(out, "Estimated      : %d\n", metrics.EstimatedTokens)
	fmt.Fprintf(out, "LLM calls      : %d\n\n", metrics.LLMCalls)

	switch groupBy {
	case "implementer":
		fmt.Fprintf(out, "By implementer:\n")
		for _, kv := range sortedMap(metrics.TokensByImplementer) {
			fmt.Fprintf(out, "  %-30s %d\n", kv.key, kv.val)
		}
	case "actor":
		fmt.Fprintf(out, "By actor:\n")
		for _, kv := range sortedMap(metrics.TokensByActor) {
			fmt.Fprintf(out, "  %-30s %d\n", kv.key, kv.val)
		}
	case "phase":
		// Compute per-phase breakdown.
		phases := make(map[string]bool)
		for _, e := range events {
			if e.PhaseID != "" {
				phases[e.PhaseID] = true
			}
		}
		phaseIDs := make([]string, 0, len(phases))
		for p := range phases {
			phaseIDs = append(phaseIDs, p)
		}
		sort.Strings(phaseIDs)
		fmt.Fprintf(out, "By phase:\n")
		for _, pid := range phaseIDs {
			pt := ReducePhaseTokens(pid, events)
			fmt.Fprintf(out, "  %-20s %d (exact: %d, estimated: %d)\n",
				pid, pt.TotalTokens, pt.ExactTokens, pt.EstimatedTokens)
		}
	default:
		// No grouping — just the summary already printed.
	}

	return nil
}

// ---------------------------------------------------------------------------
// Tool report
// ---------------------------------------------------------------------------

// ToolReport prints a tool usage report.
func ToolReport(sessDir string, out io.Writer) error {
	events, err := ReadEvents(filepath.Join(sessDir, "events.jsonl"))
	if err != nil {
		return err
	}

	type toolStats struct {
		calls    int
		failures int
	}
	tools := make(map[string]*toolStats)

	for _, e := range events {
		if e.ToolName == "" {
			continue
		}
		if _, ok := tools[e.ToolName]; !ok {
			tools[e.ToolName] = &toolStats{}
		}
		switch e.EventType {
		case EvtToolCompleted:
			tools[e.ToolName].calls++
		case EvtToolFailed:
			tools[e.ToolName].calls++
			tools[e.ToolName].failures++
		}
	}

	fmt.Fprintf(out, "Tool Usage Report\n=================\n")
	names := make([]string, 0, len(tools))
	for n := range tools {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		s := tools[n]
		fmt.Fprintf(out, "  %-20s calls: %d  failures: %d\n", n, s.calls, s.failures)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Debug report
// ---------------------------------------------------------------------------

// DebugReport prints a debug-loop report.
func DebugReport(sessDir string, out io.Writer) error {
	events, err := ReadEvents(filepath.Join(sessDir, "events.jsonl"))
	if err != nil {
		return err
	}

	loops := 0
	completed := 0
	for _, e := range events {
		switch e.EventType {
		case EvtDebugStarted:
			loops++
		case EvtDebugCompleted:
			completed++
		}
	}

	fmt.Fprintf(out, "Debug Loop Report\n=================\n")
	fmt.Fprintf(out, "  Debug loops started   : %d\n", loops)
	fmt.Fprintf(out, "  Debug loops completed : %d\n", completed)
	fmt.Fprintf(out, "  Still in progress     : %d\n", loops-completed)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// FindCurrentSessionDir locates the active session directory for a repo.
func FindCurrentSessionDir(repoPath string) (string, error) {
	idPath := filepath.Join(repoPath, ".agent", "keel", "current-session")
	data, err := os.ReadFile(idPath)
	if err != nil {
		return "", fmt.Errorf("no active session: %w", err)
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", fmt.Errorf("no active session")
	}
	dir := filepath.Join(repoPath, ".agent", "sessions", id)
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("session directory not found: %s", dir)
	}
	return dir, nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func sessionDuration(start, end string) string {
	s, err1 := time.Parse(time.RFC3339, start)
	e, err2 := time.Parse(time.RFC3339, end)
	if err1 != nil || err2 != nil {
		return ""
	}
	return e.Sub(s).Round(time.Second).String()
}

type kv struct {
	key string
	val int
}

func sortedMap(m map[string]int) []kv {
	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].val != pairs[j].val {
			return pairs[i].val > pairs[j].val
		}
		return pairs[i].key < pairs[j].key
	})
	return pairs
}
