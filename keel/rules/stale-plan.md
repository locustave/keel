# Harness Rule: Stale Plan Detection

Before phase execution, compare the requested phase file against:

- `BUILD_MANIFEST.yaml`
- `docs/PRD.md`
- `docs/TDD.md`
- `keel/README.md`
- `agent-rules.md`

`HARNESS_EXECUTION_PLAN.yaml` is retired. Do not read or compare against it.

If the phase file appears stale or materially conflicts with the manifest, stop and report:

- For a feature's phases: run `keel plan-phases --confirm --feature-slug {slug}` to regenerate only those phase files.
- For a full governance repair: run `keel plan-phases --confirm` with no feature slug.

Do not silently execute against stale planning files.
