package session

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Session types
// ---------------------------------------------------------------------------

// SessionMeta is stored in session.json.
type SessionMeta struct {
	SessionID    string      `json:"session_id"`
	CreatedAt    string      `json:"created_at"`
	EndedAt      string      `json:"ended_at,omitempty"`
	Status       string      `json:"status"` // "active" | "completed"
	SessionOwner CommandUser `json:"session_owner"`
	CreatedBy    Actor       `json:"created_by"`
	Host         HostInfo    `json:"host"`
}

// HostInfo captures local environment metadata (no secrets).
type HostInfo struct {
	Hostname         string `json:"hostname"`
	OSUser           string `json:"os_user"`
	WorkingDirectory string `json:"working_directory"`
}

// ---------------------------------------------------------------------------
// Manager
// ---------------------------------------------------------------------------

// Manager owns session lifecycle for a single repo.
type Manager struct {
	RepoPath string
	cfg      TrackingConfig
}

// NewManager creates a Manager for the given repo path.
func NewManager(repoPath string) *Manager {
	cfg, _ := LoadConfig(repoPath)
	return &Manager{RepoPath: repoPath, cfg: cfg}
}

// NewManagerWithConfig creates a Manager with an explicit config.
func NewManagerWithConfig(repoPath string, cfg TrackingConfig) *Manager {
	return &Manager{RepoPath: repoPath, cfg: cfg}
}

// IsEnabled returns true if session tracking is active.
func (m *Manager) IsEnabled() bool {
	return m.cfg.IsEnabled()
}

// ---------------------------------------------------------------------------
// EnsureSession — auto-create a session if one is not active
// ---------------------------------------------------------------------------

// EnsureSession returns the active session, creating one automatically if
// no current session exists. Returns nil (not an error) when tracking is
// disabled.
func (m *Manager) EnsureSession(cmdUser CommandUser, impl Implementer) *Session {
	if !m.IsEnabled() {
		return nil
	}
	if id := m.CurrentSessionID(); id != "" {
		if sess, err := m.loadSession(id); err == nil {
			return sess
		}
	}
	// Auto-create a session.
	identity := ResolveIdentity(ResolveOptions{Config: &m.cfg.User})
	sess, _ := m.StartSession(identity, cmdUser, impl)
	return sess
}

// ---------------------------------------------------------------------------
// StartSession
// ---------------------------------------------------------------------------

// StartSession creates a new session, writes session.json, touches ledger
// files, updates the current-session pointer, and writes the session_started
// event.
func (m *Manager) StartSession(owner Identity, cmdUser CommandUser, impl Implementer) (*Session, error) {
	if !m.IsEnabled() {
		return nil, nil
	}

	id := newSessionID()
	sessDir := m.sessionDir(id)
	if err := os.MkdirAll(filepath.Join(sessDir, "phases"), 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(sessDir, "artifacts"), 0o755); err != nil {
		return nil, fmt.Errorf("create artifacts dir: %w", err)
	}

	hostname, _ := os.Hostname()
	wd, _ := os.Getwd()

	meta := SessionMeta{
		SessionID: id,
		CreatedAt: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		Status:    "active",
		SessionOwner: CommandUser{
			UserID: owner.UserID,
			Name:   owner.Name,
			Email:  owner.Email,
		},
		CreatedBy: owner.ToActor(),
		Host: HostInfo{
			Hostname:         hostname,
			WorkingDirectory: wd,
			OSUser:           os.Getenv("USER"),
		},
	}

	if err := writeJSON(filepath.Join(sessDir, "session.json"), meta); err != nil {
		return nil, err
	}

	// Touch ledger files.
	for _, name := range []string{"events.jsonl", "state.json", "metrics.json"} {
		touchFile(filepath.Join(sessDir, name))
	}

	// Write current-session pointer.
	if err := m.setCurrentSessionID(id); err != nil {
		return nil, err
	}

	sess := m.newSession(id, sessDir, cmdUser, impl)

	// Write session_started event.
	creatorActor := owner.ToActor()
	sess.Writer.Write(EvtSessionStarted, creatorActor, cmdUser, impl, map[string]interface{}{
		"session_id": id,
	})

	// Reduce initial state.
	if m.cfg.Tracking.ReduceAfterEvent {
		sess.Reduce()
	}

	return sess, nil
}

// ---------------------------------------------------------------------------
// CurrentSessionID
// ---------------------------------------------------------------------------

