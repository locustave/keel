package audit_test

import (
	"encoding/json"
	"os"
	"testing"

	"keel/internal/audit"
)

func TestAppend_writesCanonicalSchema(t *testing.T) {
	repo := t.TempDir()
	phase := 3

	audit.Append(repo, audit.Record{
		EventType: audit.EvtPhasePassed,
		Phase:     &phase,
		Metadata:  map[string]interface{}{"name": "test-phase", "model": "claude-sonnet-4-6"},
	})

	data, err := os.ReadFile(audit.AuditPath(repo))
	if err != nil {
		t.Fatalf("audit.jsonl not written: %v", err)
	}

	var rec audit.Record
	if err := json.Unmarshal(data[:len(data)-1], &rec); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rec.EventType != audit.EvtPhasePassed {
		t.Errorf("expected event_type %q, got %q", audit.EvtPhasePassed, rec.EventType)
	}
	if rec.TimestampUTC == "" {
		t.Error("expected timestamp_utc to be set")
	}
	if rec.Phase == nil || *rec.Phase != 3 {
		t.Errorf("expected phase 3, got %v", rec.Phase)
	}
	if rec.Metadata["name"] != "test-phase" {
		t.Errorf("unexpected metadata: %v", rec.Metadata)
	}
}

func TestAppend_multipleRecords(t *testing.T) {
	repo := t.TempDir()

	audit.Append(repo, audit.Record{EventType: audit.EvtPreflightCompleted})
	audit.Append(repo, audit.Record{EventType: audit.EvtRollbackExecuted})

	data, err := os.ReadFile(audit.AuditPath(repo))
	if err != nil {
		t.Fatalf("audit.jsonl not written: %v", err)
	}

	lines := splitLines(data)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var first, second audit.Record
	json.Unmarshal(lines[0], &first)
	json.Unmarshal(lines[1], &second)

	if first.EventType != audit.EvtPreflightCompleted {
		t.Errorf("line 1: expected %q, got %q", audit.EvtPreflightCompleted, first.EventType)
	}
	if second.EventType != audit.EvtRollbackExecuted {
		t.Errorf("line 2: expected %q, got %q", audit.EvtRollbackExecuted, second.EventType)
	}
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	return lines
}
