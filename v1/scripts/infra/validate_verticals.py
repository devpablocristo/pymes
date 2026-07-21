#!/usr/bin/env python3
from __future__ import annotations

import sys

from verticals import RegistryError, load_registry, print_error, validate_registry


def main() -> int:
    try:
        registry = load_registry()
        verticals = validate_registry(registry)
    except RegistryError as exc:
        return print_error(exc)
    print(f"OK: {len(verticals)} verticals validated")
    return 0


if __name__ == "__main__":
    sys.exit(main())
