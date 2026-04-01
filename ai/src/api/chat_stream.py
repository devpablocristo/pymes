"""Re-exporta streaming de chat desde runtime.chat con fallback localizado."""

from __future__ import annotations

from runtime.chat.stream import (
    OnSuccess,
    StreamChatResult,
    ToolHandlers,
    stream_orchestrated_chat as _stream_orchestrated_chat,
)
from runtime.types import LLMProvider, Message, ToolDeclaration

_FALLBACK_REPLY_ES = "No pude generar una respuesta en este momento."


async def stream_orchestrated_chat(
    *,
    llm: LLMProvider,
    llm_messages: list[Message],
    declarations: list[ToolDeclaration],
    handlers: ToolHandlers,
    org_id: str,
    failure_event: str,
    failure_context: dict,
    on_success: OnSuccess | None = None,
):
    """Wrapper que inyecta el fallback en español."""
    async for event in _stream_orchestrated_chat(
        llm=llm,
        llm_messages=llm_messages,
        declarations=declarations,
        handlers=handlers,
        org_id=org_id,
        failure_event=failure_event,
        failure_context=failure_context,
        on_success=on_success,
        fallback_reply=_FALLBACK_REPLY_ES,
    ):
        yield event


__all__ = [
    "Message",
    "OnSuccess",
    "StreamChatResult",
    "ToolHandlers",
    "stream_orchestrated_chat",
]
