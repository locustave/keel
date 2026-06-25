# /keel-run

Run exactly one requested build phase.

This command executes a single phase from `BUILD_MANIFEST.yaml` using the matching phase phase build file.

The requested phase number must come from the slash-command argument.

Examples:

```text
/keel-run 1
/keel-run 2
/keel-run 3
```

---

## Requested Phase

Requested phase argument:

```text
$ARGUMENTS
```

If no phase argument is provided, stop and ask for a phase number.

Do not guess.
Do not default to Phase 1 unless the user explicitly requested Phase 1.

Normalize the requested phase as:

```text
PHASE_NUMBER=<argument>
PHASE_NAME=Phase <argument>
PHASE_FILE=docs/phases/phase_<argument>.md
```

Example:

```text
/keel-run 2
```

means:

```text
PHASE_NUMBER=2
PHASE_NAME=Phase 2
PHASE_FILE=docs/phases/phase_2.md
```

---

## Read

Before implementing anything, read:

- `agent-rules.md`
- `keel/README.md`
- `BUILD_MANIFEST.yaml`
-  all files under `keel/rules/*`
- `docs/phases/phase_<PHASE_NUMBER>.md`

Only read the phase file for the requested phase.

Do not read or execute future phase files unless they are needed only to confirm that they must not be touched.

---

## Mandatory Harness Rule Check

Before modifying files, read and execute all rules under:

- `keel/rules/*`

---

## Planning Freshness Check

Before execution, verify that the phase build file is not stale.

Compare the requested phase file against:

- `BUILD_MANIFEST.yaml` — confirm the phase tasks and exit criteria still match
- `docs/PRD.md` and `docs/TDD.md` — confirm no material changes have invalidated the phase scope

`HARNESS_EXECUTION_PLAN.yaml` is retired. Do not read or compare against it.

If the requested phase file appears stale or conflicts with the manifest, stop and report:

```text
Phase build file may be stale. Run `keel plan-phases --confirm` to regenerate before executing this phase.
```

Do not continue execution against stale planning files.

---

## Phase Existence Checks

Before implementation, validate:

- The requested phase number exists in `BUILD_MANIFEST.yaml`.
- `docs/phases/phase_<PHASE_NUMBER>.md` exists.
- The phase harness file matches the requested phase number.
- The requested phase is not marked blocked.
- The requested phase does not require unresolved production blockers.

If any check fails, stop and report the reason.

Emit the phase start event:

```bash
keel phase start <PHASE_NUMBER>
```

---

## Rules

- Implement only the requested phase.
- Do not implement previous phases unless the requested phase file explicitly lists them as required prerequisites and they are incomplete.
- Do not implement future phases.
- Do not modify files outside the allowed paths listed in `docs/phases/phase_<PHASE_NUMBER>.md`.
- Do not modify blocked paths.
- Do not expand scope beyond the requested phase.
- Do not change the PRD.
- Do not change the TDD.
- Do not change `BUILD_MANIFEST.yaml`.
- Do not read or update `HARNESS_EXECUTION_PLAN.yaml` — it is retired.
- Do not regenerate phase planning files during execution.
- Do not modify other `docs/phases/*.md` files.
- Stop if the requested phase harness file is missing.
- Stop if the requested phase does not exist in `BUILD_MANIFEST.yaml`.
- Stop if a required change touches blocked paths.
- Stop if a production blocker from `BUILD_MANIFEST.yaml` is encountered.
- Stop if tests fail for reasons unrelated to the requested phase.
- Stop if the requested phase requires changing planning documents.
- Prefer stopping and reporting over guessing.
- Always create `docs/build-ledger/phase_<PHASE_NUMBER>_build.md` before closing
  the phase. A phase is not complete without its build ledger entry.

---

## Required Build Workflow

For each task in the requested phase:

1. Read the task from `docs/phases/phase_<PHASE_NUMBER>.md`.
2. Confirm the task maps to the requested phase.
3. Write or update the failing test first.
4. Run the targeted test and confirm the expected failure.
5. Implement the minimum code needed to pass.
6. Run the targeted test again and confirm it passes.
7. Run the requested phase verification commands.
8. Refactor only if tests remain green.
9. Create a Drift checkpoint if Drift is available.
10. Continue to the next task only if the current task is green.

