from __future__ import annotations

from collections.abc import AsyncIterable
from typing import Any

from fastapi.responses import StreamingResponse

try:
    from sse_starlette.sse import EventSourceResponse as _EventSourceResponse

    EventSourceResponse = _EventSourceResponse
except ModuleNotFoundError:

    class EventSourceResponse(StreamingResponse):
        def __init__(self, content: AsyncIterable[dict[str, Any]], *args: Any, **kwargs: Any) -> None:
            super().__init__(self._encode(content), media_type="text/event-stream", *args, **kwargs)

        async def _encode(self, content: AsyncIterable[dict[str, Any]]):
            async for item in content:
                event = str(item.get("event", "message"))
                data = str(item.get("data", ""))
                payload = f"event: {event}\n"
                for line in data.splitlines() or [""]:
                    payload += f"data: {line}\n"
                payload += "\n"
                yield payload.encode("utf-8")
