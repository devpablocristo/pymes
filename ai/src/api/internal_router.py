from __future__ import annotations

import secrets

from fastapi import APIRouter, Depends, Header, HTTPException, status
from pydantic import BaseModel, Field

from src.agents.service import run_commercial_chat
from src.api.deps import get_backend_client, get_llm_provider, get_repository, get_settings_dep
from src.api.external_chat_support import clean_phone
from src.api.quota import check_quota
from src.backend_client.client import BackendClient
from src.config import Settings
from src.db.repository import AIRepository
from runtime.types import LLMProvider
from runtime.logging import get_logger, update_request_context

router = APIRouter(prefix="/v1/internal", tags=["internal"])
logger = get_logger(__name__)


class CustomerMessagingInboundRequest(BaseModel):
    org_id: str = Field(min_length=36, max_length=36)
    phone_number_id: str = Field(min_length=3, max_length=120)
    from_phone: str = Field(min_length=6, max_length=32)
    message: str = Field(min_length=1, max_length=4000)
    message_id: str | None = Field(default=None, max_length=255)
    profile_name: str | None = Field(default=None, max_length=255)
    conversation_id: str | None = None


class CustomerMessagingInboundResponse(BaseModel):
    conversation_id: str
    reply: str
    tokens_used: int
    tool_calls: list[str]


def require_internal_token(
    settings: Settings = Depends(get_settings_dep),
    internal_token: str = Header(default="", alias="X-Internal-Service-Token"),
) -> None:
    expected = settings.internal_service_token.strip()
    provided = internal_token.strip()
    if expected and not secrets.compare_digest(provided, expected):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="unauthorized")


@router.post(
    "/customer-messaging/inbound",
    response_model=CustomerMessagingInboundResponse,
    dependencies=[Depends(require_internal_token)],
    operation_id="customer_messaging_inbound_v1_internal_customer_messaging_inbound_post",
)
@router.post(
    "/whatsapp/message",
    response_model=CustomerMessagingInboundResponse,
    dependencies=[Depends(require_internal_token)],
    deprecated=True,
    operation_id="customer_messaging_inbound_legacy_v1_internal_whatsapp_message_post",
)
async def customer_messaging_inbound(
    req: CustomerMessagingInboundRequest,
    repo: AIRepository = Depends(get_repository),
    llm: LLMProvider = Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    org_id = req.org_id.strip()
    external_contact = clean_phone(req.from_phone)
    await check_quota(repo, org_id, mode="external")
    update_request_context(org_id=org_id, user_id=external_contact or "whatsapp")

    result = await run_commercial_chat(
        repo=repo,
        llm=llm,
        backend_client=backend_client,
        org_id=org_id,
        message=req.message,
        agent_mode="external_sales",
        channel="whatsapp",
        external_contact=external_contact,
        conversation_id=req.conversation_id,
        confirmed_actions=[],
        user_metadata={
            "channel": "whatsapp",
            "message_id": (req.message_id or "").strip(),
            "phone_number_id": req.phone_number_id.strip(),
            "profile_name": (req.profile_name or "").strip(),
        },
        assistant_metadata={"channel": "whatsapp", "phone_number_id": req.phone_number_id.strip()},
    )
    logger.info(
        "chat_customer_messaging_completed",
        org_id=org_id,
        external_contact=external_contact,
        conversation_id=result.conversation_id,
        tool_calls=len(result.tool_calls),
        tokens_used=result.tokens_used,
    )
    return CustomerMessagingInboundResponse(
        conversation_id=result.conversation_id,
        reply=result.reply,
        tokens_used=result.tokens_used,
        tool_calls=result.tool_calls,
    )
