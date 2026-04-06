from __future__ import annotations

from typing import Literal

from pydantic import Field

from runtime.chat import (
    ChatAction as BaseChatAction,
    ChatBlock,
    ChatRequest as BaseChatRequest,
    ChatResponse as BaseChatResponse,
    ChatActionsBlock,
    ChatInsightCardBlock,
    ChatKpiGroupBlock,
    ChatKpiItem,
    ChatTableBlock,
    ChatTextBlock,
    InsightCardHighlight,
)
from src.localization import LanguageCode
from src.runtime_contracts import OUTPUT_KIND_CHAT_REPLY


RoutedAgent = Literal["general", "copilot", "customers", "products", "sales", "collections", "purchases"]
ChatRouteHint = Literal["general", "copilot", "customers", "products", "sales", "collections", "purchases"]
RoutingSource = Literal["copilot_agent", "orchestrator", "read_fallback", "ui_hint"]


class ChatAction(BaseChatAction):
    route_hint: ChatRouteHint | None = None
    selection_behavior: Literal["route_and_resend", "prompt_for_query"] | None = None
    confirmed_actions: list[str] = Field(default_factory=list)


class ChatRequest(BaseChatRequest):
    confirmed_actions: list[str] = Field(default_factory=list)
    route_hint: ChatRouteHint | None = Field(
        default=None,
        description=(
            "Hint opcional para forzar el carril del turno actual: general | customers | products | sales | collections | purchases. "
            "`copilot` queda reservado para handoff explícito desde notificaciones."
        ),
    )
    preferred_language: LanguageCode | None = Field(
        default=None,
        description=(
            "Idioma preferido para contenido generado por AI. Hoy se normaliza sobre `es|en`; "
            "si falta o no se soporta, el backend cae a español."
        ),
    )


class ChatResponse(BaseChatResponse):
    output_kind: Literal["chat_reply"] = Field(default=OUTPUT_KIND_CHAT_REPLY)
    content_language: LanguageCode = Field(
        default="es",
        description="Idioma efectivo del contenido devuelto por el backend para este turno.",
    )
    pending_confirmations: list[str] = Field(default_factory=list)
    routed_agent: RoutedAgent = Field(
        ...,
        description=(
            "Agente o sub-agente seleccionado para este turno: general | copilot | customers | products | sales | collections | purchases. "
            "`copilot` se usa solo en handoff explícito desde notificaciones."
        ),
    )
    routed_mode: RoutedAgent = Field(
        ...,
        description="Alias legacy de `routed_agent` para compatibilidad hacia atrás.",
    )
    routing_source: RoutingSource = Field(
        ...,
        description="Origen efectivo del turno: copilot_agent | orchestrator | read_fallback | ui_hint",
    )


# Re-exportar tipos base para que los consumidores existentes no rompan.
__all__ = [
    "ChatAction",
    "ChatActionsBlock",
    "ChatBlock",
    "ChatInsightCardBlock",
    "ChatKpiGroupBlock",
    "ChatKpiItem",
    "ChatRequest",
    "ChatResponse",
    "ChatRouteHint",
    "ChatTableBlock",
    "ChatTextBlock",
    "InsightCardHighlight",
    "RoutedAgent",
    "RoutingSource",
]
