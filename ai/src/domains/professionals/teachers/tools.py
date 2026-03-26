from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from src.domains.professionals.teachers.backend_client import TeachersBackendClient
from runtime.contexts import AuthContext, ToolDeclaration

ToolHandler = Callable[..., Awaitable[dict[str, Any]]]


def _tool(name: str, description: str, parameters: dict[str, Any]) -> ToolDeclaration:
    return ToolDeclaration(name=name, description=description, parameters=parameters)


def build_internal_tools(
    client: TeachersBackendClient,
    auth: AuthContext,
) -> tuple[list[ToolDeclaration], dict[str, ToolHandler]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, ToolHandler] = {}

    async def _get_teacher_profiles(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.get_teachers(auth)

    declarations.append(
        _tool(
            "get_teacher_profiles",
            "Listar perfiles de docentes o profesionales del estudio",
            {"type": "object", "properties": {}},
        )
    )
    handlers["get_teacher_profiles"] = _get_teacher_profiles

    async def _get_specialties(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.get_specialties(auth)

    declarations.append(
        _tool(
            "get_specialties",
            "Listar especialidades disponibles",
            {"type": "object", "properties": {}},
        )
    )
    handlers["get_specialties"] = _get_specialties

    async def _get_teacher_catalog(org_id: str, profile_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.get_teacher_services(auth, profile_id=profile_id)

    declarations.append(
        _tool(
            "get_teacher_catalog",
            "Listar servicios de un docente o profesional",
            {
                "type": "object",
                "properties": {
                    "profile_id": {"type": "string", "description": "UUID del docente"},
                },
                "required": ["profile_id"],
            },
        )
    )
    handlers["get_teacher_catalog"] = _get_teacher_catalog

    async def _get_today_schedule(org_id: str, date: str = "today") -> dict[str, Any]:
        _ = org_id
        return await client.get_sessions(auth, filters={"date": date})

    declarations.append(
        _tool(
            "get_today_schedule",
            "Ver agenda del dia: sesiones e intakes programados",
            {
                "type": "object",
                "properties": {
                    "date": {"type": "string", "description": "YYYY-MM-DD, default hoy"},
                },
            },
        )
    )
    handlers["get_today_schedule"] = _get_today_schedule

    async def _create_intake(
        org_id: str,
        profile_id: str,
        notes: str = "",
        appointment_id: str = "",
        customer_party_id: str = "",
        product_id: str = "",
    ) -> dict[str, Any]:
        _ = org_id
        data = {"profile_id": profile_id, "payload": {"notes": notes}}
        if appointment_id:
            data["appointment_id"] = appointment_id
        if customer_party_id:
            data["customer_party_id"] = customer_party_id
        if product_id:
            data["product_id"] = product_id
        return await client.create_intake(auth, data=data)

    declarations.append(
        _tool(
            "create_intake",
            "Crear ficha de intake para un nuevo alumno o cliente",
            {
                "type": "object",
                "properties": {
                    "profile_id": {"type": "string", "description": "UUID del docente o profesional"},
                    "notes": {"type": "string", "description": "Notas adicionales"},
                    "appointment_id": {"type": "string", "description": "UUID del turno"},
                    "customer_party_id": {"type": "string", "description": "UUID del party del cliente"},
                    "product_id": {"type": "string", "description": "UUID del producto asociado"},
                },
                "required": ["profile_id"],
            },
        )
    )
    handlers["create_intake"] = _create_intake

    async def _update_intake(
        org_id: str,
        intake_id: str,
        notes: str = "",
    ) -> dict[str, Any]:
        _ = org_id
        data: dict[str, Any] = {}
        if notes:
            data["payload"] = {"notes": notes}
        return await client.update_intake(auth, intake_id=intake_id, data=data)

    declarations.append(
        _tool(
            "update_intake",
            "Actualizar una ficha de intake existente",
            {
                "type": "object",
                "properties": {
                    "intake_id": {"type": "string", "description": "UUID del intake"},
                    "notes": {"type": "string", "description": "Notas a agregar"},
                },
                "required": ["intake_id"],
            },
        )
    )
    handlers["update_intake"] = _update_intake

    async def _get_session_summary(org_id: str, session_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.get_session(auth, session_id=session_id)

    declarations.append(
        _tool(
            "get_session_summary",
            "Ver detalle de una sesion",
            {
                "type": "object",
                "properties": {
                    "session_id": {"type": "string", "description": "UUID de la sesion"},
                },
                "required": ["session_id"],
            },
        )
    )
    handlers["get_session_summary"] = _get_session_summary

    async def _book_appointment(
        org_id: str,
        customer_name: str,
        customer_phone: str,
        title: str,
        start_at: str,
        professional_id: str = "",
        duration: int = 60,
    ) -> dict[str, Any]:
        _ = org_id
        data: dict[str, Any] = {
            "customer_name": customer_name,
            "customer_phone": customer_phone,
            "title": title,
            "start_at": start_at,
            "duration": duration,
        }
        if professional_id:
            data["professional_id"] = professional_id
        return await client.book_appointment(auth, data=data)

    declarations.append(
        _tool(
            "book_appointment",
            "Reservar turno para un cliente",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "customer_phone": {"type": "string"},
                    "title": {"type": "string", "description": "Motivo o tipo de consulta"},
                    "start_at": {"type": "string", "description": "Fecha y hora RFC3339"},
                    "professional_id": {"type": "string", "description": "UUID del docente o profesional (opcional)"},
                    "duration": {"type": "integer", "description": "Duracion en minutos, default 60"},
                },
                "required": ["customer_name", "customer_phone", "title", "start_at"],
            },
        )
    )
    handlers["book_appointment"] = _book_appointment

    async def _prepare_quote(
        org_id: str,
        customer_name: str,
        items: list[dict[str, Any]],
        notes: str = "",
    ) -> dict[str, Any]:
        _ = org_id
        return await client.prepare_quote(auth, data={"customer_name": customer_name, "items": items, "notes": notes})

    declarations.append(
        _tool(
            "prepare_quote",
            "Preparar presupuesto para un cliente",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "items": {
                        "type": "array",
                        "items": {"type": "object"},
                        "description": "Lista de servicios con nombre, precio y cantidad",
                    },
                    "notes": {"type": "string"},
                },
                "required": ["customer_name", "items"],
            },
        )
    )
    handlers["prepare_quote"] = _prepare_quote

    async def _get_payment_link(org_id: str, sale_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.get_payment_link(auth, sale_id=sale_id)

    declarations.append(
        _tool(
            "get_payment_link",
            "Generar link de pago para una venta",
            {
                "type": "object",
                "properties": {
                    "sale_id": {"type": "string", "description": "UUID de la venta"},
                },
                "required": ["sale_id"],
            },
        )
    )
    handlers["get_payment_link"] = _get_payment_link

    return declarations, handlers


def build_external_tools(
    client: TeachersBackendClient,
    org_slug: str,
) -> tuple[list[ToolDeclaration], dict[str, ToolHandler]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, ToolHandler] = {}

    async def _get_public_teachers(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.get_public_teachers(org_slug)

    declarations.append(
        _tool(
            "get_public_teachers",
            "Listar docentes o profesionales del estudio con sus especialidades",
            {"type": "object", "properties": {}},
        )
    )
    handlers["get_public_teachers"] = _get_public_teachers

    async def _get_public_catalog(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await client.get_public_catalog(org_slug)

    declarations.append(
        _tool(
            "get_public_catalog",
            "Listar servicios disponibles con precios y duracion",
            {"type": "object", "properties": {}},
        )
    )
    handlers["get_public_catalog"] = _get_public_catalog

    async def _check_availability(org_id: str, date: str, professional_id: str = "") -> dict[str, Any]:
        _ = org_id
        return await client.get_public_availability(org_slug, date=date, professional_id=professional_id or None)

    declarations.append(
        _tool(
            "check_availability",
            "Consultar turnos disponibles para una fecha",
            {
                "type": "object",
                "properties": {
                    "date": {"type": "string", "description": "YYYY-MM-DD"},
                    "professional_id": {"type": "string", "description": "UUID del docente o profesional (opcional)"},
                },
                "required": ["date"],
            },
        )
    )
    handlers["check_availability"] = _check_availability

    async def _book_appointment(
        org_id: str,
        customer_name: str,
        customer_phone: str,
        title: str,
        start_at: str,
        professional_id: str = "",
        duration: int = 60,
    ) -> dict[str, Any]:
        _ = org_id
        data: dict[str, Any] = {
            "customer_name": customer_name,
            "customer_phone": customer_phone,
            "title": title,
            "start_at": start_at,
            "duration": duration,
        }
        if professional_id:
            data["professional_id"] = professional_id
        return await client.public_book_appointment(org_slug, data=data)

    declarations.append(
        _tool(
            "book_appointment",
            "Reservar turno",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "customer_phone": {"type": "string"},
                    "title": {"type": "string", "description": "Motivo de consulta"},
                    "start_at": {"type": "string", "description": "Fecha y hora RFC3339"},
                    "professional_id": {"type": "string", "description": "UUID del docente o profesional (opcional)"},
                    "duration": {"type": "integer", "description": "Duracion en minutos"},
                },
                "required": ["customer_name", "customer_phone", "title", "start_at"],
            },
        )
    )
    handlers["book_appointment"] = _book_appointment

    return declarations, handlers
