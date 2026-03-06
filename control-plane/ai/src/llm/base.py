from __future__ import annotations

from dataclasses import dataclass
from typing import AsyncIterator, Protocol


@dataclass
class Message:
    role: str
    content: str
    tool_call_id: str | None = None
    tool_calls: list[dict] | None = None


@dataclass
class ToolDeclaration:
    name: str
    description: str
    parameters: dict


@dataclass
class ChatChunk:
    type: str
    text: str | None = None
    tool_call: dict | None = None
    meta: dict | None = None


class LLMProvider(Protocol):
    async def chat(
        self,
        messages: list[Message],
        tools: list[ToolDeclaration] | None = None,
        temperature: float = 0.3,
        max_tokens: int = 2048,
    ) -> AsyncIterator[ChatChunk]:
        ...


class EchoProvider:
    async def chat(
        self,
        messages: list[Message],
        tools: list[ToolDeclaration] | None = None,
        temperature: float = 0.3,
        max_tokens: int = 2048,
    ) -> AsyncIterator[ChatChunk]:
        _ = tools
        _ = temperature
        _ = max_tokens
        last_user = next((m.content for m in reversed(messages) if m.role == "user"), "")
        if not last_user:
            yield ChatChunk(type="text", text="No recibi ningun mensaje para procesar.")
        else:
            yield ChatChunk(type="text", text=f"Recibido: {last_user}")
        yield ChatChunk(type="done")
