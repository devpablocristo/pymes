#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import sys

from verticals import RegistryError, load_registry, print_error, validate_registry


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=("ci", "deploy"), required=True)
    parser.add_argument("--pretty", action="store_true")
    args = parser.parse_args()

    try:
        registry = load_registry()
        verticals = validate_registry(registry)
    except RegistryError as exc:
        return print_error(exc)

    if args.mode == "ci":
        selected = [vertical for vertical in verticals if vertical.enabled]
    else:
        selected = [vertical for vertical in verticals if vertical.enabled and vertical.deploy]

    matrix = {"include": [vertical.matrix_item() for vertical in selected]}
    if args.pretty:
        print(json.dumps(matrix, indent=2, ensure_ascii=False))
    else:
        print(json.dumps(matrix, separators=(",", ":"), ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
