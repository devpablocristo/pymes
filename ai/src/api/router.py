from __future__ import annotations

import copy
import json
from datetime import UTC, datetime
from typing import Any

from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel, Field

from src.api.deps import get_auth_context, get_backend_client, get_llm_provider, get_repository
from src.api.sse import EventSourceResponse
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from pymes_core_shared.ai_runtime import orchestrate
from src.core.system_prompt import build_system_prompt
from src.core.dossier import summarize_dossier_for_context
from src.core.internal_conversations import can_access_internal_conversation, get_internal_conversation_user_id
from src.db.repository import AIRepository
from pymes_core_shared.ai_runtime import Message
from pymes_core_shared.ai_runtime import get_logger
from src.tools.registry import build_internal_tools

router = APIRouter(prefix="/v1/chat", tags=["chat"])
logger = get_logger(__name__)

PLAN_LIMITS: dict[str, dict[str, int | bool]] = {
    "starter": {"queries": 50, "external": False, "external_limit": 0},
    "growth": {"queries": 500, "external": True, "external_limit": 200},
    "enterprise": {"queries": -1, "external": True, "external_limit": -1},
}


class ChatRequest(BaseModel):
    conversation_id: str | None = None
    message: str = Field(min_length=1, max_length=4000)


class ConversationItem(BaseModel):
    id: str
    mode: str
    title: str
    updated_at: str
    messages_count: int


class ConversationDetail(BaseModel):
    id: str
    mode: str
    title: str
    messages: list[dict[str, Any]]
    tool_calls_count: int
    tokens_input: int
    tokens_output: int
    updated_at: str


class UsageResponse(BaseModel):
    plan: str
    month: str
    queries: int
    tokens_input: int
    tokens_output: int


def estimate_tokens(text: str) -> int:
    if not text:
        return 0
    return max(1, len(text) // 4)


def to_sse_event(event: str, payload: dict[str, Any]) -> dict[str, str]:
    return {"event": event, "data": json.dumps(payload, ensure_ascii=False)}


async def check_quota(repo: AIRepository, org_id: str, mode: str) -> str:
    now = datetime.now(UTC)
    plan = await repo.get_plan_code(org_id)
    limits = PLAN_LIMITS.get(plan, PLAN_LIMITS["starter"])
    usage = await repo.get_month_usage(org_id, now.year, now.month)

    if mode == "external" and not bool(limits["external"]):
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="AI externo no disponible para este plan")

    query_limit = int(limits["queries"])
    if query_limit != -1 and usage["queries"] >= query_limit:
        raise HTTPException(
            status_code=status.HTTP_429_TOO_MANY_REQUESTS,
            detail=f"Limite mensual alcanzado ({query_limit} consultas)",
        )

    return plan


def _history_to_messages(history: list[dict[str, Any]]) -> list[Message]:
    result: list[Message] = []
    for item in history[-10:]:
        role = str(item.get("role", "")).strip().lower()
        content = str(item.get("content", ""))
        if role not in {"user", "assistant", "tool"}:
            continue
        result.append(Message(role=role, content=content))
    return result


