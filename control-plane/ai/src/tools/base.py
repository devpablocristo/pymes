from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Awaitable, Callable

from src.llm.base import ToolDeclaration

ToolHandler = Callable[..., Awaitable[dict[str, Any]]]


@dataclass
class ToolSpec:
    declaration: ToolDeclaration
    handler: ToolHandler
    mode: str = "internal"
    role_allow: set[str] | None = None
