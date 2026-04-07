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

    async def _create_vehicle(
        org_id: str,
        license_plate: str,
        make: str,
        model: str,
        customer_name: str = "",
        customer_id: str = "",
        year: int = 0,
        vin: str = "",
        color: str = "",
        notes: str = "",
        kilometers: int = 0,
    ) -> dict[str, Any]:
        _ = org_id
        data: dict[str, Any] = {
            "license_plate": license_plate,
            "make": make,
            "model": model,
        }
        if customer_name:
            data["customer_name"] = customer_name
        if customer_id:
            data["customer_id"] = customer_id
        if year:
            data["year"] = year
        if vin:
            data["vin"] = vin
        if color:
            data["color"] = color
        if notes:
            data["notes"] = notes
        if kilometers:
            data["kilometers"] = kilometers
        return await client.create_vehicle(auth, data=data)

    declarations.append(
        _tool(
            "create_vehicle",
            "Registrar un nuevo vehiculo que ingresa al taller",
            {
                "type": "object",
                "properties": {
                    "license_plate": {"type": "string", "description": "Patente del vehiculo"},
                    "make": {"type": "string", "description": "Marca"},
                    "model": {"type": "string", "description": "Modelo"},
                    "customer_name": {"type": "string", "description": "Nombre del cliente"},
                    "customer_id": {"type": "string", "description": "UUID del cliente si ya existe"},
                    "year": {"type": "integer", "description": "Anio de fabricacion"},
                    "vin": {"type": "string", "description": "Numero de chasis"},
                    "color": {"type": "string"},
                    "notes": {"type": "string"},
                    "kilometers": {"type": "integer", "description": "Kilometraje actual"},
                },
                "required": ["license_plate", "make", "model"],
            },
        )
    )
    handlers["create_vehicle"] = _create_vehicle

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

    async def _update_work_order_status(
        org_id: str,
        work_order_id: str,
        status: str,
        notes: str = "",
    ) -> dict[str, Any]:
        _ = org_id
        data: dict[str, Any] = {"status": status}
        if notes:
            data["notes"] = notes
        return await client.update_work_order(auth, work_order_id=work_order_id, data=data)

    declarations.append(
        _tool(
            "update_work_order_status",
            "Mover una orden de trabajo por el pipeline (ingresado, en_reparacion, listo, entregado, facturado)",
            {
                "type": "object",
                "properties": {
                    "work_order_id": {"type": "string", "description": "UUID de la orden"},
                    "status": {"type": "string", "description": "ingresado, en_reparacion, listo, entregado, facturado"},
                    "notes": {"type": "string", "description": "Notas opcionales del cambio de estado"},
                },
                "required": ["work_order_id", "status"],
            },
        )
    )
    handlers["update_work_order_status"] = _update_work_order_status

    async def _create_booking(
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
        return await client.create_booking(auth, data=data)

    declarations.append(
        _tool(
            "create_booking",
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
    handlers["create_booking"] = _create_booking

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

    async def _book_scheduling(
        org_id: str,
        customer_name: str,
        customer_phone: str,
        title: str,
        start_at: str,
        duration: int = 60,
    ) -> dict[str, Any]:
        _ = org_id
        return await client.public_book_scheduling(
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
            "book_scheduling",
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
    handlers["book_scheduling"] = _book_scheduling

    return declarations, handlers
