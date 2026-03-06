from __future__ import annotations

from typing import Any

from fastapi import APIRouter, Depends, HTTPException, Path, status
from pydantic import BaseModel, Field

from src.api.deps import get_backend_client, get_llm_provider, get_repository
from src.api.external_chat_support import clean_phone, get_external_conversation, history_to_messages, resolve_org_id
from src.api.router import check_quota, estimate_tokens, to_sse_event
from src.api.sse import EventSourceResponse
from src.backend_client.client import BackendClient
from src.core.dossier import summarize_dossier_for_context
from src.core.orchestrator import orchestrate
from src.core.system_prompt import build_system_prompt
from src.db.repository import AIRepository
from src.llm.base import Message
from src.observability.logging import get_logger, update_request_context
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

    async def event_stream():
        assistant_parts: list[str] = []
        tool_calls: list[str] = []
        tokens_in = estimate_tokens("\n".join(m.content for m in llm_messages))

        try:
            async for chunk in orchestrate(
                llm=llm,
                messages=llm_messages,
                tools=declarations,
                tool_handlers=handlers,
                org_id=org_id,
            ):
                if chunk.type == "text" and chunk.text:
                    assistant_parts.append(chunk.text)
                    yield to_sse_event("text", {"content": chunk.text})
                    continue
                if chunk.type == "tool_call" and chunk.tool_call:
                    tool_name = str(chunk.tool_call.get("name", ""))
                    if tool_name:
                        tool_calls.append(tool_name)
                    yield to_sse_event("tool_call", {"tool": tool_name, "status": "executing"})
                    continue
                if chunk.type == "tool_result" and chunk.tool_call:
                    tool_name = str(chunk.tool_call.get("name", ""))
                    yield to_sse_event("tool_result", {"tool": tool_name, "status": "done"})
        except Exception as exc:  # noqa: BLE001
            logger.exception("chat_external_failed", org_id=org_id, external_contact=external_contact, error=str(exc))
            yield to_sse_event("error", {"message": str(exc)})

        assistant_text = "".join(assistant_parts).strip()
        if not assistant_text:
            assistant_text = "No pude generar una respuesta en este momento."
        tokens_out = estimate_tokens(assistant_text)
        now = datetime.now(UTC).isoformat()

        await repo.append_messages(
            org_id=org_id,
            conversation_id=conversation.id,
            new_messages=[
                {"role": "user", "content": req.message.strip(), "ts": now},
                {
                    "role": "assistant",
                    "content": assistant_text,
                    "ts": now,
                    "tool_calls": sorted(set(tool_calls)),
                },
            ],
            tool_calls_count=len(tool_calls),
            tokens_input=tokens_in,
            tokens_output=tokens_out,
        )
        await repo.track_usage(org_id, tokens_in=tokens_in, tokens_out=tokens_out)
        logger.info(
            "chat_external_completed",
            org_id=org_id,
            external_contact=external_contact,
            conversation_id=conversation.id,
            tool_calls=len(tool_calls),
            tokens_input=tokens_in,
            tokens_output=tokens_out,
        )

        yield to_sse_event(
            "done",
            {
                "conversation_id": conversation.id,
                "tokens_used": tokens_in + tokens_out,
            },
        )

    return EventSourceResponse(event_stream())


@router.post("/{org_slug}/chat/identify")
async def identify_external(req: IdentifyRequest, org_slug: str = Path(..., min_length=2)):
    _ = org_slug
    return {
        "name": req.name.strip(),
        "phone": clean_phone(req.phone),
        "status": "identified",
    }
