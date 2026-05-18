#!/usr/bin/env python3
from __future__ import annotations

import argparse
import sys
from pathlib import Path

from verticals import (
    FIREBASE_GENERATED_PATH,
    FIREBASE_LEGACY_PATH,
    RegistryError,
    firebase_config,
    load_registry,
    print_error,
    render_json,
    validate_registry,
)


def _check_file(path: Path, expected: str) -> bool:
    if not path.exists():
        print(f"ERROR: {path.name} does not exist. Run scripts/infra/generate_firebase.py --write.", file=sys.stderr)
        return False
    actual = path.read_text(encoding="utf-8")
    if actual != expected:
        print(f"ERROR: {path.name} is out of date. Run scripts/infra/generate_firebase.py --write.", file=sys.stderr)
        return False
    return True


def main() -> int:
    parser = argparse.ArgumentParser()
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--write", action="store_true", help="write generated Firebase configs")
    mode.add_argument("--check", action="store_true", help="fail if generated Firebase configs are stale")
    args = parser.parse_args()

    try:
        registry = load_registry()
        verticals = validate_registry(registry)
        rendered = render_json(firebase_config(registry, verticals))
    except RegistryError as exc:
        return print_error(exc)

    if args.write:
        FIREBASE_GENERATED_PATH.write_text(rendered, encoding="utf-8")
        FIREBASE_LEGACY_PATH.write_text(rendered, encoding="utf-8")
        print(f"Wrote {FIREBASE_GENERATED_PATH.name} and {FIREBASE_LEGACY_PATH.name}")
        return 0

    ok = _check_file(FIREBASE_GENERATED_PATH, rendered)
    ok = _check_file(FIREBASE_LEGACY_PATH, rendered) and ok
    if ok:
        print("OK: Firebase configs are current")
        return 0
    return 1


if __name__ == "__main__":
    sys.exit(main())
