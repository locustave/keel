# Harness Rule: Gates

A phase gate is the durable pass/fail record for one phase.

## Passed Gate Requirements

A passed gate must include:

- the exact phase number and name,
- `status: "passed"`,
- command entries with exit codes and log paths,
- one `exit_criteria_results` entry for every manifest exit criterion,
- all exit criteria marked `passed: true`,
- self-verification fields,
- known follow-ups,
- the next phase number.

Do not create or preserve a passed gate for a phase whose manifest exit criteria were not run successfully.

## Failed Gate Requirements

Create `.agent/phase_gates/phase_N.failed.json` when any exit criterion cannot be run or does not pass after allowed retries. Do not proceed to the next phase after a failed gate.
