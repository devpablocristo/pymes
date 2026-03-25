from __future__ import annotations

import copy
from datetime import UTC, datetime
from typing import Any

from fastapi import HTTPException, status

from src.agents.audit import has_processed_request, record_agent_event
from src.agents.contracts import CommercialContractEnvelope
from src.agents.policy import build_external_sales_policy, build_internal_procurement_policy, build_internal_sales_policy
from src.agents.service_support import (
    CommercialChatResult,
    CommercialRunState,
    _build_external_sales_tools,
    _build_internal_sales_tools,
    _build_procurement_tools,
    _build_quote_preview,
    _load_internal_conversation,
    build_commercial_prompt,
    estimate_tokens,
    sanitize_message,
)
from src.api.external_chat_support import get_external_conversation, history_to_messages
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.core.dossier import summarize_dossier_for_context
from core_ai.orchestrator import OrchestratorLimits, orchestrate
from src.db.repository import AIRepository
from core_ai.types import LLMProvider, Message
from core_ai.logging import get_logger
from src.tools import appointments, payments

logger = get_logger(__name__)


async def run_commercial_chat(
    *,
    repo: AIRepository,
    llm: LLMProvider,
    backend_client: BackendClient,
    org_id: str,
    message: str,
    agent_mode: str,
    channel: str,
    conversation_id: str | None = None,
    auth: AuthContext | None = None,
    external_contact: str = "",
    confirmed_actions: list[str] | None = None,
    user_metadata: dict[str, Any] | None = None,
    assistant_metadata: dict[str, Any] | None = None,
) -> CommercialChatResult:
    sanitized_message = sanitize_message(message)
    confirmed = {item.strip().lower() for item in (confirmed_actions or []) if item.strip()}
    state = CommercialRunState()

    if agent_mode == "external_sales":
        policy = build_external_sales_policy(channel=channel)  # type: ignore[arg-type]
        conversation = await get_external_conversation(
            repo=repo,
            org_id=org_id,
            external_contact=external_contact,
            message=sanitized_message,
            conversation_id=conversation_id,
            reuse_latest=bool(external_contact),
        )
        actor_id = external_contact or "external"
        actor_type = "external_contact"
    else:
        if auth is None:
            raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="unauthorized")
        dossier = await repo.get_or_create_dossier(org_id)
        modules_active = dossier.get("modules_active", []) if isinstance(dossier, dict) else []
        if agent_mode == "internal_sales":
            policy = build_internal_sales_policy(auth, modules_active, channel=channel)  # type: ignore[arg-type]
        else:
            policy = build_internal_procurement_policy(auth, modules_active, channel=channel)  # type: ignore[arg-type]
        if not policy.allowed_tools:
            raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="commercial role has no enabled tools")
        conversation = await _load_internal_conversation(repo, auth, conversation_id, sanitized_message)
        actor_id = auth.actor
        actor_type = "internal_user"

    dossier = await repo.get_or_create_dossier(org_id)
    dossier_snapshot = copy.deepcopy(dossier)

    if agent_mode == "external_sales":
        declarations, handlers = await _build_external_sales_tools(
            client=backend_client,
            repo=repo,
            org_id=org_id,
            conversation_id=conversation.id,
            policy=policy,
            state=state,
            confirmed_actions=confirmed,
            external_contact=external_contact,
        )
    elif agent_mode == "internal_sales":
        declarations, handlers = await _build_internal_sales_tools(
            client=backend_client,
            auth=auth,
            repo=repo,
            org_id=org_id,
            conversation_id=conversation.id,
            policy=policy,
            state=state,
            confirmed_actions=confirmed,
        )
    else:
        declarations, handlers = await _build_procurement_tools(
            client=backend_client,
            auth=auth,
            repo=repo,
            org_id=org_id,
            conversation_id=conversation.id,
            policy=policy,
            state=state,
            confirmed_actions=confirmed,
        )

    history = history_to_messages(list(conversation.messages))
    llm_messages: list[Message] = [
        Message(role="system", content=build_commercial_prompt(agent_mode, channel, auth, dossier)),
        Message(role="system", content=f"Dossier: {summarize_dossier_for_context(dossier)}"),
        *history,
        Message(role="user", content=sanitized_message),
    ]

    assistant_parts: list[str] = []
    tool_calls: list[str] = []
    tokens_in = estimate_tokens("\n".join(m.content for m in llm_messages))
    limits = OrchestratorLimits(
        max_tool_calls=policy.max_tool_calls,
        tool_timeout_seconds=policy.tool_timeout_seconds,
        total_timeout_seconds=policy.total_timeout_seconds,
    )

    try:
        async for chunk in orchestrate(
            llm=llm,
            messages=llm_messages,
            tools=declarations,
            tool_handlers=handlers,
            org_id=org_id,
            limits=limits,
        ):
            if chunk.type == "text" and chunk.text:
                assistant_parts.append(chunk.text)
            elif chunk.type == "tool_call" and chunk.tool_call:
                tool_name = str(chunk.tool_call.get("name", "")).strip()
                if tool_name:
                    tool_calls.append(tool_name)
                    if tool_name not in handlers:
                        state.add_guardrail(f"La accion {tool_name} no esta habilitada para este agente.")
                        await record_agent_event(
                            repo,
                            org_id=org_id,
                            conversation_id=conversation.id,
                            agent_mode=agent_mode,
                            channel=channel,
                            actor_id=actor_id,
                            actor_type=actor_type,
                            action=f"tool.{tool_name}",
                            result="blocked",
                            confirmed=False,
                            tool_name=tool_name,
                            metadata={"reason": "tool_not_declared"},
                        )
    except Exception as exc:  # noqa: BLE001
        logger.exception("commercial_chat_failed", org_id=org_id, agent_mode=agent_mode, error=str(exc))
        raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail="ai unavailable") from exc

    assistant_text = "".join(assistant_parts).strip()
    if state.pending_confirmations:
        assistant_text = (
            "Necesito confirmacion explicita para continuar con: "
            + ", ".join(state.pending_confirmations)
            + ". Reenviame la solicitud incluyendo la accion confirmada."
        )
    elif state.guardrail_messages:
        assistant_text = state.guardrail_messages[0]
    elif not assistant_text:
        assistant_text = "No pude generar una respuesta util en este momento."

    tokens_out = estimate_tokens(assistant_text)
    now = datetime.now(UTC).isoformat()
    user_message = {
        "role": "user",
        "content": sanitized_message,
        "ts": now,
        "agent_mode": agent_mode,
        "channel": channel,
        "confirmed_actions": sorted(confirmed),
    }
    if user_metadata:
        user_message.update(user_metadata)
    assistant_message = {
        "role": "assistant",
        "content": assistant_text,
        "ts": now,
        "tool_calls": sorted(set(tool_calls)),
        "agent_mode": agent_mode,
        "channel": channel,
        "pending_confirmations": list(state.pending_confirmations),
    }
    if assistant_metadata:
        assistant_message.update(assistant_metadata)
    await repo.append_messages(
        org_id=org_id,
        conversation_id=conversation.id,
        new_messages=[user_message, assistant_message],
        tool_calls_count=len(tool_calls),
        tokens_input=tokens_in,
        tokens_output=tokens_out,
    )
    await repo.track_usage(org_id, tokens_in=tokens_in, tokens_out=tokens_out)
    if dossier != dossier_snapshot:
        await repo.update_dossier(org_id, dossier)

    await record_agent_event(
        repo,
        org_id=org_id,
        conversation_id=conversation.id,
        agent_mode=agent_mode,
        channel=channel,
        actor_id=actor_id,
        actor_type=actor_type,
        action="chat.completed",
        result="success" if not state.guardrail_messages else "guardrail",
        confirmed=bool(confirmed),
        metadata={
            "tool_calls": sorted(set(tool_calls)),
            "pending_confirmations": list(state.pending_confirmations),
        },
    )

    return CommercialChatResult(
        conversation_id=conversation.id,
        reply=assistant_text,
        tokens_input=tokens_in,
        tokens_output=tokens_out,
        tool_calls=sorted(set(tool_calls)),
        pending_confirmations=list(state.pending_confirmations),
    )


