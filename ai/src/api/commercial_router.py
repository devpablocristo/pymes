from __future__ import annotations

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field

from src.agents.service import run_commercial_chat
from src.api.deps import get_auth_context, get_backend_client, get_llm_provider, get_repository
from src.api.router import check_quota
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from core_ai.types import LLMProvider
from core_ai.logging import get_logger

router = APIRouter(prefix="/v1/chat/commercial", tags=["commercial-chat"])
logger = get_logger(__name__)


class CommercialChatRequest(BaseModel):
    conversation_id: str | None = None
    message: str = Field(min_length=1, max_length=4000)
    confirmed_actions: list[str] = Field(default_factory=list)


class CommercialChatResponse(BaseModel):
    conversation_id: str
    reply: str
    tokens_used: int
    tool_calls: list[str]
    pending_confirmations: list[str]


@router.post("/sales", response_model=CommercialChatResponse)
async def chat_internal_sales(
    req: CommercialChatRequest,
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    llm: LLMProvider = Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    await check_quota(repo, auth.org_id, mode="internal")
    result = await run_commercial_chat(
        repo=repo,
        llm=llm,
        backend_client=backend_client,
        org_id=auth.org_id,
        message=req.message,
        agent_mode="internal_sales",
        channel="internal_ui",
        conversation_id=req.conversation_id,
        auth=auth,
        confirmed_actions=req.confirmed_actions,
    )
    logger.info(
        "commercial_sales_completed",
        org_id=auth.org_id,
        actor=auth.actor,
        conversation_id=result.conversation_id,
        tool_calls=len(result.tool_calls),
    )
    return CommercialChatResponse(
        conversation_id=result.conversation_id,
        reply=result.reply,
        tokens_used=result.tokens_used,
        tool_calls=result.tool_calls,
        pending_confirmations=result.pending_confirmations,
    )


@router.post("/procurement", response_model=CommercialChatResponse)
async def chat_internal_procurement(
    req: CommercialChatRequest,
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    llm: LLMProvider = Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    await check_quota(repo, auth.org_id, mode="internal")
    result = await run_commercial_chat(
        repo=repo,
        llm=llm,
        backend_client=backend_client,
        org_id=auth.org_id,
        message=req.message,
        agent_mode="internal_procurement",
        channel="internal_ui",
        conversation_id=req.conversation_id,
        auth=auth,
        confirmed_actions=req.confirmed_actions,
    )
    logger.info(
        "commercial_procurement_completed",
        org_id=auth.org_id,
        actor=auth.actor,
        conversation_id=result.conversation_id,
        tool_calls=len(result.tool_calls),
    )
    return CommercialChatResponse(
        conversation_id=result.conversation_id,
        reply=result.reply,
        tokens_used=result.tokens_used,
        tool_calls=result.tool_calls,
        pending_confirmations=result.pending_confirmations,
    )