Do not batch multiple tasks together unless the phase file explicitly instructs you to do so.

---

## Drift Integration

If Drift is available in the repo:

- Start or reuse a Drift session for the requested phase.
- Use the requested phase goal from `docs/phases/phase_<PHASE_NUMBER>.md`.
- Use the requested phase allowed paths from `docs/phases/phase_<PHASE_NUMBER>.md`.
- Create a Drift checkpoint after each green task.
- Create a Drift checkpoint after requested phase exit criteria pass.
- Stop if Drift score becomes red.
- Report the Drift session ID and checkpoint IDs at the end.

If Drift is not available, continue with harness-only controls and explicitly report:

```text
Drift not available; proceeding with harness-only controls.
```

---

## Stop Conditions

Stop immediately if:

- No phase number was provided.
- The requested phase number is invalid.
- The requested phase does not exist in `BUILD_MANIFEST.yaml`.
- `docs/phases/phase_<PHASE_NUMBER>.md` is missing.
- The requested phase file appears stale.
- The requested phase is blocked.
- A required change touches blocked paths.
- A production blocker from `BUILD_MANIFEST.yaml` is encountered.
- Tests fail for reasons unrelated to the requested phase.
- The requested phase requires changing planning documents.
- Drift score becomes red.
- The requested phase requires scope expansion.
- The agent needs to modify files outside the requested phase allowed paths.
- The agent cannot determine the correct test command from the phase file.

When stopping, report:

- Why execution stopped
- What was completed before stopping
- What decision or clarification is needed
- Whether `keel plan-phases --confirm` should be rerun

---

## Scope Expansion Handling

If the requested phase requires touching a file outside its allowed paths:

1. Stop.
2. Do not modify the file.
3. Report a scope expansion request.

Use this format:

```text
Scope expansion required.

Requested phase:
Phase <PHASE_NUMBER>

Required path:
<path>

Reason:
<reason>

Recommended action:
Update BUILD_MANIFEST.yaml or the phase build file, then run `keel plan-phases --confirm` to regenerate.
```

---

## Phase Completion Checklist

Before closing the phase, confirm every item below. If any exit criterion fails, run:

```bash
keel phase failed <PHASE_NUMBER>
```

Then stop and report the failure.

When all exit criteria pass, close the phase with a single command:

```bash
keel phase close <PHASE_NUMBER> \
  --agent claude \
  --model <MODEL> \
  --summary "<brief summary of what was built>" \
  --criteria-json '[{"criterion":"<criterion 1>","passed":true},{"criterion":"<criterion 2>","passed":true}]'
```

This writes all required artifacts in one step:
- `.agent/phase_gates/phase_<PHASE_NUMBER>.gate.json` — canonical gate record
- `.agent/snapshots/phase_<PHASE_NUMBER>.rollback.json` — rollback DAG
- `docs/build-ledger/phase_<PHASE_NUMBER>_build.md` — build ledger
- `docs/audit/phase_<PHASE_NUMBER>.log` — audit log
- `.agent/audit.jsonl` — audit event appended
- `.agent/run_log.jsonl` — run log event appended
- Session event (`phase_completed`) emitted

Checklist before running `keel phase close`:

- [ ] All exit criteria verified — each criterion was run as a command and produced
      the expected output.
- [ ] All tests pass for the requested phase.
- [ ] No files were modified outside the allowed paths.

---

## Final Report

End by reporting:

1. Requested phase number
2. Requested phase file
3. Files changed
4. Tests added or updated
5. Commands run
6. Test results
7. Drift session/checkpoints, if available
8. Requested phase exit criteria status
9. Any blockers
10. Any scope expansion requests
11. Whether the requested phase is complete

Use this final format:

```text
Run Phase Report

Requested phase:
Phase <PHASE_NUMBER>

Phase file:
docs/phases/phase_<PHASE_NUMBER>.md

Files changed:
- ...
- docs/build-ledger/phase_<PHASE_NUMBER>_build.md  (required)
- docs/audit/phase_<PHASE_NUMBER>.log               (required)

Tests added or updated:
- ...

Commands run:
- ...

Test results:
- ...

Drift:
- Available: yes/no
- Session ID: ...
- Checkpoints: ...

Exit criteria:
- ...

Blockers:
- ...

Scope expansion requests:
- ...

Phase complete:
yes/no
```
