# Harness Rule: Allowed Paths

Each phase may edit only paths listed in its `Allowed Paths` section, plus required audit, ledger, gate, log, and snapshot files for that phase.

## Enforcement

- Do not edit blocked paths during normal phase execution.
- If a required change falls outside allowed paths, stop and report a scope expansion request.
- Allowed and blocked paths must not overlap.
- Generated caches, virtual environments, and dependency directories must not be used as evidence of phase completion.

## Governance Repair

Harness governance repair may update `agent-rules.md`, `harness/**`, `scripts/**`, `docs/phases/**`, `BUILD_MANIFEST.yaml`, `.agent/**`, and scaffold-only files when the human explicitly requests it. Do not update `HARNESS_EXECUTION_PLAN.yaml` — it is retired.
