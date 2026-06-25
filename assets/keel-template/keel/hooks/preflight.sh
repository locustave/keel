#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

mkdir -p \
  "$ROOT_DIR/.agent/logs" \
  "$ROOT_DIR/.agent/snapshots" \
  "$ROOT_DIR/.agent/phase_gates" \
  "$ROOT_DIR/.agent/runbooks"

python3 "$ROOT_DIR/keel/scripts/preflight_context.py" "$ROOT_DIR"