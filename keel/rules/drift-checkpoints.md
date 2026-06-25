# Harness Rule: Drift And Checkpoints

Drift tooling is optional.

If Drift is available:

- start or reuse a phase-specific session,
- set allowed paths from the phase file,
- checkpoint after each green task,
- checkpoint after final verification,
- stop on red drift score or out-of-scope modifications.

If Drift is unavailable, continue with harness-only controls and record:

```text
Drift not available; proceeding with harness-only controls.
```