// CurrentSessionID reads the current-session pointer file.
// Returns "" if none is set.
func (m *Manager) CurrentSessionID() string {
	path := filepath.Join(m.RepoPath, ".agent", "keel", "current-session")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// loadSession reopens an existing session by ID.
func (m *Manager) loadSession(id string) (*Session, error) {
	sessDir := m.sessionDir(id)
	if _, err := os.Stat(sessDir); err != nil {
		return nil, err
	}
	// Load session meta to get owner.
	var meta SessionMeta
	if err := readJSON(filepath.Join(sessDir, "session.json"), &meta); err != nil {
		return nil, err
	}
	cmdUser := meta.SessionOwner
	return m.newSession(id, sessDir, cmdUser, UnknownImplementer), nil
}

// EndSession marks the session as completed and writes session_completed event.
func (m *Manager) EndSession(id string) error {
	if !m.IsEnabled() {
		return nil
	}
	if id == "" {
		id = m.CurrentSessionID()
	}
	if id == "" {
		return fmt.Errorf("no active session")
	}
	sessDir := m.sessionDir(id)
	sess := m.newSession(id, sessDir, CommandUser{UserID: "unknown", Name: "unknown"}, UnknownImplementer)
	sess.Writer.Write(EvtSessionCompleted, HarnessCLIActor, CommandUser{UserID: "unknown", Name: "unknown"}, UnknownImplementer, nil)

	// Update session.json status.
	var meta SessionMeta
	_ = readJSON(filepath.Join(sessDir, "session.json"), &meta)
	meta.Status = "completed"
	meta.EndedAt = time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	_ = writeJSON(filepath.Join(sessDir, "session.json"), meta)

	// Clear current-session pointer if it points to this session.
	if m.CurrentSessionID() == id {
		_ = m.setCurrentSessionID("")
	}
	return nil
}

// ListSessions returns all session IDs in the sessions directory, newest first.
func (m *Manager) ListSessions() ([]string, error) {
	dir := filepath.Join(m.RepoPath, ".agent", "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "" {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids, nil
}

// ---------------------------------------------------------------------------
// Session — the active session handle
// ---------------------------------------------------------------------------

// Session is an active session with a writer and state.
type Session struct {
	ID          string
	Dir         string
	Writer      *EventWriter
	CmdUser     CommandUser
	Implementer Implementer
	cfg         TrackingConfig
}

func (m *Manager) newSession(id, sessDir string, cmdUser CommandUser, impl Implementer) *Session {
	w, _ := NewEventWriter(sessDir, id)
	return &Session{
		ID:          id,
		Dir:         sessDir,
		Writer:      w,
		CmdUser:     cmdUser,
		Implementer: impl,
		cfg:         m.cfg,
	}
}

// WriteEvent writes an event using the session's command user and implementer.
func (s *Session) WriteEvent(eventType string, actor Actor, metadata interface{}, opts ...EventOpt) string {
	if s == nil || s.Writer == nil {
		return ""
	}
	eid := s.Writer.Write(eventType, actor, s.CmdUser, s.Implementer, metadata, opts...)
	if s.cfg.Tracking.ReduceAfterEvent {
		s.Reduce()
	}
	return eid
}

// SetImplementer updates the active implementer for subsequent events.
func (s *Session) SetImplementer(impl Implementer) {
	if s == nil {
		return
	}
	s.Implementer = impl
}

// Reduce runs state and metrics reducers and writes state.json / metrics.json.
func (s *Session) Reduce() {
	if s == nil {
		return
	}
	eventsPath := filepath.Join(s.Dir, "events.jsonl")
	events, err := ReadEvents(eventsPath)
	if err != nil {
		return
	}
	state := ReduceState(s.ID, events)
	metrics := ReduceMetrics(s.ID, events)
	_ = writeJSON(filepath.Join(s.Dir, "state.json"), state)
	_ = writeJSON(filepath.Join(s.Dir, "metrics.json"), metrics)
}

// EventsPath returns the path to the events ledger.
func (s *Session) EventsPath() string {
	return filepath.Join(s.Dir, "events.jsonl")
}

// ArtifactPath returns a path for a large artifact, creating the dir as needed.
func (s *Session) ArtifactPath(name string) string {
	dir := filepath.Join(s.Dir, "artifacts")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, name)
}

// PhaseDir returns the path for per-phase data, creating the dir as needed.
func (s *Session) PhaseDir(phaseID string) string {
	dir := filepath.Join(s.Dir, "phases", phaseID)
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (m *Manager) sessionDir(id string) string {
	return filepath.Join(m.RepoPath, ".agent", "sessions", id)
}

func (m *Manager) setCurrentSessionID(id string) error {
	path := filepath.Join(m.RepoPath, ".agent", "keel", "current-session")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(id+"\n"), 0o644)
}

// newSessionID generates a session ID: YYYYMMDD-HHMMSS-<6-char-random>.
func newSessionID() string {
	now := time.Now().UTC()
	suffix := make([]byte, 3)
	rand.Read(suffix)
	return fmt.Sprintf("%s-%x", now.Format("20060102-150405"), suffix)
}

func writeJSON(path string, v interface{}) error {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func touchFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = os.WriteFile(path, []byte{}, 0o644)
	}
}
