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
    sanitize_message,
)
from src.agents.sub_agents import build_registry
from src.api.external_chat_support import get_external_conversation, history_to_messages
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.config import get_settings
from src.core.dossier import summarize_dossier_for_context
from runtime.orchestrator import OrchestratorLimits, orchestrate
from runtime.services.multi_agent_orchestrator import run_routed_agent
from src.db.repository import AIRepository
from runtime.types import LLMProvider, Message
from runtime.logging import get_logger
from runtime.text import estimate_tokens
from src.tools import appointments, payments

logger = get_logger(__name__)

_INTERNAL_ASSISTANT_CHANNEL = "internal_assistant"
_INTERNAL_SENSITIVE_TOOLS = {
    "create_quote",
    "create_sale",
    "create_procurement_request",
    "submit_procurement_request",
    "generate_payment_link",
    "send_payment_info",
}

_INTERNAL_GENERAL_SYSTEM_PROMPT = """\
Sos el asistente general de una plataforma de gestión para PyMEs.
Respondé saludos y preguntas generales de forma amable, clara y concisa.
Si el usuario pide una acción concreta del negocio, indicá que puede pedírtela directamente y el sistema la va a enrutar.
Respondé siempre en español."""


def _default_internal_reply(routed_agent: str) -> str:
    if routed_agent == "general":
        return "Hola. Puedo ayudarte con clientes, productos, ventas, cobros y compras. Decime qué necesitás."
    return "No pude generar una respuesta útil en este momento."


def _build_internal_pending_confirmation(tool_name: str) -> dict[str, Any]:
    return {
        "pending_confirmation": True,
        "required_action": tool_name,
        "message": f"Necesito confirmación explícita para ejecutar {tool_name}. Reenviá la solicitud incluyendo esa acción en confirmed_actions.",
    }


def _build_internal_general_limits() -> OrchestratorLimits:
    settings = get_settings()
    return OrchestratorLimits(
        max_tool_calls=0,
        total_timeout_seconds=max(30.0, float(settings.assistant_total_timeout_seconds)),
    )


def _looks_like_customer_summary_request(message: str) -> bool:
    text = message.strip().lower()
    if "cliente" not in text:
        return False
    hints = (
        "cuantos",
        "cuántos",
        "cuantas",
        "cuántas",
        "tengo",
        "listar",
        "lista",
        "mostra",
        "mostrar",
        "resumi",
        "resumí",
        "resumen",
    )
    return any(hint in text for hint in hints)


def _summarize_customer_search(result: dict[str, Any]) -> str | None:
    items = result.get("items", [])
    if not isinstance(items, list):
        return None
    total = result.get("total")
    if not isinstance(total, int):
        total = len(items)
    if total <= 0:
        return "Hoy no veo clientes cargados para esta organización."
    names = [str(item.get("name", "")).strip() for item in items[:5] if isinstance(item, dict) and str(item.get("name", "")).strip()]
    if not names:
        return f"Tenés {total} clientes registrados."
    suffix = "" if total <= len(names) else ", ..."
    return f"Tenés {total} clientes registrados. Algunos son: {', '.join(names)}{suffix}."


async def _run_internal_read_fallback(
    *,
    registry: Any,
    routed_agent: str,
    org_id: str,
    user_message: str,
) -> tuple[str | None, list[str]]:
    agent = registry.get(routed_agent)
    if agent is None:
        return None, []

    if routed_agent == "clientes" and _looks_like_customer_summary_request(user_message):
        handler = agent.tool_handlers.get("search_customers")
        if handler is None:
            return None, []
        result = await handler(org_id=org_id, query="", limit=100)
        if isinstance(result, dict):
            return _summarize_customer_search(result), ["search_customers"]

    return None, []


def _wrap_internal_registry_handlers(
    *,
    registry: Any,
    repo: AIRepository,
    org_id: str,
    conversation_id: str,
    auth: AuthContext,
    confirmed_actions: set[str],
) -> None:
    for agent_name in registry.names():
        agent = registry.get(agent_name)
        if agent is None:
            continue

        wrapped_handlers = {}
        for tool_name, handler in agent.tool_handlers.items():
            wrapped_handlers[tool_name] = _wrap_internal_tool_handler(
                tool_name=tool_name,
                handler=handler,
                repo=repo,
                org_id=org_id,
                conversation_id=conversation_id,
                actor_id=auth.actor,
                confirmed_actions=confirmed_actions,
                agent_name=agent_name,
            )
        agent.tool_handlers = wrapped_handlers


def _wrap_internal_tool_handler(
    *,
    tool_name: str,
    handler: Any,
    repo: AIRepository,
    org_id: str,
    conversation_id: str,
    actor_id: str,
    confirmed_actions: set[str],
    agent_name: str,
):
    async def wrapped_handler(**kwargs: Any) -> dict[str, Any]:
        is_confirmed = tool_name.lower() in confirmed_actions
        if tool_name in _INTERNAL_SENSITIVE_TOOLS and not is_confirmed:
            result = _build_internal_pending_confirmation(tool_name)
            await record_agent_event(
                repo,
                org_id=org_id,
                conversation_id=conversation_id,
                agent_mode=agent_name,
                channel=_INTERNAL_ASSISTANT_CHANNEL,
                actor_id=actor_id,
                actor_type="internal_user",
                action=f"tool.{tool_name}",
                result="confirmation_required",
                confirmed=False,
                tool_name=tool_name,
                metadata={"required_action": tool_name},
            )
            return result

        result = await handler(**kwargs)
        outcome = "success"
        if isinstance(result, dict) and result.get("error"):
            outcome = "error"
        await record_agent_event(
            repo,
            org_id=org_id,
            conversation_id=conversation_id,
            agent_mode=agent_name,
            channel=_INTERNAL_ASSISTANT_CHANNEL,
            actor_id=actor_id,
            actor_type="internal_user",
            action=f"tool.{tool_name}",
            result=outcome,
            confirmed=is_confirmed,
            tool_name=tool_name,
        )
        return result

    return wrapped_handler


