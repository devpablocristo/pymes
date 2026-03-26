from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from runtime.contexts import AuthContext
from runtime.types import ToolDeclaration
from src.domains.workshops.auto_repair.backend_client import AutoRepairBackendClient

ToolHandler = Callable[..., Awaitable[dict[str, Any]]]


def _tool(name: str, description: str, parameters: dict[str, Any]) -> ToolDeclaration:
    return ToolDeclaration(name=name, description=description, parameters=parameters)


def build_internal_tools(
    client: AutoRepairBackendClient,
    auth: AuthContext,
) -> tuple[list[ToolDeclaration], dict[str, ToolHandler]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, ToolHandler] = {}

    async def _list_vehicles(org_id: str, search: str = "") -> dict[str, Any]:
        _ = org_id
        return await client.list_vehicles(auth, search=search)

    declarations.append(
        _tool(
            "list_vehicles",
            "Listar vehiculos del taller por patente, cliente o texto libre",
            {
                "type": "object",
                "properties": {
                    "search": {"type": "string", "description": "Patente, VIN, cliente o texto libre"},
                },
            },
        )
    )
    handlers["list_vehicles"] = _list_vehicles

    async def _get_vehicle(org_id: str, vehicle_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.get_vehicle(auth, vehicle_id=vehicle_id)

    declarations.append(
        _tool(
            "get_vehicle",
            "Ver detalle de un vehiculo",
            {
                "type": "object",
                "properties": {
                    "vehicle_id": {"type": "string", "description": "UUID del vehiculo"},
                },
                "required": ["vehicle_id"],
            },
        )
    )
    handlers["get_vehicle"] = _get_vehicle

    async def _list_services(org_id: str, search: str = "") -> dict[str, Any]:
        _ = org_id
        return await client.list_services(auth, search=search)

    declarations.append(
        _tool(
            "list_services",
            "Listar servicios y reparaciones del taller",
            {
                "type": "object",
                "properties": {
                    "search": {"type": "string", "description": "Codigo, nombre o categoria"},
                },
            },
        )
    )
    handlers["list_services"] = _list_services

    async def _list_work_orders(org_id: str, status: str = "", search: str = "") -> dict[str, Any]:
        _ = org_id
        return await client.list_work_orders(auth, status=status, search=search)

    declarations.append(
        _tool(
            "list_work_orders",
            "Listar ordenes de trabajo por estado o busqueda libre",
            {
                "type": "object",
                "properties": {
                    "status": {"type": "string", "description": "ingresado, en_reparacion, listo, entregado, facturado"},
                    "search": {"type": "string", "description": "Numero de orden, patente o cliente"},
                },
            },
        )
    )
    handlers["list_work_orders"] = _list_work_orders

    async def _get_work_order(org_id: str, work_order_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.get_work_order(auth, work_order_id=work_order_id)

    declarations.append(
        _tool(
            "get_work_order",
            "Ver detalle de una orden de trabajo",
            {
                "type": "object",
                "properties": {
                    "work_order_id": {"type": "string", "description": "UUID de la orden"},
                },
                "required": ["work_order_id"],
            },
        )
    )
    handlers["get_work_order"] = _get_work_order

    async def _create_appointment(
        org_id: str,
        customer_name: str,
        customer_phone: str,
        title: str,
        start_at: str,
        duration: int = 60,
        notes: str = "",
    ) -> dict[str, Any]:
        _ = org_id
        data: dict[str, Any] = {
            "customer_name": customer_name,
            "customer_phone": customer_phone,
            "title": title,
            "start_at": start_at,
            "duration": duration,
        }
        if notes:
            data["notes"] = notes
        return await client.create_appointment(auth, data=data)

    declarations.append(
        _tool(
            "create_appointment",
            "Reservar turno para ingreso al taller",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "customer_phone": {"type": "string"},
                    "title": {"type": "string", "description": "Motivo del turno"},
                    "start_at": {"type": "string", "description": "Fecha y hora RFC3339"},
                    "duration": {"type": "integer", "description": "Duracion en minutos"},
                    "notes": {"type": "string"},
                },
                "required": ["customer_name", "customer_phone", "title", "start_at"],
            },
        )
    )
    handlers["create_appointment"] = _create_appointment

    async def _create_quote(org_id: str, work_order_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.create_quote(auth, work_order_id=work_order_id)

    declarations.append(
        _tool(
            "create_quote",
            "Generar presupuesto desde una orden de trabajo",
            {
                "type": "object",
                "properties": {
                    "work_order_id": {"type": "string", "description": "UUID de la orden"},
                },
                "required": ["work_order_id"],
            },
        )
    )
    handlers["create_quote"] = _create_quote

    async def _create_sale(org_id: str, work_order_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.create_sale(auth, work_order_id=work_order_id)

    declarations.append(
        _tool(
            "create_sale",
            "Generar venta desde una orden de trabajo",
            {
                "type": "object",
                "properties": {
                    "work_order_id": {"type": "string", "description": "UUID de la orden"},
                },
                "required": ["work_order_id"],
            },
        )
    )
    handlers["create_sale"] = _create_sale

    async def _create_payment_link(org_id: str, work_order_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.create_payment_link(auth, work_order_id=work_order_id)

    declarations.append(
        _tool(
            "create_payment_link",
            "Generar link de pago desde una orden de trabajo",
            {
                "type": "object",
                "properties": {
                    "work_order_id": {"type": "string", "description": "UUID de la orden"},
                },
                "required": ["work_order_id"],
            },
        )
    )
    handlers["create_payment_link"] = _create_payment_link

    return declarations, handlers


def build_external_tools(
    client: AutoRepairBackendClient,
    org_slug: str,
) -> tuple[list[ToolDeclaration], dict[str, ToolHandler]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, ToolHandler] = {}

    async def _get_public_services(org_id: str, search: str = "") -> dict[str, Any]:
        _ = org_id
        return await client.get_public_services(org_slug, search=search)

    declarations.append(
        _tool(
            "get_public_services",
            "Listar servicios publicos del taller",
            {
                "type": "object",
                "properties": {
                    "search": {"type": "string", "description": "Filtro por texto"},
                },
            },
        )
    )
    handlers["get_public_services"] = _get_public_services

    async def _book_appointment(
        org_id: str,
        customer_name: str,
        customer_phone: str,
        title: str,
        start_at: str,
        duration: int = 60,
    ) -> dict[str, Any]:
        _ = org_id
        return await client.public_book_appointment(
            org_slug,
            data={
                "party_name": customer_name,
                "party_phone": customer_phone,
                "title": title,
                "start_at": start_at,
                "duration": duration,
            },
        )

    declarations.append(
        _tool(
            "book_appointment",
            "Reservar turno en el taller",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "customer_phone": {"type": "string"},
                    "title": {"type": "string", "description": "Motivo del turno"},
                    "start_at": {"type": "string", "description": "Fecha y hora RFC3339"},
                    "duration": {"type": "integer", "description": "Duracion en minutos"},
                },
                "required": ["customer_name", "customer_phone", "title", "start_at"],
            },
        )
    )
    handlers["book_appointment"] = _book_appointment

    return declarations, handlers
