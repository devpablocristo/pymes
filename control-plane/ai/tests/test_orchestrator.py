from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator

from src.core.orchestrator import OrchestratorLimits, orchestrate
from src.llm.base import ChatChunk, Message, ToolDeclaration


class MockLLM:
    def __init__(self) -> None:
        self.calls = 0

    async def chat(
        self,
        messages: list[Message],
        tools: list[ToolDeclaration] | None = None,
        temperature: float = 0.3,
        max_tokens: int = 2048,
    ) -> AsyncIterator[ChatChunk]:
        _ = messages
        _ = tools
        _ = temperature
        _ = max_tokens
        if self.calls == 0:
            self.calls += 1
            yield ChatChunk(type="tool_call", tool_call={"name": "get_sales_summary", "arguments": {"period": "today"}})
            return
        yield ChatChunk(type="text", text="Hoy vendiste $45.200")
        yield ChatChunk(type="done")


def test_orchestrator_executes_tool_and_returns_text() -> None:
    called: dict[str, str] = {}

    async def handler(org_id: str, period: str) -> dict:
        called["org_id"] = org_id
        called["period"] = period
        return {"total": 45200, "count": 6}

    async def run() -> list[ChatChunk]:
        items: list[ChatChunk] = []
        async for chunk in orchestrate(
            llm=MockLLM(),
            messages=[Message(role="user", content="Cuanto vendi hoy?")],
            tools=[ToolDeclaration(name="get_sales_summary", description="", parameters={"type": "object"})],
            tool_handlers={"get_sales_summary": handler},
            org_id="org-1",
        ):
            items.append(chunk)
        return items

    chunks = asyncio.run(run())

    assert any(c.type == "tool_call" for c in chunks)
    assert any(c.type == "tool_result" for c in chunks)
    assert any(c.type == "text" and c.text and "45.200" in c.text for c in chunks)
    assert called == {"org_id": "org-1", "period": "today"}


class _LoopLLM:
    async def chat(
        self,
        messages: list[Message],
        tools: list[ToolDeclaration] | None = None,
        temperature: float = 0.3,
        max_tokens: int = 2048,
    ) -> AsyncIterator[ChatChunk]:
        _ = (messages, tools, temperature, max_tokens)
        yield ChatChunk(type="tool_call", tool_call={"name": "echo", "arguments": {}})


def test_orchestrator_respects_custom_max_tool_calls() -> None:
    async def handler(org_id: str) -> dict:
        _ = org_id
        return {"ok": True}

    async def run() -> list[ChatChunk]:
        items: list[ChatChunk] = []
        async for chunk in orchestrate(
            llm=_LoopLLM(),
            messages=[Message(role="user", content="loop")],
            tools=[ToolDeclaration(name="echo", description="", parameters={"type": "object"})],
            tool_handlers={"echo": handler},
            org_id="org-1",
            limits=OrchestratorLimits(max_tool_calls=2, tool_timeout_seconds=1, total_timeout_seconds=5),
        ):
            items.append(chunk)
        return items

    chunks = asyncio.run(run())

    assert len([c for c in chunks if c.type == "tool_call"]) == 2


def test_orchestrator_respects_custom_tool_timeout() -> None:
    async def handler(org_id: str) -> dict:
        _ = org_id
        await asyncio.sleep(0.02)
        return {"ok": True}

    async def run() -> list[ChatChunk]:
        items: list[ChatChunk] = []
        async for chunk in orchestrate(
            llm=MockLLM(),
            messages=[Message(role="user", content="Cuanto vendi hoy?")],
            tools=[ToolDeclaration(name="get_sales_summary", description="", parameters={"type": "object"})],
            tool_handlers={"get_sales_summary": handler},
            org_id="org-1",
            limits=OrchestratorLimits(max_tool_calls=2, tool_timeout_seconds=0, total_timeout_seconds=5),
        ):
            items.append(chunk)
        return items

    chunks = asyncio.run(run())

    assert any(c.type == "tool_result" for c in chunks)
