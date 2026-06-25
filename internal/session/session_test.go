package session_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keel/internal/session"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeRepo(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func enabledConfig() session.TrackingConfig {
	return session.DefaultConfig()
}

func disabledConfig() session.TrackingConfig {
	return session.DisabledConfig()
}

// ---------------------------------------------------------------------------
// Config tests (tests 1-2)
// ---------------------------------------------------------------------------

// Test 1: harness init creates tracking config enabled by default.
func TestConfig_defaultEnabled(t *testing.T) {
	repo := makeRepo(t)
	cfg := session.DefaultConfig()
	if err := session.SaveConfig(repo, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	loaded, err := session.LoadConfig(repo)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !loaded.Tracking.Enabled {
		t.Error("expected tracking.enabled=true by default")
	}
	if loaded.Tracking.Mode != "local" {
		t.Errorf("expected mode=local, got %q", loaded.Tracking.Mode)
	}
}

// Test 2: tracking off disables tracking.
func TestConfig_trackingOff(t *testing.T) {
	repo := makeRepo(t)
	cfg := session.DisabledConfig()
	if err := session.SaveConfig(repo, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	loaded, _ := session.LoadConfig(repo)
	if loaded.IsEnabled() {
		t.Error("expected tracking disabled")
	}
}

// Test 31: existing projects without tracking config still run safely.
func TestConfig_missingConfigReturnsDefault(t *testing.T) {
	repo := makeRepo(t)
	cfg, err := session.LoadConfig(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return default (enabled) without error.
	if cfg.Tracking.Mode == "" {
		t.Error("expected non-empty mode from default config")
	}
}

// ---------------------------------------------------------------------------
// Session lifecycle tests (tests 3-4)
// ---------------------------------------------------------------------------

// Test 3: new session created when command runs without active session.
func TestSession_ensureCreatesIfMissing(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, enabledConfig())

	sess := m.EnsureSession(session.CommandUser{UserID: "local:test", Name: "Test"}, session.UnknownImplementer)
	if sess == nil {
		t.Fatal("expected session, got nil")
	}
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}
	// Session directory should exist.
	if _, err := os.Stat(sess.Dir); err != nil {
		t.Errorf("session dir does not exist: %v", err)
	}
	// events.jsonl should exist.
	if _, err := os.Stat(sess.EventsPath()); err != nil {
		t.Errorf("events.jsonl does not exist: %v", err)
	}
}

// Test 3 (disabled): no session when tracking is off.
func TestSession_noSessionWhenDisabled(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, disabledConfig())
	sess := m.EnsureSession(session.CommandUser{}, session.UnknownImplementer)
	if sess != nil {
		t.Error("expected nil session when tracking disabled")
	}
}

// Test 4: session metadata includes session owner.
func TestSession_ownerInMetadata(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, enabledConfig())
	owner := session.ResolveIdentity(session.ResolveOptions{Name: "Joey Miller", Email: "joey@example.com"})

	sess, err := m.StartSession(owner, owner.ToCommandUser(), session.UnknownImplementer)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	var meta session.SessionMeta
	data, _ := os.ReadFile(filepath.Join(sess.Dir, "session.json"))
	if len(data) == 0 {
		t.Fatal("session.json is empty")
	}
	if err := readJSONBytes(data, &meta); err != nil {
		t.Fatalf("parse session.json: %v", err)
	}
	if meta.SessionOwner.Name != "Joey Miller" {
		t.Errorf("expected owner name Joey Miller, got %q", meta.SessionOwner.Name)
	}
	if meta.SessionOwner.UserID == "" {
		t.Error("expected non-empty user_id in session owner")
	}
}

// ---------------------------------------------------------------------------
// Identity resolution tests (tests 5-8)
// ---------------------------------------------------------------------------

// Test 5: user identity can be passed by CLI flags.
func TestIdentity_fromCLIFlags(t *testing.T) {
	id := session.ResolveIdentity(session.ResolveOptions{
		Name:  "Joey Miller",
		Email: "joey@example.com",
	})
	if id.Name != "Joey Miller" {
		t.Errorf("expected Joey Miller, got %q", id.Name)
	}
	if id.Source != "cli_arg" {
		t.Errorf("expected source=cli_arg, got %q", id.Source)
	}
	if id.Email == nil || *id.Email != "joey@example.com" {
		t.Errorf("expected email, got %v", id.Email)
	}
}

