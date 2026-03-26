"""Enrutamiento interno: un solo asistente UI → sub-agentes comerciales existentes."""

from __future__ import annotations

import re
from typing import Any

# Debe coincidir con agent_mode en run_commercial_chat
INTERNAL_SALES = "internal_sales"
INTERNAL_PROCUREMENT = "internal_procurement"

_PROCUREMENT_PATTERNS = (
    r"\b(compras?\s+internas?|solicitud(es)?\s+de\s+compra|pedido(s)?\s+interno(s)?|"
    r"orden(es)?\s+de\s+compra|proveedor(es)?|aprobaci[oó]n(es)?\s+de\s+compra|"
    r"reposici[oó]n|insumo(s)?\s+interno(s)?|procurement|requisici[oó]n)\b",
)
_SALES_PATTERNS = (
    r"\b(venta(s)?|cliente(s)?|presupuesto(s)?|cotizaci[oó]n(es)?|"
    r"cobro(s)?|factura(s)?\s+de\s+venta|quote(s)?|stock\s+disponible|"
    r"devoluci[oó]n(es)?\s+de\s+cliente)\b",
)

_PROCUREMENT_RE = re.compile("|".join(_PROCUREMENT_PATTERNS), re.IGNORECASE)
_SALES_RE = re.compile("|".join(_SALES_PATTERNS), re.IGNORECASE)


def _last_agent_mode_from_history(messages: list[dict[str, Any]] | None) -> str | None:
    if not messages:
        return None
    for entry in reversed(messages):
        mode = entry.get("agent_mode")
        if isinstance(mode, str) and mode in (INTERNAL_SALES, INTERNAL_PROCUREMENT):
            return mode
    return None


def _score(text: str) -> tuple[int, int]:
    proc = sum(1 for _ in _PROCUREMENT_RE.finditer(text))
    sales = sum(1 for _ in _SALES_RE.finditer(text))
    return sales, proc


def route_internal_pymes(
    message: str,
    conversation_messages: list[dict[str, Any]] | None,
) -> str:
    """Elige sub-agente comercial para un turno (ventas vs compras internas).

    Heurística + contexto reciente + modo pegajoso en mensajes muy cortos.
    Misma app en todos los ambientes; reemplazable por LLM-classifier sin cambiar el contrato HTTP.
    """
    stripped = message.strip()
    if not stripped:
        return INTERNAL_SALES

    tail_parts: list[str] = []
    if conversation_messages:
        for entry in conversation_messages[-6:]:
            content = entry.get("content")
            if isinstance(content, str) and content.strip():
                tail_parts.append(content.strip())
    context_blob = " ".join(tail_parts + [stripped]).lower()

    last_mode = _last_agent_mode_from_history(conversation_messages)
    if len(stripped) < 24 and last_mode is not None:
        sales_hits, proc_hits = _score(stripped.lower())
        if proc_hits == 0 and sales_hits == 0:
            return last_mode

    sales_hits, proc_hits = _score(context_blob)
    if proc_hits > sales_hits:
        return INTERNAL_PROCUREMENT
    if sales_hits > proc_hits:
        return INTERNAL_SALES
    return last_mode or INTERNAL_SALES
