#!/usr/bin/env python3
"""Merge a feature BUILD_MANIFEST.yaml into the project BUILD_MANIFEST.yaml.

Usage:
    python3 scripts/merge_feature.py <feature_slug> [--confirm]

Run from the repository root.

Without --confirm: dry run only (prints what would change).
With    --confirm: writes the updated project BUILD_MANIFEST.yaml.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path


def find_phases(text: str) -> list[int]:
    return [int(m) for m in re.findall(r"^\s{2}- phase: (\d+)\s*$", text, re.MULTILINE)]


def last_phase(text: str) -> int:
    phases = find_phases(text)
    return max(phases) if phases else -1


def split_phases_block(text: str) -> tuple[str, str]:
    """Return (preamble, phases_block) where phases_block starts at the first '  - phase:' line."""
    match = re.search(r"^  - phase: \d+", text, re.MULTILINE)
    if not match:
        return text, ""
    return text[: match.start()], text[match.start():]


def renumber_phases(feature_text: str, start: int) -> tuple[str, list[tuple[int, int]]]:
    """Replace '  - phase: N' entries sequentially from `start`. Returns (new_text, mapping)."""
    feature_phases = find_phases(feature_text)
    if not feature_phases:
        return feature_text, []

    mapping: list[tuple[int, int]] = []
    result = feature_text
    for i, old in enumerate(sorted(feature_phases)):
        new = start + i
        mapping.append((old, new))
        result = re.sub(
            rf"^(\s{{2}}- phase: ){old}(\s*)$",
            rf"\g<1>{new}\2",
            result,
            flags=re.MULTILINE,
        )
    return result, mapping


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print("Usage: python3 scripts/merge_feature.py <feature_slug> [--confirm]", file=sys.stderr)
        return 2

    slug = argv[1]
    confirm = "--confirm" in argv
    root = Path(".").resolve()

    project_manifest_path = root / "BUILD_MANIFEST.yaml"
    feature_manifest_path = root / "docs" / "features" / slug / "BUILD_MANIFEST.yaml"

    if not project_manifest_path.exists():
        print(f"ERROR: {project_manifest_path} not found.", file=sys.stderr)
        return 1
    if not feature_manifest_path.exists():
        print(f"ERROR: {feature_manifest_path} not found.", file=sys.stderr)
        return 1

    project_text = project_manifest_path.read_text(encoding="utf-8")
    feature_text = feature_manifest_path.read_text(encoding="utf-8")

    feature_phases = find_phases(feature_text)
    if not feature_phases:
        print(f"ERROR: No phases found in {feature_manifest_path}.", file=sys.stderr)
        return 1

    start = last_phase(project_text) + 1
    _, feature_phases_block = split_phases_block(feature_text)
    renumbered_block, mapping = renumber_phases(feature_phases_block, start)

    merged = project_text.rstrip("\n") + "\n" + renumbered_block
    if not merged.endswith("\n"):
        merged += "\n"

    print(f"Feature   : {slug}")
    print(f"Source    : docs/features/{slug}/BUILD_MANIFEST.yaml")
    print(f"Mapping   : {', '.join(f'{old}→{new}' for old, new in mapping)}")
    print(f"Inserting : phases {', '.join(str(new) for _, new in mapping)} into BUILD_MANIFEST.yaml")

    if not confirm:
        print("\nDry run — pass --confirm to write.")
        print("\nPhases that would be added:")
        for _, new in mapping:
            print(f"  phase {new}")
        return 0

    project_manifest_path.write_text(merged, encoding="utf-8")
    print(f"\nMerged. First new phase: {mapping[0][1]}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
