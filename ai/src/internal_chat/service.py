from __future__ import annotations

import copy
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import Any

from runtime.logging import get_logger
from runtime.text import estimate_tokens
from runtime.types import LLMProvider, Message

from src.agents.audit import record_agent_event
from src.agents.catalog import ROUTING_SOURCE_ORCHESTRATOR
from src.agents.service_support import hydrate_dossier_from_backend_settings, _load_internal_conversation, sanitize_message
from src.api.chat_contract import ChatHandoff
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from src.internal_chat.evidence import EvidenceUnavailable, build_evidence_packet
from src.internal_chat.facts import AnswerMode, build_fact_pack, classify_answer_mode
from src.internal_chat.prompts import INTERNAL_CHAT_SYSTEM_PROMPT, build_internal_chat_user_prompt
from src.internal_chat.routing import AnalysisScope, route_internal_message
from src.runtime_contracts import DEFAULT_LANGUAGE_CODE

logger = get_logger(__name__)

_INTERNAL_ASSISTANT_CHANNEL = "internal_assistant"


@dataclass(frozen=True)
class InternalChatError(Exception):
    status_code: int
    code: str
    message: str
    details: dict[str, Any] = field(default_factory=dict)


@dataclass
class InternalChatResult:
    conversation_id: str
    reply: str
    tokens_input: int
    tokens_output: int
    tool_calls: list[str]
    pending_confirmations: list[str]
    blocks: list[dict[str, Any]] = field(default_factory=list)
    routed_agent: str | None = None
    routing_source: str | None = None
    content_language: str = DEFAULT_LANGUAGE_CODE
    analysis_scope: AnalysisScope = "general"
    answer_mode: AnswerMode = "analysis"
    deterministic: dict[str, Any] = field(default_factory=dict)
    dashboard_links: list[dict[str, Any]] = field(default_factory=list)
    llm: dict[str, Any] = field(default_factory=dict)
    evidence: dict[str, Any] = field(default_factory=dict)

    @property
    def tokens_used(self) -> int:
        return self.tokens_input + self.tokens_output


