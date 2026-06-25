// Package session implements local-first session tracking, event ledger
// management, identity resolution, state/metrics reduction, token tracking,
// hook installation, and report generation for the harness CLI.
//
// Design principles:
//   - The event ledger (.agent/sessions/<id>/events.jsonl) is the source of truth.
//   - State and metrics are derived deterministically from the ledger.
//   - All event writes include actor, command_user, and implementer context.
//   - Tracking is best-effort: errors in observability never fail the main command.
//   - MCP is out of scope; this is local CLI + hooks only.
package session

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Config types
// ---------------------------------------------------------------------------

// TrackingConfig is the full content of .agent/keel/config.yaml.
type TrackingConfig struct {
	User          UserConfig          `yaml:"user"`
	Tracking      TrackingBlock       `yaml:"tracking"`
	TokenTracking TokenTrackingConfig `yaml:"token_tracking"`
}

// UserConfig stores the session owner's identity.
type UserConfig struct {
	Name  *string `yaml:"name"`
	Email *string `yaml:"email"`
	ID    *string `yaml:"id"`
}

// TrackingBlock controls what is captured and how.
type TrackingBlock struct {
	Enabled              bool           `yaml:"enabled"`
	Mode                 string         `yaml:"mode"` // "local" | "off"
	IncludeUserMetadata  bool           `yaml:"include_user_metadata"`
	EventLogPath         string         `yaml:"event_log_path"`
	StatePath            string         `yaml:"state_path"`
	MetricsPath          string         `yaml:"metrics_path"`
	Capture              CaptureConfig  `yaml:"capture"`
	ReduceAfterEvent     bool           `yaml:"reduce_after_event"`
	WritePhaseOnComplete bool           `yaml:"write_phase_summary_on_completion"`
}

// CaptureConfig selects which event categories are captured.
type CaptureConfig struct {
	Sessions           bool `yaml:"sessions"`
	Phases             bool `yaml:"phases"`
	Commands           bool `yaml:"commands"`
	Tools              bool `yaml:"tools"`
	FileChanges        bool `yaml:"file_changes"`
	GitDiff            bool `yaml:"git_diff"`
	Tests              bool `yaml:"tests"`
	DebugLoops         bool `yaml:"debug_loops"`
	TokenUsage         bool `yaml:"token_usage"`
	ActorContext       bool `yaml:"actor_context"`
	UserContext        bool `yaml:"user_context"`
	ImplementerContext bool `yaml:"implementer_context"`
}

// TokenTrackingConfig controls how token usage is tracked.
type TokenTrackingConfig struct {
	Mode     string              `yaml:"mode"` // "exact_or_estimated"
	Fallback TokenFallbackConfig `yaml:"fallback"`
}

// TokenFallbackConfig controls estimated token behavior.
type TokenFallbackConfig struct {
	EstimateTokens bool `yaml:"estimate_tokens"`
	MarkEstimated  bool `yaml:"mark_estimated"`
}

// ---------------------------------------------------------------------------
// Default config
// ---------------------------------------------------------------------------

// DefaultConfig returns the default TrackingConfig.
func DefaultConfig() TrackingConfig {
	return TrackingConfig{
		User: UserConfig{},
		Tracking: TrackingBlock{
			Enabled:              true,
			Mode:                 "local",
			IncludeUserMetadata:  true,
			EventLogPath:         ".agent/sessions/{session_id}/events.jsonl",
			StatePath:            ".agent/sessions/{session_id}/state.json",
			MetricsPath:          ".agent/sessions/{session_id}/metrics.json",
			ReduceAfterEvent:     true,
			WritePhaseOnComplete: true,
			Capture: CaptureConfig{
				Sessions:           true,
				Phases:             true,
				Commands:           true,
				Tools:              true,
				FileChanges:        true,
				GitDiff:            true,
				Tests:              true,
				DebugLoops:         true,
				TokenUsage:         true,
				ActorContext:       true,
				UserContext:        true,
				ImplementerContext: true,
			},
		},
		TokenTracking: TokenTrackingConfig{
			Mode: "exact_or_estimated",
			Fallback: TokenFallbackConfig{
				EstimateTokens: true,
				MarkEstimated:  true,
			},
		},
	}
}

// DisabledConfig returns a config with tracking disabled.
func DisabledConfig() TrackingConfig {
	c := DefaultConfig()
	c.Tracking.Enabled = false
	c.Tracking.Mode = "off"
	return c
}

// ---------------------------------------------------------------------------
// Load / Save
// ---------------------------------------------------------------------------

// ConfigPath returns the canonical config file path for a repo.
func ConfigPath(repoPath string) string {
	return filepath.Join(repoPath, ".agent", "keel", "config.yaml")
}

// LoadConfig reads the tracking config from the repo. Returns the default
// config (with tracking enabled) if the file does not exist.
func LoadConfig(repoPath string) (TrackingConfig, error) {
	path := ConfigPath(repoPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), err
	}
	var cfg TrackingConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}
	return cfg, nil
}

// SaveConfig writes the tracking config to the repo.
func SaveConfig(repoPath string, cfg TrackingConfig) error {
	path := ConfigPath(repoPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// IsEnabled returns true if tracking is enabled and mode is "local".
func (cfg TrackingConfig) IsEnabled() bool {
	return cfg.Tracking.Enabled && cfg.Tracking.Mode == "local"
}
