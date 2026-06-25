# keel/README.md

This is the canonical execution harness for this repository. Code agents must use it with `agent-rules.md`, `BUILD_MANIFEST.yaml`, the current file under `docs/phases/`, and every rule file under `keel/rules/`.

## Canonical Sources

Use these names exactly:

- Repo constitution: `agent-rules.md`
- Execution harness: `keel/README.md`
- Ordered build plan: `BUILD_MANIFEST.yaml`
- Phase build prompts: `docs/phases/phase_N.md`
- Phase gate truth: `.agent/phase_gates/phase_N.gate.json`
- Product source of truth: `docs/PRD.md`
- Technical source of truth: `docs/TDD.md`
- Frontend design source of truth: `DESIGN.md`

`HARNESS_EXECUTION_PLAN.yaml` is retired. Do not generate, read, or update it. Phase state is determined by reading `BUILD_MANIFEST.yaml` and scanning `.agent/phase_gates/`.

There is no repo-root `HARNESS.md`, no repo-root `BUILD_MANIFEST.md`, and no `phase_prompts/` directory in the active harness model. If a stale instruction references those names, prefer the canonical names above and update the stale instruction as harness maintenance.

## Repository Layout

Expected top-level structure:

```text
agent-rules.md
BUILD_MANIFEST.yaml
DESIGN.md
backend/
web/
runtime/
deploy/
docs/
  PRD.md
  TDD.md
  BUILD_MANIFEST.yaml
  phases/
  audit/
  build-ledger/
  decisions/
  features/
    {slug}/
      PRD.md
      TDD.md
      BUILD_MANIFEST.yaml
      STATUS.md
harness/
  commands/
  hooks/
  rules/
  scripts/
scripts/
.agent/
```

`web/` is the frontend directory. `runtime/` is the generated customer-hosted MCP runtime package. Do not create a parallel `frontend/` tree.

## Feature Workflow

New features follow a draft-then-approve flow so they can be reviewed before entering the build queue:

1. `/new-feature "feature name"` — drafts `docs/features/{slug}/` with scoped PRD, TDD, BUILD_MANIFEST, and STATUS files. Does not touch the project manifest.
2. Human reviews and edits the feature workspace.
3. `/approve-feature {slug}` — merges feature phases into `BUILD_MANIFEST.yaml`, renumbers them sequentially after the current last phase, updates `STATUS.md`, and generates phase build files. Reports the first ready phase number.
4. `/keel-run N` — executes as normal. The harness is unaware that the phase originated from a feature.

Feature workspaces are the authoring layer. The project `BUILD_MANIFEST.yaml` is the execution source of truth.

## Phase Execution

Run exactly one phase at a time with `/keel-run N`.

Before editing files, the agent must:

1. Read `agent-rules.md`.
2. Read `keel/README.md`.
3. Read `BUILD_MANIFEST.yaml`.
4. Read every file under `keel/rules/`.
5. Read only the requested `docs/phases/phase_N.md`.
6. Scan `.agent/phase_gates/` to confirm all prior phases have passed gates and the requested phase is the next unpassed phase, unless the human explicitly requested a governance correction.
7. Capture pre-phase status and diff under `.agent/snapshots/`.

During execution, implement only the requested phase. Do not change PRD, TDD, manifest, or other phase files during normal product phases. Do not read or update `HARNESS_EXECUTION_PLAN.yaml` — it is retired.

## Verification

Verification order:

1. Run every exit criterion listed for the current phase in `BUILD_MANIFEST.yaml`.
2. Run `bash scripts/verify.sh --phase N` when available.
3. Run `bash scripts/ci.sh` only when the phase scope makes full local CI appropriate.

Focused helpers:

- `bash scripts/verify_phase0.sh` validates the Phase 0 harness generation.
- `bash scripts/verify.sh` validates repository-wide harness consistency.
- `bash scripts/verify.sh --phase N` validates repository-wide consistency plus phase-gate shape for `N`.

Do not mark a phase passed unless the manifest exit criteria for that phase passed. A harness-only verification is not a substitute for manifest exit criteria.

## Gates

Passed gates live at `.agent/phase_gates/phase_N.gate.json`.

Failed gates live at `.agent/phase_gates/phase_N.failed.json`.

A passed gate must include:

- `phase`
- `name`
- `status: "passed"`
- `timestamp_utc`
- `git_sha_before`
- `git_sha_after`
- `files_changed`
- `commands`
- `exit_criteria_results`
- `self_verification`
- `known_followups`
- `next_phase`

Every manifest exit criterion must appear in `exit_criteria_results` and must have `passed: true`. If an environment cannot run a criterion, write a failed gate instead of a passed gate.

## Audit And Ledgers

Every phase must update:

- `docs/audit/phase_N.log`
- `docs/build-ledger/phase_N_build.md`
- `.agent/audit.jsonl`
- `.agent/run_log.jsonl`

If no architecture decision was made, the build ledger must say that no ADR was created. If an architecture decision was made, create an ADR under `docs/decisions/` and reference it from the build ledger.

## Retry And Rollback

Retry only failures caused by the current phase. Record each retry in the same phase build ledger and `.agent/audit.jsonl`.

Rollback must preserve audit logs, build ledgers, ADRs, and failed gates. Do not use destructive git commands unless the human explicitly requests them.

## CI Policy

Local bash scripts under `scripts/` are the authoritative CI model. Hosted workflow files under `.github/workflows/` are allowed only as thin wrappers around local scripts when the manifest requires workflow stubs. They must not define different checks from local scripts.

## Package Managers

- Backend: `uv`
- Runtime: `uv`
- Frontend: `pnpm --dir web ...`

Do not introduce npm, Yarn, Poetry, or Pipenv lockfiles.

## Drift Controls

Drift tooling is optional. If unavailable, continue with harness-only controls and record: `Drift not available; proceeding with harness-only controls.`

If Drift is available:

- Start or reuse a phase-specific session.
- Use the current phase allowed paths.
- Checkpoint after each green task and after final verification.
- Stop if Drift reports red or if a modified path is outside the phase scope.

## Stop Conditions

Stop and report instead of guessing when:

- Required canonical files are missing.
- The requested phase does not exist.
- A prior phase failed or is missing a required passed gate.
- The requested phase is not the next unpassed phase, unless the human explicitly requested a governance correction.
- The phase file conflicts with the manifest.
- Required changes touch blocked paths.
- Tests fail for unrelated reasons.
- Production credentials, package publishing, remote pushes, or production deployment would be required.
- A manifest blocker prevents safe execution.
