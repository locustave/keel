# Agent Rules

## Identity

This is the repo constitution. Read it before touching any file. It governs all code agent behavior in this repository.

## Canonical Sources

| File | Purpose |
|------|---------|
| `agent-rules.md` | Repo constitution (this file) |
| `keel/README.md` | Execution harness reference |
| `BUILD_MANIFEST.yaml` | Ordered build plan |
| `docs/phases/phase_N.md` | Phase build prompts |
| `.agent/phase_gates/phase_N.gate.json` | Phase gate truth |
| `docs/PRD.md` | Product requirements |
| `docs/TDD.md` | Technical design |
| `DESIGN.md` | Interface design |

`HARNESS_EXECUTION_PLAN.yaml` is retired. Do not generate, read, or update it.

## Before Editing Any File

1. Read `agent-rules.md` (this file).
2. Read `keel/README.md`.
3. Read `BUILD_MANIFEST.yaml`.
4. Read every file under `keel/rules/`.
5. Read only the requested `docs/phases/phase_N.md`.
6. Scan `.agent/phase_gates/` to confirm all prior phases have passed and the requested phase is next.
7. Capture pre-phase status under `.agent/snapshots/`.

## Execution Rules

- Run exactly one phase at a time with `/keel-run N`.
- Do not implement scope outside the requested phase.
- Do not modify `docs/PRD.md`, `docs/TDD.md`, `DESIGN.md`, `BUILD_MANIFEST.yaml`, or other phase files during product phases.
- Confirm all prior phases have passed gates before starting the next phase.
- Write audit, ledger, run-log, and gate records at the end of every phase.

## Harness Governance

- Phase state is determined by reading `BUILD_MANIFEST.yaml` and scanning `.agent/phase_gates/`.
- Harness governance repair may update `agent-rules.md`, `harness/**`, `scripts/**`, `docs/phases/**`, `BUILD_MANIFEST.yaml`, and `.agent/**` when explicitly requested by the human.

## Feature Workflow

1. `/new-feature "feature name"` — draft feature workspace under `docs/features/{slug}/`.
2. Human reviews and edits.
3. `/approve-feature {slug}` — merge feature phases into `BUILD_MANIFEST.yaml` and generate phase build files.
4. `/keel-run N` — execute as normal.

## Tech Stack

> Populated during Phase 0. All phases and `/new-feature` must use this stack.
> Until Phase 0 completes, this section is a placeholder — run `/keel-run 0` to populate it.

| Dimension | Decision |
|-----------|----------|
| Language / runtime | — |
| Data storage | — |
| Frontend / UI | — |
| Test runner | — |
| Deployment target | — |

## Stop Conditions

Stop and report instead of guessing when:

- A required canonical file is missing.
- The requested phase does not exist in `BUILD_MANIFEST.yaml`.
- A prior phase is missing a passed gate.
- The requested phase is not the next unpassed phase, unless the human explicitly requested a governance correction.
- Required changes touch blocked paths.
- Tests fail for unrelated reasons.
- Network access, external credentials, remote pushes, or production deployment would be required.
