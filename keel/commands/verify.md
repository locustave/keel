# Verify Command

Purpose: validate that the Phase 0 execution harness scaffold exists and is internally consistent.

Primary command:

```bash
bash scripts/verify.sh
```

Focused command:

```bash
bash scripts/verify_phase0.sh
```

The verification step checks only the harness scaffold and audit prerequisites. It must not add business logic or pull external dependencies.