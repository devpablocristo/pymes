from __future__ import annotations

import json
from typing import Any

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field

from src.api.deps import get_auth_context, get_backend_client, get_llm_provider
from src.api.sse import EventSourceResponse
from src.backend_client import BackendClient
from pymes_py_pkg.ai_runtime import orchestrate
from src.core.system_prompt import build_system_prompt
from pymes_py_pkg.ai_runtime import Message
from pymes_py_pkg.ai_runtime import AuthContext
from pymes_py_pkg.ai_runtime import get_logger
from src.tools.registry import build_internal_tools

router = APIRouter(prefix="/v1/chat", tags=["chat"])
logger = get_logger(__name__)


class ChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=4000)


def estimate_tokens(text: str) -> int:
    if not text:
        return 0
    return max(1, len(text) // 4)


def to_sse_event(event: str, payload: dict[str, Any]) -> dict[str, str]:
    return {"event": event, "data": json.dumps(payload, ensure_ascii=False)}


def _history_to_messages(history: list[dict[str, Any]]) -> list[Message]:
    result: list[Message] = []
    for item in history[-10:]:
        role = str(item.get("role", "")).strip().lower()
        content = str(item.get("content", ""))
        if role not in {"user", "assistant", "tool"}:
            continue
        result.append(Message(role=role, content=content))
    return result


@router.post("")
async def chat_internal(
    req: ChatRequest,
    auth: AuthContext = Depends(get_auth_context),
    llm=Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    logger.info("chat_internal_started", org_id=auth.org_id, user_id=auth.actor)

    declarations, handlers = build_internal_tools(backend_client, auth)

    context = {
        "actor": auth.actor,
        "role": auth.role,
        "org_name": "el estudio",
    }
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
            logger.exception("chat_internal_failed", org_id=auth.org_id, user_id=auth.actor, error=str(exc))
            yield to_sse_event("error", {"message": str(exc)})

        assistant_text = "".join(assistant_parts).strip()
        if not assistant_text:
            assistant_text = "No pude generar una respuesta en este momento."
        tokens_out = estimate_tokens(assistant_text)

        logger.info(
            "chat_internal_completed",
            org_id=auth.org_id,
            user_id=auth.actor,
            tool_calls=len(tool_calls),
            tokens_input=tokens_in,
            tokens_output=tokens_out,
        )

        yield to_sse_event(
            "done",
            {
                "tokens_used": tokens_in + tokens_out,
            },
        )

    return EventSourceResponse(event_stream())
