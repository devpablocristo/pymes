"""Builders de bloques de chat — re-exporta genéricos desde runtime.chat
y agrega los específicos de pymes (confirmación de acciones, selección de ruta)."""

from __future__ import annotations

from typing import Any

from runtime.chat.blocks import (
    build_insight_card_block,
    build_kpi_group_block,
    build_table_block,
    build_text_block,
)


def build_confirm_actions_block(actions: list[str]) -> dict[str, Any]:
    clean_actions = [item.strip() for item in actions if item and item.strip()]
    return {
        "type": "actions",
        "actions": [
            {
                "id": "confirm_pending_actions",
                "label": "Confirmar acciones",
                "kind": "confirm_action",
                "message": "Confirmo las acciones pendientes.",
                "confirmed_actions": clean_actions,
                "style": "primary",
            }
        ],
    }


def build_route_selection_block(
    *,
    original_message: str,
    route_options: list[tuple[str, str]],
    selection_behavior: str = "route_and_resend",
) -> dict[str, Any]:
    clean_message = original_message.strip()
    actions: list[dict[str, Any]] = []
    for route_hint, label in route_options:
        if not route_hint.strip() or not label.strip():
            continue
        actions.append(
            {
                "id": f"clarify_route_{route_hint.strip()}",
                "label": label.strip(),
                "kind": "send_message",
                "message": clean_message,
                "route_hint": route_hint.strip(),
                "selection_behavior": selection_behavior,
                "style": "secondary",
            }
        )
    return {
        "type": "actions",
        "actions": actions,
    }


__all__ = [
    "build_confirm_actions_block",
    "build_insight_card_block",
    "build_kpi_group_block",
    "build_route_selection_block",
    "build_table_block",
    "build_text_block",
]
