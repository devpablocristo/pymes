from __future__ import annotations

import asyncio
from datetime import UTC, datetime
from types import SimpleNamespace

import pytest
from fastapi import HTTPException

from runtime.contexts import AuthContext
from src.api.router import get_conversation, list_conversations
from src.core.internal_conversations import can_access_internal_conversation, get_internal_conversation_user_id


class FakeRepo:
    def __init__(self, rows=None, conversation=None) -> None:
        self.rows = rows or []
        self.conversation = conversation
        self.list_call: dict[str, object] | None = None
        self.get_call: dict[str, object] | None = None

    async def list_conversations(self, org_id: str, mode: str, user_id: str | None, limit: int = 50):
        self.list_call = {"org_id": org_id, "mode": mode, "user_id": user_id, "limit": limit}
        return self.rows

    async def get_conversation(self, org_id: str, conversation_id: str):
        self.get_call = {"org_id": org_id, "conversation_id": conversation_id}
        return self.conversation


def _auth(*, actor: str) -> AuthContext:
    return AuthContext(
        tenant_id="11111111-1111-1111-1111-111111111111",
        actor=actor,
        role="service" if actor.startswith("api_key:") else "member",
        scopes=[],
        mode="internal",
    )


def _conversation(*, user_id: str | None):
    return SimpleNamespace(
        id="22222222-2222-2222-2222-222222222222",
        mode="internal",
        title="Conversation",
        messages=[],
        tool_calls_count=0,
        tokens_input=0,
        tokens_output=0,
        updated_at=datetime.now(UTC),
        user_id=user_id,
    )


def test_internal_conversation_user_id_normalizes_real_users_only() -> None:
    assert get_internal_conversation_user_id(_auth(actor="33333333-3333-3333-3333-333333333333")) == (
        "33333333-3333-3333-3333-333333333333"
    )
    assert get_internal_conversation_user_id(_auth(actor="api_key:key-123")) is None


def test_internal_conversation_access_matches_caller_scope() -> None:
    assert can_access_internal_conversation(_auth(actor="33333333-3333-3333-3333-333333333333"), None) is False
    assert can_access_internal_conversation(
        _auth(actor="33333333-3333-3333-3333-333333333333"),
        "33333333-3333-3333-3333-333333333333",
    ) is True
    assert can_access_internal_conversation(_auth(actor="api_key:key-123"), None) is True
    assert can_access_internal_conversation(
        _auth(actor="api_key:key-123"),
        "33333333-3333-3333-3333-333333333333",
    ) is False


def test_list_conversations_uses_org_wide_scope_for_api_keys() -> None:
    repo = FakeRepo()

    result = asyncio.run(list_conversations(repo=repo, auth=_auth(actor="api_key:key-123")))

    assert result == []
    assert repo.list_call == {
        "org_id": "11111111-1111-1111-1111-111111111111",
        "mode": "internal",
        "user_id": None,
        "limit": 50,
    }


def test_get_conversation_allows_org_wide_service_conversations() -> None:
    repo = FakeRepo(conversation=_conversation(user_id=None))

    result = asyncio.run(
        get_conversation(
            "22222222-2222-2222-2222-222222222222",
            repo=repo,
            auth=_auth(actor="api_key:key-123"),
        )
    )

    assert result.id == "22222222-2222-2222-2222-222222222222"


def test_get_conversation_rejects_user_owned_conversation_for_api_keys() -> None:
    repo = FakeRepo(conversation=_conversation(user_id="33333333-3333-3333-3333-333333333333"))

    with pytest.raises(HTTPException) as exc:
        asyncio.run(
            get_conversation(
                "22222222-2222-2222-2222-222222222222",
                repo=repo,
                auth=_auth(actor="api_key:key-123"),
            )
        )

    assert exc.value.status_code == 404
