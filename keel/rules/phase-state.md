# Harness Rule: Phase State

The gate files under `.agent/phase_gates/` define phase execution state.

## Detection

1. Read every `.agent/phase_gates/phase_*.gate.json`.
2. Treat only `status: "passed"` as complete.
3. The next phase is the lowest manifest phase without a passed gate.
4. A `.failed.json` for the next phase blocks forward progress until retried or rolled back.

## Current Repository State

This repository contains a passed gate for Phase 0 after harness generation. Unless that gate is explicitly rolled back, the next implementation phase is Phase 1.

## Governance Repair Exception

When the human explicitly asks for harness governance repair, the agent may edit harness planning, rules, gates, ledgers, and scaffold files needed to reconcile state. The repair must not add product business logic.
