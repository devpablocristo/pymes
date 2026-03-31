from __future__ import annotations

from datetime import UTC, datetime
from uuid import uuid4

from fastapi import APIRouter, Depends, HTTPException, Request, status

from src.agents.catalog import normalize_routed_agent, normalize_routing_source
from src.agents.service import run_internal_orchestrated_chat
from src.api.chat_contract import ChatRequest, ChatResponse
from src.api.deps import get_auth_context, get_backend_client, get_llm_provider, get_repository
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.config import get_settings
from src.db.repository import AIRepository
from runtime.logging import get_logger, get_request_id
from src.runtime_contracts import OUTPUT_KIND_CHAT_REPLY
from src.localization import resolve_preferred_language

router = APIRouter(prefix="/v1/chat", tags=["chat"])
logger = get_logger(__name__)

PLAN_LIMITS: dict[str, dict[str, int | bool]] = {
    "starter": {"queries": 50, "external": False, "external_limit": 0},
    "growth": {"queries": 500, "external": True, "external_limit": 200},
    "enterprise": {"queries": -1, "external": True, "external_limit": -1},
}


async def check_quota(repo: AIRepository, org_id: str, mode: str) -> str:
    settings = get_settings()
    now = datetime.now(UTC)
    plan = await repo.get_plan_code(org_id)
    if not settings.ai_enforce_plan_limits:
        return plan
    limits = PLAN_LIMITS.get(plan, PLAN_LIMITS["starter"])
    usage = await repo.get_month_usage(org_id, now.year, now.month)

    if mode == "external" and not bool(limits["external"]):
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="AI externo no disponible para este plan")

    query_limit = int(limits["queries"])
    if query_limit != -1 and usage["queries"] >= query_limit:
        raise HTTPException(
            status_code=status.HTTP_429_TOO_MANY_REQUESTS,
            detail=f"Limite mensual alcanzado ({query_limit} consultas)",
        )

    return plan

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
        routed_mode=normalize_routed_agent(result.routed_mode),
        routing_source=normalize_routing_source(result.routing_source),
    )
