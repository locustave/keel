# /approve-feature

Approve a feature draft and merge it into the project build queue.

This command reads a draft feature workspace from `docs/features/{slug}/`, merges its phase entries into the project `BUILD_MANIFEST.yaml`, appends reference entries to `docs/PRD.md` and `docs/TDD.md`, updates the feature lifecycle status, and generates phase phase files for the new phases. After this command completes, the feature's first phase is ready to run with `/keel-run N`.

---

## Argument

Feature slug argument:

```text
$ARGUMENTS
```

If no argument is provided, stop and list the slugs of all draft features found under `docs/features/` and ask the human which one to approve.

---

## Read Before Merging

Before modifying any files, read:

- `docs/features/{slug}/BUILD_MANIFEST.yaml` — the feature's phase plan
- `docs/features/{slug}/STATUS.md` — to confirm status is `draft`
- `docs/features/{slug}/PRD.md` — to confirm the feature is sufficiently specified
- `docs/features/{slug}/TDD.md` — to confirm technical design is present
- `BUILD_MANIFEST.yaml` — to determine the current last phase number

Stop and report if:

- `docs/features/{slug}/` does not exist.
- `docs/features/{slug}/BUILD_MANIFEST.yaml` does not exist or has no phases.
- `docs/features/{slug}/STATUS.md` shows `status: approved`, `in-progress`, or `shipped`. Do not re-approve an already-approved feature without explicit human instruction.
- The feature PRD or TDD is a skeleton with unfilled placeholder text. Ask the human to complete the draft before approving.

---

## Merge Steps

Execute in order:

### Step 1 — Dry-run merge

Run the merge command in dry-run mode and show the output to the human:

```bash
keel merge-feature {slug}
```

Review the output. Confirm the phase mapping looks correct (feature phase 1 → project phase N, etc.).

### Step 2 — Write merge

If the dry run output looks correct, execute the confirmed merge:

```bash
keel merge-feature {slug} --confirm
```

This appends the renumbered feature phases to `BUILD_MANIFEST.yaml`. Record the assigned phase numbers — you will need them for the final report.

### Step 3 — Update feature STATUS.md

Update `docs/features/{slug}/STATUS.md`:

- Set `status: approved`
- Set `approved:` to the current UTC date (ISO format)
- Under `## Project Phases`, list the assigned phase numbers:

```markdown
## Project Phases

Approved phases: {N}, {N+1}, ...
First phase to run: /keel-run {N}
```

### Step 4 — Update docs/PRD.md and docs/TDD.md

Append a one-line reference entry to each project source file under a `## Feature Extensions` section.

**`docs/PRD.md`** — append:

```markdown
## Feature Extensions

- See `docs/features/{slug}/PRD.md` for the {Feature Name} feature scope.
```

If `## Feature Extensions` already exists in the file, add the new line to it instead of creating a duplicate section.

**`docs/TDD.md`** — same pattern using `docs/features/{slug}/TDD.md`.

Do not rewrite or restructure the project PRD or TDD. Append only.

### Step 5 — Generate phase phase files

Run `keel plan-phases` to generate phase files for the newly merged phases only:

```bash
keel plan-phases --confirm --feature-slug {slug}
```

This generates `docs/phases/phase_{N}.md` for the feature's phases only. It does not regenerate phase files for existing phases.

---

## Rules

- Do not modify any passed gate files.
- Do not read or update `HARNESS_EXECUTION_PLAN.yaml` — it is retired.
- Do not implement product code.
- Do not approve a feature that references unresolved blockers in its PRD without first reporting them to the human.
- If the merge script exits non-zero, stop and report the error. Do not continue to Step 3 or Step 4.

## Hard Stop After Final Report

After printing the Final Report, **stop**. This command's job is to prepare the build queue, not to start execution.

Do not run `/keel-run`. Do not invoke any build or implementation command. Do not treat "Ready to execute: /keel-run {N}" in the Final Report as an instruction to yourself — it is information for the human.

Wait for an explicit human instruction before taking any further action.

---

## Audit Record

Append one entry to `.agent/audit.jsonl`:

```json
{"event": "feature.approved", "slug": "{slug}", "feature_name": "{Feature Name}", "phases_assigned": [{N}, ...], "timestamp_utc": "{ISO datetime}"}
```

---

## Final Report

End by reporting:

```text
Feature Approved

Feature name   : {Feature Name}
Slug           : {slug}

Phases merged into BUILD_MANIFEST.yaml:
  {old feature phase} → project phase {N}
  ...

Phase phase files generated:
  docs/phases/phase_{N}.md
  ...

Feature status : approved

---

This command is complete. To begin implementation, you (the human) can run:

  /keel-run {N}

Do not run this automatically.
```
