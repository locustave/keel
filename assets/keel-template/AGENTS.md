# AGENTS.md — Codex Agent Instructions

This file is the Codex-style agent constitution for this repository. It mirrors `agent-rules.md` with Codex-specific execution guidance.

## Start Every Session

Read these files before touching anything:

1. `agent-rules.md`
2. `keel/README.md`
3. `BUILD_MANIFEST.yaml`
4. All files under `keel/rules/`
5. The requested `docs/phases/phase_N.md`

## Execution

- Use `/keel-run N` to execute a phase.
- Use `keel plan-phases --confirm` to regenerate phase build files from `BUILD_MANIFEST.yaml`.
- Use `/new-feature "name"` to draft a feature workspace.
- Use `/approve-feature {slug}` to merge a feature into the build queue.

## Phase Discipline

- Implement exactly one phase per session.
- Do not read ahead into future phase build files.
- Do not modify `BUILD_MANIFEST.yaml`, `docs/PRD.md`, `docs/TDD.md`, or `DESIGN.md` during product phases.
- Write gate, audit, ledger, and run-log records before marking a phase complete.

## Canonical File Names

| Purpose | Path |
|---------|------|
| Repo constitution | `agent-rules.md` |
| Harness reference | `keel/README.md` |
| Build plan | `BUILD_MANIFEST.yaml` |
| Phase build | `docs/phases/phase_N.md` |
| Phase gate | `.agent/phase_gates/phase_N.gate.json` |

`HARNESS_EXECUTION_PLAN.yaml` is retired. Do not reference it.

## Stop Conditions

Stop and report when:

- Required canonical files are missing.
- The requested phase is not the next unpassed phase.
- A prior phase gate is missing or failed.
- The phase file conflicts with the manifest.
- Tests fail for reasons outside the current phase scope.
