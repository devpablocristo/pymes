from __future__ import annotations

import sys
from pathlib import Path


TESTS_DIR = Path(__file__).resolve().parent
AI_ROOT = TESTS_DIR.parent

for path in (
    AI_ROOT,
    AI_ROOT / "../control-plane/shared/ai/src",
    AI_ROOT / "../pkgs/py-pkg/src",
):
    resolved = path.resolve()
    if str(resolved) not in sys.path:
        sys.path.insert(0, str(resolved))
