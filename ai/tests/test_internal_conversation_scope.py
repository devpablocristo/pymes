from __future__ import annotations

from runtime.contexts import AuthContext
from src.core.internal_conversations import can_access_internal_conversation, get_internal_conversation_user_id


def _auth(*, actor: str) -> AuthContext:
    return AuthContext(
        tenant_id="11111111-1111-1111-1111-111111111111",
        actor=actor,
        role="service" if actor.startswith("api_key:") else "member",
        scopes=[],
        mode="internal",
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
