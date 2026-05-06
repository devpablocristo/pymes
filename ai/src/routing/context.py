from __future__ import annotations

from dataclasses import dataclass

from src.api.chat_contract import ChatHandoff


@dataclass(frozen=True)
class TurnContext:
    """Minimal, side-effect-free input for the routing pipeline."""

    message: str
    route_hint: str | None = None
    route_hint_source: str | None = None
    handoff: ChatHandoff | None = None
    is_menu_request: bool = False
    is_ambiguous_query: bool = False
    handoff_is_structured_insight: bool = False
    handoff_is_valid: bool = False
    handoff_validation_reason: str = ""