async def process_contract(
    *,
    repo: AIRepository,
    backend_client: BackendClient,
    org_id: str,
    envelope: CommercialContractEnvelope,
    actor_id: str,
) -> dict[str, Any]:
    contract = envelope.contract
    if contract.org_id.strip() != org_id:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="org mismatch")
    if contract.channel not in {"api", "embedded", "web_public", "whatsapp"}:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="invalid channel")
    if await has_processed_request(repo, org_id, contract.request_id):
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail="request_id already processed")

    response_payload: dict[str, Any]
    status_label = "processed"

    if contract.intent == "availability_request":
        date_value = str(contract.metadata.get("date", "")).strip()
        duration = int(contract.metadata.get("duration", 60) or 60)
        response_payload = {
            "intent": "availability_response",
            "request_id": contract.request_id,
            "availability": await appointments.check_availability(backend_client, org_id=org_id, date=date_value, duration=duration),
        }
    elif contract.intent == "request_quote":
        preview = await _build_quote_preview(
            backend_client,
            org_id,
            items=[item.model_dump() for item in contract.items],
            customer_name=envelope.contact_name or str(contract.metadata.get("customer_name", "")),
            notes=str(contract.metadata.get("notes", "")),
        )
        response_payload = {
            "intent": "quote_response",
            "request_id": contract.request_id,
            "quote_preview": preview,
        }
    elif contract.intent == "payment_request":
        quote_id = str(contract.metadata.get("quote_id", "")).strip()
        if not quote_id:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="quote_id is required for payment_request")
        response_payload = {
            "intent": "payment_request",
            "request_id": contract.request_id,
            "payment": await payments.get_public_quote_payment_link(backend_client, org_id=org_id, quote_id=quote_id),
        }
    elif contract.intent == "reservation_request":
        if "book_appointment" not in envelope.confirmed_actions:
            status_label = "confirmation_required"
            response_payload = {
                "intent": "reservation_request",
                "request_id": contract.request_id,
                "status": "confirmation_required",
                "required_action": "book_appointment",
                "message": "La reserva requiere confirmacion explicita antes de escribir en el backend.",
            }
        else:
            response_payload = {
                "intent": "reservation_request",
                "request_id": contract.request_id,
                "reservation": await appointments.book_appointment(
                    backend_client,
                    org_id=org_id,
                    customer_name=envelope.contact_name or str(contract.metadata.get("customer_name", "")),
                    customer_phone=envelope.contact_phone or str(contract.metadata.get("customer_phone", "")),
                    title=str(contract.metadata.get("title", "Reserva")),
                    start_at=str(contract.metadata.get("start_at", "")),
                    duration=int(contract.metadata.get("duration", 60) or 60),
                ),
            }
    else:
        status_label = "accepted_for_review"
        response_payload = {
            "intent": contract.intent,
            "request_id": contract.request_id,
            "status": status_label,
            "message": "La propuesta estructurada fue recibida y queda marcada para revision controlada.",
        }

    await record_agent_event(
        repo,
        org_id=org_id,
        conversation_id=None,
        agent_mode="external_sales",
        channel=contract.channel,
        actor_id=actor_id,
        actor_type="external_agent",
        action=f"contract.{contract.intent}",
        result=status_label,
        confirmed="book_appointment" in envelope.confirmed_actions,
        request_id=contract.request_id,
        metadata={
            "counterparty_id": contract.counterparty_id,
            "intent": contract.intent,
            "signature_present": bool(contract.signature),
        },
    )
    return response_payload
