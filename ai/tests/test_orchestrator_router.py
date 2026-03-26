"""Enrutador interno Asistente Pymes → sub-agentes comerciales."""

from __future__ import annotations

from src.agents.orchestrator_router import (
    INTERNAL_PROCUREMENT,
    INTERNAL_SALES,
    route_internal_pymes,
)


def test_route_procurement_keywords() -> None:
    assert route_internal_pymes("Estado de la solicitud de compra 12", None) == INTERNAL_PROCUREMENT
    assert route_internal_pymes("Aprobación de compra pendiente", None) == INTERNAL_PROCUREMENT


def test_route_sales_keywords() -> None:
    assert route_internal_pymes("Resumen de ventas del mes", None) == INTERNAL_SALES
    assert route_internal_pymes("Cliente Juan debe factura", None) == INTERNAL_SALES


def test_route_default_sales() -> None:
    assert route_internal_pymes("Hola", None) == INTERNAL_SALES


def test_route_sticky_short_followup() -> None:
    history = [
        {"role": "user", "content": "Solicitud de compra 5", "agent_mode": INTERNAL_PROCUREMENT},
        {"role": "assistant", "content": "Listo", "agent_mode": INTERNAL_PROCUREMENT},
    ]
    assert route_internal_pymes("ok gracias", history) == INTERNAL_PROCUREMENT
