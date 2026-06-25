# TDD Builder Procedure

This procedure is invoked by `/keel-run 0` when `docs/TDD.md` is missing, and by `/new-feature` to confirm the tech stack is understood. It is not a standalone slash command.

---

## Step 1 — Read the PRD

Read `docs/PRD.md` before asking any questions. Use it to:

- Understand the product type (web app, CLI tool, library, service, etc.)
- Identify any tech constraints already stated
- Scope the questions to what is actually relevant

Do not ask about technologies already decided in the PRD.

---

## Step 2 — Resolve Tech Stack

First, read `agent-rules.md` and check the `## Tech Stack` table.

If the table already contains confirmed values (not placeholder dashes `—`), **skip the questions below** — use those values directly and proceed to Step 3. Report:

```text
Tech stack already confirmed in agent-rules.md — using existing values.
```

Only if the table is missing or every dimension is still `—`, present all questions in a single message:

```text
Before I generate the TDD, I need a few answers about your tech stack.
Answer as many as you can — leave any blank if you're unsure and I'll choose a sensible default.

1. Language & runtime
   What language(s) and runtime will this project use?
   (e.g. Go 1.22, Python 3.12 + FastAPI, Node 22 + TypeScript, Rust, etc.)

2. Data storage
   What database or storage will this project use, if any?
   (e.g. PostgreSQL, SQLite, Redis, S3, none)

3. Frontend / UI
   Does this project have a user interface?
   If yes — what framework or approach?
   (e.g. React + Vite, HTMX, plain HTML, native CLI, none)

4. Testing approach
   How should tests be run and verified?
   (e.g. pytest, go test, jest, rspec — or "not decided yet")

5. Deployment target
   Where will this run?
   (e.g. Docker + Compose, Kubernetes, serverless, binary, desktop, not decided yet)
```

Wait for the response. Do not proceed to Step 3 until an answer is received.

If the human provides partial answers, fill in reasonable defaults for the blanks based on what they did answer. State any defaults you are choosing before generating the TDD.

---

## Step 3 — Generate `docs/TDD.md`

Read `docs/TDD.template.md` and use it as the structural template.

Fill every section using:
- The PRD for product context, functional requirements, and component shapes
- The human's tech stack answers (and your stated defaults) for all stack decisions

Rules:
- Every section in the template must be present in the output — do not omit sections.
- Replace all `{placeholder}` tokens with real content.
- Where a decision is genuinely not yet made, write a brief note like "To be decided — defaulting to Docker for local development."
- Do not invent product features not present in the PRD.
- Keep the TDD concise — one paragraph per section is enough unless the product genuinely requires more detail.

Write the completed TDD to `docs/TDD.md`.

---

## Step 4 — Write Tech Stack to `agent-rules.md`

After writing `docs/TDD.md`, update the `## Tech Stack` section in `agent-rules.md` with the confirmed stack values.

If the section already contains placeholder dashes (`—`), replace them with the confirmed values. If the section does not exist, append it:

```markdown
## Tech Stack

> Confirmed during Phase 0. All future phases and `/new-feature` commands must use this stack.

| Dimension | Decision |
|-----------|----------|
| Language / runtime | {confirmed value} |
| Data storage | {confirmed value or "none"} |
| Frontend / UI | {confirmed value or "none"} |
| Test runner | {confirmed value} |
| Deployment target | {confirmed value or "TBD"} |
```

---

## Step 5 — Report

```text
TDD Generated

docs/TDD.md    written ({N} lines)
agent-rules.md updated (## Tech Stack section populated)

Tech stack confirmed:
  Language / runtime  : {value}
  Data storage        : {value}
  Frontend / UI       : {value}
  Test runner         : {value}
  Deployment target   : {value}
```
