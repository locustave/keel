# Keel Roadmap

Items for future versions. Nothing here is committed — this is a backlog to revisit after the current track is complete.

---

## Budgets

Keel has no execution limits today. An agent running `/keel-run N` can loop indefinitely with no ceiling on tokens, steps, time, or cost.

**What to build:**

- `keel/rules/budgets.md` — governance rule that agents read, declaring per-phase limits
- Budget fields in `BUILD_MANIFEST.yaml` per phase (e.g. `max_tokens: 500000`, `max_steps: 100`, `timeout_minutes: 30`)
- `keel phase start N` writes the budget to the session; hooks or the agent self-check against it
- `keel report tokens --group-by phase` already exists — extend to show budget vs actual
- Graceful termination: when budget is hit, agent writes a failed gate with reason `budget_exhausted`

**Why:** The single biggest gap identified in the harness-production comparison. Runaway loops waste money and produce sprawling, unreviewable changes.

**Complexity:** Medium. The session ledger already tracks tokens. The missing piece is enforcement — either via hooks that check cumulative token count, or via a budget-aware wrapper in the agentic loop rule.

---

## Security Evals

Keel trusts the agent to follow rules. There is no test suite that verifies the harness itself holds up under adversarial conditions.

**What to build:**

- `keel/evals/` directory with test scenarios:
  - **Injection resistance** — can a crafted file comment trick the agent into writing outside allowed paths?
  - **Timeout resilience** — does the harness behave correctly if a verification script hangs?
  - **Scope escape** — if a phase file references files outside its allowed paths, does the agent refuse?
  - **Gate integrity** — can a malformed gate JSON cause `keel current-phase` or `keel rollback` to misbehave?
- `keel eval` CLI command that runs the eval suite against a test project
- Results written to `.agent/evals/` with pass/fail per scenario

**Why:** Any governance system that relies on voluntary compliance has a trust boundary. Evals verify that boundary holds.

**Complexity:** High. Injection resistance evals require running an actual agent, which is expensive and non-deterministic. Gate integrity and timeout evals are deterministic and should come first.

---

## Risk Taxonomy

Keel's risk model is implicit: rollback requires `--confirm`, phase files declare allowed paths, hooks fire on tool calls. There is no formal classification of actions by risk level.

**What to build:**

- Three risk levels in the harness vocabulary: `read` (autonomous), `scoped_write` (within allowed paths), `destructive` (rollback, gate deletion, force push)
- Phase files annotate tasks with risk level
- `keel/rules/risk-levels.md` — governance rule that maps risk to required behavior (e.g. destructive actions require explicit human confirmation)
- Gate JSON records the highest risk level encountered during the phase

**Why:** Makes the implicit explicit. Today an agent could `rm -rf` inside allowed paths and keel wouldn't distinguish that from a normal file write.

**Complexity:** Low-medium. Mostly documentation and rule changes. CLI enforcement (blocking destructive commands without confirmation) is the harder part.

---

## Cost Tracking

The session ledger records token counts per event but there is no cost calculation, no per-phase cost summary, and no cost budget enforcement.

**What to build:**

- Model pricing table in config (user sets $/input-token, $/output-token per model)
- `keel report cost` — total session cost, broken down by phase
- `keel report cost --phase 3` — cost for a single phase
- Cost field in gate JSON and build ledger
- Optional cost budget per phase in `BUILD_MANIFEST.yaml`

**Why:** Developers using AI agents to build software need to know what each phase cost. This is table stakes for any team with a budget.

**Complexity:** Low. Token counts already exist. Multiply by price. The only design question is where the pricing table lives.

---

## Parallel Phase Execution

Today keel is strictly sequential: phase N must pass before phase N+1 starts. The planner already computes a dependency DAG with topological layers — phases in the same layer have no dependencies on each other.

**What to build:**

- `BUILD_MANIFEST.yaml` gains a `depends_on` field per phase (already computed by planner, just not exposed)
- `keel current-phase` returns all phases whose dependencies are satisfied, not just the lowest number
- `/keel-run` accepts multiple phase numbers if they are independent
- Gate files track which phases were running concurrently
- Rollback handles the DAG correctly (already does — `collectDownstream` walks the graph)

**Why:** For large projects, sequential execution is slow. If Phase 3 (auth) and Phase 4 (dashboard) are independent, they can run in parallel in separate agent sessions.

**Complexity:** High. Requires concurrent session support, conflict detection (two phases writing the same file), and merge strategy.

---

