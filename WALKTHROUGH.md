# Keel Walkthrough — Building a URL Shortener with AI Agents

This document walks through a real scenario: a solo developer who wants an AI coding agent to build a URL shortener. It explains why keel exists, what it does, and then follows every step from empty folder to working product.

---

## The Problem

You open Claude Code in an empty project and say:

> Build me a URL shortener with Go, PostgreSQL, and a simple web UI.

The agent starts coding. Two hours and 47 files later you have:

- Half a database migration that references a table from a different migration that was never committed
- A React frontend in a `/frontend` directory and also a second one in `/web` because the agent forgot it already started one
- No tests
- No record of what was built or in what order
- A bug in the redirect handler that the agent introduced while "refactoring" the auth layer it built at the same time

You cannot roll back. You cannot tell which files belong to which feature. You cannot point the agent at just the broken part because everything was done in one shot.

This is the problem keel solves.

---

## What Keel Is

Keel is a governance harness for AI coding agents. It wraps the agent in a set of rules, phase files, gate records, and audit logs so that the agent builds your project one phase at a time with full accountability.

Keel is not a framework. It does not generate application code. It does not run your tests. It installs a set of files into your project that your AI agent reads before touching any code. Those files tell the agent:

- What phase it is working on
- Which files it is allowed to touch
- What exit criteria it must meet before moving on
- Where to record what it did

The agent does the building. Keel keeps it in lane.

---

## What You Get

When you use keel, every phase of your build produces:

| Artifact | Location | Purpose |
|----------|----------|---------|
| Gate record | `.agent/phase_gates/phase_N.gate.json` | Machine-readable pass/fail with exit criteria results, git SHAs, files changed |
| Rollback DAG | `.agent/snapshots/phase_N.rollback.json` | Enough information to undo this phase and everything downstream |
| Build ledger | `docs/build-ledger/phase_N_build.md` | Human-readable summary: what was built, what model, what commands ran |
| Audit log | `docs/audit/phase_N.log` | Timestamped record of inputs, actions, verification, result |
| Audit event | `.agent/audit.jsonl` | Append-only project event log |
| Run log event | `.agent/run_log.jsonl` | Append-only execution log |
| Session events | `.agent/sessions/<id>/events.jsonl` | Every tool call, token count, and phase transition |

You can query any of these after the fact. You can roll back any phase. You can hand the project to a different agent or a different person and they can see exactly what happened.

---

## The Scenario

**You are:** A developer named Alex who wants to build a URL shortener.

**Your tools:** A terminal, Claude Code (or any AI coding agent), and keel installed via Homebrew.

**Your goal:** Go from an idea to a working URL shortener, built phase by phase, with full audit trail and the ability to roll back any phase.

---

## Step 1 — Create the Project

```bash
mkdir url-shortener
cd url-shortener
git init
```

You have an empty git repo.

---

## Step 2 — Write Your PRD

Create `docs/PRD.md`. This is the only document you need to write by hand. It describes what you want, not how to build it.

```markdown
# URL Shortener — Product Requirements

## Overview

A self-hosted URL shortener that creates short links, tracks click counts,
and serves a simple dashboard.

## User Stories

1. As a user, I can submit a long URL and receive a short link.
2. As a user, I can visit a short link and be redirected to the original URL.
3. As a user, I can view a dashboard showing all my short links and their
   click counts.
4. As a user, I can delete a short link from the dashboard.

## Requirements

- Short codes are 6 characters, alphanumeric, case-sensitive.
- Duplicate URLs return the existing short code.
- Click counts increment on every redirect.
- The dashboard is a single HTML page with no JavaScript framework.
- The service exposes a JSON API at /api/v1/.
- Data is stored in PostgreSQL.
- The service runs in Docker Compose (app + database).

## Non-Requirements

- No authentication (single-user, self-hosted).
- No analytics beyond click count.
- No custom short codes.
- No expiration dates.

## Success Metrics

- All API endpoints return correct status codes.
- Redirects complete in under 50ms (excluding network).
- The dashboard loads and displays links correctly.
```

