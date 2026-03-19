from __future__ import annotations

import json
from typing import Any

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field

from pymes_control_plane_shared.ai_runtime import AuthContext, Message, get_logger, orchestrate
from src.api.sse import EventSourceResponse
from src.domains.professionals.teachers.backend_client import TeachersBackendClient
from src.domains.professionals.teachers.deps import (
    get_auth_context,
    get_llm_provider,
    get_teachers_backend_client,
)
from src.domains.professionals.teachers.system_prompt import build_system_prompt
from src.domains.professionals.teachers.tools import build_internal_tools

router = APIRouter(tags=["professionals-teachers-chat"])
logger = get_logger(__name__)


class ChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=4000)


def estimate_tokens(text: str) -> int:
    if not text:
        return 0
    return max(1, len(text) // 4)


def to_sse_event(event: str, payload: dict[str, Any]) -> dict[str, str]:
    return {"event": event, "data": json.dumps(payload, ensure_ascii=False)}


@router.post("/v1/professionals/chat", include_in_schema=False)
@router.post("/v1/professionals/teachers/chat")
async def chat_teachers(
    req: ChatRequest,
    auth: AuthContext = Depends(get_auth_context),
    llm=Depends(get_llm_provider),
    backend_client: TeachersBackendClient = Depends(get_teachers_backend_client),
):
    logger.info("teachers_chat_started", org_id=auth.org_id, user_id=auth.actor)

    declarations, handlers = build_internal_tools(backend_client, auth)
    context = {"actor": auth.actor, "role": auth.role, "org_name": "la institucion"}
    llm_messages: list[Message] = [
        Message(role="system", content=build_system_prompt("internal", context)),
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
                org_id=auth.org_id,
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
            logger.exception("teachers_chat_failed", org_id=auth.org_id, user_id=auth.actor, error=str(exc))
            yield to_sse_event("error", {"message": "error processing request"})

        assistant_text = "".join(assistant_parts).strip() or "No pude generar una respuesta en este momento."
        tokens_out = estimate_tokens(assistant_text)

        logger.info(
            "teachers_chat_completed",
            org_id=auth.org_id,
            user_id=auth.actor,
            tool_calls=len(tool_calls),
            tokens_input=tokens_in,
            tokens_output=tokens_out,
        )
        yield to_sse_event("done", {"tokens_used": tokens_in + tokens_out})

    return EventSourceResponse(event_stream())
