# Codex Command Adapters

This directory contains Codex-compatible command prompt documents for the repository harness.

Codex does not use Claude's `$ARGUMENTS` substitution or `.claude/settings*.json` permission model. These adapters explain how Codex should extract arguments from the user message, then delegate to the canonical harness command documents under `keel/commands/`.

## Commands

- `commands/keel-run.md`: Codex adapter for `/keel-run N`.

## Canonical Sources

The canonical command behavior remains in `keel/commands/`. Files under `.codex/commands/` are adapters, not independent sources of truth.