async def run_internal_orchestrated_chat(
    *,
    repo: AIRepository,
    llm: LLMProvider,
    backend_client: BackendClient,
    org_id: str,
    message: str,
    conversation_id: str | None,
    auth: AuthContext,
    confirmed_actions: list[str] | None = None,
) -> CommercialChatResult:
    """Punto de entrada canónico del assistant interno de Pymes."""
    sanitized_message = sanitize_message(message)
    conversation = await _load_internal_conversation(repo, auth, conversation_id, sanitized_message)
    confirmed = {item.strip().lower() for item in (confirmed_actions or []) if item.strip()}

    registry = build_registry(backend_client, auth)
    _wrap_internal_registry_handlers(
        registry=registry,
        repo=repo,
        org_id=org_id,
        conversation_id=conversation.id,
        auth=auth,
        confirmed_actions=confirmed,
    )
    history = history_to_messages(list(conversation.messages))

    assistant_parts: list[str] = []
    tool_calls: list[str] = []
    pending_confirmations: list[str] = []
    routed_agent = "general"
    tokens_in = estimate_tokens(sanitized_message)

    try:
        async for chunk in run_routed_agent(
            llm=llm,
            registry=registry,
            user_message=sanitized_message,
            history=history,
            context={"org_id": org_id},
            general_system_prompt=_INTERNAL_GENERAL_SYSTEM_PROMPT,
            general_limits=_build_internal_general_limits(),
        ):
            if chunk.type == "route" and chunk.text:
                routed_agent = chunk.text
                logger.info(
                    "internal_assistant_routed",
                    org_id=org_id,
                    conversation_id=conversation.id,
                    routed_agent=routed_agent,
                )
            elif chunk.type == "text" and chunk.text:
                assistant_parts.append(chunk.text)
            elif chunk.type == "tool_call" and chunk.tool_call:
                tool_name = str(chunk.tool_call.name).strip()
                if tool_name:
                    tool_calls.append(tool_name)
            elif chunk.type == "tool_result" and chunk.tool_call:
                result = chunk.tool_call.arguments
                if isinstance(result, dict) and result.get("pending_confirmation"):
                    required_action = str(result.get("required_action", "")).strip()
                    if required_action and required_action not in pending_confirmations:
                        pending_confirmations.append(required_action)
    except Exception as exc:  # noqa: BLE001
        logger.exception("internal_assistant_failed", org_id=org_id, conversation_id=conversation.id, error=str(exc))
        raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail="ai unavailable") from exc

    reply = "".join(assistant_parts).strip() or _default_internal_reply(routed_agent)
    if not pending_confirmations and not tool_calls:
        fallback_reply, fallback_tool_calls = await _run_internal_read_fallback(
            registry=registry,
            routed_agent=routed_agent,
            org_id=org_id,
            user_message=sanitized_message,
        )
        if fallback_reply:
            reply = fallback_reply
            tool_calls.extend(fallback_tool_calls)
    if pending_confirmations:
        reply = (
            "Necesito confirmación explícita para continuar con: "
            + ", ".join(pending_confirmations)
            + ". Reenviame la solicitud incluyendo esas acciones en confirmed_actions."
        )
    tokens_out = estimate_tokens(reply)
    now = datetime.now(UTC).isoformat()
    user_message = {"role": "user", "content": sanitized_message, "ts": now}
    if confirmed:
        user_message["confirmed_actions"] = sorted(confirmed)
    assistant_message = {
        "role": "assistant",
        "content": reply,
        "ts": now,
        "tool_calls": sorted(set(tool_calls)),
        "routed_agent": routed_agent,
        "routed_mode": routed_agent,
        "agent_mode": routed_agent,
        "channel": _INTERNAL_ASSISTANT_CHANNEL,
        "pending_confirmations": list(pending_confirmations),
    }

    await repo.append_messages(
        org_id=org_id,
        conversation_id=conversation.id,
        new_messages=[
            user_message,
            assistant_message,
        ],
        tool_calls_count=len(tool_calls),
        tokens_input=tokens_in,
        tokens_output=tokens_out,
    )
    await repo.track_usage(org_id, tokens_in=tokens_in, tokens_out=tokens_out)
    await record_agent_event(
        repo,
        org_id=org_id,
        conversation_id=conversation.id,
        agent_mode=routed_agent,
        channel=_INTERNAL_ASSISTANT_CHANNEL,
        actor_id=auth.actor,
        actor_type="internal_user",
        action="chat.completed",
        result="confirmation_required" if pending_confirmations else "success",
        confirmed=bool(confirmed),
        metadata={
            "tool_calls": sorted(set(tool_calls)),
            "pending_confirmations": list(pending_confirmations),
        },
    )

    return CommercialChatResult(
        conversation_id=conversation.id,
        reply=reply,
        tokens_input=tokens_in,
        tokens_output=tokens_out,
        tool_calls=sorted(set(tool_calls)),
        pending_confirmations=list(pending_confirmations),
        routed_agent=routed_agent,
    )


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
            context={"org_id": org_id},
            limits=limits,
        ):
            if chunk.type == "text" and chunk.text:
                assistant_parts.append(chunk.text)
            elif chunk.type == "tool_call" and chunk.tool_call:
                tool_name = str(chunk.tool_call.name).strip()
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
