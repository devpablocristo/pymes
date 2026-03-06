from __future__ import annotations

import asyncio

from src.tools.help import search_help_docs


def test_help_tool_finds_known_topic() -> None:
    out = asyncio.run(search_help_docs("como cargo una devolucion"))
    assert "devolucion" in out["answer"].lower()
