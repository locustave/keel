package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// HookInstaller creates hook scripts in .agent/hooks/ and registers them
// in .claude/settings.json for Claude Code.
type HookInstaller struct {
	RepoPath string
}

// Install creates all hook scripts under .agent/hooks/.
func (h *HookInstaller) Install() error {
	hooksDir := filepath.Join(h.RepoPath, ".agent", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	hooks := map[string]string{
		"pre-tool":     preToolHook,
		"post-tool":    postToolHook,
		"notification": notificationHook,
		"stop":         stopHook,
	}

	for name, content := range hooks {
		path := filepath.Join(hooksDir, name)
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			return fmt.Errorf("write hook %s: %w", name, err)
		}
	}
	return nil
}

// InstallClaudeSettings writes or updates .claude/settings.json to register
// keel hooks with Claude Code. Existing settings keys are preserved; only the
// "hooks" key is replaced.
func (h *HookInstaller) InstallClaudeSettings() error {
	settingsDir := filepath.Join(h.RepoPath, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")

	// Read existing settings if present.
	existing := map[string]interface{}{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &existing) // preserve other keys; ignore parse errors
	}

	hookEntry := func(script string) map[string]interface{} {
		return map[string]interface{}{
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": script,
				},
			},
		}
	}

	existing["hooks"] = map[string]interface{}{
		"PreToolUse":  []interface{}{hookEntry(".agent/hooks/pre-tool")},
		"PostToolUse": []interface{}{hookEntry(".agent/hooks/post-tool")},
		"Stop":        []interface{}{hookEntry(".agent/hooks/stop")},
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write .claude/settings.json: %w", err)
	}
	return nil
}

// InstallCodexSettings writes or updates .codex/setup.sh to source keel hooks.
// Codex runs setup.sh at the start of each session. We use it to register
// hook paths as environment variables that Codex can invoke.
// Note: Codex doesn't have the same hook protocol as Claude Code (no
// PreToolUse/PostToolUse/Notification events). Instead, keel's Codex
// integration relies on the agent calling `keel event append` directly
// or on the AGENTS.md instructions to use keel CLI commands.
func (h *HookInstaller) InstallCodexSettings() error {
	codexDir := filepath.Join(h.RepoPath, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return fmt.Errorf("create .codex dir: %w", err)
	}

	// Write a setup.sh that exports KEEL_HOOKS_DIR so scripts can find hooks.
	setupPath := filepath.Join(codexDir, "setup.sh")
	content := `#!/bin/sh
# keel: Codex session setup
# Exports hook paths for keel event tracking.
export KEEL_HOOKS_DIR=".agent/hooks"
export KEEL_TRACKING="enabled"
`
	return os.WriteFile(setupPath, []byte(content), 0o755)
}

// ---------------------------------------------------------------------------
// Hook script templates
// ---------------------------------------------------------------------------

// All hooks read a JSON payload from stdin (Claude Code's hook protocol).
// They call `keel event append` and are fail-open: always exit 0 so they
// never block agent execution.

const preToolHook = `#!/bin/sh
# keel hook: PreToolUse
# Called by Claude Code / Codex before every tool invocation.

PAYLOAD=$(cat)
TMPFILE=$(mktemp)
printf '%s' "$PAYLOAD" > "$TMPFILE"

RESULT=$(python3 - "$TMPFILE" <<'PYEOF'
import sys, json, os
with open(sys.argv[1]) as f:
    d = json.load(f)
os.unlink(sys.argv[1])
tool = d.get("tool_name", "unknown")
sid  = d.get("session_id", "")
cwd  = d.get("cwd", ".")
print(f"{tool}|{sid}|{cwd}")
PYEOF
) || RESULT="unknown||."

TOOL=$(echo "$RESULT" | cut -d'|' -f1)
SESSION=$(echo "$RESULT" | cut -d'|' -f2)
REPO=$(echo "$RESULT" | cut -d'|' -f3)

keel event append \
  --repo "$REPO" \
  --type tool_call_started \
  --actor-type hook \
  --actor-name pre-tool \
  --tool-name "$TOOL" \
  --metadata '{"tool":"'"$TOOL"'","session_id":"'"$SESSION"'"}' 2>/dev/null || true

exit 0
`

