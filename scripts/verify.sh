#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "${1:-}" == "--quick" ]]; then
  exec "$ROOT_DIR/scripts/verify_phase0.sh"
fi

if [[ "${1:-}" == "--current-phase" ]]; then
  exec bash "$ROOT_DIR/scripts/current_phase.sh"
fi

if [[ "${1:-}" == "--phase" ]]; then
  if [[ -z "${2:-}" ]]; then
    echo "Usage: bash scripts/verify.sh --phase N" >&2
    exit 2
  fi
  exec python3 "$ROOT_DIR/keel/scripts/verify_repo.py" "$ROOT_DIR" --phase "$2"
fi

exec python3 "$ROOT_DIR/keel/scripts/verify_repo.py" "$ROOT_DIR"