That is your entire input. Everything else is generated.

---

## Step 3 — Initialize Keel

```bash
keel init
```

Keel finds your PRD, copies it to `docs/PRD.md` if it is not already there, and asks you to confirm your tech stack:

```
  Using PRD: docs/PRD.md

  Detected tech stack:
    Language / runtime  : (not detected)
    Data storage        : (not detected)
    Frontend / UI       : (not detected)
    Test runner         : (not detected)
    Deployment target   : (not detected)

  Confirm or edit each value:

  Language / runtime []: Go 1.22
  Data storage []: PostgreSQL
  Frontend / UI []: Server-rendered HTML (html/template)
  Test runner []: go test
  Deployment target []: Docker Compose

Bootstrapped keel in /Users/alex/url-shortener
Timestamp UTC: 2026-06-23T14:00:00Z
Next: run /keel-run 0 in your agent to generate the project manifest and phase build files.
```

Your project now looks like this:

```
url-shortener/
├── docs/
│   ├── PRD.md                      ← your requirements
│   ├── TDD.template.md             ← template for tech design
│   ├── phases/
│   │   └── phase_0.md              ← Phase 0 instructions
│   ├── audit/
│   │   └── phase_0.log             ← stub (pending)
│   └── build-ledger/
│       └── phase_0_build.md        ← stub (pending)
├── keel/
│   ├── README.md                   ← harness reference
│   ├── commands/                   ← slash commands the agent reads
│   │   ├── keel-run.md
│   │   ├── new-feature.md
│   │   ├── approve-feature.md
│   │   ├── verify.md
│   │   └── rollback-phase.md
│   ├── rules/                      ← governance rules
│   │   ├── gates.md
│   │   ├── allowed-paths.md
│   │   ├── phase-state.md
│   │   ├── tdd-builder.md
│   │   ├── audit-ledger.md
│   │   └── ...
│   ├── hooks/
│   │   └── preflight.sh
│   └── scripts/
│       ├── verify_repo.py
│       └── ...
├── scripts/
│   ├── verify.sh
│   └── verify_phase0.sh
├── agent-rules.md                  ← repo constitution (tech stack filled in)
├── AGENTS.md                       ← Codex agent rules
├── BUILD_MANIFEST.yaml             ← stub (Phase 0 only, pending)
├── .agent/
│   ├── audit.jsonl
│   ├── run_log.jsonl
│   ├── phase_gates/
│   │   └── phase_0.gate.json       ← stub (pending)
│   └── sessions/
└── .claude/
    └── commands/                   ← Claude Code slash command adapters
```

No application code. No database. No Docker files. Just the governance harness.

---

## Step 4 — Run Phase 0 (Planning)

Open Claude Code in your project and type:

```
/keel-run 0
```

The agent reads the PRD, reads `agent-rules.md` (which already has your tech stack from `keel init`), and follows the Phase 0 instructions in `docs/phases/phase_0.md`.

Phase 0 does not write any product code. It:

1. **Generates `docs/TDD.md`** — a technical design document derived from your PRD. Since your tech stack is already confirmed in `agent-rules.md`, the agent skips the tech stack questionnaire and uses those values directly. The TDD includes architecture decisions, data model, API contract, key flows, error handling, testing strategy, and a `## Deliverables` section that maps every piece of work to files, exit criteria, and dependencies.

2. **Generates `BUILD_MANIFEST.yaml`** — the full ordered build plan. For your URL shortener, it might look like:

```yaml
phases:
  - phase: 0
    name: Controlled Execution Harness Bootstrap
    goal: Generate TDD, confirm tech stack, produce build manifest and phase files.

  - phase: 1
    name: Database Schema and Migrations
    goal: Create PostgreSQL schema, migration files, and Docker Compose with database service.
    exit_criteria:
      - docker compose up -d exits 0
      - migration applies cleanly against fresh database

  - phase: 2
    name: Core API — Create and Redirect
    goal: Implement POST /api/v1/shorten and GET /:code redirect endpoints.
    exit_criteria:
      - go test ./internal/api/... exits 0
      - curl POST /api/v1/shorten returns 201 with short URL
      - curl GET /:code returns 302 redirect

  - phase: 3
    name: Click Tracking and Duplicate Detection
    goal: Increment click count on redirect, return existing code for duplicate URLs.
    exit_criteria:
      - go test ./internal/api/... exits 0
      - second POST with same URL returns existing short code
      - click count increments after redirect

  - phase: 4
    name: Dashboard and Delete
    goal: Implement GET /dashboard (HTML) and DELETE /api/v1/:code endpoint.
    exit_criteria:
      - go test ./... exits 0
      - dashboard page renders with link list
      - DELETE returns 204 and link is removed

  - phase: 5
    name: Docker Compose and Integration Tests
    goal: Full Docker Compose stack, end-to-end integration tests, README.
    exit_criteria:
      - docker compose up --build exits 0
      - go test -tags=integration ./... exits 0
      - README documents setup and usage
```

3. **Generates one `docs/phases/phase_N.md`** for every phase — each contains the objective, allowed file paths, tasks, exit criteria, and stop conditions for that phase.

4. **Closes Phase 0** by running:

```bash
keel phase close 0 \
  --agent claude --model opus \
  --summary "Generated TDD, manifest with 5 product phases, and all phase build files" \
  --criteria-json '[{"criterion":"preflight exits 0","passed":true},{"criterion":"verify_repo exits 0","passed":true},{"criterion":"verify_phase0 exits 0","passed":true}]'
```

This single command writes the gate file, rollback DAG, build ledger, audit log, and all event records.

---

## Step 5 — Review the Plan

Before building anything, you review:

- `docs/TDD.md` — does the architecture make sense?
- `BUILD_MANIFEST.yaml` — are the phases in the right order? Are the exit criteria specific enough?
- `docs/phases/phase_1.md` through `phase_5.md` — is the scope of each phase reasonable?

You can edit any of these files. The agent will read whatever is there when you start the next phase.

This is the human-in-the-loop checkpoint. Keel gives you a clear moment to review the plan before any code is written.

---

## Step 6 — Build Phase 1 (Database)

```
/keel-run 1
```

The agent reads `docs/phases/phase_1.md` and sees:

- **Allowed paths:** `docker-compose.yml`, `migrations/`, `internal/db/`
- **Tasks:** Create schema, write migration, configure Docker Compose, verify migration applies
- **Exit criteria:** `docker compose up -d` exits 0, migration applies cleanly

The agent writes only those files. It cannot touch `internal/api/` because that is not in the allowed paths for Phase 1. It cannot start building the API because Phase 1's scope is database only.

When the tasks are done and exit criteria pass, the agent runs:

```bash
keel phase close 1 \
  --agent claude --model opus \
  --summary "Created PostgreSQL schema with urls table, migration, Docker Compose" \
  --criteria-json '[{"criterion":"docker compose up -d exits 0","passed":true},{"criterion":"migration applies cleanly","passed":true}]'
```

Phase 1 is now closed. The gate file records exactly what happened. The rollback DAG records how to undo it.

---

## Step 7 — Build Phases 2 through 5

Each phase follows the same pattern:

```
/keel-run 2
```

The agent:
1. Reads the rules, manifest, and phase file
2. Confirms prior phases have passed gates
3. Implements only the current phase's tasks
4. Runs tests after each task (test-first workflow)
5. Verifies all exit criteria
6. Closes the phase with `keel phase close N`

You can review between phases. You can stop and come back tomorrow. You can hand the project to a different agent. The gate files and manifest tell any agent exactly where the project is.

---

## Step 8 — Something Goes Wrong