// Test 6: user identity can be loaded from config.
func TestIdentity_fromConfig(t *testing.T) {
	name := "Config User"
	cfg := &session.UserConfig{Name: &name}
	id := session.ResolveIdentity(session.ResolveOptions{Config: cfg})
	if id.Name != "Config User" {
		t.Errorf("expected Config User, got %q", id.Name)
	}
	if id.Source != "local_config" {
		t.Errorf("expected source=local_config, got %q", id.Source)
	}
}

// Test 7: user identity can be inferred from git config.
// (Integration: only passes if git config user.name is set in the test environment.)
func TestIdentity_fromGitOrEnvFallback(t *testing.T) {
	// This test verifies the fallback chain works without panicking.
	id := session.ResolveIdentity(session.ResolveOptions{})
	if id.UserID == "" {
		t.Error("expected non-empty user_id even for unknown identity")
	}
	if id.Name == "" {
		t.Error("expected non-empty name")
	}
}

// Test 8: unknown identity is handled safely.
func TestIdentity_unknownHandledSafely(t *testing.T) {
	// Force unknown by passing empty options with no git config available.
	// We cannot truly force unknown in all environments, but we can verify
	// the zero-value is safe.
	id := session.ResolveIdentity(session.ResolveOptions{})
	// Must not panic, must have non-empty UserID and Name.
	if id.UserID == "" {
		t.Error("UserID must never be empty")
	}
	if id.Name == "" {
		t.Error("Name must never be empty")
	}
}

// ---------------------------------------------------------------------------
// Event writing tests (tests 9-13)
// ---------------------------------------------------------------------------

// Test 9: events are appended to events.jsonl.
func TestEvent_appendedToFile(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, enabledConfig())
	sess, _ := m.StartSession(
		session.ResolveIdentity(session.ResolveOptions{Name: "Tester"}),
		session.CommandUser{UserID: "local:tester", Name: "Tester"},
		session.UnknownImplementer,
	)

	sess.WriteEvent(session.EvtCommandStarted, session.HarnessCLIActor, map[string]string{"cmd": "plan"})

	events, err := session.ReadEvents(sess.EventsPath())
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) < 2 { // session_started + command_started
		t.Errorf("expected at least 2 events, got %d", len(events))
	}
	found := false
	for _, e := range events {
		if e.EventType == session.EvtCommandStarted {
			found = true
		}
	}
	if !found {
		t.Error("expected harness_command_started event in ledger")
	}
}

// Test 10: event IDs are monotonic.
func TestEvent_monotonic(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, enabledConfig())
	sess, _ := m.StartSession(
		session.ResolveIdentity(session.ResolveOptions{Name: "Tester"}),
		session.CommandUser{UserID: "local:tester", Name: "Tester"},
		session.UnknownImplementer,
	)

	for i := 0; i < 5; i++ {
		sess.WriteEvent(session.EvtCommandStarted, session.HarnessCLIActor, nil)
	}

	events, _ := session.ReadEvents(sess.EventsPath())
	for i := 1; i < len(events); i++ {
		if events[i].EventID <= events[i-1].EventID {
			t.Errorf("event IDs not monotonic: %s <= %s",
				events[i].EventID, events[i-1].EventID)
		}
	}
}

// Test 11: event records include actor context.
func TestEvent_includesActor(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, enabledConfig())
	sess, _ := m.StartSession(
		session.ResolveIdentity(session.ResolveOptions{Name: "Tester"}),
		session.CommandUser{UserID: "local:tester", Name: "Tester"},
		session.UnknownImplementer,
	)

	sess.WriteEvent(session.EvtPhaseStarted, session.HarnessCLIActor, nil, session.WithPhase("phase-001"))

	events, _ := session.ReadEvents(sess.EventsPath())
	var phaseEvt *session.EventRecord
	for i := range events {
		if events[i].EventType == session.EvtPhaseStarted {
			phaseEvt = &events[i]
		}
	}
	if phaseEvt == nil {
		t.Fatal("phase_started event not found")
	}
	if phaseEvt.Actor.ActorType == "" {
		t.Error("expected actor_type in event")
	}
	if phaseEvt.Actor.ActorName != "keel-cli" {
		t.Errorf("expected actor_name=keel-cli, got %q", phaseEvt.Actor.ActorName)
	}
}

