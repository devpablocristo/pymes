"""Endpoint callback para recibir resoluciones de Nexus Review."""
from __future__ import annotations

import logging
from datetime import UTC, datetime
from typing import Any
from uuid import UUID

from fastapi import APIRouter, HTTPException, Request, status
from pydantic import BaseModel

from src.db.repository import AIRepository
from src.db.engine import get_session

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/v1/internal", tags=["review-callback"])


class ReviewCallbackPayload(BaseModel):
    request_id: str
    decision: str  # approved | rejected | denied
    decided_by: str = ""
    decision_note: str = ""


@router.post("/review-callback", status_code=status.HTTP_200_OK)
async def review_callback(payload: ReviewCallbackPayload, request: Request) -> dict[str, str]:
    """Recibe notificación de Review cuando una aprobación se resuelve."""
    settings = request.app.state.settings

    # Validar token interno
    token = request.headers.get("X-Internal-Service-Token", "")
    if not token or token != settings.review_callback_token:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="invalid token")

    try:
        UUID(payload.request_id)
    except ValueError:
        logger.warning(
            "review_callback_invalid_request_id",
            extra={"request_id": payload.request_id},
        )
        return {"status": "ignored", "reason": "invalid request_id"}

    async with get_session() as session:
        repo = AIRepository(session)

        # Buscar conversación con este review_request_id pendiente
        conversation = await repo.find_conversation_by_review_request(payload.request_id)
        if conversation is None:
            logger.warning(
                "review_callback_no_conversation",
                extra={"request_id": payload.request_id},
            )
            return {"status": "ignored", "reason": "no matching conversation"}

        pending_action = conversation.pending_action
        if not pending_action:
            logger.warning(
                "review_callback_no_pending_action",
                extra={"request_id": payload.request_id, "conversation_id": conversation.id},
            )
            return {"status": "ignored", "reason": "no pending action"}

        decision_normalized = payload.decision.strip().lower()
        is_approved = decision_normalized in ("approved", "allow", "allowed")
        is_rejected = decision_normalized in ("rejected", "denied", "deny")

        reply: str
        if is_approved:
            # Ejecutar la acción pendiente
            reply = await _execute_pending_action(
                request=request,
                repo=repo,
                conversation=conversation,
                pending_action=pending_action,
                decided_by=payload.decided_by,
            )
        elif is_rejected:
            reply = "Tu solicitud fue revisada y no pudo ser aprobada en este momento. Por favor, comunicate directamente con el local para más información."
        else:
            logger.warning("review_callback_unknown_decision", extra={"decision": payload.decision})
            return {"status": "ignored", "reason": "unknown decision"}

        # Enviar respuesta por WhatsApp si hay teléfono de contacto
        if conversation.contact_phone and reply:
            await _send_whatsapp_reply(
                request=request,
                org_id=conversation.org_id,
                contact_phone=conversation.contact_phone,
                party_id=conversation.party_id,
                reply=reply,
            )

        # Guardar mensaje de respuesta y limpiar estado pendiente
        now = datetime.now(UTC).isoformat()
        assistant_message = {
            "role": "assistant",
            "content": reply,
            "ts": now,
            "review_decision": payload.decision,
            "decided_by": payload.decided_by,
        }
        await repo.append_messages(
            org_id=conversation.org_id,
            conversation_id=conversation.id,
            new_messages=[assistant_message],
            tool_calls_count=0,
            tokens_input=0,
            tokens_output=0,
        )
        await repo.clear_pending_review(conversation.id)

    return {"status": "processed"}


async def _execute_pending_action(
    *,
    request: Request,
    repo: AIRepository,
    conversation: Any,
    pending_action: dict[str, Any],
    decided_by: str,
) -> str:
    """Ejecuta la acción pendiente que fue aprobada."""
    action_type = str(pending_action.get("type", ""))
    tool_name = str(pending_action.get("tool_name", ""))
    tool_args = dict(pending_action.get("tool_args", {}))

    logger.info(
        "review_callback_executing",
        extra={
            "action_type": action_type,
            "tool_name": tool_name,
            "conversation_id": conversation.id,
            "decided_by": decided_by,
        },
    )

    # Ejecutar el tool aprobado vía el backend client
    backend_client = request.app.state.backend_client
    try:
        if tool_name == "book_appointment":
            from src.tools import appointments
            result = await appointments.book_appointment(backend_client, org_id=conversation.org_id, **tool_args)
            return f"Tu turno fue confirmado. {result.get('message', 'Te esperamos!')}"

        if tool_name == "cancel_appointment":
            from src.tools import appointments
            result = await appointments.cancel_appointment(backend_client, org_id=conversation.org_id, **tool_args)
            return "Tu turno fue cancelado exitosamente."

        if tool_name == "create_sale":
            from src.tools import sales
            auth_stub = type("Auth", (), {"org_id": conversation.org_id, "actor": decided_by, "role": "admin"})()
            result = await sales.create_sale(backend_client, auth_stub, **tool_args)
            return f"La venta fue registrada. {result.get('message', '')}"

        if tool_name == "generate_payment_link":
            result = await backend_client.request(
                "POST",
                f"/v1/sales/{tool_args.get('reference_id', '')}/payment-link",
                include_internal=True,
            )
            link = result.get("payment_url", "")
            return f"Link de pago generado: {link}" if link else "El link de pago fue generado."

        # Acción genérica
        return "Tu solicitud fue aprobada y procesada."

    except Exception:
        logger.exception("review_callback_execution_failed", extra={"tool_name": tool_name})
        return "Tu solicitud fue aprobada pero hubo un problema al procesarla. El equipo fue notificado."


async def _send_whatsapp_reply(
    *,
    request: Request,
    org_id: str,
    contact_phone: str,
    party_id: str | None,
    reply: str,
) -> None:
    """Envía respuesta por WhatsApp vía Pymes Core."""
    backend_client = request.app.state.backend_client
    try:
        body: dict[str, Any] = {"body": reply}
        if party_id:
            body["party_id"] = party_id
        else:
            body["phone"] = contact_phone

        await backend_client.request(
            "POST",
            "/v1/whatsapp/send/text",
            json=body,
            include_internal=True,
        )
    except Exception:
        logger.exception(
            "review_callback_whatsapp_send_failed",
            extra={"org_id": org_id, "contact_phone": contact_phone},
        )
