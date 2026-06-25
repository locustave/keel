#!/usr/bin/env python3
"""DEPRECATED — use scripts/merge_feature.py instead.

This file has been superseded by scripts/merge_feature.py, which is the
canonical location for the feature merge script.
"""
import sys

print(
    "ERROR: harness/scripts/merge_feature.py is deprecated.\n"
    "Use: python3 scripts/merge_feature.py <feature_slug> [--confirm]",
    file=sys.stderr,
)
sys.exit(1)
