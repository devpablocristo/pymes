from __future__ import annotations

from fastapi import APIRouter, Depends, Path
from pydantic import BaseModel, Field

from src.agents.contracts import CommercialContractEnvelope
from src.agents.service import process_contract, run_commercial_chat
from src.api.deps import get_backend_client, get_llm_provider, get_repository
from src.api.external_chat_support import clean_phone, resolve_org_id
from src.api.router import check_quota
from src.backend_client.client import BackendClient
from src.db.repository import AIRepository
from runtime.types import LLMProvider
from runtime.logging import get_logger, update_request_context

router = APIRouter(prefix="/v1/public", tags=["commercial-public"])
logger = get_logger(__name__)


class ExternalSalesChatRequest(BaseModel):
    conversation_id: str | None = None
    message: str = Field(min_length=1, max_length=4000)
    phone: str | None = Field(default=None, max_length=32)
    confirmed_actions: list[str] = Field(default_factory=list)
    channel: str = Field(default="web_public", pattern="^(web_public|whatsapp|api|embedded)$")


class ExternalSalesChatResponse(BaseModel):
    conversation_id: str
    reply: str
    tokens_used: int
    tool_calls: list[str]
    pending_confirmations: list[str]


@router.post("/{org_slug}/sales-agent/chat", response_model=ExternalSalesChatResponse)
async def external_sales_chat(
    req: ExternalSalesChatRequest,
    org_slug: str = Path(..., min_length=2),
    repo: AIRepository = Depends(get_repository),
    llm: LLMProvider = Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    org_id = await resolve_org_id(backend_client, org_slug)
    await check_quota(repo, org_id, mode="external")
    external_contact = clean_phone(req.phone or "")
    update_request_context(org_id=org_id, user_id=external_contact or "external")

    result = await run_commercial_chat(
        repo=repo,
        llm=llm,
        backend_client=backend_client,
        org_id=org_id,
        message=req.message,
        agent_mode="external_sales",
        channel=req.channel,
        conversation_id=req.conversation_id,
        external_contact=external_contact,
        confirmed_actions=req.confirmed_actions,
    )
    logger.info(
        "commercial_external_completed",
        org_id=org_id,
        external_contact=external_contact,
        conversation_id=result.conversation_id,
        tool_calls=len(result.tool_calls),
    )
    return ExternalSalesChatResponse(
        conversation_id=result.conversation_id,
        reply=result.reply,
        tokens_used=result.tokens_used,
        tool_calls=result.tool_calls,
        pending_confirmations=result.pending_confirmations,
    )


@router.post("/{org_slug}/sales-agent/contracts")
async def external_sales_contract(
    envelope: CommercialContractEnvelope,
    org_slug: str = Path(..., min_length=2),
    repo: AIRepository = Depends(get_repository),
    backend_client: BackendClient = Depends(get_backend_client),
):
    org_id = await resolve_org_id(backend_client, org_slug)
    update_request_context(org_id=org_id, user_id=envelope.contact_phone or envelope.contract.counterparty_id)
    payload = await process_contract(
        repo=repo,
        backend_client=backend_client,
        org_id=org_id,
        envelope=envelope,
        actor_id=envelope.contact_phone or envelope.contract.counterparty_id,
    )
    logger.info(
        "commercial_contract_processed",
        org_id=org_id,
        request_id=envelope.contract.request_id,
        intent=envelope.contract.intent,
    )
    return payload
