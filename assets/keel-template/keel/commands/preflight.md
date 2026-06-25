# Preflight Command

Purpose: capture local repository context before running a manifest phase.

Command:

```bash
bash keel/hooks/preflight.sh
```

Expected effects:

- Creates or refreshes `.agent/preflight_context.md`.
- Appends a `preflight.completed` event to `.agent/audit.jsonl`.
- Records discovered tool availability without installing dependencies.
- Leaves product code untouched.