from __future__ import annotations

from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest

from src.tools import settings


@pytest.mark.asyncio
async def test_update_business_info_sends_scheduling_enabled() -> None:
    client = SimpleNamespace(request=AsyncMock(return_value={"ok": True}))
    auth = SimpleNamespace()

    await settings.update_business_info(
        client,
        auth,
        business_name="Demo Org",
        scheduling_enabled=True,
    )

    client.request.assert_awaited_once_with(
        "PATCH",
        "/v1/tenant-settings",
        auth=auth,
        json={
            "business_name": "Demo Org",
            "scheduling_enabled": True,
        },
    )