const postToolHook = `#!/bin/sh
# keel hook: PostToolUse
# Called by Claude Code / Codex after every tool invocation completes.
# Derives file_edit_completed and test_passed/test_failed from tool data.

PAYLOAD=$(cat)

# Write payload to temp file to avoid shell escaping issues.
TMPFILE=$(mktemp)
printf '%s' "$PAYLOAD" > "$TMPFILE"

RESULT=$(python3 - "$TMPFILE" <<'PYEOF'
import sys, json, os
with open(sys.argv[1]) as f:
    d = json.load(f)
os.unlink(sys.argv[1])

tool = d.get("tool_name", "unknown")
sid  = d.get("session_id", "")
cwd  = d.get("cwd", ".")
resp = d.get("tool_response", {})
inp  = d.get("tool_input", {})
is_err = "1" if isinstance(resp, dict) and resp.get("is_error") else "0"

is_test = "0"
if tool == "Bash":
    cmd = ""
    if isinstance(inp, dict):
        cmd = inp.get("command", "")
    elif isinstance(inp, str):
        cmd = inp
    test_patterns = ["go test", "pytest", "jest", "vitest", "npm test", "yarn test",
                     "pnpm test", "cargo test", "dotnet test", "rspec", "mix test",
                     "flutter test", "bun test", "make test", "verify.sh", "verify_phase"]
    for p in test_patterns:
        if p in cmd:
            is_test = "1"
            break

# Output: tool|session|cwd|is_error|is_test  (pipe-delimited, single line)
print(f"{tool}|{sid}|{cwd}|{is_err}|{is_test}")
PYEOF
) || RESULT="unknown||.|0|0"

TOOL=$(echo "$RESULT" | cut -d'|' -f1)
SESSION=$(echo "$RESULT" | cut -d'|' -f2)
REPO=$(echo "$RESULT" | cut -d'|' -f3)
IS_ERROR=$(echo "$RESULT" | cut -d'|' -f4)
IS_TEST=$(echo "$RESULT" | cut -d'|' -f5)

# 1. Emit tool_call_completed / tool_call_failed
if [ "$IS_ERROR" = "1" ]; then
  EVENT_TYPE="tool_call_failed"
else
  EVENT_TYPE="tool_call_completed"
fi

keel event append \
  --repo "$REPO" \
  --type "$EVENT_TYPE" \
  --actor-type hook \
  --actor-name post-tool \
  --tool-name "$TOOL" \
  --metadata '{"tool":"'"$TOOL"'","session_id":"'"$SESSION"'","is_error":'"$IS_ERROR"'}' 2>/dev/null || true

# 2. Emit file_edit_completed for Edit / Write tools
case "$TOOL" in
  Edit|Write|NotebookEdit)
    if [ "$IS_ERROR" != "1" ]; then
      keel event append \
        --repo "$REPO" \
        --type file_edit_completed \
        --actor-type hook \
        --actor-name post-tool \
        --tool-name "$TOOL" \
        --metadata '{"tool":"'"$TOOL"'"}' 2>/dev/null || true
    fi
    ;;
esac

# 3. Emit test_passed / test_failed for Bash test commands
if [ "$IS_TEST" = "1" ]; then
  if [ "$IS_ERROR" = "1" ]; then
    keel event append \
      --repo "$REPO" \
      --type test_failed \
      --actor-type hook \
      --actor-name post-tool \
      --tool-name "$TOOL" \
      --metadata '{"tool":"Bash","source":"hook_inferred"}' 2>/dev/null || true
  else
    keel event append \
      --repo "$REPO" \
      --type test_passed \
      --actor-type hook \
      --actor-name post-tool \
      --tool-name "$TOOL" \
      --metadata '{"tool":"Bash","source":"hook_inferred"}' 2>/dev/null || true
  fi
fi

exit 0
`

const notificationHook = `#!/bin/sh
# keel hook: Notification
# Claude Code Notification hook — receives message notifications only.
# Token tracking is handled by the stop hook via transcript_path.
exit 0
`

const stopHook = `#!/bin/sh
# keel hook: Stop
# Called by Claude Code when the agent stops responding (end of turn).
# Also reads transcript_path to extract token usage from LLM calls.

PAYLOAD=$(cat)
TMPFILE=$(mktemp)
printf '%s' "$PAYLOAD" > "$TMPFILE"

python3 - "$TMPFILE" <<'PYEOF'
import sys, json, os, subprocess

with open(sys.argv[1]) as f:
    d = json.load(f)
os.unlink(sys.argv[1])

sid = d.get("session_id", "")
cwd = d.get("cwd", ".")
transcript = d.get("transcript_path", "")

# 1. Emit session_completed
subprocess.run(["keel", "event", "append",
    "--repo", cwd,
    "--type", "session_completed",
    "--actor-type", "hook",
    "--actor-name", "stop",
    "--metadata", json.dumps({"session_id": sid})],
    capture_output=True)

# 2. Extract token usage from transcript (if available)
if not transcript or not os.path.isfile(transcript):
    sys.exit(0)

# Track offset so we only process new lines each time stop fires.
agent_dir = os.path.join(cwd, ".agent")
offset_file = os.path.join(agent_dir, ".transcript_offset")

last_offset = 0
if os.path.isfile(offset_file):
    try:
        last_offset = int(open(offset_file).read().strip())
    except (ValueError, OSError):
        last_offset = 0

with open(transcript) as f:
    lines = f.readlines()

new_lines = lines[last_offset:]

# Save new offset
try:
    os.makedirs(agent_dir, exist_ok=True)
    with open(offset_file, "w") as f:
        f.write(str(len(lines)))
except OSError:
    pass

# Process each line for LLM usage data
for line in new_lines:
    line = line.strip()
    if not line:
        continue
    try:
        rec = json.loads(line)
    except json.JSONDecodeError:
        continue

    msg = rec.get("message", {})
    if not isinstance(msg, dict):
        continue
    usage = msg.get("usage")
    if not usage or not isinstance(usage, dict):
        continue

    input_t  = int(usage.get("input_tokens", 0))
    output_t = int(usage.get("output_tokens", 0))
    cache_r  = int(usage.get("cache_read_input_tokens", 0))
    cache_w  = int(usage.get("cache_creation_input_tokens", 0))
    total    = input_t + output_t
    if total == 0:
        continue

    model = msg.get("model", "")
    tu = json.dumps({
        "input_tokens": input_t,
        "output_tokens": output_t,
        "cache_read_tokens": cache_r,
        "cache_write_tokens": cache_w,
        "total_tokens": total,
        "estimated": False
    })

    subprocess.run(["keel", "event", "append",
        "--repo", cwd,
        "--type", "llm_call_completed",
        "--actor-type", "hook",
        "--actor-name", "stop",
        "--token-usage", tu,
        "--metadata", json.dumps({"source": "transcript", "model": model, "session_id": sid})],
        capture_output=True)
PYEOF

exit 0
`