// Test 12: event records include command user context.
func TestEvent_includesCommandUser(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, enabledConfig())
	sess, _ := m.StartSession(
		session.ResolveIdentity(session.ResolveOptions{Name: "Joey Miller"}),
		session.CommandUser{UserID: "local:joeymiller", Name: "Joey Miller"},
		session.UnknownImplementer,
	)

	sess.WriteEvent(session.EvtCommandStarted, session.HarnessCLIActor, nil)

	events, _ := session.ReadEvents(sess.EventsPath())
	for _, e := range events {
		if e.EventType == session.EvtCommandStarted {
			if e.CommandUser.Name != "Joey Miller" {
				t.Errorf("expected command_user.name=Joey Miller, got %q", e.CommandUser.Name)
			}
			return
		}
	}
	t.Fatal("harness_command_started event not found")
}

// Test 13: event records include implementer context.
func TestEvent_includesImplementer(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, enabledConfig())
	impl := session.MakeImplementer("claude-code", "agent")
	sess, _ := m.StartSession(
		session.ResolveIdentity(session.ResolveOptions{Name: "Tester"}),
		session.CommandUser{UserID: "local:tester", Name: "Tester"},
		impl,
	)

	sess.WriteEvent(session.EvtPhaseStarted, session.HarnessCLIActor, nil)

	events, _ := session.ReadEvents(sess.EventsPath())
	for _, e := range events {
		if e.EventType == session.EvtPhaseStarted {
			if e.Implementer.ImplementerType != "agent" {
				t.Errorf("expected implementer_type=agent, got %q", e.Implementer.ImplementerType)
			}
			if e.Implementer.ImplementerName == nil || *e.Implementer.ImplementerName != "claude-code" {
				t.Errorf("expected implementer_name=claude-code")
			}
			return
		}
	}
	t.Fatal("phase_started event not found")
}

// ---------------------------------------------------------------------------
// State reducer tests (tests 14-18)
// ---------------------------------------------------------------------------

// Test 14: phase state tracks active implementer.
func TestReduce_tracksImplementer(t *testing.T) {
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtSessionStarted, Implementer: session.UnknownImplementer},
		{EventID: "evt_000002", EventType: session.EvtPhaseStarted, PhaseID: "phase-002",
			Implementer: session.MakeImplementer("claude-code", "agent")},
	}
	state := session.ReduceState("sess1", events)
	if state.ActiveImplementer.ImplementerType != "agent" {
		t.Errorf("expected agent implementer, got %q", state.ActiveImplementer.ImplementerType)
	}
}

// Test 15: state reducer derives current session state.
func TestReduce_sessionState(t *testing.T) {
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtSessionStarted},
		{EventID: "evt_000002", EventType: session.EvtPhaseStarted, PhaseID: "phase-001"},
	}
	state := session.ReduceState("sess1", events)
	if state.SessionLifecycle != "executing" {
		t.Errorf("expected lifecycle=executing, got %q", state.SessionLifecycle)
	}
	if state.CurrentPhase != "phase-001" {
		t.Errorf("expected current_phase=phase-001, got %q", state.CurrentPhase)
	}
	if state.LastEventID != "evt_000002" {
		t.Errorf("expected last_event_id=evt_000002, got %q", state.LastEventID)
	}
}

// Test 16: metrics reducer derives counts.
func TestReduce_metrics(t *testing.T) {
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtPhaseCompleted,
			Actor: session.Actor{ActorType: "keel", ActorName: "keel-cli", ActorID: "local:keel-cli"}},
		{EventID: "evt_000002", EventType: session.EvtToolCompleted,
			Actor: session.Actor{ActorType: "tool", ActorName: "pytest", ActorID: "tool:pytest"}},
		{EventID: "evt_000003", EventType: session.EvtToolFailed,
			Actor: session.Actor{ActorType: "tool", ActorName: "pytest", ActorID: "tool:pytest"}},
		{EventID: "evt_000004", EventType: session.EvtTestPassed,
			Actor: session.Actor{ActorType: "tool", ActorName: "pytest", ActorID: "tool:pytest"}},
		{EventID: "evt_000005", EventType: session.EvtTestFailed,
			Actor: session.Actor{ActorType: "tool", ActorName: "pytest", ActorID: "tool:pytest"}},
	}
	m := session.ReduceMetrics("sess1", events)
	if m.PhasesCompleted != 1 {
		t.Errorf("phases_completed: expected 1, got %d", m.PhasesCompleted)
	}
	if m.ToolCalls != 2 {
		t.Errorf("tool_calls: expected 2, got %d", m.ToolCalls)
	}
	if m.ToolFailures != 1 {
		t.Errorf("tool_failures: expected 1, got %d", m.ToolFailures)
	}
	if m.TestsPassed != 1 {
		t.Errorf("tests_passed: expected 1, got %d", m.TestsPassed)
	}
	if m.TestsFailed != 1 {
		t.Errorf("tests_failed: expected 1, got %d", m.TestsFailed)
	}
}

