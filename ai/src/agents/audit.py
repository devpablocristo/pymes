from __future__ import annotations

from typing import Any


async def record_agent_event(
    repo: Any,
    *,
    org_id: str,
    conversation_id: str | None,
    agent_mode: str,
    channel: str,
    actor_id: str,
    actor_type: str,
    action: str,
    result: str,
    confirmed: bool,
    tool_name: str = "",
    entity_type: str = "",
    entity_id: str = "",
    request_id: str | None = None,
    capability_id: str | None = None,
    confirmation_id: str | None = None,
    review_request_id: str | None = None,
    idempotency_key: str | None = None,
    payload_hash: str | None = None,
    metadata: dict[str, Any] | None = None,
) -> None:
    handler = getattr(repo, "record_agent_event", None)
    if handler is None:
        return
    await handler(
        org_id=org_id,
        conversation_id=conversation_id,
        agent_mode=agent_mode,
        channel=channel,
        actor_id=actor_id,
        actor_type=actor_type,
        action=action,
        result=result,
        confirmed=confirmed,
        tool_name=tool_name,
        entity_type=entity_type,
        entity_id=entity_id,
        external_request_id=request_id,
        capability_id=capability_id,
        confirmation_id=confirmation_id,
        review_request_id=review_request_id,
        idempotency_key=idempotency_key,
        payload_hash=payload_hash,
        metadata=metadata or {},
    )


async def has_processed_request(repo: Any, org_id: str, request_id: str) -> bool:
    checker = getattr(repo, "has_agent_request", None)
    if checker is None:
        return False
    return bool(await checker(org_id, request_id))
