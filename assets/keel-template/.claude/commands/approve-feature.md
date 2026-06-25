# /approve-feature

Read and execute the canonical command instructions in:

`keel/commands/approve-feature.md`

Rules:
- Treat `keel/commands/approve-feature.md` as the source of truth.
- Do not rewrite or summarize the canonical command.
- Follow the canonical command exactly.
- Run the merge script dry-run first and show output before writing.
- Do not implement product code.
- Do not run `/keel-run` — not now, not after the final report, not for any reason within this command.
- After printing the Final Report, stop completely. Wait for an explicit human instruction.
- The final report tells the human what phase they *can* run. It is not an instruction for the agent to run it.
