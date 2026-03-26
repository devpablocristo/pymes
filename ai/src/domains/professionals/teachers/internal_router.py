from __future__ import annotations

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field

from runtime.contexts import AuthContext
from runtime.logging import get_logger
from src.api.chat_stream import Message, stream_orchestrated_chat
from src.api.sse import EventSourceResponse
from src.domains.professionals.teachers.backend_client import TeachersBackendClient
from src.domains.professionals.teachers.deps import (
    get_auth_context,
    get_llm_provider,
    get_teachers_backend_client,
)
from src.domains.professionals.teachers.system_prompt import build_system_prompt
from src.domains.professionals.teachers.tools import build_internal_tools

router = APIRouter(tags=["professionals-teachers-chat"])
logger = get_logger(__name__)


class ChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=4000)


@router.post("/v1/professionals/teachers/chat")
async def chat_teachers(
    req: ChatRequest,
    auth: AuthContext = Depends(get_auth_context),
    llm=Depends(get_llm_provider),
    backend_client: TeachersBackendClient = Depends(get_teachers_backend_client),
):
    logger.info("teachers_chat_started", org_id=auth.org_id, user_id=auth.actor)

    declarations, handlers = build_internal_tools(backend_client, auth)
    context = {"actor": auth.actor, "role": auth.role, "org_name": "la institucion"}
    llm_messages: list[Message] = [
        Message(role="system", content=build_system_prompt("internal", context)),
        Message(role="user", content=req.message.strip()),
    ]

    async def on_success(result):
        logger.info(
            "teachers_chat_completed",
            org_id=auth.org_id,
            user_id=auth.actor,
            tool_calls=len(result.tool_calls),
            tokens_input=result.tokens_input,
            tokens_output=result.tokens_output,
        )
        return None

    return EventSourceResponse(
        stream_orchestrated_chat(
            llm=llm,
            llm_messages=llm_messages,
            declarations=declarations,
            handlers=handlers,
            org_id=auth.org_id,
            failure_event="teachers_chat_failed",
            failure_context={"org_id": auth.org_id, "user_id": auth.actor},
            on_success=on_success,
        )
    )
