from __future__ import annotations

import asyncio
from unittest.mock import AsyncMock, patch

import pytest

from pymes_control_plane_shared.ai_runtime import ChatChunk, Message
from pymes_control_plane_shared.ai_runtime import GeminiProvider


def build_provider() -> GeminiProvider:
    with patch("pymes_control_plane_shared.ai_runtime.gemini.genai.Client", return_value=object()):
        return GeminiProvider(api_key="test-key")


def test_gemini_provider_retries_transient_failures() -> None:
    provider = build_provider()
    provider._collect_chunks = AsyncMock(  # type: ignore[method-assign]
        side_effect=[
            RuntimeError("temporary-1"),
            RuntimeError("temporary-2"),
            [ChatChunk(type="text", text="ok"), ChatChunk(type="done")],
        ]
    )

    async def run() -> list[ChatChunk]:
        return [chunk async for chunk in provider.chat([Message(role="user", content="hola")])]

    with patch("pymes_control_plane_shared.ai_runtime.gemini.asyncio.sleep", new=AsyncMock()) as sleep_mock:
        chunks = asyncio.run(run())

    assert provider._collect_chunks.await_count == 3  # type: ignore[attr-defined]
    assert sleep_mock.await_count == 2
    assert any(chunk.type == "text" and chunk.text == "ok" for chunk in chunks)


def test_gemini_provider_raises_after_retry_exhaustion() -> None:
    provider = build_provider()
    provider._collect_chunks = AsyncMock(side_effect=RuntimeError("still failing"))  # type: ignore[method-assign]

    async def run() -> None:
        async for _ in provider.chat([Message(role="user", content="hola")]):
            pass

    with patch("pymes_control_plane_shared.ai_runtime.gemini.asyncio.sleep", new=AsyncMock()) as sleep_mock:
        with pytest.raises(RuntimeError, match="still failing"):
            asyncio.run(run())

    assert provider._collect_chunks.await_count == 3  # type: ignore[attr-defined]
    assert sleep_mock.await_count == 2
