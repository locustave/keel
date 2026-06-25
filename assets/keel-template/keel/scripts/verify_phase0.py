#!/usr/bin/env python3
from __future__ import annotations

import sys
from pathlib import Path


def main(argv: list[str]) -> int:
    repo_root = Path(argv[1]).resolve() if len(argv) > 1 else Path(__file__).resolve().parents[2]
    required_paths = [
        "agent-rules.md",
        "BUILD_MANIFEST.yaml",
        "DESIGN.md",
        "docs/PRD.md",
        "docs/TDD.md",
        "docs/BUILD_MANIFEST.yaml",
        "keel/README.md",
        "harness/skills.md",
        "harness/commands/preflight.md",
        "harness/commands/run-phase.md",
        "harness/commands/verify.md",
        "harness/commands/harness-plan.md",
        "harness/hooks/preflight.sh",
        "harness/scripts/preflight_context.py",
        "harness/scripts/verify_phase0.py",
        "harness/scripts/verify_repo.py",
        "harness/scripts/validate_workflows.py",
        "scripts/verify.sh",
        "scripts/verify_phase0.sh",
        "scripts/plan_phases.py",
        "docs/phases/phase_0.prompt.md",
        "docs/phases/phase_0.harness_tdd.md",
        "docs/phases/phase_1.harness_tdd.md",
        "docs/audit/phase_0.log",
        "docs/build-ledger/phase_0_build.md",
        "docs/decisions",
        ".agent/preflight_context.md",
        ".agent/audit.jsonl",
        ".agent/run_log.jsonl",
        ".agent/phase_gates/phase_0.gate.json",
    ]
    missing = [item for item in required_paths if not (repo_root / item).exists()]
    for rule in ["pre-flight.md", "phase-state.md", "allowed-paths.md", "gates.md", "retry-rollback.md", "audit-ledger.md", "drift-checkpoints.md", "stale-plan.md"]:
        if not (repo_root / "harness/rules" / rule).exists():
            missing.append(f"harness/rules/{rule}")
    if missing:
        print("Phase 0 harness verification failed:")
        for item in missing:
            print(f"- missing: {item}")
        return 1
    text = (repo_root / "keel/README.md").read_text(encoding="utf-8")
    if "Bootstrap Harness README" in text:
        print("Phase 0 harness verification failed:")
        print("- keel/README.md is still the bootstrap placeholder")
        return 1
    print("Phase 0 harness verification passed.")
    print(f"Repo root: {repo_root}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