async def run_internal_orchestrated_chat(
    *,
    repo: AIRepository,
    llm: LLMProvider,
    backend_client: BackendClient,
    tenant_id: str,
    message: str,
    conversation_id: str | None,
    auth: AuthContext,
    confirmed_actions: list[str] | None = None,
    handoff: ChatHandoff | None = None,
    route_hint: str | None = None,
    preferred_language: str | None = None,
) -> InternalChatResult:
    """Read-only internal assistant: evidence first, Gemini second, no synthetic replies."""

    _ = confirmed_actions
    sanitized_message = sanitize_message(message)
    conversation = await _load_internal_conversation(repo, auth, conversation_id, sanitized_message)
    decision = route_internal_message(sanitized_message, route_hint=route_hint, handoff=handoff)

    try:
        evidence = await build_evidence_packet(
            backend_client=backend_client,
            auth=auth,
            decision=decision,
            message=sanitized_message,
        )
    except EvidenceUnavailable as exc:
        logger.warning(
            "internal_chat_evidence_unavailable",
            tenant_id=tenant_id,
            conversation_id=conversation.id,
            tool_name=exc.tool_name,
            error=str(exc),
        )
        raise InternalChatError(
            status_code=502,
            code="business_evidence_unavailable",
            message="No pude leer datos reales del negocio para responder. Revisá el backend o permisos del módulo.",
            details={"tool": exc.tool_name, "error": str(exc)},
        ) from exc

    answer_mode = classify_answer_mode(sanitized_message, decision)
    fact_pack = build_fact_pack(evidence=evidence, decision=decision)
    deterministic_metadata = fact_pack.metadata()
    dashboard_links = [link.as_dict() for link in fact_pack.dashboard_links]

    if answer_mode == "facts_only":
        reply = fact_pack.summary or "No encontre datos suficientes del negocio para responder esa consulta."
        tokens_in = estimate_tokens(sanitized_message) + estimate_tokens(fact_pack.summary)
        tokens_out = estimate_tokens(reply)
        blocks = [{"type": "text", "text": reply}, *copy.deepcopy(fact_pack.blocks)]
        llm_metadata = {
            "used": False,
            "provider": None,
            "model": None,
            "status": "unavailable",
        }
    else:
        dossier, _snapshot = await hydrate_dossier_from_backend_settings(
            repo=repo,
            backend_client=backend_client,
            tenant_id=tenant_id,
            auth=auth,
        )
        user_prompt = build_internal_chat_user_prompt(
            message=sanitized_message,
            decision=decision,
            evidence=evidence,
            business_context=dossier,
            deterministic_summary=fact_pack.summary,
        )
        tokens_in = estimate_tokens(INTERNAL_CHAT_SYSTEM_PROMPT) + estimate_tokens(user_prompt)
        reply = await _complete_with_gemini(
            llm=llm,
            tenant_id=tenant_id,
            conversation_id=conversation.id,
            user_prompt=user_prompt,
        )
        tokens_out = estimate_tokens(reply)
        blocks = [*copy.deepcopy(fact_pack.blocks), {"type": "text", "text": reply}]
        llm_metadata = {
            "used": True,
            "provider": _infer_provider(llm),
            "model": _infer_model(llm),
            "status": "ok",
        }

    evidence_metadata = evidence.metadata()
    now = datetime.now(UTC).isoformat()
    user_message: dict[str, Any] = {
        "role": "user",
        "content": sanitized_message,
        "ts": now,
        "route_hint": route_hint,
        "channel": _INTERNAL_ASSISTANT_CHANNEL,
    }
    assistant_message: dict[str, Any] = {
        "role": "assistant",
        "content": reply,
        "ts": now,
        "tool_calls": sorted(set(evidence.tools)),
        "routed_agent": decision.routed_agent,
        "agent_mode": decision.routed_agent,
        "channel": _INTERNAL_ASSISTANT_CHANNEL,
        "routing_source": ROUTING_SOURCE_ORCHESTRATOR,
        "pending_confirmations": [],
        "blocks": copy.deepcopy(blocks),
        "analysis_scope": decision.scope,
        "answer_mode": answer_mode,
        "deterministic": copy.deepcopy(deterministic_metadata),
        "dashboard_links": copy.deepcopy(dashboard_links),
        "llm": copy.deepcopy(llm_metadata),
        "evidence": copy.deepcopy(evidence_metadata),
    }
    await repo.append_messages(
        tenant_id=tenant_id,
        conversation_id=conversation.id,
        new_messages=[user_message, assistant_message],
        tool_calls_count=len(evidence.tools),
        tokens_input=tokens_in,
        tokens_output=tokens_out,
    )
    await repo.track_usage(tenant_id, tokens_in=tokens_in, tokens_out=tokens_out)
    await record_agent_event(
        repo,
        tenant_id=tenant_id,
        conversation_id=conversation.id,
        agent_mode=decision.routed_agent,
        channel=_INTERNAL_ASSISTANT_CHANNEL,
        actor_id=auth.actor,
        actor_type="internal_user",
        action="chat.completed",
        result="success",
        confirmed=False,
        metadata={
            "routing_reason": decision.reason,
            "analysis_scope": decision.scope,
            "answer_mode": answer_mode,
            "tool_calls": sorted(set(evidence.tools)),
            "deterministic": copy.deepcopy(deterministic_metadata),
            "dashboard_links": copy.deepcopy(dashboard_links),
            "llm": copy.deepcopy(llm_metadata),
            "evidence": copy.deepcopy(evidence_metadata),
        },
    )
    logger.info(
        "internal_chat_completed",
        tenant_id=tenant_id,
        conversation_id=conversation.id,
        routed_agent=decision.routed_agent,
        analysis_scope=decision.scope,
        answer_mode=answer_mode,
        tool_calls=len(evidence.tools),
        llm_used=llm_metadata["used"],
        model=llm_metadata["model"],
    )
    return InternalChatResult(
        conversation_id=conversation.id,
        reply=reply,
        tokens_input=tokens_in,
        tokens_output=tokens_out,
        tool_calls=sorted(set(evidence.tools)),
        pending_confirmations=[],
        blocks=blocks,
        routed_agent=decision.routed_agent,
        routing_source=ROUTING_SOURCE_ORCHESTRATOR,
        content_language=preferred_language or DEFAULT_LANGUAGE_CODE,
        analysis_scope=decision.scope,
        answer_mode=answer_mode,
        deterministic=deterministic_metadata,
        dashboard_links=dashboard_links,
        llm=llm_metadata,
        evidence=evidence_metadata,
    )


async def _complete_with_gemini(
    *,
    llm: LLMProvider,
    tenant_id: str,
    conversation_id: str,
    user_prompt: str,
) -> str:
    if llm is None or not callable(getattr(llm, "chat", None)):
        raise InternalChatError(
            status_code=503,
            code="gemini_unavailable",
            message="Gemini no está disponible para el asistente interno.",
            details={"reason": "llm_provider_missing_chat"},
        )
    chunks: list[str] = []
    try:
        async for chunk in llm.chat(
            [
                Message(role="system", content=INTERNAL_CHAT_SYSTEM_PROMPT),
                Message(role="user", content=user_prompt),
            ],
            tools=[],
            temperature=0.2,
            max_tokens=900,
        ):
            if chunk.type == "text" and chunk.text:
                chunks.append(chunk.text)
    except InternalChatError:
        raise
    except Exception as exc:  # noqa: BLE001
        logger.warning(
            "internal_chat_llm_unavailable",
            tenant_id=tenant_id,
            conversation_id=conversation_id,
            error=str(exc),
        )
        raise InternalChatError(
            status_code=503,
            code="gemini_unavailable",
            message="Gemini no está disponible para el asistente interno.",
            details={"error": str(exc)},
        ) from exc

    reply = "".join(chunks).strip()
    if not reply:
        raise InternalChatError(
            status_code=503,
            code="gemini_unavailable",
            message="Gemini no devolvió contenido para esta consulta.",
            details={"reason": "empty_llm_response"},
        )
    return reply


def _infer_provider(llm: LLMProvider) -> str:
    name = type(llm).__name__.lower()
    if "gemini" in name or hasattr(llm, "client"):
        return "gemini"
    return "gemini"


def _infer_model(llm: LLMProvider) -> str:
    model = getattr(llm, "model", None)
    if isinstance(model, str) and model.strip():
        return model.strip()
    return "unknown"