## Phase Retry with Diff

Today a failed phase requires manual intervention or a full re-run. There is no mechanism to retry just the failed exit criterion.

**What to build:**

- `keel phase retry N` — re-runs only the failed exit criteria from the failed gate
- If criteria now pass, promotes the failed gate to passed
- If criteria still fail, updates the failed gate with new attempt details
- Retry count and history recorded in gate JSON

**Why:** Small failures (a flaky test, a missing import) shouldn't require re-running the entire phase.

**Complexity:** Medium. The tricky part is scoping what the agent is allowed to change during a retry — ideally only the files related to the failed criterion.

---

## Pre-Phase Snapshot

`keel phase close` can auto-detect `git_sha_before` if a snapshot file exists at `.agent/snapshots/phase_N.pre.sha`. But nothing writes that file today.

**What to build:**

- `keel phase start N` writes `.agent/snapshots/phase_N.pre.sha` with the current git SHA
- `keel phase close N` reads it automatically — no `--git-sha-before` flag needed
- Snapshot includes working tree status (clean/dirty) for better rollback fidelity

**Why:** Completes the git SHA chain. Today `git_sha_before` is often empty because nothing captures the starting point.

**Complexity:** Low. One line of code in `phase start`.

---

## Harness Self-Test

`keel verify` checks file existence and gate schemas. It does not verify that the harness rules, commands, and scripts are internally consistent or that they match the current keel version.

**What to build:**

- `keel verify --deep` — checks that:
  - Every phase in `BUILD_MANIFEST.yaml` has a corresponding `docs/phases/phase_N.md`
  - Every exit criterion in the manifest is a runnable command
  - `agent-rules.md` tech stack section has no placeholder dashes
  - All rule files referenced in `keel/README.md` exist
  - Template version matches installed keel version (detects stale harness after keel upgrade)
- `keel upgrade` — re-runs `keel init --force-template` and reports what changed

**Why:** After upgrading keel, the harness files in a project may be stale. There's no way to detect or fix this today.

**Complexity:** Low-medium. Most checks are file existence and string matching.

---

## Web Dashboard

`docs/index.html` exists as a static dashboard. It could be more useful.

**What to build:**

- `keel dashboard` — serves a local web UI on localhost that reads gate files, audit logs, session events, and manifest
- Phase timeline visualization (which phases passed, failed, rolled back, and when)
- Per-phase detail view (files changed, exit criteria, cost, token usage)
- Session replay (event-by-event timeline)

**Why:** The audit data is rich but hard to consume from JSONL files. A visual dashboard makes it accessible to non-technical stakeholders.

**Complexity:** Medium-high. Requires a web server, template rendering, and JS for interactivity. Could start as a static HTML generator (`keel dashboard --export`) before adding a live server.

---

## MCP Server

Expose keel's read operations as an MCP (Model Context Protocol) server so any MCP-compatible agent can query phase state, gate records, and audit logs without shell access.

**What to build:**

- `keel mcp serve` — starts an MCP server exposing tools:
  - `get_current_phase` — returns current phase state
  - `get_gate` — returns gate JSON for a phase
  - `get_manifest` — returns BUILD_MANIFEST.yaml as structured data
  - `get_phase_file` — returns docs/phases/phase_N.md content
  - `get_audit_log` — returns recent audit events
- Read-only — no write operations via MCP

**Why:** As MCP adoption grows, agents that can't run shell commands could still participate in keel-governed builds.

**Complexity:** Medium. MCP server implementation is straightforward. The question is which tools to expose and how to handle auth.

---

## Priority Order (suggested)

| Priority | Item | Effort | Impact |
|----------|------|--------|--------|
| 1 | Pre-phase snapshot | Low | Completes the git SHA chain for every phase close |
| 2 | Cost tracking | Low | Immediate visibility into what each phase costs |
| 3 | Harness self-test / upgrade | Low-medium | Prevents stale harness after keel version bump |
| 4 | Budgets | Medium | Prevents runaway loops — the biggest operational risk |
| 5 | Risk taxonomy | Low-medium | Makes implicit safety model explicit and auditable |
| 6 | Phase retry with diff | Medium | Quality-of-life for failed phases |
| 7 | Security evals | High | Validates trust boundary — important but expensive |
| 8 | Web dashboard | Medium-high | Nice to have — audit data is already accessible via CLI |
| 9 | MCP server | Medium | Future-proofing for MCP ecosystem |
| 10 | Parallel phase execution | High | High value for large projects, high complexity |