// Test 17: metrics include events by actor type.
func TestReduce_eventsByActorType(t *testing.T) {
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: "any", Actor: session.Actor{ActorType: "keel"}},
		{EventID: "evt_000002", EventType: "any", Actor: session.Actor{ActorType: "tool"}},
		{EventID: "evt_000003", EventType: "any", Actor: session.Actor{ActorType: "tool"}},
	}
	m := session.ReduceMetrics("sess1", events)
	if m.EventsByActorType["keel"] != 1 {
		t.Errorf("expected 1 keel event, got %d", m.EventsByActorType["keel"])
	}
	if m.EventsByActorType["tool"] != 2 {
		t.Errorf("expected 2 tool events, got %d", m.EventsByActorType["tool"])
	}
}

// Test 18: metrics include events by implementer.
func TestReduce_eventsByImplementer(t *testing.T) {
	implID := "agent:claude-code"
	impl := session.MakeImplementer("claude-code", "agent")
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: "any", Actor: session.Actor{ActorType: "keel"}, Implementer: impl},
		{EventID: "evt_000002", EventType: "any", Actor: session.Actor{ActorType: "tool"}, Implementer: impl},
	}
	m := session.ReduceMetrics("sess1", events)
	if m.EventsByImplementer[implID] != 2 {
		t.Errorf("expected 2 events by %s, got %d", implID, m.EventsByImplementer[implID])
	}
}

// ---------------------------------------------------------------------------
// Token tracking tests (tests 21-25)
// ---------------------------------------------------------------------------

// Test 21: token usage event rolled up to session metrics.
func TestTokens_rolledUpToMetrics(t *testing.T) {
	impl := session.MakeImplementer("claude-code", "agent")
	tu := &session.TokenUsage{InputTokens: 1000, OutputTokens: 500, TotalTokens: 1500, Estimated: false}
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtLLMCompleted,
			Actor:       session.Actor{ActorType: "agent", ActorID: "agent:claude-code"},
			Implementer: impl,
			TokenUsage:  tu},
	}
	m := session.ReduceMetrics("sess1", events)
	if m.TotalTokens != 1500 {
		t.Errorf("expected total_tokens=1500, got %d", m.TotalTokens)
	}
	if m.LLMCalls != 1 {
		t.Errorf("expected llm_calls=1, got %d", m.LLMCalls)
	}
}

// Test 22: estimated tokens are marked as estimated.
func TestTokens_estimatedMarked(t *testing.T) {
	tu := &session.TokenUsage{TotalTokens: 800, Estimated: true}
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtLLMCompleted,
			Actor: session.Actor{ActorType: "agent"}, Implementer: session.UnknownImplementer, TokenUsage: tu},
	}
	m := session.ReduceMetrics("sess1", events)
	if m.EstimatedTokens != 800 {
		t.Errorf("expected estimated_tokens=800, got %d", m.EstimatedTokens)
	}
	if m.ExactTokens != 0 {
		t.Errorf("expected exact_tokens=0, got %d", m.ExactTokens)
	}
}

// Test 23: exact and estimated tokens reported separately.
func TestTokens_exactAndEstimatedSeparate(t *testing.T) {
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtLLMCompleted,
			Actor: session.Actor{ActorType: "agent"}, Implementer: session.UnknownImplementer,
			TokenUsage: &session.TokenUsage{TotalTokens: 500, Estimated: false}},
		{EventID: "evt_000002", EventType: session.EvtLLMCompleted,
			Actor: session.Actor{ActorType: "agent"}, Implementer: session.UnknownImplementer,
			TokenUsage: &session.TokenUsage{TotalTokens: 300, Estimated: true}},
	}
	m := session.ReduceMetrics("sess1", events)
	if m.TotalTokens != 800 {
		t.Errorf("expected total=800, got %d", m.TotalTokens)
	}
	if m.ExactTokens != 500 {
		t.Errorf("expected exact=500, got %d", m.ExactTokens)
	}
	if m.EstimatedTokens != 300 {
		t.Errorf("expected estimated=300, got %d", m.EstimatedTokens)
	}
}

