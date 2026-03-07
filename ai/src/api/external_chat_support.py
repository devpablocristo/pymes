from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

from fastapi import HTTPException, status

from src.backend_client.client import BackendClient
from src.core.dossier import summarize_dossier_for_context
from pymes_control_plane_shared.ai_runtime import orchestrate
from src.core.system_prompt import build_system_prompt
from src.db.repository import AIRepository
from pymes_control_plane_shared.ai_runtime import LLMProvider, Message
from pymes_control_plane_shared.ai_runtime import get_logger
from src.tools.registry import build_external_tools

logger = get_logger(__name__)


@dataclass
class ExternalChatResult:
    conversation_id: str
    reply: str
    tokens_input: int
    tokens_output: int
    tool_calls: list[str]

    @property
    def tokens_used(self) -> int:
        return self.tokens_input + self.tokens_output


def clean_phone(raw: str) -> str:
    return "".join(ch for ch in raw if ch.isdigit() or ch == "+")


async def resolve_org_id(backend_client: BackendClient, org_slug: str) -> str:
    try:
        payload = await backend_client.request("GET", f"/v1/public/{org_slug}/info", include_internal=True)
    except Exception as exc:
        status_code = getattr(getattr(exc, "response", None), "status_code", None)
        if status_code == status.HTTP_404_NOT_FOUND:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="organization not found") from exc
        raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail="backend unavailable") from exc

    org_id = str(payload.get("org_id", "")).strip()
    if not org_id:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="organization not found")
    return org_id


def history_to_messages(history: list[dict[str, Any]]) -> list[Message]:
    result: list[Message] = []
    for item in history[-10:]:
        role = str(item.get("role", "")).strip().lower()
        content = str(item.get("content", ""))
        if role not in {"user", "assistant", "tool"}:
            continue
        result.append(Message(role=role, content=content))
    return result


async def get_external_conversation(
    repo: AIRepository,
    org_id: str,
    external_contact: str,
    message: str,
    conversation_id: str | None = None,
    reuse_latest: bool = False,
):
    if conversation_id:
        conversation = await repo.get_conversation(org_id, conversation_id)
        if conversation is None or conversation.mode != "external":
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
        return conversation

    if reuse_latest and external_contact:
        conversation = await repo.get_latest_external_conversation(org_id, external_contact)
        if conversation is not None:
            return conversation

    await enforce_external_conversation_limit(repo, org_id)
    return await repo.create_conversation(
        org_id=org_id,
        mode="external",
        external_contact=external_contact,
        title=message.strip()[:60],
    )


async def enforce_external_conversation_limit(repo: AIRepository, org_id: str) -> None:
    from src.api.router import PLAN_LIMITS  # local import to avoid router cycle at import time

    now = datetime.now(UTC)
    plan = await repo.get_plan_code(org_id)
    limits = PLAN_LIMITS.get(plan, PLAN_LIMITS["starter"])
    external_limit = int(limits["external_limit"])
    if external_limit == -1:
        return

    used = await repo.count_external_conversations_in_month(org_id, now.year, now.month)
    if used >= external_limit:
        raise HTTPException(
            status_code=status.HTTP_429_TOO_MANY_REQUESTS,
            detail=f"Limite mensual de conversaciones externas alcanzado ({external_limit})",
        )


async def run_external_chat(
    *,
    repo: AIRepository,
    llm: LLMProvider,
    backend_client: BackendClient,
    org_id: str,
    message: str,
    external_contact: str,
    conversation_id: str | None = None,
    reuse_latest: bool = False,
    user_metadata: dict[str, Any] | None = None,
    assistant_metadata: dict[str, Any] | None = None,
) -> ExternalChatResult:
    conversation = await get_external_conversation(
        repo=repo,
        org_id=org_id,
        external_contact=external_contact,
        message=message,
        conversation_id=conversation_id,
        reuse_latest=reuse_latest,
    )
    dossier = await repo.get_or_create_dossier(org_id)
    declarations, handlers = build_external_tools(backend_client)
    llm_messages: list[Message] = [
        Message(role="system", content=build_system_prompt("external", None, dossier)),
        Message(role="system", content=f"Dossier: {summarize_dossier_for_context(dossier)}"),
        *history_to_messages(list(conversation.messages)),
        Message(role="user", content=message.strip()),
    ]

    from src.api.router import estimate_tokens  # local import to avoid router cycle at import time

    assistant_parts: list[str] = []
    tool_calls: list[str] = []
    tokens_in = estimate_tokens("\n".join(m.content for m in llm_messages))

    try:
        async for chunk in orchestrate(
            llm=llm,
            messages=llm_messages,
            tools=declarations,
            tool_handlers=handlers,
            org_id=org_id,
        ):
            if chunk.type == "text" and chunk.text:
                assistant_parts.append(chunk.text)
                continue
            if chunk.type == "tool_call" and chunk.tool_call:
                tool_name = str(chunk.tool_call.get("name", "")).strip()
                if tool_name:
                    tool_calls.append(tool_name)
    except Exception as exc:
        logger.exception("chat_external_failed", org_id=org_id, external_contact=external_contact, error=str(exc))
        raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail="ai unavailable") from exc

    assistant_text = "".join(assistant_parts).strip() or "No pude generar una respuesta en este momento."
    tokens_out = estimate_tokens(assistant_text)
    now = datetime.now(UTC).isoformat()

    user_message = {"role": "user", "content": message.strip(), "ts": now}
    if user_metadata:
        user_message.update(user_metadata)

    assistant_message = {
        "role": "assistant",
        "content": assistant_text,
        "ts": now,
        "tool_calls": sorted(set(tool_calls)),
    }
    if assistant_metadata:
        assistant_message.update(assistant_metadata)

    await repo.append_messages(
        org_id=org_id,
        conversation_id=conversation.id,
        new_messages=[user_message, assistant_message],
        tool_calls_count=len(tool_calls),
        tokens_input=tokens_in,
        tokens_output=tokens_out,
    )
    await repo.track_usage(org_id, tokens_in=tokens_in, tokens_out=tokens_out)

    return ExternalChatResult(
        conversation_id=conversation.id,
        reply=assistant_text,
        tokens_input=tokens_in,
        tokens_output=tokens_out,
        tool_calls=sorted(set(tool_calls)),
    )
