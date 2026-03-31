from __future__ import annotations

import json
import os
import sys
from pathlib import Path


def main() -> int:
    repo_root = Path(__file__).resolve().parents[2]
    ai_root = repo_root / "ai"

    pythonpath_entries = [str(ai_root)]
    existing_pythonpath = os.environ.get("PYTHONPATH", "")
    if existing_pythonpath:
        pythonpath_entries.append(existing_pythonpath)
    os.environ["PYTHONPATH"] = os.pathsep.join(pythonpath_entries)

    if str(ai_root) not in sys.path:
        sys.path.insert(0, str(ai_root))

    from src.main import app

    schema = app.openapi()
    output_path = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else None
    payload = json.dumps(schema, ensure_ascii=True, indent=2)

    if output_path is None:
        print(payload)
        return 0

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(payload + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
