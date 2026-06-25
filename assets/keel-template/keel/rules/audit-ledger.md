# Harness Rule: Audit And Ledger

Every phase, gate, retry, rollback, and governance repair must leave a durable record.

## Required Files

- `docs/audit/phase_N.log`
- `docs/build-ledger/phase_N_build.md`
- `.agent/audit.jsonl`
- `.agent/run_log.jsonl`

## ADRs

Create an ADR under `docs/decisions/` when a phase or governance repair changes architecture, verification policy, source-of-truth priority, package manager policy, gate behavior, or CI policy.

If no ADR is required, the build ledger must explicitly say so.
