#!/usr/bin/env python3
"""Lee `.env` y emite líneas `export KEY=value` con quoting seguro para bash (evita `<>` como redirección).

Usado por `scripts/seeds/lib.sh` en lugar de `source .env` cuando hay placeholders tipo Clerk.
"""

from __future__ import annotations

import re
import shlex
import sys
from pathlib import Path

_KEY_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")


def main() -> None:
    if len(sys.argv) < 2:
        return
    path = Path(sys.argv[1])
    if not path.is_file():
        return
    try:
        text = path.read_text(encoding="utf-8")
    except OSError:
        return
    for raw in text.splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            continue
        key, _, rest = line.partition("=")
        key = key.strip()
        if not _KEY_RE.match(key):
            continue
        val = rest.strip()
        if len(val) >= 2 and val[0] == val[-1] and val[0] in "\"'":
            val = val[1:-1]
        print(f"export {shlex.quote(key)}={shlex.quote(val)}")


if __name__ == "__main__":
    main()
