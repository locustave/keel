# /new-feature

Draft a feature workspace for a new product feature.

This command creates a scoped authoring workspace under `docs/features/{slug}/`. It does not touch the project `BUILD_MANIFEST.yaml` and does not implement product code. The feature stays in draft state until the human runs `/approve-feature {slug}`.

---

## Argument

Feature name argument:

```text
$ARGUMENTS
```

If no argument is provided, stop and ask for a feature name.

Derive the slug by lowercasing the name and replacing spaces and special characters with hyphens.

Examples:

| Argument | Slug |
|---|---|
| `notification center` | `notification-center` |
| `User Authentication` | `user-authentication` |
| `CSV Export v2` | `csv-export-v2` |

---

## Ask for a Description Before Drafting

After parsing the feature name and slug, stop and ask the human:

```text
Feature: {Feature Name}

Before I draft the workspace, give me a short description of this feature.
What problem does it solve and roughly what should it do?
```

Wait for the response. Do not read source files or create any files until the human provides a description.

Use that description as the seed for all generated content — the problem statement, goals, user stories, technical approach, and phase breakdown should all flow from it. Do not invent motivation or scope that contradicts or ignores the description.

If the description is too vague to produce meaningful PRD/TDD content, ask one targeted follow-up question before proceeding. Do not ask more than one follow-up.

---

## Read Before Drafting

Before creating any files, read:

- `agent-rules.md` — specifically the `## Tech Stack` section for confirmed language, runtime, test runner, and deployment target
- `docs/PRD.md` — to understand the product scope this feature extends
- `docs/TDD.md` — to understand the technical constraints and architecture
- `BUILD_MANIFEST.yaml` — to find the current last phase number and name conventions

Do not read `HARNESS_EXECUTION_PLAN.yaml` — it is retired.

Do not read future phase files or product source files unless they are directly relevant to scoping this feature.

---

## Output Files

Create exactly these files under `docs/features/{slug}/`:

### `docs/features/{slug}/PRD.md`

Scoped product requirements for this feature only. Structure:

```markdown
# PRD: {Feature Name}

> Extends: docs/PRD.md — see project PRD for global product context and constraints.

## Problem Statement

{What user problem does this feature solve?}

## Goals

- {Goal 1}
- {Goal 2}

## Non-Goals

- {What this feature explicitly will not do}

## User Stories

- As a {role}, I want {action} so that {outcome}.

## Acceptance Criteria

- {Measurable, testable criteria}

## Out of Scope

- {Explicitly excluded items}
```

### `docs/features/{slug}/TDD.md`

Scoped technical design for this feature only. Structure:

```markdown
# TDD: {Feature Name}

> Extends: docs/TDD.md — see project TDD for global technical constraints and stack decisions.

## Design Summary

{One paragraph describing the technical approach.}

## Components Affected

- {File or module}: {what changes}

## New Components

- {File or module}: {purpose}

## Data Model Changes

{Any schema or state changes. "None" if not applicable.}

## API Changes

{Any new or modified endpoints. "None" if not applicable.}

## Dependencies

{New packages or services required. "None" if not applicable.}

## Tech Stack

Use the stack confirmed in `agent-rules.md ## Tech Stack`. Do not introduce new languages, frameworks, or runtimes unless explicitly approved by the human for this feature.

| Dimension | This feature uses |
|-----------|------------------|
| Language / runtime | {from agent-rules.md} |
| Test runner | {from agent-rules.md} |
| New dependencies | {list or "none"} |

## Risks and Open Questions

- {Risk or question}
```

### `docs/features/{slug}/BUILD_MANIFEST.yaml`

Phase plan for this feature only. Use sequential phase numbers starting from 1 — they will be renumbered when the feature is approved. Structure:

```yaml
title: BUILD_MANIFEST.yaml
feature: {slug}
feature_name: {Feature Name}
status: draft
phases:
  - phase: 1
    name: {Phase Name}
    goal: {What this phase accomplishes}
    inputs:
      - docs/features/{slug}/PRD.md
      - docs/features/{slug}/TDD.md
    tasks:
      - {Task 1}
      - {Task 2}
    exit_criteria:
      - {Criterion 1}
    out_of_scope:
      - {Item 1}
```

Add one phase entry per logical unit of work. Keep phases small enough for an AI coding agent to execute independently without scope expansion.

### `docs/features/{slug}/STATUS.md`

Lifecycle tracking file. Structure:

```markdown
# Feature: {Feature Name}

feature: {slug}
status: draft
created: {ISO date}
approved:
started:
shipped:
deferred:

## Project Phases

Assigned at approval. Will be populated by /approve-feature.

## Notes

{Any open questions, blockers, or deferred decisions.}
```

---

## Rules

- Do not modify `BUILD_MANIFEST.yaml` (project root).
- Do not modify `docs/PRD.md` or `docs/TDD.md` — reference entries are added at approval time by `/approve-feature`.
- Do not read or update `HARNESS_EXECUTION_PLAN.yaml` — it is retired and must not be referenced.
- Do not introduce tech stack changes outside what is confirmed in `agent-rules.md ## Tech Stack`.
- Do not create or modify `docs/phases/` files.
- Do not implement product code.
- Do not run `keel plan-phases`.
- Do not number feature phases relative to the project — use 1, 2, 3… in the feature manifest.
- Stop if `docs/features/{slug}/` already exists and is non-empty. Report the conflict and ask the human whether to overwrite or pick a different slug.

---

## Final Report

End by reporting:

```text
New Feature Draft

Feature name : {Feature Name}
Slug         : {slug}
Workspace    : docs/features/{slug}/

Files created:
- docs/features/{slug}/PRD.md
- docs/features/{slug}/TDD.md
- docs/features/{slug}/BUILD_MANIFEST.yaml
- docs/features/{slug}/STATUS.md

Feature phases drafted: {N} phases (numbered 1–{N}, renumbered at approval)
Current last project phase: {last project phase number}
Phases will be assigned starting from: {last + 1}

Next step:
Review and edit docs/features/{slug}/ then run:

  /approve-feature {slug}
```
