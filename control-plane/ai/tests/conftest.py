from __future__ import annotations

import os
import sys
from pathlib import Path


def pytest_configure() -> None:
    root = Path(__file__).resolve().parents[1]
    if str(root) not in sys.path:
        sys.path.insert(0, str(root))

    os.environ.setdefault("LLM_PROVIDER", "gemini")
    os.environ.setdefault("GEMINI_API_KEY", "")
    os.environ.setdefault("BACKEND_URL", "http://backend:8080")
    os.environ.setdefault("INTERNAL_SERVICE_TOKEN", "local-internal-token")
