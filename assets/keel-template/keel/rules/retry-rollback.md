# Harness Rule: Retry And Rollback

Retries are allowed only for failures caused by the current phase or local environment issues that can be resolved without broadening scope.

## Retry Records

Each retry must be recorded in:

- `.agent/audit.jsonl`
- `docs/audit/phase_N.log`
- `docs/build-ledger/phase_N_build.md`

## Rollback DAG — Required At Phase Completion

At the end of every phase, before marking the gate passed, write a rollback DAG to:

```
.agent/snapshots/phase_N.rollback.json
```

The rollback DAG must contain:

```json
{
  "phase": N,
  "name": "phase name from BUILD_MANIFEST.yaml",
  "git_sha_before": "<sha captured before editing any files>",
  "git_sha_after": "<sha after final commit>",
  "timestamp_utc": "<ISO 8601>",
  "deliverables": ["deliverable-id", ...],
  "files_created": ["path/to/new/file", ...],
  "files_modified": ["path/to/changed/file", ...],
  "depends_on_phases": [N-1, ...],
  "downstream_phases": [N+1, ...],
  "rollback": {
    "git_reset_to": "<git_sha_before>",
    "files_to_delete": ["path/to/new/file", ...],
    "files_to_restore": ["path/to/changed/file", ...]
  }
}
```

- `files_created` = files that did not exist before this phase. These are deleted on rollback.
- `files_modified` = files that existed before this phase and were changed. These are restored via git on rollback.
- `downstream_phases` = all phases with a higher number that have passed gates at the time of writing.
- `git_sha_before` must be captured with `git rev-parse HEAD` before editing any file in this phase.

If the rollback DAG cannot be written, do not mark the gate as passed. Report the failure instead.

## Rollback

To roll back a phase, use `/rollback-phase N`. This command:

1. Reads `.agent/snapshots/phase_N.rollback.json` and all downstream rollback DAGs.
2. Shows a dry-run plan for human review.
3. After explicit human confirmation, deletes `files_created` for each unwound phase and renames each gate to `phase_N.rolled_back.json`.
4. Prints the `git reset --hard` commands for the human to run.

Rollback must preserve:

- audit logs
- build ledgers
- ADRs
- failed and rolled-back gate files
- run logs

Do not run destructive git commands automatically. Print them for the human to review and execute.
