#!/usr/bin/env python3
from __future__ import annotations

import json
import re
import sys
from pathlib import Path


def manifest_phases(root: Path) -> list[int]:
    manifest = root / "BUILD_MANIFEST.yaml"
    if not manifest.exists():
        return []
    text = manifest.read_text(encoding="utf-8")
    return [int(m) for m in re.findall(r"^- phase: (\d+)$", text, re.MULTILINE)]


def passed_phases(root: Path) -> set[int]:
    gates_dir = root / ".agent" / "phase_gates"
    passed = set()
    if not gates_dir.exists():
        return passed
    for gate_file in gates_dir.glob("phase_*.gate.json"):
        try:
            data = json.loads(gate_file.read_text(encoding="utf-8"))
            if data.get("status") == "passed":
                m = re.search(r"phase_(\d+)\.gate\.json$", gate_file.name)
                if m:
                    passed.add(int(m.group(1)))
        except (json.JSONDecodeError, OSError):
            pass
    return passed


def failed_phase(root: Path, phase: int) -> bool:
    return (root / ".agent" / "phase_gates" / f"phase_{phase}.failed.json").exists()


def main(argv: list[str]) -> int:
    root = Path(argv[1] if len(argv) > 1 else ".").resolve()

    phases = manifest_phases(root)
    if not phases:
        print("ERROR: BUILD_MANIFEST.yaml not found or has no phases.", file=sys.stderr)
        return 1

    passed = passed_phases(root)
    remaining = [p for p in sorted(phases) if p not in passed]

    if not remaining:
        last = max(phases)
        print(f"All {len(phases)} phases passed. Last phase: {last}.")
        print("STATUS: complete")
        return 0

    next_phase = remaining[0]
    done = sorted(passed & set(phases))

    if failed_phase(root, next_phase):
        print(f"BLOCKED: phase {next_phase} has a failed gate.")
        print(f"Passed phases : {done if done else 'none'}")
        print(f"Next phase    : {next_phase} (blocked — resolve .agent/phase_gates/phase_{next_phase}.failed.json)")
        print("STATUS: blocked")
        return 2

    print(f"Passed phases : {done if done else 'none'}")
    print(f"Next phase    : {next_phase}")
    print(f"Remaining     : {remaining}")
    print("STATUS: ready")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
