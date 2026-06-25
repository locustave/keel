# Phase 0 — Controlled Execution Harness Bootstrap

## Objective

Generate the controlled execution harness for this repository. Do not build the product. This phase creates only the harness, rules, hooks, commands, verification helpers, build manifest, execution plan, and phase build prompts needed for later agents to build the product under phase control.

## Inputs

- `docs/PRD.md` — product requirements (required)
- `DESIGN.md` — interface design (optional, read if present)
- `agent-rules.md` — repo constitution (already present)
- `keel/README.md` — harness reference (already present)

## Required Outputs

Generate or update:

- `docs/TDD.md` — technical design (generated from docs/PRD.md via tdd-builder procedure)
- `BUILD_MANIFEST.yaml` — full product build manifest with all phases
- `docs/phases/phase_0.md` — this file (update if needed)
- `docs/phases/phase_<N>.md` — one file per manifest phase
- `docs/audit/phase_0.log` — phase 0 audit log
- `docs/build-ledger/phase_0_build.md` — phase 0 build ledger
- `docs/decisions/` — ADRs if any architectural decisions are made
- `.agent/audit.jsonl` — append audit event
- `.agent/run_log.jsonl` — append run log event
- `.agent/phase_gates/phase_0.gate.json` — passed gate record

## Steps

1. Read `docs/PRD.md`.
2. Check whether `docs/TDD.md` exists.
   - If **missing**: follow the tdd-builder procedure in `keel/rules/tdd-builder.md` (ask tech stack questions, generate `docs/TDD.md`, write `## Tech Stack` to `agent-rules.md`). Do not proceed until this is complete.
   - If **present**: read it, then confirm the `## Tech Stack` section in `agent-rules.md` is populated. If the section still contains placeholder dashes (`—`), extract the stack from `docs/TDD.md` and update `agent-rules.md` now.
3. Read `DESIGN.md` if it exists.
4. Run `keel plan-phases --dry-run` (or review `BUILD_MANIFEST.yaml` stub).
5. Generate the full `BUILD_MANIFEST.yaml` from the product sources.
6. Run `keel plan-phases --confirm` to generate all `docs/phases/phase_N.md` files.
7. Update `docs/phases/phase_0.md` to reflect the actual plan.
8. Run harness verification:
   ```bash
   bash keel/hooks/preflight.sh
   python3 keel/scripts/verify_repo.py .
   bash scripts/verify_phase0.sh
   ```
9. Write phase 0 gate, audit, and ledger records.

## Exit Criteria

- `bash keel/hooks/preflight.sh` exits 0.
- `python3 keel/scripts/verify_repo.py .` exits 0.
- `bash scripts/verify_phase0.sh` exits 0.
- `docs/TDD.md` exists and is non-empty.
- `agent-rules.md` `## Tech Stack` section contains no placeholder dashes (`—`).
- `BUILD_MANIFEST.yaml` contains all product phases derived from `docs/PRD.md` and `docs/TDD.md`.
- One `docs/phases/phase_N.md` file exists for every phase in `BUILD_MANIFEST.yaml`.
- `.agent/phase_gates/phase_0.gate.json` has `status: "passed"`.

## Out of Scope

Do not create:

- Product backend, web, frontend, runtime, database, compose, worker, or UI files.
- Dependency install files (`go.sum`, `package-lock.json`, virtual environments).
- Product tests or product build artifacts.

## Stop Conditions

Stop and ask the human if:

- `docs/PRD.md` is missing or empty.
- `docs/TDD.md` is missing **and** the tech stack cannot be determined from the PRD alone after one follow-up question.
- Requirements conflict and cannot be resolved by reading existing documents.
- Product files would need to be created to satisfy an exit criterion.
- Dependency installs or product tests would be required.
