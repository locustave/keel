# Releases

## v0.0.1 — 2026-06-25

Initial public release of keel.

### What's included

- `keel init` — bootstrap a governed harness into any project (PRD discovery, tech stack detection, session tracking)
- `keel plan-phases` — derive build phases from TDD deliverables via topological DAG sort
- `keel phase start|complete|failed` — phase lifecycle management
- `keel phase close` — write all phase-closing artifacts (gate, rollback DAG, ledger, audit log)
- `keel current-phase` — report phase execution state
- `keel verify` — validate harness integrity in target repository
- `keel session start` — session tracking with owner/implementer context
- `keel event append` — append events to session ledger (used by hooks)
- `keel report session|phase` — human-readable reports from session ledger
- `keel rollback` — roll back a phase and its downstream dependencies
- `keel merge-feature` — merge feature manifests into project manifest
- Embedded template with rules, commands, hooks, and scripts for Claude Code and Codex
- Homebrew distribution via `brew tap locustave/keel`
