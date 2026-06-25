#!/usr/bin/env python3
from __future__ import annotations

import json
import shutil
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


@dataclass(frozen=True)
class ToolStatus:
    name: str
    version: str


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


def get_tool_version(command: list[str]) -> str:
    executable = shutil.which(command[0])
    if executable is None:
        return "missing"

    try:
        completed = subprocess.run(
            command,
            check=True,
            capture_output=True,
            text=True,
        )
    except (OSError, subprocess.CalledProcessError):
        return "unavailable"

    output = (completed.stdout or completed.stderr).strip().splitlines()
    return output[0] if output else "available"


def resolve_repo_root(argv: list[str]) -> Path:
    if len(argv) > 1:
        return Path(argv[1]).resolve()
    return Path(__file__).resolve().parents[2]


def append_jsonl(path: Path, payload: dict[str, object]) -> None:
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(payload, sort_keys=True))
        handle.write("\n")


def write_preflight_context(path: Path, content: str) -> None:
    path.write_text(content, encoding="utf-8")


def main(argv: list[str]) -> int:
    repo_root = resolve_repo_root(argv)
    agent_dir = repo_root / ".agent"
    agent_dir.mkdir(exist_ok=True)

    timestamp = utc_now()
    prd_path = repo_root / "docs/PRD.md"
    tdd_path = repo_root / "docs/TDD.md"
    manifest_path = repo_root / "BUILD_MANIFEST.yaml"
    git_dir = repo_root / ".git"

    tools = {
        "uv": get_tool_version(["uv", "--version"]),
        "pnpm": get_tool_version(["pnpm", "--version"]),
        "python": get_tool_version(["python3", "--version"]),
        "node": get_tool_version(["node", "--version"]),
        "pytest": get_tool_version(["pytest", "--version"]),
        "eslint": get_tool_version(["eslint", "--version"]),
        "podman": get_tool_version(["podman", "--version"]),
        "docker": get_tool_version(["docker", "--version"]),
    }

    missing_inputs = [
        str(path.relative_to(repo_root))
        for path in (prd_path, tdd_path, manifest_path)
        if not path.exists()
    ]
    repo_state = "git repo" if git_dir.exists() else "non-git workspace"
    next_action = "Run bash scripts/verify.sh" if not missing_inputs else "Restore missing source documents before verification"

    summary = "\n".join(
        [
            "# Preflight Context",
            "",
            f"- Timestamp UTC: {timestamp}",
            f"- Repo root: {repo_root}",
            f"- Repo state: {repo_state}",
            f"- PRD: {'present' if prd_path.exists() else 'missing'} ({prd_path})",
            f"- TDD: {'present' if tdd_path.exists() else 'missing'} ({tdd_path})",
            f"- Manifest: {'present' if manifest_path.exists() else 'missing'} ({manifest_path})",
            "- Tool versions:",
            *(f"  - {name}: {value}" for name, value in tools.items()),
            f"- Missing required inputs: {', '.join(missing_inputs) if missing_inputs else 'none'}",
            f"- Recommended next action: {next_action}",
        ]
    )
    write_preflight_context(agent_dir / "preflight_context.md", summary + "\n")

    event = {
        "event_type": "preflight.completed",
        "timestamp_utc": timestamp,
        "metadata": {
            "repo_root": str(repo_root),
            "git_sha": "unknown",
            "prd_path": str(prd_path) if prd_path.exists() else "missing",
            "tdd_path": str(tdd_path) if tdd_path.exists() else "missing",
            "manifest_path": str(manifest_path) if manifest_path.exists() else "missing",
            "tools": tools,
            "status": "completed" if not missing_inputs else "incomplete",
        },
    }
    append_jsonl(agent_dir / "audit.jsonl", event)
    append_jsonl(
        agent_dir / "run_log.jsonl",
        {
            "event_type": "phase.command.completed",
            "timestamp_utc": timestamp,
            "repo_root": str(repo_root),
            "git_sha": "unknown",
            "status": "completed" if not missing_inputs else "incomplete",
            "command": "bash harness/hooks/preflight.sh",
        },
    )

    run_log_path = agent_dir / "run_log.md"
    existing = run_log_path.read_text(encoding="utf-8") if run_log_path.exists() else "# Run Log\n\n"
    run_log_path.write_text(
        existing
        + f"## {timestamp}\n\n"
        + "- Event: preflight.completed\n"
        + f"- Repo state: {repo_state}\n"
        + f"- Recommended next action: {next_action}\n\n",
        encoding="utf-8",
    )

    if missing_inputs:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))