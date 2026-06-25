package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Event types (controlled vocabulary)
// ---------------------------------------------------------------------------

const (
	// Session lifecycle
	EvtSessionStarted   = "session_started"
	EvtSessionCompleted = "session_completed"

	// Command lifecycle
	EvtCommandStarted   = "keel_command_started"
	EvtCommandCompleted = "keel_command_completed"
	EvtCommandFailed    = "keel_command_failed"

	// Phase lifecycle
	EvtPhaseStarted  = "phase_started"
	EvtPhaseCompleted = "phase_completed"
	EvtPhaseFailed   = "phase_failed"
	EvtPhaseBlocked  = "phase_blocked"

	// Preflight / verification
	EvtPreflightStarted  = "preflight_started"
	EvtPreflightPassed   = "preflight_passed"
	EvtPreflightFailed   = "preflight_failed"
	EvtVerifyStarted     = "verification_started"
	EvtVerifyCompleted   = "verification_completed"
	EvtVerifyFailed      = "verification_failed"

	// Tool calls
	EvtToolStarted   = "tool_call_started"
	EvtToolCompleted = "tool_call_completed"
	EvtToolFailed    = "tool_call_failed"

	// File edits
	EvtFileEditStarted   = "file_edit_started"
	EvtFileEditCompleted = "file_edit_completed"

	// Tests
	EvtTestStarted = "test_started"
	EvtTestPassed  = "test_passed"
	EvtTestFailed  = "test_failed"

	// Debug loops
	EvtDebugStarted     = "debug_started"
	EvtDebugHypothesis  = "debug_hypothesis_recorded"
	EvtDebugFixAttempt  = "debug_fix_attempted"
	EvtDebugCompleted   = "debug_completed"

	// LLM / tokens
	EvtLLMCompleted   = "llm_call_completed"
	EvtTokenRecorded  = "token_usage_recorded"

	// Diffs
	EvtDiffCaptured = "diff_captured"
)

// ---------------------------------------------------------------------------
// TokenUsage
// ---------------------------------------------------------------------------

// TokenUsage holds token counts for one LLM call.
type TokenUsage struct {
	InputTokens      int  `json:"input_tokens"`
	OutputTokens     int  `json:"output_tokens"`
	ReasoningTokens  int  `json:"reasoning_tokens"`
	CacheReadTokens  int  `json:"cache_read_tokens"`
	CacheWriteTokens int  `json:"cache_write_tokens"`
	TotalTokens      int  `json:"total_tokens"`
	Estimated        bool `json:"estimated"`
}

// ---------------------------------------------------------------------------
// EventRecord — the wire format for events.jsonl
// ---------------------------------------------------------------------------

// EventRecord is one line in the event ledger.
type EventRecord struct {
	EventID     string      `json:"event_id"`
	Timestamp   string      `json:"timestamp"`
	SessionID   string      `json:"session_id"`
	EventType   string      `json:"event_type"`
	PhaseID     string      `json:"phase_id,omitempty"`
	Activity    string      `json:"activity,omitempty"`
	SpanID      string      `json:"span_id,omitempty"`
	ParentSpanID string     `json:"parent_span_id,omitempty"`
	ToolName    string      `json:"tool_name,omitempty"`
	SkillName   string      `json:"skill_name,omitempty"`
	Model       string      `json:"model,omitempty"`
	TokenUsage  *TokenUsage `json:"token_usage,omitempty"`
	Actor       Actor       `json:"actor"`
	CommandUser CommandUser `json:"command_user"`
	Implementer Implementer `json:"implementer"`
	Metadata    interface{} `json:"metadata"`
}

// ---------------------------------------------------------------------------
// EventWriter
// ---------------------------------------------------------------------------

// EventWriter appends events to a session's events.jsonl ledger.
type EventWriter struct {
	mu        sync.Mutex
	eventsPath string
	sessionID  string
	counter    int
}

// NewEventWriter creates an EventWriter for the given session directory.
func NewEventWriter(sessDir, sessionID string) (*EventWriter, error) {
	eventsPath := filepath.Join(sessDir, "events.jsonl")
	// Count existing events to set the monotonic counter.
	counter := countEvents(eventsPath)
	return &EventWriter{
		eventsPath: eventsPath,
		sessionID:  sessionID,
		counter:    counter,
	}, nil
}

// Write appends one event to the ledger. Errors are best-effort (logged to
// stderr) and do not propagate to the caller.
func (w *EventWriter) Write(eventType string, actor Actor, cmdUser CommandUser, impl Implementer, metadata interface{}, opts ...EventOpt) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.counter++
	eventID := fmt.Sprintf("evt_%06d", w.counter)

	rec := EventRecord{
		EventID:     eventID,
		Timestamp:   time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		SessionID:   w.sessionID,
		EventType:   eventType,
		Actor:       actor,
		CommandUser: cmdUser,
		Implementer: impl,
		Metadata:    metadata,
	}
	for _, o := range opts {
		o(&rec)
	}

	data, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keel: event marshal error: %v\n", err)
		return eventID
	}

	f, err := os.OpenFile(w.eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keel: event write error: %v\n", err)
		return eventID
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "keel: event write error: %v\n", err)
	}
	return eventID
}

// ---------------------------------------------------------------------------
// EventOpt — functional options for event fields
// ---------------------------------------------------------------------------

// EventOpt modifies an EventRecord before it is written.
type EventOpt func(*EventRecord)

// WithPhase sets the phase_id field.
func WithPhase(phaseID string) EventOpt {
	return func(r *EventRecord) { r.PhaseID = phaseID }
}

// WithActivity sets the activity field.
func WithActivity(activity string) EventOpt {
	return func(r *EventRecord) { r.Activity = activity }
}

// WithTool sets the tool_name field.
func WithTool(name string) EventOpt {
	return func(r *EventRecord) { r.ToolName = name }
}

// WithModel sets the model field.
func WithModel(model string) EventOpt {
	return func(r *EventRecord) { r.Model = model }
}

// WithTokenUsage sets the token_usage field.
func WithTokenUsage(tu TokenUsage) EventOpt {
	return func(r *EventRecord) { r.TokenUsage = &tu }
}

// WithSpan sets span tracking fields.
func WithSpan(spanID, parentID string) EventOpt {
	return func(r *EventRecord) {
		r.SpanID = spanID
		r.ParentSpanID = parentID
	}
}

// ---------------------------------------------------------------------------
// Read / count helpers
// ---------------------------------------------------------------------------

// ReadEvents reads all event records from a ledger file.
// Records with missing fields are returned with zero values (backward-safe).
func ReadEvents(eventsPath string) ([]EventRecord, error) {
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []EventRecord
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var rec EventRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			// Skip malformed lines — best-effort.
			continue
		}
		// Backward compatibility: fill zero-value fields.
		if rec.Actor.ActorType == "" {
			rec.Actor = Actor{ActorType: "unknown", ActorName: "unknown", ActorID: "unknown"}
		}
		if rec.CommandUser.UserID == "" {
			rec.CommandUser = CommandUser{UserID: "unknown", Name: "unknown"}
		}
		if rec.Implementer.ImplementerType == "" {
			rec.Implementer = UnknownImplementer
		}
		events = append(events, rec)
	}
	return events, nil
}

func countEvents(eventsPath string) int {
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range splitLines(data) {
		if len(line) > 0 {
			count++
		}
	}
	return count
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
