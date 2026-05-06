from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from runtime.types import ToolDeclaration

ToolHandler = Callable[..., Awaitable[dict[str, Any]]]


def tool(name: str, description: str, parameters: dict[str, Any]) -> ToolDeclaration:
    return ToolDeclaration(name=name, description=description, parameters=parameters)

