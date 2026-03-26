from __future__ import annotations

from uuid import UUID

from runtime.contexts import AuthContext


def get_internal_conversation_user_id(auth: AuthContext) -> str | None:
    normalized = str(auth.actor).strip()
    if not normalized:
        return None
    try:
        return str(UUID(normalized))
    except ValueError:
        return None


def can_access_internal_conversation(auth: AuthContext, owner_user_id: str | None) -> bool:
    return owner_user_id == get_internal_conversation_user_id(auth)