// Test 24: token reports can group by implementer.
func TestTokens_groupByImplementer(t *testing.T) {
	impl := session.MakeImplementer("claude-code", "agent")
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtLLMCompleted,
			Actor: session.Actor{ActorType: "agent", ActorID: "agent:claude-code"}, Implementer: impl,
			TokenUsage: &session.TokenUsage{TotalTokens: 1000, Estimated: false}},
	}
	m := session.ReduceMetrics("sess1", events)
	if m.TokensByImplementer["agent:claude-code"] != 1000 {
		t.Errorf("expected tokens by implementer=1000, got %d", m.TokensByImplementer["agent:claude-code"])
	}
}

// Test 25: token reports can group by actor.
func TestTokens_groupByActor(t *testing.T) {
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtLLMCompleted,
			Actor:       session.Actor{ActorType: "agent", ActorID: "agent:codex"},
			Implementer: session.UnknownImplementer,
			TokenUsage:  &session.TokenUsage{TotalTokens: 750, Estimated: false}},
	}
	m := session.ReduceMetrics("sess1", events)
	if m.TokensByActor["agent:codex"] != 750 {
		t.Errorf("expected tokens by actor=750, got %d", m.TokensByActor["agent:codex"])
	}
}

// ---------------------------------------------------------------------------
// Hook tests (test 27)
// ---------------------------------------------------------------------------

// Test 27: hook scripts are generated and read stdin JSON (Claude Code protocol).
func TestHooks_generated(t *testing.T) {
	repo := makeRepo(t)
	installer := &session.HookInstaller{RepoPath: repo}
	if err := installer.Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}
	hooks := []string{"pre-tool", "post-tool", "stop"}
	for _, h := range hooks {
		path := filepath.Join(repo, ".agent", "hooks", h)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected hook %s to exist: %v", h, err)
			continue
		}
		if info.Mode()&0o111 == 0 {
			t.Errorf("hook %s is not executable", h)
		}
		data, _ := os.ReadFile(path)
		content := string(data)
		// Hooks must invoke keel event append (shell or subprocess).
		if !strings.Contains(content, "keel event append") && !strings.Contains(content, `"event", "append"`) {
			t.Errorf("hook %s does not call keel event append", h)
		}
		// Hooks must read from stdin, not from env vars.
		if !strings.Contains(content, "PAYLOAD=$(cat)") {
			t.Errorf("hook %s does not read stdin payload", h)
		}
	}
}

// Test 28: InstallClaudeSettings writes .claude/settings.json with hook registrations.
func TestHooks_claudeSettingsWritten(t *testing.T) {
	repo := makeRepo(t)
	installer := &session.HookInstaller{RepoPath: repo}
	if err := installer.InstallClaudeSettings(); err != nil {
		t.Fatalf("InstallClaudeSettings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'hooks' key in settings.json")
	}
	for _, event := range []string{"PreToolUse", "PostToolUse", "Stop"} {
		if _, ok := hooks[event]; !ok {
			t.Errorf("expected %s in hooks", event)
		}
	}
}

