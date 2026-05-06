from __future__ import annotations

from typing import Annotated, Literal

from pydantic import BaseModel, Field, model_validator

from runtime.chat import (
    ChatAction as BaseChatAction,
    ChatRequest as BaseChatRequest,
    ChatResponse as BaseChatResponse,
    ChatInsightCardBlock,
    ChatKpiGroupBlock,
    ChatKpiItem,
    ChatTableBlock,
    ChatTextBlock,
    InsightCardHighlight,
)
from src.localization import LanguageCode
from src.runtime_contracts import OUTPUT_KIND_CHAT_REPLY


RoutedAgent = Literal[
    "general",
    "insight_chat",
    "customers",
    "products",
    "services",
    "sales",
    "collections",
    "purchases",
    "employees",
]
ChatRouteHint = Literal[
    "general",
    "insight_chat",
    "customers",
    "products",
    "services",
    "sales",
    "collections",
    "purchases",
    "employees",
]
RoutingSource = Literal["copilot_agent", "orchestrator", "ui_hint"]
ChatHandoffSource = Literal["in_app_notification", "direct"]
InsightScope = Literal["sales_collections", "inventory_profit", "customers_retention"]
InsightPeriod = Literal["today", "week", "month"]
ChatAnalysisScope = Literal[
    "general",
    "sales_collections",
    "customers",
    "products",
    "services",
    "purchases",
    "scheduling",
    "operations",
    "employees",
]
ChatLLMStatus = Literal["ok", "unavailable", "error"]
ChatAnswerMode = Literal["facts_only", "analysis"]


class ChatHandoff(BaseModel):
    source: ChatHandoffSource = Field(
        ...,
        description="Origen estructurado del turno anclado. En Fase 1 se usa para validar contrato, sin cambiar el routing todavía.",
    )
    notification_id: str | None = Field(
        default=None,
        min_length=1,
        description="Identificador de la notificación origen cuando el turno viene desde notificaciones in-app.",
    )
    insight_scope: InsightScope | None = Field(
        default=None,
        description="Scope estable del insight al que se ancla el turno.",
    )
    period: InsightPeriod | None = Field(
        default=None,
        description="Período usado para calcular el insight que originó el turno.",
    )
    compare: bool | None = Field(
        default=None,
        description="Indica si el insight origen usó comparación contra período anterior.",
    )
    top_limit: int | None = Field(
        default=None,
        ge=1,
        le=10,
        description="Límite superior usado por el insight origen para rankings o listados resumidos.",
    )

    @model_validator(mode="after")
    def validate_notification_handoff(self) -> "ChatHandoff":
        if self.source == "in_app_notification" and not self.notification_id:
            raise ValueError("notification_id is required when source='in_app_notification'")
        return self


class ChatAction(BaseChatAction):
    route_hint: ChatRouteHint | None = None
    selection_behavior: Literal["route_and_resend", "prompt_for_query"] | None = None
    confirmed_actions: list[str] = Field(default_factory=list)


class ChatActionsBlock(BaseModel):
    """Bloque actions con ChatAction extendido (route_hint, etc.)."""

    type: Literal["actions"]
    actions: list[ChatAction] = Field(default_factory=list)


ChatBlock = Annotated[
    ChatTextBlock | ChatActionsBlock | ChatInsightCardBlock | ChatKpiGroupBlock | ChatTableBlock,
    Field(discriminator="type"),
]


class ChatRequest(BaseChatRequest):
    confirmed_actions: list[str] = Field(default_factory=list)
    handoff: ChatHandoff | None = Field(
        default=None,
        description=(
            "Contexto estructurado opcional para anclar el turno actual a una notificación o insight "
            "sin depender solo de `message` y `route_hint`. "
            "Si `handoff` no se envía, el request se enruta con `message` y `route_hint`."
        ),
    )
    route_hint: ChatRouteHint | None = Field(
        default=None,
        description=(
            "Hint opcional para forzar el carril del turno actual: general | insight_chat | customers | products | services | sales | collections | purchases."
        ),
    )
    preferred_language: LanguageCode | None = Field(
        default=None,
        description=(
            "Idioma preferido para contenido generado por AI. Hoy se normaliza sobre `es|en`; "
            "si falta o no se soporta, el backend cae a español."
        ),
    )


