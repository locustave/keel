// Package audit manages the project-level audit log at .agent/audit.jsonl.
//
// This is distinct from the per-session event ledger (.agent/sessions/<id>/events.jsonl).
// The audit log records major project lifecycle events — preflight, phase passage,
// rollbacks, feature approvals — without requiring an active session.
//
// Canonical schema (one JSON object per line):
//
//	{
//	  "event_type":    "phase.passed",        // always present
//	  "timestamp_utc": "2026-06-10T00:00:00Z", // always present, RFC3339
//	  "phase":         3,                      // omitted when not phase-specific
//	  "metadata":      { ... }                 // event-specific payload
//	}
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ---------------------------------------------------------------------------
// Controlled vocabulary
// ---------------------------------------------------------------------------

const (
	EvtPreflightCompleted = "preflight.completed"
	EvtPhasePassed        = "phase.passed"
	EvtPhaseCompleted     = "phase.completed"
	EvtRollbackExecuted   = "rollback.executed"
	EvtFeatureApproved    = "feature.approved"
)

// ---------------------------------------------------------------------------
// Record — the wire format for audit.jsonl
// ---------------------------------------------------------------------------

// Record is one line in the project audit log.
type Record struct {
	EventType    string                 `json:"event_type"`
	TimestampUTC string                 `json:"timestamp_utc"`
	Phase        *int                   `json:"phase,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ---------------------------------------------------------------------------
// Append
// ---------------------------------------------------------------------------

// AuditPath returns the canonical path to the project audit log.
func AuditPath(repoPath string) string {
	return filepath.Join(repoPath, ".agent", "audit.jsonl")
}

// Append writes one record to the project audit log. It is best-effort:
// errors are printed to stderr but never returned to the caller.
func Append(repoPath string, rec Record) {
	if rec.TimestampUTC == "" {
		rec.TimestampUTC = time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	}

	data, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keel: audit marshal error: %v\n", err)
		return
	}

	path := AuditPath(repoPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "keel: audit mkdir error: %v\n", err)
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keel: audit open error: %v\n", err)
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}
