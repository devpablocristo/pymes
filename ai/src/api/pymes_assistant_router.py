"""Asistente Pymes — un solo chat; orquestador enruta a sub-agentes internos."""

from __future__ import annotations

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field

from src.agents.service import run_internal_orchestrated_chat
from src.api.deps import get_auth_context, get_backend_client, get_llm_provider, get_repository
from src.api.router import check_quota
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from runtime.types import LLMProvider
from runtime.logging import get_logger

router = APIRouter(prefix="/v1/chat/pymes", tags=["pymes-assistant"])
logger = get_logger(__name__)


class PymesAssistantChatRequest(BaseModel):
    conversation_id: str | None = None
    message: str = Field(min_length=1, max_length=4000)
    confirmed_actions: list[str] = Field(default_factory=list)


class PymesAssistantChatResponse(BaseModel):
    conversation_id: str
    reply: str
    tokens_used: int
    tool_calls: list[str]
    pending_confirmations: list[str]
    routed_mode: str = Field(
        ...,
        description="Sub-agente usado en este turno: internal_sales | internal_procurement",
    )


@router.post("/", response_model=PymesAssistantChatResponse)
async def chat_pymes_assistant(
    req: PymesAssistantChatRequest,
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    llm: LLMProvider = Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    await check_quota(repo, auth.org_id, mode="internal")
    result = await run_internal_orchestrated_chat(
        repo=repo,
        llm=llm,
        backend_client=backend_client,
        org_id=auth.org_id,
        message=req.message,
        conversation_id=req.conversation_id,
        auth=auth,
        confirmed_actions=req.confirmed_actions,
    )
    logger.info(
        "pymes_assistant_completed",
        org_id=auth.org_id,
        actor=auth.actor,
        conversation_id=result.conversation_id,
        routed_mode=result.routed_mode,
        tool_calls=len(result.tool_calls),
    )
    return PymesAssistantChatResponse(
        conversation_id=result.conversation_id,
        reply=result.reply,
        tokens_used=result.tokens_used,
        tool_calls=result.tool_calls,
        pending_confirmations=result.pending_confirmations,
        routed_mode=result.routed_mode,
    )
