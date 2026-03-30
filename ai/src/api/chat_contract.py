from __future__ import annotations

from typing import Annotated, Literal

from pydantic import AliasChoices, BaseModel, Field
from runtime import OUTPUT_KIND_CHAT_REPLY


class ChatRequest(BaseModel):
    chat_id: str | None = Field(
        default=None,
        validation_alias=AliasChoices("chat_id", "conversation_id"),
        serialization_alias="chat_id",
    )
    message: str = Field(min_length=1, max_length=4000)
    confirmed_actions: list[str] = Field(default_factory=list)


class ChatAction(BaseModel):
    id: str = Field(min_length=1)
    label: str = Field(min_length=1)
    kind: Literal["send_message", "open_url", "confirm_action"]
    style: Literal["primary", "secondary", "ghost"] = "secondary"
    message: str | None = None
    url: str | None = None
    confirmed_actions: list[str] = Field(default_factory=list)


class ChatTextBlock(BaseModel):
    type: Literal["text"]
    text: str = Field(min_length=1)


class ChatActionsBlock(BaseModel):
    type: Literal["actions"]
    actions: list[ChatAction] = Field(default_factory=list)


class InsightCardHighlight(BaseModel):
    label: str = Field(min_length=1)
    value: str = Field(min_length=1)


class ChatInsightCardBlock(BaseModel):
    type: Literal["insight_card"]
    title: str = Field(min_length=1)
    summary: str = Field(min_length=1)
    scope: str | None = None
    highlights: list[InsightCardHighlight] = Field(default_factory=list)
    recommendations: list[str] = Field(default_factory=list)


class ChatKpiItem(BaseModel):
    label: str = Field(min_length=1)
    value: str = Field(min_length=1)
    trend: Literal["up", "down", "flat", "unknown"] | None = None
    context: str | None = None


class ChatKpiGroupBlock(BaseModel):
    type: Literal["kpi_group"]
    title: str | None = None
    items: list[ChatKpiItem] = Field(default_factory=list)


class ChatTableBlock(BaseModel):
    type: Literal["table"]
    title: str = Field(min_length=1)
    columns: list[str] = Field(default_factory=list)
    rows: list[list[str]] = Field(default_factory=list)
    empty_state: str | None = None


ChatBlock = Annotated[
    ChatTextBlock | ChatActionsBlock | ChatInsightCardBlock | ChatKpiGroupBlock | ChatTableBlock,
    Field(discriminator="type"),
]


RoutedAgent = Literal["general", "copilot", "clientes", "productos", "ventas", "cobros", "compras"]
RoutingSource = Literal["copilot_agent", "orchestrator", "read_fallback"]


class ChatResponse(BaseModel):
    request_id: str
    output_kind: Literal["chat_reply"] = Field(default=OUTPUT_KIND_CHAT_REPLY)
    chat_id: str = Field(serialization_alias="chat_id")
    reply: str
    tokens_used: int
    tool_calls: list[str]
    pending_confirmations: list[str]
    blocks: list[ChatBlock] = Field(default_factory=list)
    routed_agent: RoutedAgent = Field(
        ...,
        description="Agente o sub-agente seleccionado para este turno: general | copilot | clientes | productos | ventas | cobros | compras",
    )
    routed_mode: RoutedAgent = Field(
        ...,
        description="Alias legacy de `routed_agent` para compatibilidad hacia atrás.",
    )
    routing_source: RoutingSource = Field(
        ...,
        description="Origen efectivo del turno: copilot_agent | orchestrator | read_fallback",
    )
