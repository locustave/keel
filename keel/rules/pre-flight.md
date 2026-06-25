# Harness Rule: Pre-Flight

Before executing any phase, command, gate, retry, rollback, or implementation task, complete pre-flight.

## Required Reads

- `agent-rules.md`
- `keel/README.md`
- `BUILD_MANIFEST.yaml`
- all files under `keel/rules/`
- requested `docs/phases/phase_N.md`
- `DESIGN.md` before frontend work

Determine phase state from `BUILD_MANIFEST.yaml` and `.agent/phase_gates/`. Do not read `HARNESS_EXECUTION_PLAN.yaml` — it is retired.

## Required Checks

1. Confirm the requested phase exists in `BUILD_MANIFEST.yaml`.
2. Confirm the requested phase file exists under `docs/phases/`.
3. Confirm the requested phase file number matches the requested phase.
4. Confirm prior phases have passed gates unless the human explicitly requested governance repair.
5. Confirm the requested phase is not blocked by an unresolved manifest blocker.
6. Inspect `git status --short`.
7. Capture pre-phase status and diff under `.agent/snapshots/`.
8. Confirm the phase allowed paths and blocked paths do not overlap.
9. Confirm required tooling is available or record the missing tool before verification.

## Stop Conditions

Stop before editing when:

- a required canonical file is missing,
- the requested phase is absent,
- prior gates are missing or failed,
- the requested phase is not the next unpassed phase,
- the phase file conflicts with `BUILD_MANIFEST.yaml`,
- required edits would touch blocked paths,
- production credentials or external publication are required.