Phase 3 passes, but during Phase 4 you realize the dashboard query is wrong — it needed a JOIN that Phase 3's schema migration should have created.

You roll back:

```bash
keel rollback 3
```

Dry-run output:

```
Rollback plan — 2 phase(s) will be unwound:

  Phase 4: Dashboard and Delete
    git reset --hard abc1234
    delete 4 file(s): [internal/dashboard/handler.go ...]
    gate → phase_4.rolled_back.json

  Phase 3: Click Tracking and Duplicate Detection
    git reset --hard def5678
    delete 3 file(s): [internal/api/click.go ...]
    gate → phase_3.rolled_back.json

Dry run. Run with --confirm to execute rollback.
```

You confirm, fix the Phase 3 schema, and re-run:

```
/keel-run 3
/keel-run 4
```

The rolled-back gates are preserved in the audit trail. Nothing is lost.

---

## Step 9 — Check the Record

At any point you can see where the project stands:

```bash
keel current-phase
```

```
Passed phases : [0, 1, 2, 3, 4, 5]
STATUS: complete
```

Or query the session:

```bash
keel report session
```

This prints a summary of every phase: what was built, how many tokens were used, which tools were called, and whether any debug loops occurred.

The audit log at `.agent/audit.jsonl` has one line per major event:

```json
{"event_type":"phase.passed","timestamp_utc":"2026-06-23T14:05:00Z","phase":0}
{"event_type":"phase.passed","timestamp_utc":"2026-06-23T14:12:00Z","phase":1}
{"event_type":"phase.passed","timestamp_utc":"2026-06-23T14:28:00Z","phase":2}
{"event_type":"phase.passed","timestamp_utc":"2026-06-23T14:45:00Z","phase":3}
{"event_type":"rollback.executed","timestamp_utc":"2026-06-23T15:00:00Z","metadata":{"phases_unwound":[4,3]}}
{"event_type":"phase.passed","timestamp_utc":"2026-06-23T15:20:00Z","phase":3}
{"event_type":"phase.passed","timestamp_utc":"2026-06-23T15:40:00Z","phase":4}
{"event_type":"phase.passed","timestamp_utc":"2026-06-23T16:00:00Z","phase":5}
```

Every build ledger at `docs/build-ledger/phase_N_build.md` records what model was used, what files were created, what commands ran, and whether any architectural decisions were made.

---

## Why This Works

Without keel, the agent sees your entire project as one task. It makes decisions about scope, ordering, and file boundaries on the fly with no record and no constraints.

With keel, the agent sees one phase at a time. Each phase has:

- **Defined scope** — allowed paths, tasks, exit criteria
- **Defined boundaries** — it cannot touch files outside its phase
- **Defined verification** — it must pass exit criteria before moving on
- **Defined record** — every action is logged to the gate, ledger, and audit trail
- **Defined undo** — every phase has a rollback DAG

The agent is still doing all the work. Keel just makes sure it does it in order, one piece at a time, with a paper trail.

---

## Summary of Commands

| When | What you do | What happens |
|------|-------------|--------------|
| Once | `keel init` | Installs harness, detects tech stack, copies PRD |
| Phase 0 | `/keel-run 0` | Agent generates TDD, manifest, and all phase files |
| Review | Read `BUILD_MANIFEST.yaml` and phase files | Human approves the plan |
| Each phase | `/keel-run N` | Agent builds one phase, writes gate and audit records |
| Anytime | `keel current-phase` | See which phase is next |
| Anytime | `keel verify` | Check harness consistency |
| Anytime | `keel report session` | Review what happened in the current session |
| On failure | `keel rollback N` | Undo phase N and everything downstream |
| New feature | `/new-feature "name"` then `/approve-feature slug` | Draft and merge a new feature into the build plan |

---

## Key Principle

Keel does not make the agent smarter. It makes the agent accountable. The difference between an AI agent writing code and an AI agent building software is governance. Keel is that governance.
