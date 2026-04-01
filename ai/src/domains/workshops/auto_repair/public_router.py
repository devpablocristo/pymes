from __future__ import annotations

from fastapi import APIRouter, Depends, Path
from pydantic import BaseModel, Field

from runtime.logging import get_logger, update_request_context
from runtime.types import Message
from src.api.chat_stream import stream_orchestrated_chat
from src.api.sse import EventSourceResponse
from src.domains.workshops.auto_repair.backend_client import AutoRepairBackendClient
from src.domains.workshops.auto_repair.deps import get_auto_repair_backend_client, get_llm_provider
from src.domains.workshops.auto_repair.system_prompt import build_system_prompt
from src.domains.workshops.auto_repair.tools import build_external_tools

router = APIRouter(tags=["workshops-auto-repair-public-chat"])
logger = get_logger(__name__)


class PublicChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=4000)
    phone: str | None = None


@router.post("/v1/workshops/auto-repair/public/{org_slug}/chat")
async def chat_auto_repair_public(
    req: PublicChatRequest,
    org_slug: str = Path(..., min_length=2),
    llm=Depends(get_llm_provider),
    backend_client: AutoRepairBackendClient = Depends(get_auto_repair_backend_client),
):
    update_request_context(org_id=org_slug, user_id=req.phone or "external")
    logger.info("auto_repair_public_chat_started", org_slug=org_slug, phone=req.phone or "")

    declarations, handlers = build_external_tools(backend_client, org_slug=org_slug)
    llm_messages: list[Message] = [
        Message(role="system", content=build_system_prompt("external", {"org_name": org_slug})),
        Message(role="user", content=req.message.strip()),
    ]

    async def on_success(result):
        logger.info(
            "auto_repair_public_chat_completed",
            org_slug=org_slug,
            tool_calls=len(result.tool_calls),
            tokens_input=result.tokens_input,
            tokens_output=result.tokens_output,
        )
        return None

    return EventSourceResponse(
        stream_orchestrated_chat(
            llm=llm,
            llm_messages=llm_messages,
            declarations=declarations,
            handlers=handlers,
            org_id=org_slug,
            failure_event="auto_repair_public_chat_failed",
            failure_context={"org_slug": org_slug},
            on_success=on_success,
        )
    )
