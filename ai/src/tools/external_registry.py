from __future__ import annotations

from typing import Any

from src.backend_client.client import BackendClient
from src.tools import payments, products, scheduling
from src.tools.registry_common import ToolHandler, tool
from runtime.types import ToolDeclaration


def build_external_tools(client: BackendClient) -> tuple[list[ToolDeclaration], dict[str, ToolHandler]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, ToolHandler] = {}

    async def _check_availability(tenant_id: str, date: str, duration: int = 60) -> dict[str, Any]:
        return await scheduling.check_availability(client, tenant_id=tenant_id, date=date, duration=duration)

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

    async def _get_public_services(tenant_id: str, limit: int = 20) -> dict[str, Any]:
        return await products.get_public_services(client, tenant_id=tenant_id, limit=limit)

    async def _get_business_info(tenant_id: str) -> dict[str, Any]:
        return await client.request("GET", f"/v1/public/{tenant_id}/info", include_internal=True)

    async def _get_my_bookings(tenant_id: str, phone: str) -> dict[str, Any]:
        return await scheduling.get_my_bookings(client, tenant_id=tenant_id, phone=phone)

    async def _get_payment_link(tenant_id: str, quote_id: str) -> dict[str, Any]:
        return await payments.get_public_quote_payment_link(client, tenant_id=tenant_id, quote_id=quote_id)

    declarations.append(
        tool(
            "check_availability",
            "Consultar slots disponibles",
            {
                "type": "object",
                "properties": {
                    "date": {"type": "string", "description": "YYYY-MM-DD"},
                    "duration": {"type": "integer", "description": "duracion en minutos"},
                },
                "required": ["date"],
            },
        )
    )
    handlers["check_availability"] = _check_availability

    declarations.append(
        tool(
            "book_scheduling",
            "Reservar turno",
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
        )
    )
    handlers["book_scheduling"] = _book_scheduling

    declarations.append(
        tool(
            "get_public_services",
            "Listar servicios/productos publicos",
            {
                "type": "object",
                "properties": {"limit": {"type": "integer", "description": "max 100"}},
            },
        )
    )
    handlers["get_public_services"] = _get_public_services

    declarations.append(tool("get_business_info", "Informacion del negocio", {"type": "object", "properties": {}}))
    handlers["get_business_info"] = _get_business_info

    declarations.append(
        tool(
            "get_my_bookings",
            "Consultar turnos de un cliente por telefono",
            {
                "type": "object",
                "properties": {"phone": {"type": "string"}},
                "required": ["phone"],
            },
        )
    )
    handlers["get_my_bookings"] = _get_my_bookings

    declarations.append(
        tool(
            "get_payment_link",
            "Obtener link de pago de un presupuesto",
            {
                "type": "object",
                "properties": {"quote_id": {"type": "string", "description": "UUID del presupuesto"}},
                "required": ["quote_id"],
            },
        )
    )
    handlers["get_payment_link"] = _get_payment_link

    return declarations, handlers

