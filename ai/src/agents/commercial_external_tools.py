from __future__ import annotations

from typing import Any

from src.agents.commercial_runtime import CommercialRunState, _wrap_tool
from src.agents.policy import CommercialPolicy
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from runtime.types import ToolDeclaration
from src.tools import payments, products, scheduling
from src.agents.commercial_quote import _build_quote_preview


def _tool(name: str, description: str, parameters: dict[str, Any]) -> ToolDeclaration:
    return ToolDeclaration(name=name, description=description, parameters=parameters)

async def _build_external_sales_tools(
    *,
    client: BackendClient,
    repo: AIRepository,
    tenant_id: str,
    conversation_id: str | None,
    policy: CommercialPolicy,
    state: CommercialRunState,
    confirmed_actions: set[str],
    external_contact: str,
) -> tuple[list[ToolDeclaration], dict[str, Any]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, Any] = {}

    async def _get_business_info(tenant_id: str) -> dict[str, Any]:
        payload = await client.request("GET", f"/v1/public/{tenant_id}/info", include_internal=True)
        return {
            "business_name": str(payload.get("business_name") or payload.get("name") or "").strip(),
            "address": str(payload.get("business_address", "")).strip(),
            "phone": str(payload.get("business_phone", "")).strip(),
            "email": str(payload.get("business_email", "")).strip(),
            "scheduling_enabled": bool(payload.get("scheduling_enabled", False)),
        }

    async def _get_public_services(tenant_id: str, limit: int = 20) -> dict[str, Any]:
        payload = await products.get_public_services(client, tenant_id=tenant_id, limit=max(1, min(limit, 100)))
        items = []
        for row in list(payload.get("items", [])):
            items.append(
                {
                    "id": str(row.get("id", "")),
                    "name": str(row.get("name", "")),
                    "type": str(row.get("type", "")),
                    "description": str(row.get("description", "")),
                    "unit": str(row.get("unit", "unit")),
                    "price": float(row.get("price", 0) or 0),
                    "currency": str(row.get("currency", "ARS") or "ARS"),
                }
            )
        return {"items": items}

    async def _check_availability(tenant_id: str, date: str, duration: int = 60) -> dict[str, Any]:
        return await scheduling.check_availability(client, tenant_id=tenant_id, date=date, duration=duration)

    async def _get_my_bookings(tenant_id: str, phone: str) -> dict[str, Any]:
        return await scheduling.get_my_bookings(client, tenant_id=tenant_id, phone=phone)

    async def _request_quote(tenant_id: str, items: list[dict[str, Any]], customer_name: str = "", notes: str = "") -> dict[str, Any]:
        return await _build_quote_preview(client, tenant_id, items=items, customer_name=customer_name, notes=notes)

    async def _get_quote_payment_link(tenant_id: str, quote_id: str) -> dict[str, Any]:
        return await payments.get_public_quote_payment_link(client, tenant_id=tenant_id, quote_id=quote_id)

    async def _book_scheduling(
        tenant_id: str,
        customer_name: str,
        customer_phone: str,
        title: str,
        start_at: str,
        duration: int = 60,
    ) -> dict[str, Any]:
        return await scheduling.book_scheduling(
            client,
            tenant_id=tenant_id,
            customer_name=customer_name,
            customer_phone=customer_phone,
            title=title,
            start_at=start_at,
            duration=duration,
        )

    specs = [
        (
            _tool("get_business_info", "Obtener informacion publica del negocio", {"type": "object", "properties": {}}),
            _get_business_info,
        ),
        (
            _tool(
                "get_public_services",
                "Listar servicios o productos publicos",
                {"type": "object", "properties": {"limit": {"type": "integer"}}},
            ),
            _get_public_services,
        ),
        (
            _tool(
                "check_availability",
                "Consultar disponibilidad publica",
                {
                    "type": "object",
                    "properties": {
                        "date": {"type": "string", "description": "YYYY-MM-DD"},
                        "duration": {"type": "integer", "description": "duracion en minutos"},
                    },
                    "required": ["date"],
                },
            ),
            _check_availability,
        ),
        (
            _tool(
                "get_my_bookings",
                "Consultar turnos del cliente",
                {
                    "type": "object",
                    "properties": {"phone": {"type": "string"}},
                    "required": ["phone"],
                },
            ),
            _get_my_bookings,
        ),
        (
            _tool(
                "request_quote",
                "Preparar presupuesto preliminar controlado con catalogo publico",
                {
                    "type": "object",
                    "properties": {
                        "customer_name": {"type": "string"},
                        "notes": {"type": "string"},
                        "items": {
                            "type": "array",
                            "items": {
                                "type": "object",
                                "properties": {
                                    "product_id": {"type": "string"},
                                    "name": {"type": "string"},
                                    "quantity": {"type": "number"},
                                },
                            },
                        },
                    },
                    "required": ["items"],
                },
            ),
            _request_quote,
        ),
        (
            _tool(
                "get_quote_payment_link",
                "Obtener link publico de pago para un presupuesto existente",
                {
                    "type": "object",
                    "properties": {"quote_id": {"type": "string"}},
                    "required": ["quote_id"],
                },
            ),
            _get_quote_payment_link,
        ),
        (
            _tool(
                "book_scheduling",
                "Reservar un turno publico",
                {
                    "type": "object",
                    "properties": {
                        "customer_name": {"type": "string"},
                        "customer_phone": {"type": "string"},
                        "title": {"type": "string"},
                        "start_at": {"type": "string", "description": "RFC3339"},
                        "duration": {"type": "integer"},
                    },
                    "required": ["customer_name", "customer_phone", "title", "start_at"],
                },
            ),
            _book_scheduling,
        ),
    ]

    for declaration, raw_handler in specs:
        declarations.append(declaration)
        handlers[declaration.name] = await _wrap_tool(
            name=declaration.name,
            handler=raw_handler,
            repo=repo,
            tenant_id=tenant_id,
            conversation_id=conversation_id,
            policy=policy,
            state=state,
            actor_id=external_contact or "external",
            actor_type="external_contact",
            channel=policy.channel,
            confirmed_actions=confirmed_actions,
        )
    return declarations, handlers
