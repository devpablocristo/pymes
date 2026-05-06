from __future__ import annotations

import unicodedata
from dataclasses import dataclass
from typing import Literal

from src.agents.catalog import (
    COLLECTIONS_DOMAIN_AGENT_NAME,
    CUSTOMERS_DOMAIN_AGENT_NAME,
    EMPLOYEES_DOMAIN_AGENT_NAME,
    INSIGHT_CHAT_AGENT_NAME,
    PRODUCTS_DOMAIN_AGENT_NAME,
    PRODUCT_AGENT_NAME,
    PURCHASES_DOMAIN_AGENT_NAME,
    SALES_DOMAIN_AGENT_NAME,
    SERVICES_DOMAIN_AGENT_NAME,
)
from src.api.chat_contract import ChatHandoff

AnalysisScope = Literal[
    "general",
    "sales_collections",
    "customers",
    "products",
    "services",
    "purchases",
    "scheduling",
    "operations",
    "employees",
]


@dataclass(frozen=True)
class InternalRouteDecision:
    scope: AnalysisScope
    routed_agent: str
    reason: str


_SALES_HINTS = (
    "venta",
    "ventas",
    "vendi",
    "vendimos",
    "vendio",
    "vendido",
    "vendiste",
    "vender",
    "factura",
    "facturacion",
    "facturación",
    "facture",
    "facturamos",
    "facturo",
    "ticket",
    "ingreso",
    "ingresos",
)
_COLLECTIONS_HINTS = (
    "cobro",
    "cobros",
    "cobrar",
    "deuda",
    "deudas",
    "deudor",
    "deudores",
    "me debe",
    "me deben",
    "debe plata",
    "deben plata",
    "debe dinero",
    "deben dinero",
    "saldo",
    "saldos",
    "pago pendiente",
    "pagos pendientes",
    "cuenta corriente",
)
_CUSTOMER_HINTS = ("cliente", "clientes", "compradores")
_PRODUCT_HINTS = ("producto", "productos", "stock", "inventario", "reponer", "reposicion", "reposición")
_SERVICE_HINTS = ("servicio", "servicios", "turno de servicio")
_PURCHASE_HINTS = ("compra", "compras", "proveedor", "proveedores", "solicitud de compra", "procurement")
_SCHEDULING_HINTS = ("agenda", "turno", "turnos", "reserva", "reservas", "calendario")
_EMPLOYEE_HINTS = ("empleado", "empleados", "personal", "equipo", "staff")
_OPERATIONS_HINTS = (
    "negocio",
    "operacion",
    "operación",
    "priorizar",
    "prioridad",
    "decisiones",
    "acciones",
    "resumen",
    "resumi",
    "resumí",
    "analiza",
    "analizá",
    "panorama",
    "como viene",
    "cómo viene",
)

_SCOPE_BY_ROUTE_HINT: dict[str, AnalysisScope] = {
    "general": "general",
    "insight_chat": "operations",
    "customers": "customers",
    "products": "products",
    "services": "services",
    "sales": "sales_collections",
    "collections": "sales_collections",
    "purchases": "purchases",
    "employees": "employees",
}


