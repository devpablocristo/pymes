"""Tool para que el dueño configure reglas de atención automática vía chat."""
from __future__ import annotations

import re
import logging
from typing import Any

from runtime.types import ToolDeclaration
from src.review_client.client import ReviewClient

logger = logging.getLogger(__name__)

manage_review_policy = ToolDeclaration(
    name="manage_review_policy",
    description=(
        "Configura las reglas de atencion automatica al cliente. "
        "Permite definir que acciones el asistente puede hacer automaticamente (allow), "
        "cuales requieren aprobacion del dueno (require_approval), y cuales estan prohibidas (deny). "
        "Usa action=list para ver reglas actuales, action=create para crear, "
        "action=update para modificar, action=delete para eliminar."
    ),
    parameters={
        "type": "object",
        "properties": {
            "action": {
                "type": "string",
                "description": "create | update | delete | list",
                "enum": ["create", "update", "delete", "list"],
            },
            "action_type": {
                "type": "string",
                "description": "Tipo de accion (appointment.book, discount.apply, etc.)",
            },
            "effect": {
                "type": "string",
                "description": "allow | require_approval | deny",
                "enum": ["allow", "require_approval", "deny"],
            },
            "condition": {
                "type": "string",
                "description": "Condicion en lenguaje natural, ej: 'descuento mayor a 15%'",
            },
            "policy_id": {
                "type": "string",
                "description": "ID de la politica a modificar o eliminar",
            },
        },
        "required": ["action"],
    },
)

# Nombres amigables para action types
ACTION_TYPE_DISPLAY: dict[str, str] = {
    "appointment.book": "Agendar turno",
    "appointment.reschedule": "Reagendar turno",
    "appointment.cancel": "Cancelar turno",
    "discount.apply": "Aplicar descuento",
    "payment_link.generate": "Generar link de pago",
    "sale.create": "Crear venta",
    "quote.create": "Crear presupuesto",
    "refund.create": "Reembolso",
    "cashflow.movement": "Movimiento de caja",
    "purchase.draft": "Borrador de compra",
    "procurement.request": "Solicitud de compra",
    "notification.send": "Enviar notificacion",
    "notification.bulk_send": "Envio masivo de notificaciones",
    "work_order.delay_notify": "Avisar demora de OT",
    "vehicle.service_reminder": "Recordatorio de service",
}

EFFECT_DISPLAY: dict[str, str] = {
    "allow": "Automatico",
    "require_approval": "Requiere aprobacion",
    "deny": "No permitido",
}


def _build_cel_expression(action_type: str, condition: str | None) -> str:
    """Traduce una condición en lenguaje natural a expresión CEL."""
    base = f'request.action_type == "{action_type}"'

    if not condition:
        return base

    normalized = condition.strip().lower()

    # "descuento mayor a X%" o "porcentaje mayor a X"
    match = re.search(r"(?:descuento|porcentaje)\s+mayor\s+(?:a|al?)\s+(\d+(?:\.\d+)?)\s*%?", normalized)
    if match:
        threshold = float(match.group(1))
        return f'{base} && double(request.params.percentage) > {threshold}'

    # "descuento menor o igual a X%"
    match = re.search(r"(?:descuento|porcentaje)\s+menor\s+o?\s*igual\s+(?:a|al?)\s+(\d+(?:\.\d+)?)\s*%?", normalized)
    if match:
        threshold = float(match.group(1))
        return f'{base} && double(request.params.percentage) <= {threshold}'

    # "dentro de X dias"
    match = re.search(r"dentro\s+de\s+(\d+)\s+d[ií]as?", normalized)
    if match:
        days = int(match.group(1))
        return f'{base} && int(request.params.days_from_now) <= {days}'

    # "monto mayor a X"
    match = re.search(r"monto\s+mayor\s+(?:a|al?)\s+(\d+(?:\.\d+)?)", normalized)
    if match:
        amount = float(match.group(1))
        return f'{base} && double(request.params.amount) > {amount}'

    # Si no matchea ningún patrón, usar solo el action_type
    logger.warning("cel_condition_not_recognized", extra={"condition": condition, "action_type": action_type})
    return base


async def handle_manage_review_policy(
    review_client: ReviewClient,
    *,
    org_id: str,
    action: str,
    action_type: str = "",
    effect: str = "",
    condition: str | None = None,
    policy_id: str = "",
) -> dict[str, Any]:
    """Handler del tool manage_review_policy."""

    if action == "list":
        policies = await review_client.list_policies()
        if not policies:
            return {"message": "No hay reglas configuradas todavia."}
        lines = []
        for p in policies:
            display = ACTION_TYPE_DISPLAY.get(p.action_type, p.action_type)
            effect_display = EFFECT_DISPLAY.get(p.effect, p.effect)
            lines.append(f"- {display}: {effect_display} (ID: {p.id})")
        return {"message": "Reglas actuales:\n" + "\n".join(lines), "policies": [{"id": p.id, "name": p.name, "action_type": p.action_type, "effect": p.effect} for p in policies]}

    if action == "create":
        if not action_type or not effect:
            return {"error": "Se necesita action_type y effect para crear una regla."}
        expression = _build_cel_expression(action_type, condition)
        display = ACTION_TYPE_DISPLAY.get(action_type, action_type)
        effect_display = EFFECT_DISPLAY.get(effect, effect)
        name = f"{action_type}-{effect}"
        if condition:
            name = f"{action_type}-{effect}-custom"

        result = await review_client.create_policy(
            name=name,
            action_type=action_type,
            expression=expression,
            effect=effect,
            mode="enforced",
        )
        if result is None:
            return {"error": "No se pudo crear la regla. Intenta de nuevo."}
        condition_text = f" cuando {condition}" if condition else ""
        return {"message": f"Regla creada: {display} -> {effect_display}{condition_text}.", "policy_id": result.id}

    if action == "update":
        if not policy_id:
            return {"error": "Se necesita policy_id para modificar una regla."}
        updates: dict[str, Any] = {}
        if effect:
            updates["effect"] = effect
        if condition is not None:
            updates["expression"] = _build_cel_expression(action_type or "", condition)
        if not updates:
            return {"error": "No se indicaron cambios."}
        result = await review_client.update_policy(policy_id, **updates)
        if result is None:
            return {"error": "No se pudo actualizar la regla."}
        return {"message": f"Regla {policy_id} actualizada."}

    if action == "delete":
        if not policy_id:
            return {"error": "Se necesita policy_id para eliminar una regla."}
        ok = await review_client.delete_policy(policy_id)
        if not ok:
            return {"error": "No se pudo eliminar la regla."}
        return {"message": f"Regla {policy_id} eliminada."}

    return {"error": f"Accion no reconocida: {action}"}
