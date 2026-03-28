from __future__ import annotations

from typing import Any


def build_text_block(text: str) -> dict[str, Any]:
    return {
        "type": "text",
        "text": text,
    }


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
