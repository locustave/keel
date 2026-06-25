# /keel-create

Generate the full controlled-execution harness for this repository.

Run this command after `keel init` has scaffolded the file structure. It reads the project's `docs/PRD.md` and `docs/TDD.md`, derives the phase plan, and generates all harness artifacts that require product understanding. It does not implement product code.

---

## Prerequisites

Confirm these files exist before proceeding. Stop and report any that are missing:

- `docs/PRD.md`
- `docs/TDD.md`
- `docs/TDD.md` must contain a `## Deliverables` fenced YAML block
- `harness/` directory (created by `keel init`)
- `BUILD_MANIFEST.yaml` stub (created by `keel init`)

If `keel init` has not been run, stop and tell the human to run it first.

---

## Steps

Execute in order:

### Step 1 — Derive phase plan

Run the planner in dry-run mode and show the output:

```bash
keel plan-phases --tdd docs/TDD.md
```

Review the proposed phases. If the output looks correct, run with `--confirm` to write `BUILD_MANIFEST.yaml`:

```bash
keel plan-phases --tdd docs/TDD.md --confirm
```

If `plan-phases` exits non-zero (missing `## Deliverables`, cycle detected, etc.), stop and report the error. Do not continue to Step 2.

### Step 2 — Generate phase build files

Read `BUILD_MANIFEST.yaml` to get the full phase list. For every phase, generate `docs/phases/phase_N.md`.

Each phase file must include:

- `## Phase Goal` — one paragraph describing what this phase delivers
- `## Manifest Tasks` — numbered list from the manifest's `tasks:` field
- `## Allowed Paths` — files this phase may create or modify
- `## Blocked Paths` — files this phase must not touch
- `## Tasks` — red/green/refactor loop for each task
- `## Verification Commands` — bash commands to confirm exit criteria
- `## Drift Controls` — YAML block with `allowed_paths`, `blocked_paths`, `checkpoint_after`, `stop_if`
- `## Stop Conditions` — explicit conditions that halt the phase
- `## Exit Criteria` — verifiable pass/fail checklist
- `## Out of Scope` — explicit exclusions

Do not generate phase files for phases that already have a passed gate in `.agent/phase_gates/`.

### Step 3 — Update phase_0.prompt.md

Rewrite `docs/phases/phase_0.prompt.md` to reflect the actual project — replace the generic placeholder content with a summary derived from `docs/PRD.md` and `docs/TDD.md`.

### Step 4 — Write phase 0 harness build file

Generate `docs/phases/phase_0.md` describing the harness bootstrap phase itself (the work this command just performed).

---

## Rules

- Do not implement product code.
- Do not modify `docs/PRD.md` or `docs/TDD.md`.
- Do not modify files under `keel/rules/` or `keel/commands/`.
- Do not re-run `keel init` — the file structure is already in place.
- If `plan-phases` exits non-zero, stop. Do not hand-write `BUILD_MANIFEST.yaml`.
- If any phase build file already exists and its gate is passed, skip regeneration for that phase.

---

## Final Report

End by reporting:

```text
Harness created.

Phases planned : {N} (from BUILD_MANIFEST.yaml)
phase files written:
  docs/phases/phase_0.md
  docs/phases/phase_1.md
  ...

Next: run /keel-run 1 to begin implementation.
```

This command is complete. Do not run `/keel-run` automatically. Wait for an explicit human instruction.