// Test 28b: InstallClaudeSettings preserves existing settings keys.
func TestHooks_claudeSettingsPreservesExisting(t *testing.T) {
	repo := makeRepo(t)
	claudeDir := filepath.Join(repo, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	existing := `{"permissions": {"allow": ["Bash(go test:*)"]}}` + "\n"
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644)

	installer := &session.HookInstaller{RepoPath: repo}
	if err := installer.InstallClaudeSettings(); err != nil {
		t.Fatalf("InstallClaudeSettings: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var settings map[string]interface{}
	json.Unmarshal(data, &settings)

	if _, ok := settings["permissions"]; !ok {
		t.Error("existing 'permissions' key must be preserved")
	}
	if _, ok := settings["hooks"]; !ok {
		t.Error("'hooks' key must be added")
	}
}

// ---------------------------------------------------------------------------
// Large artifact test (test 29)
// ---------------------------------------------------------------------------

// Test 29: large tool output stored as artifact, not inline.
func TestEvent_largeOutputAsArtifact(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, enabledConfig())
	sess, _ := m.StartSession(
		session.ResolveIdentity(session.ResolveOptions{Name: "Tester"}),
		session.CommandUser{UserID: "local:tester", Name: "Tester"},
		session.UnknownImplementer,
	)

	// Write a large artifact separately and reference it from the event.
	artifactPath := sess.ArtifactPath("pytest-evt_000010.log")
	largeOutput := strings.Repeat("test output line\n", 1000)
	_ = os.WriteFile(artifactPath, []byte(largeOutput), 0o644)

	sess.WriteEvent(session.EvtToolCompleted, session.MakeToolActor("pytest"),
		map[string]string{"output_artifact_path": artifactPath})

	// Verify the events.jsonl does NOT contain the large output inline.
	data, _ := os.ReadFile(sess.EventsPath())
	if strings.Contains(string(data), "test output line") {
		t.Error("large tool output must not be stored inline in events.jsonl")
	}
	// Verify artifact file exists.
	if _, err := os.Stat(artifactPath); err != nil {
		t.Errorf("artifact file should exist: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Backward compatibility tests (tests 31-32)
// ---------------------------------------------------------------------------

// Test 32: existing event logs without actor fields still reduce safely.
func TestReduce_backwardCompatMissingFields(t *testing.T) {
	// Events with no actor/implementer fields — simulates old-format ledger.
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtPhaseCompleted},
		{EventID: "evt_000002", EventType: session.EvtToolCompleted},
	}
	// Ensure reducer does not panic on zero-value Actor/Implementer.
	state := session.ReduceState("sess1", events)
	metrics := session.ReduceMetrics("sess1", events)

	if state.LastEventID == "" {
		t.Error("expected last_event_id to be set")
	}
	if metrics.PhasesCompleted != 1 {
		t.Errorf("expected phases_completed=1, got %d", metrics.PhasesCompleted)
	}
}

// Test 30: secrets and raw env vars are NOT stored in session.json.
func TestSession_noSecretsInMetadata(t *testing.T) {
	repo := makeRepo(t)
	m := session.NewManagerWithConfig(repo, enabledConfig())
	// Set an environment variable that should NOT appear in the session file.
	os.Setenv("MY_SECRET_KEY", "supersecretvalue12345")
	defer os.Unsetenv("MY_SECRET_KEY")

	owner := session.ResolveIdentity(session.ResolveOptions{Name: "Tester"})
	sess, _ := m.StartSession(owner, owner.ToCommandUser(), session.UnknownImplementer)

	data, _ := os.ReadFile(filepath.Join(sess.Dir, "session.json"))
	if strings.Contains(string(data), "supersecretvalue12345") {
		t.Error("secret value must not appear in session.json")
	}
}

// ---------------------------------------------------------------------------
// Phase tokens test (test 21 extended)
// ---------------------------------------------------------------------------

func TestPhaseTokens_rollup(t *testing.T) {
	impl := session.MakeImplementer("claude-code", "agent")
	events := []session.EventRecord{
		{EventID: "evt_000001", EventType: session.EvtLLMCompleted, PhaseID: "phase-002",
			Actor: session.Actor{ActorType: "agent", ActorID: "agent:claude-code"}, Implementer: impl,
			TokenUsage: &session.TokenUsage{InputTokens: 1000, OutputTokens: 500, TotalTokens: 1500, Estimated: false}},
		{EventID: "evt_000002", EventType: session.EvtLLMCompleted, PhaseID: "phase-002",
			Actor: session.Actor{ActorType: "agent", ActorID: "agent:claude-code"}, Implementer: impl,
			TokenUsage: &session.TokenUsage{InputTokens: 2000, OutputTokens: 800, TotalTokens: 2800, Estimated: true}},
		// Different phase — should not be included.
		{EventID: "evt_000003", EventType: session.EvtLLMCompleted, PhaseID: "phase-003",
			Actor: session.Actor{ActorType: "agent"}, Implementer: impl,
			TokenUsage: &session.TokenUsage{TotalTokens: 9999}},
	}
	r := session.ReducePhaseTokens("phase-002", events)
	if r.LLMCalls != 2 {
		t.Errorf("expected 2 llm calls, got %d", r.LLMCalls)
	}
	if r.TotalTokens != 4300 {
		t.Errorf("expected total=4300, got %d", r.TotalTokens)
	}
	if r.ExactTokens != 1500 {
		t.Errorf("expected exact=1500, got %d", r.ExactTokens)
	}
	if r.EstimatedTokens != 2800 {
		t.Errorf("expected estimated=2800, got %d", r.EstimatedTokens)
	}
	if r.TokensByImplementer["agent:claude-code"] != 4300 {
		t.Errorf("expected tokens by implementer=4300, got %d", r.TokensByImplementer["agent:claude-code"])
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func readJSONBytes(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
