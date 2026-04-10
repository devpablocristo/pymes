from __future__ import annotations

from uuid import uuid4

from fastapi import APIRouter, Depends, HTTPException, Path, Query, Request, status
from pydantic import BaseModel, Field

from src.agents.catalog import normalize_routed_agent, normalize_routing_source
from src.agents.service import run_internal_orchestrated_chat
from src.api.chat_contract import ChatBlock, ChatRequest, ChatResponse
from src.api.deps import get_auth_context, get_backend_client, get_llm_provider, get_repository
from src.api.quota import check_quota
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.core.internal_conversations import can_access_internal_conversation, get_internal_conversation_user_id
from src.db.repository import AIRepository
from runtime.logging import get_logger, get_request_id
from src.runtime_contracts import OUTPUT_KIND_CHAT_REPLY
from src.localization import resolve_preferred_language

router = APIRouter(prefix="/v1/chat", tags=["chat"])
logger = get_logger(__name__)


# ── DTOs para listado y detalle de conversaciones ──


class ConversationSummary(BaseModel):
    id: str
    title: str
    created_at: str
    updated_at: str
    message_count: int


class ConversationMessage(BaseModel):
    role: str
    content: str
    ts: str | None = None
    tool_calls: list[str] = Field(default_factory=list)
    blocks: list[ChatBlock] = Field(default_factory=list)


class ConversationDetail(BaseModel):
    id: str
    title: str
    messages: list[ConversationMessage]
    created_at: str
    updated_at: str


class ConversationListResponse(BaseModel):
    items: list[ConversationSummary]


# ── Endpoints de conversaciones ──


@router.get("/conversations", response_model=ConversationListResponse)
async def list_conversations(
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    limit: int = Query(default=50, ge=1, le=200),
):
    """Lista conversaciones internas del usuario autenticado."""
    rows = await repo.list_conversations(
        auth.org_id,
        mode="internal",
        user_id=get_internal_conversation_user_id(auth),
        limit=limit,
    )
    return ConversationListResponse(
        items=[
            ConversationSummary(
                id=r.id,
                title=r.title or "Sin título",
                created_at=r.created_at.isoformat() if r.created_at else "",
                updated_at=r.updated_at.isoformat() if r.updated_at else "",
                message_count=len(r.messages) if r.messages else 0,
            )
            for r in rows
        ]
    )


@router.get("/conversations/{conversation_id}", response_model=ConversationDetail)
async def get_conversation(
    conversation_id: str = Path(..., min_length=36, max_length=36),
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
):
    """Devuelve una conversación con su historial de mensajes."""
    row = await repo.get_conversation(auth.org_id, conversation_id)
    if row is None or row.mode != "internal" or not can_access_internal_conversation(auth, row.user_id):
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
    return ConversationDetail(
        id=row.id,
        title=row.title or "Sin título",
        messages=[
            ConversationMessage(
                role=m.get("role", ""),
                content=m.get("content", ""),
                ts=m.get("ts"),
                tool_calls=m.get("tool_calls", []),
                blocks=m.get("blocks", []),
            )
            for m in (row.messages or [])
        ],
        created_at=row.created_at.isoformat() if row.created_at else "",
        updated_at=row.updated_at.isoformat() if row.updated_at else "",
    )

@router.post("", response_model=ChatResponse)
async def chat_internal(
    req: ChatRequest,
    request: Request,
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    llm=Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    request_id = get_request_id() or str(uuid4())
    preferred_language = resolve_preferred_language(
        req.preferred_language,
        accept_language=request.headers.get("Accept-Language"),
    )
    await check_quota(repo, auth.org_id, mode="internal")
    logger.info(
        "chat_internal_started",
        request_id=request_id,
        org_id=auth.org_id,
        user_id=auth.actor,
        conversation_id=req.chat_id or "",
        endpoint_kind="chat_json",
        route_hint=req.route_hint or "",
        preferred_language=preferred_language,
    )
    try:
        result = await run_internal_orchestrated_chat(
            repo=repo,
            llm=llm,
            backend_client=backend_client,
            org_id=auth.org_id,
            message=req.message,
            conversation_id=req.chat_id,
            auth=auth,
            confirmed_actions=req.confirmed_actions,
            handoff=req.handoff,
            route_hint=req.route_hint,
            preferred_language=preferred_language,
        )
    except HTTPException:
        raise
    except Exception as exc:  # noqa: BLE001
        logger.exception("chat_internal_failed", org_id=auth.org_id, user_id=auth.actor, error=str(exc))
        raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail="ai unavailable") from exc

    logger.info(
        "chat_internal_completed",
        request_id=request_id,
        org_id=auth.org_id,
        user_id=auth.actor,
        conversation_id=result.conversation_id,
        routed_agent=normalize_routed_agent(result.routed_agent),
        routing_source=normalize_routing_source(result.routing_source),
        tool_calls=len(result.tool_calls),
        tokens_input=result.tokens_input,
        tokens_output=result.tokens_output,
    )
    return ChatResponse(
        request_id=request_id,
        output_kind=OUTPUT_KIND_CHAT_REPLY,
        content_language=result.content_language,
        chat_id=result.conversation_id,
        reply=result.reply,
        tokens_used=result.tokens_used,
        tool_calls=result.tool_calls,
        pending_confirmations=result.pending_confirmations,
        blocks=result.blocks,
        routed_agent=normalize_routed_agent(result.routed_agent),
        routing_source=normalize_routing_source(result.routing_source),
    )