class ChatLLMMetadata(BaseModel):
    used: bool = Field(default=False, description="Indica si el turno fue generado por Gemini/Vertex.")
    provider: str | None = Field(default=None, description="Proveedor LLM efectivo.")
    model: str | None = Field(default=None, description="Modelo LLM efectivo.")
    status: ChatLLMStatus = Field(default="unavailable", description="Estado de la llamada al LLM.")


class ChatEvidencePeriod(BaseModel):
    label: str = Field(default="", description="Etiqueta humana del período usado para evidencia.")
    from_: str = Field(default="", alias="from", serialization_alias="from", description="Fecha inicial ISO.")
    to: str = Field(default="", description="Fecha final ISO.")


class ChatEvidenceMetadata(BaseModel):
    tools: list[str] = Field(default_factory=list, description="Herramientas read-only usadas para armar evidencia.")
    record_counts: dict[str, int] = Field(
        default_factory=dict,
        description="Cantidad de registros o agregados leídos por herramienta.",
    )
    period: ChatEvidencePeriod | None = Field(default=None, description="Período principal analizado.")


class ChatDeterministicMetadata(BaseModel):
    used: bool = Field(default=False, description="Indica si se armo una respuesta o resumen determinista.")
    summary: str = Field(default="", description="Resumen calculado desde evidencia read-only del negocio.")
    blocks: list[ChatBlock] = Field(
        default_factory=list,
        description="Bloques deterministas renderizables: KPI, tablas y acciones de detalle.",
    )


class ChatDashboardLink(BaseModel):
    label: str = Field(..., min_length=1, description="Texto visible del link de detalle.")
    url: str = Field(..., min_length=1, description="Ruta del panel o reporte sugerido.")
    kind: Literal["dashboard", "reports", "module"] = Field(default="dashboard", description="Tipo de destino.")


class ChatResponse(BaseChatResponse):
    output_kind: Literal["chat_reply"] = Field(default=OUTPUT_KIND_CHAT_REPLY)
    content_language: LanguageCode = Field(
        default="es",
        description="Idioma efectivo del contenido devuelto por el backend para este turno.",
    )
    pending_confirmations: list[str] = Field(default_factory=list)
    blocks: list[ChatBlock] = Field(default_factory=list)
    routed_agent: RoutedAgent = Field(
        ...,
        description=(
            "Agente o sub-agente seleccionado para este turno: general | insight_chat | customers | products | services | sales | collections | purchases."
        ),
    )
    routing_source: RoutingSource = Field(
        ...,
        description="Origen efectivo del turno: copilot_agent (insight_chat) | orchestrator | ui_hint",
    )
    analysis_scope: ChatAnalysisScope = Field(
        default="general",
        description="Alcance de análisis usado para seleccionar evidencia real del negocio.",
    )
    answer_mode: ChatAnswerMode = Field(
        default="analysis",
        description="Modo de respuesta: facts_only usa solo datos deterministas; analysis suma Gemini.",
    )
    deterministic: ChatDeterministicMetadata = Field(
        default_factory=ChatDeterministicMetadata,
        description="Metadata y bloques calculados por backend/reportes antes de Gemini.",
    )
    dashboard_links: list[ChatDashboardLink] = Field(
        default_factory=list,
        description="Links de detalle sugeridos para ver numeros completos en dashboard o reportes.",
    )
    llm: ChatLLMMetadata = Field(
        default_factory=ChatLLMMetadata,
        description="Metadata explícita sobre uso de Gemini/Vertex.",
    )
    evidence: ChatEvidenceMetadata = Field(
        default_factory=ChatEvidenceMetadata,
        description="Resumen de la evidencia read-only consultada antes de llamar al LLM.",
    )


# Re-exportar tipos base para que los consumidores existentes no rompan.
__all__ = [
    "ChatAction",
    "ChatActionsBlock",
    "ChatAnswerMode",
    "ChatAnalysisScope",
    "ChatBlock",
    "ChatDashboardLink",
    "ChatDeterministicMetadata",
    "ChatEvidenceMetadata",
    "ChatEvidencePeriod",
    "ChatHandoff",
    "ChatHandoffSource",
    "ChatInsightCardBlock",
    "ChatKpiGroupBlock",
    "ChatKpiItem",
    "ChatLLMMetadata",
    "ChatLLMStatus",
    "ChatRequest",
    "ChatResponse",
    "ChatRouteHint",
    "ChatTableBlock",
    "ChatTextBlock",
    "InsightPeriod",
    "InsightScope",
    "InsightCardHighlight",
    "RoutedAgent",
    "RoutingSource",
]
