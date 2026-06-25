#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

ROOT_REQUIRED = [
    "agent-rules.md",
    "keel/README.md",
    "BUILD_MANIFEST.yaml",
    "docs/PRD.md",
    "docs/phases/phase_0.md",
    "scripts/verify.sh",
    "scripts/verify_phase0.sh",
    "scripts/current_phase.sh",
    "scripts/merge_feature.py",
]

RULE_FILES = [
    "pre-flight.md",
    "phase-state.md",
    "allowed-paths.md",
    "gates.md",
    "retry-rollback.md",
    "audit-ledger.md",
    "drift-checkpoints.md",
    "stale-plan.md",
]

COMMAND_FILES = [
    "preflight.md",
    "keel-run.md",
    "verify.md",
    "new-feature.md",
    "approve-feature.md",
    "rollback-phase.md",
]

KEEL_ASSETS = [
    "keel/hooks/preflight.sh",
    "keel/scripts/preflight_context.py",
    "keel/scripts/verify_repo.py",
    "keel/scripts/verify_phase0.py",
    "keel/scripts/validate_workflows.py",
    "keel/scripts/current_phase.py",
    "keel/scripts/merge_feature.py",
    "keel/skills.md",
]


def rel(path: Path, root: Path) -> str:
    return str(path.relative_to(root))


def manifest_phases(text: str) -> list[int]:
    return [int(match) for match in re.findall(r"^- phase: (\d+)$", text, re.MULTILINE)]


def extract_list_block(text: str, start: str, end: str) -> set[str]:
    pattern = rf"## {re.escape(start)}\n\n(.*?)\n\n## {re.escape(end)}"
    match = re.search(pattern, text, re.DOTALL)
    if not match:
        return set()
    return set(re.findall(r"- `([^`]+)`", match.group(1)))


def gate_for_phase(root: Path, phase: int) -> dict[str, object] | None:
    gate_path = root / f".agent/phase_gates/phase_{phase}.gate.json"
    if not gate_path.exists():
        return None
    return json.loads(gate_path.read_text(encoding="utf-8"))


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("repo_root", nargs="?", default=".")
    parser.add_argument("--phase", type=int)
    args = parser.parse_args(argv[1:])

    root = Path(args.repo_root).resolve()
    failures: list[str] = []

    for item in ROOT_REQUIRED:
        if not (root / item).exists():
            failures.append(f"missing required path: {item}")

    for item in RULE_FILES:
        if not (root / "keel/rules" / item).exists():
            failures.append(f"missing keel rule: keel/rules/{item}")

    for item in COMMAND_FILES:
        if not (root / "keel/commands" / item).exists():
            failures.append(f"missing keel command: keel/commands/{item}")

    for item in KEEL_ASSETS:
        if not (root / item).exists():
            failures.append(f"missing keel asset: {item}")

    manifest_path = root / "BUILD_MANIFEST.yaml"
    manifest_text = manifest_path.read_text(encoding="utf-8") if manifest_path.exists() else ""
    phases = manifest_phases(manifest_text)
    if not phases:
        failures.append("manifest has no phases")
    elif phases != list(range(0, max(phases) + 1)):
        failures.append(f"manifest phases are not contiguous from 0: {phases}")

    phase_dir = root / "docs/phases"
    title_markers = ["## Phase Summary", "## Phase Goal"]
    for phase in phases:
        path = phase_dir / f"phase_{phase}.md"
        if not path.exists():
            failures.append(f"missing phase file: {rel(path, root)}")
            continue
        text = path.read_text(encoding="utf-8")
        if not any(m in text for m in title_markers):
            failures.append(f"phase {phase} missing marker ## Phase Summary or ## Phase Goal")
        for marker in ["## Allowed Paths", "## Blocked Paths", "## Manifest Tasks", "## Tasks", "## Verification Commands", "## Exit Criteria", "## Out of Scope"]:
            if marker not in text:
                failures.append(f"phase {phase} missing marker {marker}")
        allowed = extract_list_block(text, "Allowed Paths", "Blocked Paths")
        blocked = extract_list_block(text, "Blocked Paths", "Manifest Tasks")
        overlap = sorted(allowed & blocked)
        if overlap:
            failures.append(f"phase {phase} allowed/blocked path overlap: {overlap}")

    scan_paths = [
        root / "agent-rules.md",
        root / "keel",
        root / "docs/phases",
        root / "BUILD_MANIFEST.yaml",
    ]
    for scan_path in scan_paths:
        files = [scan_path] if scan_path.is_file() else list(scan_path.rglob("*"))
        for file_path in files:
            if not file_path.is_file():
                continue
            if "__pycache__" in file_path.parts or file_path.suffix == ".pyc":
                continue
            if file_path.name == "verify_repo.py":
                continue
            text = file_path.read_text(encoding="utf-8", errors="ignore")
            if "Agent Inbox" in text:
                failures.append(f"stale Agent Inbox reference: {rel(file_path, root)}")
            if "HARNESS.md" in text and file_path.name != "README.md":
                failures.append(f"stale HARNESS.md reference: {rel(file_path, root)}")
            if "phase_prompts" in text and file_path.name != "README.md":
                failures.append(f"stale phase_prompts reference: {rel(file_path, root)}")
            if "harness/" in text and file_path.name != "README.md":
                failures.append(f"stale harness/ reference: {rel(file_path, root)}")

    # Every phase with a passed gate must have a build ledger and audit log.
    ledger_dir = root / "docs/build-ledger"
    audit_dir = root / "docs/audit"
    for phase in phases:
        gate = gate_for_phase(root, phase)
        if gate and gate.get("status") == "passed":
            ledger_path = ledger_dir / f"phase_{phase}_build.md"
            if not ledger_path.exists():
                failures.append(
                    f"phase {phase} gate is passed but build ledger is missing: {rel(ledger_path, root)}"
                )
            audit_path = audit_dir / f"phase_{phase}.log"
            if not audit_path.exists():
                failures.append(
                    f"phase {phase} gate is passed but audit log is missing: {rel(audit_path, root)}"
                )
            # Gate schema: passed gates must have exit_criteria_results
            ecr = gate.get("exit_criteria_results")
            if not ecr:
                failures.append(
                    f"phase {phase} passed gate is missing exit_criteria_results"
                )
            elif isinstance(ecr, list) and any(
                not r.get("passed", False) for r in ecr if isinstance(r, dict)
            ):
                failures.append(
                    f"phase {phase} passed gate has failing exit criteria in exit_criteria_results"
                )

    if args.phase is not None:
        gate = gate_for_phase(root, args.phase)
        if gate is None:
            failures.append(f"missing passed gate for phase {args.phase}")
        elif gate.get("status") != "passed":
            failures.append(f"phase {args.phase} gate is not passed")

    if failures:
        print("Repository verification failed:")
        for failure in failures:
            print(f"- {failure}")
        return 1

    print("Repository verification passed.")
    print(f"Repo root: {root}")
    print(f"Manifest phases: {len(phases)}")
    print(f"Keel rules: {len(RULE_FILES)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
