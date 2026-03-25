from __future__ import annotations

from datetime import UTC, datetime

from fastapi import APIRouter, Depends, Path
from pydantic import BaseModel, Field

from src.api.chat_stream import Message, stream_orchestrated_chat
from src.api.deps import get_backend_client, get_llm_provider, get_repository
from src.api.external_chat_support import clean_phone, get_external_conversation, history_to_messages, resolve_org_id
from src.api.router import check_quota
from src.api.sse import EventSourceResponse
from src.backend_client.client import BackendClient
from src.core.dossier import summarize_dossier_for_context
from src.core.system_prompt import build_system_prompt
from src.db.repository import AIRepository
from core_ai.logging import get_logger, update_request_context
from src.tools.registry import build_external_tools

router = APIRouter(prefix="/v1/public", tags=["public-chat"])
logger = get_logger(__name__)


class PublicChatRequest(BaseModel):
    conversation_id: str | None = None
    message: str = Field(min_length=1, max_length=4000)
    phone: str | None = None


class IdentifyRequest(BaseModel):
    name: str = Field(min_length=1, max_length=120)
    phone: str = Field(min_length=6, max_length=32)


@router.post("/{org_slug}/chat")
async def chat_external(
    req: PublicChatRequest,
    org_slug: str = Path(..., min_length=2),
    repo: AIRepository = Depends(get_repository),
    llm=Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    org_id = await resolve_org_id(backend_client, org_slug)
    await check_quota(repo, org_id, mode="external")

    conversation = None
    external_contact = clean_phone(req.phone or "")
    update_request_context(org_id=org_id, user_id=external_contact or "external")
    logger.info("chat_external_started", org_id=org_id, external_contact=external_contact, conversation_id=req.conversation_id or "")
    conversation = await get_external_conversation(
        repo=repo,
        org_id=org_id,
        external_contact=external_contact,
        message=req.message,
        conversation_id=req.conversation_id,
    )

    dossier = await repo.get_or_create_dossier(org_id)
    declarations, handlers = build_external_tools(backend_client)

    history_messages = list(conversation.messages)
    llm_messages: list[Message] = [
        Message(role="system", content=build_system_prompt("external", None, dossier)),
        Message(role="system", content=f"Dossier: {summarize_dossier_for_context(dossier)}"),
        *history_to_messages(history_messages),
        Message(role="user", content=req.message.strip()),
    ]

    async def on_success(result) -> dict[str, str]:
        now = datetime.now(UTC).isoformat()
        await repo.append_messages(
            org_id=org_id,
            conversation_id=conversation.id,
            new_messages=[
                {"role": "user", "content": req.message.strip(), "ts": now},
                {
                    "role": "assistant",
                    "content": result.assistant_text,
                    "ts": now,
                    "tool_calls": result.unique_tool_calls,
                },
            ],
            tool_calls_count=len(result.tool_calls),
            tokens_input=result.tokens_input,
            tokens_output=result.tokens_output,
        )
        await repo.track_usage(org_id, tokens_in=result.tokens_input, tokens_out=result.tokens_output)
        logger.info(
            "chat_external_completed",
            org_id=org_id,
            external_contact=external_contact,
            conversation_id=conversation.id,
            tool_calls=len(result.tool_calls),
            tokens_input=result.tokens_input,
            tokens_output=result.tokens_output,
        )
        return {"conversation_id": conversation.id}

    return EventSourceResponse(
        stream_orchestrated_chat(
            llm=llm,
            llm_messages=llm_messages,
            declarations=declarations,
            handlers=handlers,
            org_id=org_id,
            failure_event="chat_external_failed",
            failure_context={"org_id": org_id, "external_contact": external_contact},
            on_success=on_success,
        )
    )


@router.post("/{org_slug}/chat/identify")
async def identify_external(req: IdentifyRequest, org_slug: str = Path(..., min_length=2)):
    _ = org_slug
    return {
        "name": req.name.strip(),
        "phone": clean_phone(req.phone),
        "status": "identified",
    }