def route_internal_message(
    message: str,
    *,
    route_hint: str | None = None,
    handoff: ChatHandoff | None = None,
) -> InternalRouteDecision:
    text = _normalize(message)

    if handoff is not None and handoff.insight_scope:
        if handoff.insight_scope == "sales_collections":
            return InternalRouteDecision("sales_collections", INSIGHT_CHAT_AGENT_NAME, "structured_handoff")
        if handoff.insight_scope == "inventory_profit":
            return InternalRouteDecision("products", INSIGHT_CHAT_AGENT_NAME, "structured_handoff")
        if handoff.insight_scope == "customers_retention":
            return InternalRouteDecision("customers", INSIGHT_CHAT_AGENT_NAME, "structured_handoff")

    has_sales = _has_any(text, _SALES_HINTS)
    has_collections = _has_any(text, _COLLECTIONS_HINTS)
    has_customers = _has_any(text, _CUSTOMER_HINTS)
    has_priority = _has_any(text, ("priorizar", "prioridad", "seguimiento", "seguir", "accion", "acción"))

    if has_collections and (has_sales or has_customers or has_priority):
        return InternalRouteDecision("sales_collections", COLLECTIONS_DOMAIN_AGENT_NAME, "collections_with_sales_context")
    if has_sales and (has_customers or has_priority or route_hint == "sales"):
        return InternalRouteDecision("sales_collections", SALES_DOMAIN_AGENT_NAME, "sales_analysis")
    if has_collections:
        return InternalRouteDecision("sales_collections", COLLECTIONS_DOMAIN_AGENT_NAME, "collections_analysis")
    if has_sales:
        return InternalRouteDecision("sales_collections", SALES_DOMAIN_AGENT_NAME, "sales_analysis")
    if _has_any(text, _PRODUCT_HINTS):
        return InternalRouteDecision("products", PRODUCTS_DOMAIN_AGENT_NAME, "product_analysis")
    if _has_any(text, _SERVICE_HINTS):
        return InternalRouteDecision("services", SERVICES_DOMAIN_AGENT_NAME, "service_analysis")
    if _has_any(text, _PURCHASE_HINTS):
        return InternalRouteDecision("purchases", PURCHASES_DOMAIN_AGENT_NAME, "purchase_analysis")
    if _has_any(text, _SCHEDULING_HINTS):
        return InternalRouteDecision("scheduling", PRODUCT_AGENT_NAME, "scheduling_analysis")
    if _has_any(text, _EMPLOYEE_HINTS):
        return InternalRouteDecision("employees", EMPLOYEES_DOMAIN_AGENT_NAME, "employee_analysis")
    if has_customers:
        return InternalRouteDecision("customers", CUSTOMERS_DOMAIN_AGENT_NAME, "customer_analysis")
    if _has_any(text, _OPERATIONS_HINTS):
        return InternalRouteDecision("operations", PRODUCT_AGENT_NAME, "business_overview")

    hinted_scope = _SCOPE_BY_ROUTE_HINT.get(str(route_hint or "").strip())
    if hinted_scope:
        return InternalRouteDecision(hinted_scope, _routed_agent_for_scope(hinted_scope, route_hint), "route_hint")

    return InternalRouteDecision("general", PRODUCT_AGENT_NAME, "general_chat")


def _routed_agent_for_scope(scope: AnalysisScope, route_hint: str | None) -> str:
    if route_hint in {
        CUSTOMERS_DOMAIN_AGENT_NAME,
        PRODUCTS_DOMAIN_AGENT_NAME,
        SERVICES_DOMAIN_AGENT_NAME,
        SALES_DOMAIN_AGENT_NAME,
        COLLECTIONS_DOMAIN_AGENT_NAME,
        PURCHASES_DOMAIN_AGENT_NAME,
        EMPLOYEES_DOMAIN_AGENT_NAME,
        INSIGHT_CHAT_AGENT_NAME,
    }:
        return str(route_hint)
    if scope == "customers":
        return CUSTOMERS_DOMAIN_AGENT_NAME
    if scope == "products":
        return PRODUCTS_DOMAIN_AGENT_NAME
    if scope == "services":
        return SERVICES_DOMAIN_AGENT_NAME
    if scope == "purchases":
        return PURCHASES_DOMAIN_AGENT_NAME
    if scope == "sales_collections":
        return SALES_DOMAIN_AGENT_NAME
    if scope == "employees":
        return EMPLOYEES_DOMAIN_AGENT_NAME
    return PRODUCT_AGENT_NAME


def _normalize(value: str) -> str:
    decomposed = unicodedata.normalize("NFKD", value or "")
    without_accents = "".join(ch for ch in decomposed if not unicodedata.combining(ch))
    return f" {without_accents.lower()} "


def _has_any(text: str, hints: tuple[str, ...]) -> bool:
    return any(_normalize(hint).strip() in text for hint in hints)
