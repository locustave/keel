# /rollback-phase

Roll back a build phase and all downstream phases that depend on it.

This command reads `.agent/snapshots/phase_N.rollback.json`, determines all downstream phases that must be unwound, runs a dry-run for human review, and — after explicit confirmation — executes the rollback: deletes files created by each phase, invalidates passed gates, and appends a rollback audit event.

---

## Argument

Phase number to roll back:

```text
$ARGUMENTS
```

If no argument is provided, stop and list all phases that have rollback DAGs under `.agent/snapshots/` and ask the human which one to roll back.

---

## Safety Rules

- Always run the dry-run first. Never execute without showing the plan and receiving explicit confirmation.
- Do not roll back phase 0 without explicit human instruction. Phase 0 is harness scaffolding.
- Rollback preserves audit logs, build ledgers, ADRs, and run logs. Do not delete them.
- Do not use `git reset` automatically — print the git command for the human to run after reviewing. The human controls git history.
- If any rollback DAG file is missing for a downstream phase, stop and report. Do not partially roll back.

---

## Steps

### Step 1 — Dry-run

Run the rollback in dry-run mode and show the full output:

```bash
keel rollback {N}
```

The output lists every phase that will be unwound, files that will be deleted, and the git SHA to reset to for each phase.

### Step 2 — Confirm with the human

Show the dry-run output and ask the human:

```text
The above phases will be unwound. This cannot be undone automatically.
Type YES to confirm rollback of phase {N} and all downstream phases listed above.
```

Do not proceed unless the human explicitly confirms.

### Step 3 — Execute rollback

After confirmation, execute:

```bash
keel rollback {N} --confirm
```

This deletes files created by each unwound phase and renames each passed gate to `phase_N.rolled_back.json`.

### Step 4 — Print git reset instructions

After execution, print the git reset commands the human should run — one per unwound phase, in order (highest first):

```text
To complete the rollback, run these git commands in order:

  git reset --hard {git_sha_before_of_highest_downstream}
  git reset --hard {git_sha_before_of_next}
  ...
  git reset --hard {git_sha_before_of_phase_N}
```

Do not run these commands automatically.

---

## Audit Record

`keel rollback {N} --confirm` writes the audit event automatically. Verify it appeared:

```bash
tail -1 .agent/audit.jsonl
```

---

## Final Report

End by reporting:

```text
Rollback Complete

Phases unwound:
  Phase {M}: {name} — gate invalidated, {K} files deleted
  ...
  Phase {N}: {name} — gate invalidated, {K} files deleted

Git reset commands (run manually):
  git reset --hard {sha}
  ...

Active phase is now: {N-1}
Run /keel-run {N} to re-implement phase {N} from a clean state.
```

---

## Stop Conditions

Stop and report if:

- `.agent/snapshots/phase_N.rollback.json` does not exist.
- A rollback DAG is missing for any downstream phase.
- The human does not confirm after seeing the dry-run output.
- A gate file cannot be read or renamed.
