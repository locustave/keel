package session

// ---------------------------------------------------------------------------
// State reducer
// ---------------------------------------------------------------------------

// SessionState is derived from the event ledger.
type SessionState struct {
	SessionID          string      `json:"session_id"`
	SessionLifecycle   string      `json:"session_lifecycle"` // "active" | "executing" | "completed"
	CurrentPhase       string      `json:"current_phase,omitempty"`
	CurrentActivity    string      `json:"current_activity,omitempty"`
	LastEventID        string      `json:"last_event_id"`
	ActiveImplementer  Implementer `json:"active_implementer"`
}

// ReduceState derives session state from events. Deterministic: same events → same state.
func ReduceState(sessionID string, events []EventRecord) SessionState {
	state := SessionState{
		SessionID:         sessionID,
		SessionLifecycle:  "active",
		ActiveImplementer: UnknownImplementer,
	}
	for _, e := range events {
		state.LastEventID = e.EventID

		// Update lifecycle.
		switch e.EventType {
		case EvtSessionStarted:
			state.SessionLifecycle = "active"
		case EvtSessionCompleted:
			state.SessionLifecycle = "completed"
		case EvtPhaseStarted:
			state.SessionLifecycle = "executing"
			state.CurrentPhase = e.PhaseID
			state.CurrentActivity = "phase_execution"
		case EvtPhaseCompleted, EvtPhaseFailed:
			state.CurrentActivity = ""
		case EvtDebugStarted:
			state.CurrentActivity = "debugging"
		case EvtDebugCompleted:
			state.CurrentActivity = ""
		case EvtToolStarted:
			state.CurrentActivity = "tool_call"
		case EvtToolCompleted, EvtToolFailed:
			state.CurrentActivity = ""
		}

		// Track activity from event field.
		if e.Activity != "" {
			state.CurrentActivity = e.Activity
		}

		// Update active implementer if specified.
		if e.Implementer.ImplementerType != "" && e.Implementer.ImplementerType != "unknown" {
			state.ActiveImplementer = e.Implementer
		}
	}
	return state
}

// ---------------------------------------------------------------------------
// Metrics reducer
// ---------------------------------------------------------------------------

// SessionMetrics is derived from the event ledger.
type SessionMetrics struct {
	SessionID            string                 `json:"session_id"`
	PhasesCompleted      int                    `json:"phases_completed"`
	PhasesFailed         int                    `json:"phases_failed"`
	ToolCalls            int                    `json:"tool_calls"`
	ToolFailures         int                    `json:"tool_failures"`
	FileEdits            int                    `json:"file_edits"`
	DebugLoops           int                    `json:"debug_loops"`
	TestsPassed          int                    `json:"tests_passed"`
	TestsFailed          int                    `json:"tests_failed"`
	TotalTokens          int                    `json:"total_tokens"`
	ExactTokens          int                    `json:"exact_tokens"`
	EstimatedTokens      int                    `json:"estimated_tokens"`
	LLMCalls             int                    `json:"llm_calls"`
	EventsByActorType    map[string]int         `json:"events_by_actor_type"`
	EventsByImplementer  map[string]int         `json:"events_by_implementer"`
	TokensByImplementer  map[string]int         `json:"tokens_by_implementer"`
	TokensByActor        map[string]int         `json:"tokens_by_actor"`
}

// ReduceMetrics derives session metrics from events. Deterministic.
func ReduceMetrics(sessionID string, events []EventRecord) SessionMetrics {
	m := SessionMetrics{
		SessionID:           sessionID,
		EventsByActorType:   make(map[string]int),
		EventsByImplementer: make(map[string]int),
		TokensByImplementer: make(map[string]int),
		TokensByActor:       make(map[string]int),
	}

	for _, e := range events {
		// Actor type counts.
		if e.Actor.ActorType != "" {
			m.EventsByActorType[e.Actor.ActorType]++
		}
		// Implementer counts.
		if e.Implementer.ImplementerID != nil && *e.Implementer.ImplementerID != "" {
			m.EventsByImplementer[*e.Implementer.ImplementerID]++
		}

		switch e.EventType {
		case EvtPhaseCompleted:
			m.PhasesCompleted++
		case EvtPhaseFailed:
			m.PhasesFailed++
		case EvtToolCompleted:
			m.ToolCalls++
		case EvtToolFailed:
			m.ToolCalls++
			m.ToolFailures++
		case EvtFileEditCompleted:
			m.FileEdits++
		case EvtDebugStarted:
			m.DebugLoops++
		case EvtTestPassed:
			m.TestsPassed++
		case EvtTestFailed:
			m.TestsFailed++
		case EvtLLMCompleted:
			m.LLMCalls++
			if tu := e.TokenUsage; tu != nil {
				m.TotalTokens += tu.TotalTokens
				if tu.Estimated {
					m.EstimatedTokens += tu.TotalTokens
				} else {
					m.ExactTokens += tu.TotalTokens
				}
				implKey := "unknown"
				if e.Implementer.ImplementerID != nil {
					implKey = *e.Implementer.ImplementerID
				}
				m.TokensByImplementer[implKey] += tu.TotalTokens
				m.TokensByActor[e.Actor.ActorID] += tu.TotalTokens
			}
		}
	}

	return m
}

// ---------------------------------------------------------------------------
// Phase token roll-up
// ---------------------------------------------------------------------------

// PhaseTokenReport aggregates token usage for a single phase.
type PhaseTokenReport struct {
	PhaseID             string         `json:"phase_id"`
	InputTokens         int            `json:"input_tokens"`
	OutputTokens        int            `json:"output_tokens"`
	ReasoningTokens     int            `json:"reasoning_tokens"`
	CacheReadTokens     int            `json:"cache_read_tokens"`
	CacheWriteTokens    int            `json:"cache_write_tokens"`
	TotalTokens         int            `json:"total_tokens"`
	EstimatedTokens     int            `json:"estimated_tokens"`
	ExactTokens         int            `json:"exact_tokens"`
	LLMCalls            int            `json:"llm_calls"`
	TokensByImplementer map[string]int `json:"tokens_by_implementer"`
}

// ReducePhaseTokens computes token roll-up for the given phaseID.
func ReducePhaseTokens(phaseID string, events []EventRecord) PhaseTokenReport {
	r := PhaseTokenReport{
		PhaseID:             phaseID,
		TokensByImplementer: make(map[string]int),
	}
	for _, e := range events {
		if e.PhaseID != phaseID || e.EventType != EvtLLMCompleted || e.TokenUsage == nil {
			continue
		}
		tu := e.TokenUsage
		r.LLMCalls++
		r.InputTokens += tu.InputTokens
		r.OutputTokens += tu.OutputTokens
		r.ReasoningTokens += tu.ReasoningTokens
		r.CacheReadTokens += tu.CacheReadTokens
		r.CacheWriteTokens += tu.CacheWriteTokens
		r.TotalTokens += tu.TotalTokens
		if tu.Estimated {
			r.EstimatedTokens += tu.TotalTokens
		} else {
			r.ExactTokens += tu.TotalTokens
		}
		implKey := "unknown"
		if e.Implementer.ImplementerID != nil {
			implKey = *e.Implementer.ImplementerID
		}
		r.TokensByImplementer[implKey] += tu.TotalTokens
	}
	return r
}