@router.post("")
async def chat_internal(
    req: ChatRequest,
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
    llm=Depends(get_llm_provider),
    backend_client: BackendClient = Depends(get_backend_client),
):
    await check_quota(repo, auth.org_id, mode="internal")
    logger.info("chat_internal_started", org_id=auth.org_id, user_id=auth.actor, conversation_id=req.conversation_id or "")
    conversation_user_id = get_internal_conversation_user_id(auth)

    conversation = None
    if req.conversation_id:
        conversation = await repo.get_conversation(auth.org_id, req.conversation_id)
        if conversation is None:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
        if conversation.mode != "internal":
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="invalid conversation mode")
        if not can_access_internal_conversation(auth, conversation.user_id):
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
    else:
        conversation = await repo.create_conversation(
            org_id=auth.org_id,
            mode="internal",
            user_id=conversation_user_id,
            title=req.message.strip()[:60],
        )

    dossier = await repo.get_or_create_dossier(auth.org_id)
    dossier_snapshot = copy.deepcopy(dossier)
    declarations, handlers = build_internal_tools(backend_client, auth, dossier)

    conversation_messages = list(conversation.messages)
    llm_messages: list[Message] = [
        Message(role="system", content=build_system_prompt("internal", auth, dossier)),
        Message(role="system", content=f"Dossier: {summarize_dossier_for_context(dossier)}"),
        *_history_to_messages(conversation_messages),
        Message(role="user", content=req.message.strip()),
    ]

    async def event_stream():
        assistant_parts: list[str] = []
        tool_calls: list[str] = []
        tokens_in = estimate_tokens("\n".join(m.content for m in llm_messages))

        try:
            async for chunk in orchestrate(
                llm=llm,
                messages=llm_messages,
                tools=declarations,
                tool_handlers=handlers,
                org_id=auth.org_id,
            ):
                if chunk.type == "text" and chunk.text:
                    assistant_parts.append(chunk.text)
                    yield to_sse_event("text", {"content": chunk.text})
                    continue
                if chunk.type == "tool_call" and chunk.tool_call:
                    tool_name = str(chunk.tool_call.get("name", ""))
                    if tool_name:
                        tool_calls.append(tool_name)
                    yield to_sse_event("tool_call", {"tool": tool_name, "status": "executing"})
                    continue
                if chunk.type == "tool_result" and chunk.tool_call:
                    tool_name = str(chunk.tool_call.get("name", ""))
                    yield to_sse_event("tool_result", {"tool": tool_name, "status": "done"})
        except Exception as exc:  # noqa: BLE001
            logger.exception("chat_internal_failed", org_id=auth.org_id, user_id=auth.actor, error=str(exc))
            yield to_sse_event("error", {"message": "error processing request"})

        assistant_text = "".join(assistant_parts).strip()
        if not assistant_text:
            assistant_text = "No pude generar una respuesta en este momento."
        tokens_out = estimate_tokens(assistant_text)
        now = datetime.now(UTC).isoformat()

        await repo.append_messages(
            org_id=auth.org_id,
            conversation_id=conversation.id,
            new_messages=[
                {"role": "user", "content": req.message.strip(), "ts": now},
                {
                    "role": "assistant",
                    "content": assistant_text,
                    "ts": now,
                    "tool_calls": sorted(set(tool_calls)),
                },
            ],
            tool_calls_count=len(tool_calls),
            tokens_input=tokens_in,
            tokens_output=tokens_out,
        )
        await repo.track_usage(auth.org_id, tokens_in=tokens_in, tokens_out=tokens_out)

        if dossier != dossier_snapshot:
            await repo.update_dossier(auth.org_id, dossier)

        logger.info(
            "chat_internal_completed",
            org_id=auth.org_id,
            user_id=auth.actor,
            conversation_id=conversation.id,
            tool_calls=len(tool_calls),
            tokens_input=tokens_in,
            tokens_output=tokens_out,
        )

        yield to_sse_event(
            "done",
            {
                "conversation_id": conversation.id,
                "tokens_used": tokens_in + tokens_out,
            },
        )

    return EventSourceResponse(event_stream())


@router.get("/conversations", response_model=list[ConversationItem])
async def list_conversations(
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
):
    rows = await repo.list_conversations(
        org_id=auth.org_id,
        mode="internal",
        user_id=get_internal_conversation_user_id(auth),
        limit=50,
    )
    out: list[ConversationItem] = []
    for row in rows:
        out.append(
            ConversationItem(
                id=row.id,
                mode=row.mode,
                title=row.title,
                updated_at=row.updated_at.isoformat(),
                messages_count=len(row.messages),
            )
        )
    return out


@router.get("/conversations/{conversation_id}", response_model=ConversationDetail)
async def get_conversation(
    conversation_id: str,
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
):
    row = await repo.get_conversation(auth.org_id, conversation_id)
    if row is None or row.mode != "internal":
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
    if not can_access_internal_conversation(auth, row.user_id):
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")

    return ConversationDetail(
        id=row.id,
        mode=row.mode,
        title=row.title,
        messages=row.messages,
        tool_calls_count=row.tool_calls_count,
        tokens_input=row.tokens_input,
        tokens_output=row.tokens_output,
        updated_at=row.updated_at.isoformat(),
    )


@router.delete("/conversations/{conversation_id}")
async def delete_conversation(
    conversation_id: str,
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
):
    row = await repo.get_conversation(auth.org_id, conversation_id)
    if row is None or row.mode != "internal":
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
    if not can_access_internal_conversation(auth, row.user_id):
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")

    ok = await repo.delete_conversation(auth.org_id, conversation_id)
    if not ok:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
    return {"ok": True}


@router.get("/usage", response_model=UsageResponse)
async def get_usage(
    repo: AIRepository = Depends(get_repository),
    auth: AuthContext = Depends(get_auth_context),
):
    now = datetime.now(UTC)
    usage = await repo.get_month_usage(auth.org_id, now.year, now.month)
    plan = await repo.get_plan_code(auth.org_id)
    return UsageResponse(
        plan=plan,
        month=f"{now.year:04d}-{now.month:02d}",
        queries=usage["queries"],
        tokens_input=usage["tokens_input"],
        tokens_output=usage["tokens_output"],
    )
