# /keel-run

Read and execute the canonical command instructions in:

`keel/commands/keel-run.md`

Requested phase argument:

`$ARGUMENTS`

Rules:
- Treat `keel/commands/keel-run.md` as the source of truth.
- Do not rewrite or summarize the canonical command.
- Follow the canonical command exactly.
- Execute only the requested phase.
- If no phase argument is provided, stop and ask for a phase number.
- Do not regenerate harness planning files.
- Do not modify PRD, TDD, BUILD_MANIFEST.yaml, or docs/phases/*.md.
- Do not read or update HARNESS_EXECUTION_PLAN.yaml — it is retired.
- Stop if the requested phase file is missing.
- Stop if the requested phase does not exist in BUILD_MANIFEST.yaml.
- Stop and report rather than guessing.
