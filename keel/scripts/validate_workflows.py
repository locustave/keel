#!/usr/bin/env python3
from __future__ import annotations

import sys
from pathlib import Path


def main(argv: list[str]) -> int:
    root = Path(argv[1]).resolve() if len(argv) > 1 else Path.cwd()
    workflow_dir = root / ".github/workflows"
    workflows = sorted(workflow_dir.glob("*.yml")) + sorted(workflow_dir.glob("*.yaml"))
    if not workflows:
        print("No workflow files found.")
        return 1

    failures: list[str] = []
    for workflow in workflows:
        text = workflow.read_text(encoding="utf-8")
        required_markers = ["name:", "on:", "jobs:"]
        for marker in required_markers:
            if marker not in text:
                failures.append(f"{workflow.relative_to(root)} missing {marker}")

    if failures:
        print("Workflow validation failed:")
        for failure in failures:
            print(f"- {failure}")
        return 1

    print(f"Workflow validation passed for {len(workflows)} file(s).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
