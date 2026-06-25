# /rollback-phase

Read and execute the canonical command instructions in:

`keel/commands/rollback-phase.md`

Rules:
- Treat `keel/commands/rollback-phase.md` as the source of truth.
- Do not rewrite or summarize the canonical command.
- Follow the canonical command exactly.
- Always run `keel rollback {N}` dry-run first and show the full output before asking for confirmation.
- Do not execute `keel rollback {N} --confirm` without explicit human confirmation.
- Do not run `git reset` automatically — print the commands for the human to run.
- After printing the Final Report, stop completely. Wait for an explicit human instruction.
