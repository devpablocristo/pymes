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


def build_insight_card_block(
    *,
    title: str,
    summary: str,
    scope: str | None = None,
    highlights: list[dict[str, str]] | None = None,
    recommendations: list[str] | None = None,
) -> dict[str, Any]:
    return {
        "type": "insight_card",
        "title": title,
        "summary": summary,
        "scope": scope,
        "highlights": highlights or [],
        "recommendations": recommendations or [],
    }


def build_kpi_group_block(*, title: str | None = None, items: list[dict[str, str]]) -> dict[str, Any]:
    return {
        "type": "kpi_group",
        "title": title,
        "items": items,
    }


def build_table_block(
    *,
    title: str,
    columns: list[str],
    rows: list[list[str]],
    empty_state: str | None = None,
) -> dict[str, Any]:
    return {
        "type": "table",
        "title": title,
        "columns": columns,
        "rows": rows,
        "empty_state": empty_state,
    }
