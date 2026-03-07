from __future__ import annotations

import json
from typing import Any

from fastapi import APIRouter, Depends, Path
from pydantic import BaseModel, Field

from src.api.deps import get_backend_client, get_llm_provider
from src.api.sse import EventSourceResponse
from src.backend_client import BackendClient
from pymes_py_pkg.ai_runtime import orchestrate
from src.core.system_prompt import build_system_prompt
from pymes_py_pkg.ai_runtime import Message
from pymes_py_pkg.ai_runtime import get_logger, update_request_context
from src.tools.registry import build_external_tools

router = APIRouter(prefix="/v1/public", tags=["public-chat"])
logger = get_logger(__name__)


class PublicChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=4000)
    phone: str | None = None


def estimate_tokens(text: str) -> int:
    if not text:
        return 0
    return max(1, len(text) // 4)


def to_sse_event(event: str, payload: dict[str, Any]) -> dict[str, str]:
    return {"event": event, "data": json.dumps(payload, ensure_ascii=False)}


@router.post("/{org_slug}/chat")
async def chat_external(
    req: PublicChatRequest,
    org_slug: str = Path(..., min_length=2),
    llm=Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    update_request_context(org_id=org_slug, user_id=req.phone or "external")
    logger.info("chat_external_started", org_slug=org_slug, phone=req.phone or "")

    declarations, handlers = build_external_tools(backend_client, org_slug=org_slug)

    context = {
        "org_name": org_slug,
    }
    llm_messages: list[Message] = [
        Message(role="system", content=build_system_prompt("external", context)),
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
                org_id=org_slug,
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
            logger.exception("chat_external_failed", org_slug=org_slug, error=str(exc))
            yield to_sse_event("error", {"message": str(exc)})

        assistant_text = "".join(assistant_parts).strip()
        if not assistant_text:
            assistant_text = "No pude generar una respuesta en este momento."
        tokens_out = estimate_tokens(assistant_text)

        logger.info(
            "chat_external_completed",
            org_slug=org_slug,
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
