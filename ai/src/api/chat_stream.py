from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any, Awaitable, Callable

from runtime.types import LLMProvider, Message, ToolDeclaration
from runtime.logging import get_logger
from runtime.orchestrator import orchestrate

logger = get_logger(__name__)

ToolHandlers = dict[str, Callable[..., Awaitable[Any]]]
OnSuccess = Callable[["StreamChatResult"], Awaitable[dict[str, Any] | None]]


@dataclass(slots=True)
class StreamChatResult:
    assistant_text: str
    tool_calls: list[str]
    tokens_input: int
    tokens_output: int

    @property
    def tokens_used(self) -> int:
        return self.tokens_input + self.tokens_output

    @property
    def unique_tool_calls(self) -> list[str]:
        return sorted(set(self.tool_calls))


def estimate_tokens(text: str) -> int:
    if not text:
        return 0
    return max(1, len(text) // 4)


def to_sse_event(event: str, payload: dict[str, Any]) -> dict[str, str]:
    return {"event": event, "data": json.dumps(payload, ensure_ascii=False)}


async def stream_orchestrated_chat(
    *,
    llm: LLMProvider,
    llm_messages: list[Message],
    declarations: list[ToolDeclaration],
    handlers: ToolHandlers,
    org_id: str,
    failure_event: str,
    failure_context: dict[str, Any],
    on_success: OnSuccess | None = None,
):
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
                tool_name = str(chunk.tool_call.get("name", "")).strip()
                if tool_name:
                    tool_calls.append(tool_name)
                yield to_sse_event("tool_call", {"tool": tool_name, "status": "executing"})
                continue
            if chunk.type == "tool_result" and chunk.tool_call:
                tool_name = str(chunk.tool_call.get("name", "")).strip()
                yield to_sse_event("tool_result", {"tool": tool_name, "status": "done"})

        result = StreamChatResult(
            assistant_text="".join(assistant_parts).strip() or "No pude generar una respuesta en este momento.",
            tool_calls=tool_calls,
            tokens_input=tokens_in,
            tokens_output=estimate_tokens("".join(assistant_parts).strip() or "No pude generar una respuesta en este momento."),
        )
        done_payload = await on_success(result) if on_success is not None else None
    except Exception as exc:  # noqa: BLE001
        logger.exception(failure_event, **failure_context, error=str(exc))
        yield to_sse_event("error", {"message": "error processing request"})
        return

    payload = {"tokens_used": result.tokens_used}
    if done_payload:
        payload.update(done_payload)
    yield to_sse_event("done", payload)
