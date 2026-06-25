# Harness Skills

This repository uses a phase-gated execution harness.

Phase 0 rules:

- Read `agent-rules.md`, `BUILD_MANIFEST.yaml`, and `keel/README.md` before making changes.
- Limit work to harness scaffolding, verification scripts, and audit artifacts.
- Do not add backend, frontend, or domain business logic.
- Record preflight context before verification.
- End Phase 0 with verification output, a gate result, and an audit log entry.

Operational expectations:

- Shell scripts must use `set -euo pipefail`.
- Python helpers should rely on the standard library unless Phase 0 explicitly requires a dependency.
- Verification should fail loudly on missing scaffold files and pass quietly otherwise.