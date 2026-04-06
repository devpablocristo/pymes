from __future__ import annotations

import asyncio
import copy
import json
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any, Literal

from fastapi import HTTPException, status
from pydantic import BaseModel, Field

from src.agents.audit import has_processed_request, record_agent_event
from src.agents.catalog import (
    CUSTOMERS_DOMAIN_AGENT_NAME,
    COLLECTIONS_DOMAIN_AGENT_NAME,
    PURCHASES_DOMAIN_AGENT_NAME,
    COPILOT_AGENT_NAME,
    PRODUCTS_DOMAIN_AGENT_NAME,
    PRODUCT_AGENT_NAME,
    ROUTING_SOURCE_ORCHESTRATOR,
    ROUTING_SOURCE_READ_FALLBACK,
    ROUTING_SOURCE_UI_HINT,
    SALES_DOMAIN_AGENT_NAME,
    is_known_routed_agent,
    normalize_routed_agent,
)
from src.agents.copilot_service import maybe_build_copilot_response
from src.agents.contracts import CommercialContractEnvelope
from src.agents.policy import build_external_sales_policy, build_internal_procurement_policy, build_internal_sales_policy
from src.agents.service_support import (
    CommercialChatResult,
    CommercialRunState,
    _build_external_sales_tools,
    _build_internal_sales_tools,
    _build_procurement_tools,
    _build_quote_preview,
    _load_internal_conversation,
    build_commercial_prompt,
    sanitize_message,
)
from src.agents.sub_agents import build_registry
from src.api.external_chat_support import get_external_conversation, history_to_messages
from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from runtime.chat.blocks import (
    build_insight_card_block,
    build_kpi_group_block,
    build_table_block,
    build_text_block,
)
from src.chat_blocks import (
    build_confirm_actions_block,
    build_route_selection_block,
)
from src.config import get_settings
from src.core.dossier import summarize_dossier_for_context
from runtime import LLMError, build_llm_client, validate_json_completion
from runtime.orchestrator import OrchestratorLimits, orchestrate
from runtime.services.multi_agent_orchestrator import run_routed_agent
from src.db.repository import AIRepository
from runtime.types import LLMProvider, Message
from runtime.logging import get_logger
from runtime.text import estimate_tokens
from src.tools import payments, scheduling

logger = get_logger(__name__)

_INTERNAL_ASSISTANT_CHANNEL = "internal_assistant"
_INTERNAL_SENSITIVE_TOOLS = {
    "create_quote",
    "create_sale",
    "create_procurement_request",
    "submit_procurement_request",
    "generate_payment_link",
    "send_payment_info",
}

_INTERNAL_GENERAL_SYSTEM_PROMPT = """\
Sos el asistente general de una plataforma de gestión para PyMEs.
Respondé saludos y preguntas generales de forma amable, clara y concisa.
Si el usuario pide una acción concreta del negocio, indicá que puede pedírtela directamente y el sistema la va a enrutar.
Respondé siempre en español."""

_AMBIGUOUS_ROUTE_OPTIONS: tuple[tuple[str, str], ...] = (
    (SALES_DOMAIN_AGENT_NAME, "Ventas"),
    (COLLECTIONS_DOMAIN_AGENT_NAME, "Cobros"),
    (PURCHASES_DOMAIN_AGENT_NAME, "Compras"),
    (CUSTOMERS_DOMAIN_AGENT_NAME, "Clientes"),
    (PRODUCTS_DOMAIN_AGENT_NAME, "Productos"),
)

_COLLECTIONS_DOMAIN_HINTS: tuple[str, ...] = (
    "cobro",
    "cobros",
    "pago",
    "pagos",
    "deuda",
    "deudas",
    "saldo",
    "saldos",
    "cuenta corriente",
    "cuentas corrientes",
)
_PRODUCT_DOMAIN_HINTS: tuple[str, ...] = (
    "producto",
    "productos",
    "stock",
    "inventario",
    "catálogo",
    "catalogo",
)
_ANALYTICS_HINTS: tuple[str, ...] = (
    "como viene",
    "cómo viene",
    "como van",
    "cómo van",
    "como va",
    "cómo va",
    "como estamos",
    "cómo estamos",
    "resumi",
    "resumí",
    "resumen",
    "analiza",
    "analizá",
    "analisis",
    "análisis",
    "explica",
    "explicá",
    "explicame",
    "explicame",
    "qué significa",
    "que significa",
    "por qué",
    "por que",
    "tendencia",
    "tendencias",
    "panorama",
    "recomend",
    "riesgo",
    "oportunidad",
    "oportunidades",
    "impacto",
)
_ANALYTICS_SYSTEM_PROMPT = """\
Sos un analista operacional para PyMEs.
Tu trabajo es interpretar evidencia determinística ya calculada por el backend.
No inventes datos. No uses markdown. No agregues texto fuera del JSON.
Respondé siempre en español.

Devolvé JSON con esta forma exacta:
{
  "reply": "respuesta breve para el usuario",
  "summary": "interpretación compacta y accionable",
  "scope": "alcance opcional",
  "highlights": [{"label": "texto", "value": "texto"}],
  "recommendations": ["texto"],
  "kpis": [{"label": "texto", "value": "texto", "trend": "up|down|flat|unknown|null", "context": "texto opcional"}],
  "table": {
    "title": "texto",
    "columns": ["columna 1", "columna 2"],
    "rows": [["valor", "valor"]],
    "empty_state": "texto opcional"
  }
}

Reglas:
- Basate solo en la evidencia provista.
- `reply` y `summary` deben ser concretos y entendibles.
- Máximo 3 highlights, 3 recomendaciones, 4 KPIs y 5 filas.
- Si falta evidencia para una conclusión, decilo explícitamente.
"""


@dataclass(frozen=True)
class _InternalDomainSnapshot:
    routed_agent: str
    scope: str
    summary: str
    tool_calls: list[str]
    blocks: list[dict[str, Any]]
    raw_result: dict[str, Any]


@dataclass(frozen=True)
class _InternalAnalysisCompletionSettings:
    llm_provider: str
    llm_model: str | None
    llm_api_key: str | None
    llm_base_url: str | None
    llm_timeout_ms: int
    llm_max_retries: int
    llm_max_output_tokens: int
    llm_max_calls_per_request: int
    llm_budget_tokens_per_request: int
    llm_rate_limit_rps: float


class _AnalysisHighlight(BaseModel):
    label: str = Field(min_length=1)
    value: str = Field(min_length=1)


class _AnalysisKPI(BaseModel):
    label: str = Field(min_length=1)
    value: str = Field(min_length=1)
    trend: Literal["up", "down", "flat", "unknown"] | None = None
    context: str | None = None


class _AnalysisTable(BaseModel):
    title: str = Field(min_length=1)
    columns: list[str] = Field(default_factory=list)
    rows: list[list[str]] = Field(default_factory=list)
    empty_state: str | None = None


class _AnalysisCompletion(BaseModel):
    reply: str = Field(min_length=1)
    summary: str = Field(min_length=1)
    scope: str | None = None
    highlights: list[_AnalysisHighlight] = Field(default_factory=list)
    recommendations: list[str] = Field(default_factory=list)
    kpis: list[_AnalysisKPI] = Field(default_factory=list)
    table: _AnalysisTable

def _default_internal_reply(routed_agent: str) -> str:
    if routed_agent == PRODUCT_AGENT_NAME:
        return "Hola. Puedo ayudarte con clientes, productos, ventas, cobros y compras. Decime qué necesitás."
    return "No pude generar una respuesta útil en este momento."


def _build_internal_pending_confirmation(tool_name: str) -> dict[str, Any]:
    return {
        "pending_confirmation": True,
        "required_action": tool_name,
        "message": f"Necesito confirmación explícita para ejecutar {tool_name}. Reenviá la solicitud incluyendo esa acción en confirmed_actions.",
    }


def _build_internal_general_limits() -> OrchestratorLimits:
    settings = get_settings()
    return OrchestratorLimits(
        max_tool_calls=0,
        total_timeout_seconds=max(30.0, float(settings.assistant_total_timeout_seconds)),
    )


def _build_internal_blocks(reply: str, pending_confirmations: list[str]) -> list[dict[str, Any]]:
    blocks: list[dict[str, Any]] = []
    if reply.strip():
        blocks.append(build_text_block(reply))
    if pending_confirmations:
        blocks.append(build_confirm_actions_block(pending_confirmations))
    return blocks


def _looks_like_smalltalk(message: str) -> bool:
    text = f" {message.strip().lower()} "
    hints = (
        " hola ",
        " buenas ",
        " buen dia ",
        " buen día ",
        " buenas tardes ",
        " buenas noches ",
        " gracias ",
        " ok ",
        " dale ",
        " perfecto ",
    )
    return any(hint in text for hint in hints)


def _looks_like_ambiguous_internal_query(message: str) -> bool:
    text = message.strip().lower()
    if not text or _looks_like_smalltalk(text):
        return False

    explicit_domain_hints = (
        "venta",
        "cobro",
        "compra",
        "cliente",
        "producto",
        "proveedor",
        "presupuesto",
        "pago",
        "stock",
        "inventario",
    )
    if any(hint in text for hint in explicit_domain_hints):
        return False

    ambiguity_hints = (
        "cuanto hay",
        "cuánto hay",
        "como viene",
        "cómo viene",
        "como va",
        "cómo va",
        "que paso",
        "qué pasó",
        "que pasó",
        "que hay",
        "qué hay",
        "estado",
        "resumen",
        "resumi",
        "resumí",
    )
    if any(hint in text for hint in ambiguity_hints):
        return True

    words = [item for item in text.replace("?", " ").split() if item]
    vague_words = {
        "cuanto",
        "cuánto",
        "como",
        "cómo",
        "hay",
        "va",
        "viene",
        "estado",
        "paso",
        "pasó",
        "resumen",
        "resumi",
        "resumí",
    }
    return len(words) <= 3 and any(word in vague_words for word in words)


def _looks_like_menu_request(message: str) -> bool:
    text = message.strip().lower()
    if not text:
        return False
    menu_hints = (
        "menu",
        "menú",
        "mostrame el menu",
        "mostrame el menú",
        "mostrar menu",
        "mostrar menú",
    )
    return any(hint in text for hint in menu_hints)


def _looks_like_broad_information_request(message: str) -> bool:
    text = message.strip().lower()
    hints = (
        "info",
        "informacion",
        "información",
        "detalle",
        "detalles",
        "disponible",
        "disponibles",
        "completo",
        "completa",
        "completos",
        "completas",
        "todo",
        "toda",
        "todos",
        "todas",
    )
    return any(hint in text for hint in hints)


def _looks_like_procurement_write_request(message: str) -> bool:
    text = message.strip().lower()
    action_hints = (
        "crear",
        "crea",
        "creá",
        "generar",
        "genera",
        "generá",
        "armar",
        "arma",
        "armá",
        "hacer",
        "hace",
        "hacé",
    )
    domain_hints = (
        "solicitud de compra",
        "solicitudes de compra",
        "solicitud interna",
        "solicitudes internas",
        "compra",
        "compras",
    )
    return any(hint in text for hint in action_hints) and any(hint in text for hint in domain_hints)


def _build_internal_route_clarification(user_message: str) -> tuple[str, list[dict[str, Any]]]:
    reply = (
        "Necesito un poco más de contexto para responder eso. "
        "Elegí una categoría y tomo esa selección sobre tu mensaje anterior."
    )
    return reply, [
        build_text_block(reply),
        build_route_selection_block(
            original_message=user_message,
            route_options=list(_AMBIGUOUS_ROUTE_OPTIONS),
            selection_behavior="route_and_resend",
        ),
    ]


def _build_internal_route_menu(user_message: str) -> tuple[str, list[dict[str, Any]]]:
    reply = "Elegí una categoría para continuar."
    return reply, [
        build_text_block(reply),
        build_route_selection_block(
            original_message=user_message,
            route_options=list(_AMBIGUOUS_ROUTE_OPTIONS),
            selection_behavior="prompt_for_query",
        ),
    ]


def _looks_like_customer_domain_request(message: str) -> bool:
    text = message.strip().lower()
    return "cliente" in text


def _looks_like_sales_domain_request(message: str) -> bool:
    text = message.strip().lower()
    sales_hints = ("venta", "ventas", "presupuesto", "presupuestos", "factura", "facturas")
    return any(hint in text for hint in sales_hints)


def _looks_like_collections_domain_request(message: str) -> bool:
    text = message.strip().lower()
    return any(hint in text for hint in _COLLECTIONS_DOMAIN_HINTS)


def _looks_like_procurement_domain_request(message: str) -> bool:
    text = message.strip().lower()
    procurement_hints = (
        "compra",
        "compras",
        "solicitud",
        "solicitudes",
        "proveedor",
        "proveedores",
        "abastecimiento",
        "insumo",
        "insumos",
    )
    return any(hint in text for hint in procurement_hints)


def _looks_like_internal_analysis_request(message: str, *, assume_domain_context: bool = False) -> bool:
    text = message.strip().lower()
    if not assume_domain_context and not any(
        (
            _looks_like_customer_domain_request(text),
            _looks_like_sales_domain_request(text),
            _looks_like_collections_domain_request(text),
            _looks_like_procurement_domain_request(text),
            _looks_like_product_domain_request(text),
        )
    ):
        return False
    return any(hint in text for hint in _ANALYTICS_HINTS)


def _looks_like_contextual_follow_up_request(message: str) -> bool:
    text = message.strip().lower()
    hints = (
        "dame",
        "decime",
        "decí",
        "cuales",
        "cuáles",
        "lista",
        "listar",
        "listame",
        "listáme",
        "mostra",
        "mostrar",
        "cuanto",
        "cuánto",
        "cuantos",
        "cuántos",
        "cuanta",
        "cuánta",
        "cuantas",
        "cuántas",
        "info",
        "informacion",
        "información",
        "detalle",
        "detalles",
        "resumen",
        "resumi",
        "resumí",
        "estado",
    )
    return any(hint in text for hint in hints) or _looks_like_broad_information_request(text)


def _looks_like_customer_summary_request(message: str, *, assume_domain_context: bool = False) -> bool:
    text = message.strip().lower()
    if not assume_domain_context and "cliente" not in text:
        return False
    hints = (
        "decime",
        "decí",
        "cuantos",
        "cuántos",
        "cuantas",
        "cuántas",
        "cuales",
        "cuáles",
        "tengo",
        "listar",
        "lista",
        "listame",
        "listáme",
        "mostra",
        "mostrar",
        "resumi",
        "resumí",
        "resumen",
    )
    return any(hint in text for hint in hints) or _looks_like_broad_information_request(text)


def _looks_like_procurement_summary_request(message: str, *, assume_domain_context: bool = False) -> bool:
    text = message.strip().lower()
    if not assume_domain_context and "solicitud" not in text and "compra" not in text:
        return False
    if _looks_like_procurement_write_request(text):
        return False
    hints = (
        "solicitud de compra",
        "solicitudes de compra",
        "solicitud interna",
        "solicitudes internas",
        "estado",
        "pendiente",
        "pendientes",
        "listar",
        "lista",
        "listame",
        "listáme",
        "cuales",
        "cuáles",
        "fueron",
        "mostra",
        "mostrar",
        "resumi",
        "resumí",
        "resumen",
        "cuantos",
        "cuántos",
        "cuantas",
        "cuántas",
        "tengo",
    )
    return any(hint in text for hint in hints) or _looks_like_broad_information_request(text)


def _looks_like_sales_summary_request(message: str, *, assume_domain_context: bool = False) -> bool:
    text = message.strip().lower()
    if not assume_domain_context and "venta" not in text:
        return False
    hints = (
        "cuantos",
        "cuántos",
        "cuantas",
        "cuántas",
        "hay",
        "tengo",
        "hice",
        "hicimos",
        "listar",
        "lista",
        "listame",
        "listáme",
        "cuales",
        "cuáles",
        "fueron",
        "mostra",
        "mostrar",
        "resumi",
        "resumí",
        "resumen",
        "estado",
        "hoy",
        "mes",
        "semana",
        "recientes",
    )
    return any(hint in text for hint in hints) or _looks_like_broad_information_request(text)


def _looks_like_collections_summary_request(message: str, *, assume_domain_context: bool = False) -> bool:
    text = message.strip().lower()
    if not assume_domain_context and not any(hint in text for hint in _COLLECTIONS_DOMAIN_HINTS):
        return False
    hints = (
        "cuantos",
        "cuántos",
        "cuantas",
        "cuántas",
        "hay",
        "tengo",
        "listar",
        "lista",
        "listame",
        "listáme",
        "cuales",
        "cuáles",
        "fueron",
        "mostra",
        "mostrar",
        "resumi",
        "resumí",
        "resumen",
        "estado",
        "pendiente",
        "pendientes",
        "abierto",
        "abiertos",
        "vencido",
        "vencidos",
    )
    return any(hint in text for hint in hints) or _looks_like_broad_information_request(text)


def _looks_like_product_domain_request(message: str) -> bool:
    text = message.strip().lower()
    return any(hint in text for hint in _PRODUCT_DOMAIN_HINTS)


def _looks_like_product_catalog_request(message: str, *, assume_domain_context: bool = False) -> bool:
    text = message.strip().lower()
    if not assume_domain_context and not any(hint in text for hint in _PRODUCT_DOMAIN_HINTS):
        return False
    hints = (
        "producto",
        "productos",
        "lista",
        "listar",
        "listame",
        "listáme",
        "cuales",
        "cuáles",
        "fueron",
        "disponible",
        "disponibles",
        "mostrar",
        "mostra",
        "catálogo",
        "catalogo",
        "stock",
    )
    return any(hint in text for hint in hints) or _looks_like_broad_information_request(text)


def _looks_like_product_low_stock_request(message: str) -> bool:
    text = message.strip().lower()
    hints = (
        "stock bajo",
        "faltante",
        "faltantes",
        "reponer",
        "reposición",
        "reposicion",
        "sin stock",
        "poco stock",
        "crítico",
        "critico",
    )
    return any(hint in text for hint in hints)


def _infer_internal_read_route(user_message: str) -> str | None:
    if _looks_like_product_low_stock_request(user_message):
        return PRODUCTS_DOMAIN_AGENT_NAME
    if _looks_like_product_catalog_request(user_message):
        return PRODUCTS_DOMAIN_AGENT_NAME
    if _looks_like_customer_summary_request(user_message):
        return CUSTOMERS_DOMAIN_AGENT_NAME
    if _looks_like_sales_summary_request(user_message):
        return SALES_DOMAIN_AGENT_NAME
    if _looks_like_collections_summary_request(user_message):
        return COLLECTIONS_DOMAIN_AGENT_NAME
    if _looks_like_procurement_summary_request(user_message):
        return PURCHASES_DOMAIN_AGENT_NAME
    return None


def _infer_internal_analysis_route(user_message: str) -> str | None:
    if not _looks_like_internal_analysis_request(user_message):
        return None
    if _looks_like_product_domain_request(user_message):
        return PRODUCTS_DOMAIN_AGENT_NAME
    if _looks_like_customer_domain_request(user_message):
        return CUSTOMERS_DOMAIN_AGENT_NAME
    if _looks_like_sales_domain_request(user_message):
        return SALES_DOMAIN_AGENT_NAME
    if _looks_like_collections_domain_request(user_message):
        return COLLECTIONS_DOMAIN_AGENT_NAME
    if _looks_like_procurement_domain_request(user_message):
        return PURCHASES_DOMAIN_AGENT_NAME
    return None


def _summarize_customer_search(result: dict[str, Any]) -> str | None:
    items = result.get("items", [])
    if not isinstance(items, list):
        return None
    total = result.get("total")
    if not isinstance(total, int):
        total = len(items)
    if total <= 0:
        return "Hoy no veo clientes cargados para esta organización."
    names = [str(item.get("name", "")).strip() for item in items[:5] if isinstance(item, dict) and str(item.get("name", "")).strip()]
    if not names:
        return f"Tenés {total} clientes registrados."
    suffix = "" if total <= len(names) else ", ..."
    return f"Tenés {total} clientes registrados. Algunos son: {', '.join(names)}{suffix}."


def _summarize_procurement_requests(result: dict[str, Any]) -> str | None:
    items = result.get("items", [])
    if not isinstance(items, list):
        return None
    total = result.get("total")
    if not isinstance(total, int):
        total = len(items)
    if total <= 0:
        return "No veo solicitudes de compra activas en este momento."

    status_labels = {
        "draft": "en borrador",
        "pending_approval": "pendientes de aprobación",
        "submitted": "enviadas",
        "approved": "aprobadas",
        "rejected": "rechazadas",
        "cancelled": "canceladas",
    }
    status_counts: dict[str, int] = {}
    titles: list[str] = []
    for raw_item in items:
        if not isinstance(raw_item, dict):
            continue
        status = str(raw_item.get("status", "")).strip().lower()
        if status:
            status_counts[status] = status_counts.get(status, 0) + 1
        title = str(raw_item.get("title") or raw_item.get("id") or "").strip()
        if title:
            titles.append(title)

    status_summary = ", ".join(
        f"{count} {status_labels.get(status, status)}"
        for status, count in status_counts.items()
        if count > 0
    )
    visible_titles = titles[:3]
    titles_summary = ""
    if visible_titles:
        suffix = "" if len(titles) <= len(visible_titles) else ", ..."
        if total == 1:
            titles_summary = f" Es: {', '.join(visible_titles)}{suffix}."
        else:
            titles_summary = f" Algunas son: {', '.join(visible_titles)}{suffix}."

    subject = "solicitud de compra activa" if total == 1 else "solicitudes de compra activas"

    if status_summary:
        return f"Tenés {total} {subject}: {status_summary}.{titles_summary}".strip()
    return f"Tenés {total} {subject}.{titles_summary}".strip()


def _summarize_recent_sales(result: dict[str, Any]) -> str | None:
    items_raw = result.get("items", [])
    if not isinstance(items_raw, list):
        return None

    total_raw = result.get("total")
    total = int(total_raw) if isinstance(total_raw, int) else len(items_raw)
    if total <= 0:
        return "No veo ventas registradas en el período consultado."

    visible_items = [item for item in items_raw if isinstance(item, dict)]
    amount = 0.0
    amount_found = False
    customer_names: list[str] = []
    for item in visible_items[:5]:
        item_total = item.get("total")
        if isinstance(item_total, int | float):
            amount += float(item_total)
            amount_found = True
        customer_name = str(item.get("customer_name", "")).strip()
        if customer_name:
            customer_names.append(customer_name)

    summary = f"Tenés {total} ventas registradas"
    if amount_found:
        summary += f" por ${amount:,.2f}"

    if customer_names:
        suffix = "" if total <= len(customer_names) else ", ..."
        summary += f". Algunas corresponden a: {', '.join(customer_names)}{suffix}."
        return summary

    return summary + "."


def _summarize_account_balances(result: dict[str, Any]) -> str | None:
    items_raw = result.get("items", [])
    if not isinstance(items_raw, list):
        return None

    visible_items = [item for item in items_raw if isinstance(item, dict)]
    total = len(visible_items)
    if total <= 0:
        return "No veo cuentas corrientes con saldo abierto en este momento."

    total_balance = 0.0
    balance_found = False
    names: list[str] = []
    for item in visible_items[:5]:
        balance = item.get("balance")
        if isinstance(balance, int | float):
            total_balance += float(balance)
            balance_found = True
        name = str(item.get("entity_name", "")).strip()
        if name:
            names.append(name)

    summary = f"Tenés {total} cuentas con saldo abierto"
    if balance_found:
        summary += f" por ${total_balance:,.2f}"
    if names:
        suffix = "" if total <= len(names) else ", ..."
        summary += f". Algunas son: {', '.join(names)}{suffix}."
        return summary
    return summary + "."


def _summarize_product_search(result: dict[str, Any]) -> str | None:
    items_raw = result.get("items", [])
    if not isinstance(items_raw, list):
        return None

    visible_items = [item for item in items_raw if isinstance(item, dict)]
    total_raw = result.get("total")
    total = int(total_raw) if isinstance(total_raw, int) else len(visible_items)
    if total <= 0:
        return "No encontré productos disponibles con ese criterio."

    names: list[str] = []
    for item in visible_items[:5]:
        name = str(item.get("name", "")).strip()
        if name:
            names.append(name)

    summary = f"Tenés {total} productos disponibles"
    if names:
        suffix = "" if total <= len(names) else ", ..."
        summary += f". Algunos son: {', '.join(names)}{suffix}."
        return summary
    return summary + "."


def _summarize_low_stock_products(result: dict[str, Any]) -> str | None:
    items_raw = result.get("items", [])
    if not isinstance(items_raw, list):
        return None

    visible_items = [item for item in items_raw if isinstance(item, dict)]
    total_raw = result.get("total")
    total = int(total_raw) if isinstance(total_raw, int) else len(visible_items)
    if total <= 0:
        return "No veo productos con stock bajo en este momento."

    names: list[str] = []
    for item in visible_items[:5]:
        name = str(item.get("product_name") or item.get("name") or "").strip()
        if name:
            names.append(name)

    summary = f"Tenés {total} productos con stock bajo"
    if names:
        suffix = "" if total <= len(names) else ", ..."
        summary += f". Algunos son: {', '.join(names)}{suffix}."
        return summary
    return summary + "."


def _format_currency(value: int | float | None) -> str:
    numeric = float(value or 0.0)
    return f"${numeric:,.2f}"


def _format_count(value: int | float | None) -> str:
    numeric = int(value or 0)
    return f"{numeric:,}"


def _status_label(status: str) -> str:
    mapping = {
        "draft": "En borrador",
        "pending_approval": "Pendiente de aprobación",
        "submitted": "Enviada",
        "approved": "Aprobada",
        "rejected": "Rechazada",
        "cancelled": "Cancelada",
    }
    return mapping.get(status.strip().lower(), status.strip() or "-")


def _build_customer_fallback_blocks(result: dict[str, Any]) -> list[dict[str, Any]]:
    items_raw = result.get("items", [])
    items = [item for item in items_raw if isinstance(item, dict)]
    total_raw = result.get("total")
    total = int(total_raw) if isinstance(total_raw, int) else len(items)
    summary = _summarize_customer_search(result) or "Hoy no veo clientes cargados para esta organización."

    return [
        build_insight_card_block(
            title="Clientes",
            summary=summary,
            scope="Consulta rápida",
            highlights=[
                {"label": "Clientes", "value": _format_count(total)},
                {"label": "Mostrados", "value": _format_count(len(items[:5]))},
            ],
        ),
        build_kpi_group_block(
            title="KPIs clave",
            items=[
                {"label": "Clientes totales", "value": _format_count(total)},
                {"label": "Resultados visibles", "value": _format_count(len(items[:5]))},
            ],
        ),
        build_table_block(
            title="Clientes",
            columns=["Cliente"],
            rows=[[str(item.get("name", "")).strip() or "-"] for item in items[:10]],
            empty_state="No hay clientes para mostrar.",
        ),
    ]


def _build_procurement_fallback_blocks(result: dict[str, Any]) -> list[dict[str, Any]]:
    items_raw = result.get("items", [])
    items = [item for item in items_raw if isinstance(item, dict)]
    total_raw = result.get("total")
    total = int(total_raw) if isinstance(total_raw, int) else len(items)
    summary = _summarize_procurement_requests(result) or "No veo solicitudes de compra activas en este momento."

    draft_count = 0
    pending_count = 0
    for item in items:
        status = str(item.get("status", "")).strip().lower()
        if status == "draft":
            draft_count += 1
        if status == "pending_approval":
            pending_count += 1

    return [
        build_insight_card_block(
            title="Compras",
            summary=summary,
            scope="Solicitudes internas",
            highlights=[
                {"label": "Solicitudes", "value": _format_count(total)},
                {"label": "Borradores", "value": _format_count(draft_count)},
                {"label": "Pendientes", "value": _format_count(pending_count)},
            ],
        ),
        build_kpi_group_block(
            title="KPIs clave",
            items=[
                {"label": "Solicitudes activas", "value": _format_count(total)},
                {"label": "En borrador", "value": _format_count(draft_count)},
                {"label": "Pendientes de aprobación", "value": _format_count(pending_count)},
            ],
        ),
        build_table_block(
            title="Solicitudes",
            columns=["Solicitud", "Estado"],
            rows=[
                [
                    str(item.get("title") or item.get("id") or "").strip() or "-",
                    _status_label(str(item.get("status", ""))),
                ]
                for item in items[:10]
            ],
            empty_state="No hay solicitudes activas para mostrar.",
        ),
    ]


def _build_sales_fallback_blocks(result: dict[str, Any]) -> list[dict[str, Any]]:
    items_raw = result.get("items", [])
    items = [item for item in items_raw if isinstance(item, dict)]
    total_raw = result.get("total")
    total = int(total_raw) if isinstance(total_raw, int) else len(items)
    summary = _summarize_recent_sales(result) or "No veo ventas registradas en el período consultado."

    total_amount = 0.0
    counted_amounts = 0
    for item in items:
        item_total = item.get("total")
        if isinstance(item_total, int | float):
            total_amount += float(item_total)
            counted_amounts += 1
    average_ticket = total_amount / counted_amounts if counted_amounts else 0.0

    return [
        build_insight_card_block(
            title="Ventas",
            summary=summary,
            scope="Consulta rápida",
            highlights=[
                {"label": "Operaciones", "value": _format_count(total)},
                {"label": "Facturado", "value": _format_currency(total_amount)},
                {"label": "Ticket promedio", "value": _format_currency(average_ticket)},
            ],
        ),
        build_kpi_group_block(
            title="KPIs clave",
            items=[
                {"label": "Ventas", "value": _format_count(total)},
                {"label": "Total facturado", "value": _format_currency(total_amount)},
                {"label": "Ticket promedio", "value": _format_currency(average_ticket)},
            ],
        ),
        build_table_block(
            title="Ventas recientes",
            columns=["Cliente", "Total"],
            rows=[
                [
                    str(item.get("customer_name", "")).strip() or str(item.get("id", "")).strip() or "-",
                    _format_currency(item.get("total") if isinstance(item.get("total"), int | float) else 0.0),
                ]
                for item in items[:10]
            ],
            empty_state="No hay ventas para mostrar.",
        ),
    ]


def _build_collections_fallback_blocks(result: dict[str, Any]) -> list[dict[str, Any]]:
    items_raw = result.get("items", [])
    items = [item for item in items_raw if isinstance(item, dict)]
    total = len(items)
    summary = _summarize_account_balances(result) or "No veo cuentas corrientes con saldo abierto en este momento."

    total_balance = 0.0
    counted_balances = 0
    for item in items:
        balance = item.get("balance")
        if isinstance(balance, int | float):
            total_balance += float(balance)
            counted_balances += 1
    avg_balance = total_balance / counted_balances if counted_balances else 0.0

    return [
        build_insight_card_block(
            title="Cobros",
            summary=summary,
            scope="Cuentas corrientes",
            highlights=[
                {"label": "Cuentas abiertas", "value": _format_count(total)},
                {"label": "Saldo total", "value": _format_currency(total_balance)},
                {"label": "Saldo promedio", "value": _format_currency(avg_balance)},
            ],
        ),
        build_kpi_group_block(
            title="KPIs clave",
            items=[
                {"label": "Cuentas con deuda", "value": _format_count(total)},
                {"label": "Saldo total", "value": _format_currency(total_balance)},
                {"label": "Saldo promedio", "value": _format_currency(avg_balance)},
            ],
        ),
        build_table_block(
            title="Cuentas corrientes",
            columns=["Cliente", "Saldo"],
            rows=[
                [
                    str(item.get("entity_name", "")).strip() or "-",
                    _format_currency(item.get("balance") if isinstance(item.get("balance"), int | float) else 0.0),
                ]
                for item in items[:10]
            ],
            empty_state="No hay cuentas con saldo abierto.",
        ),
    ]


def _build_product_catalog_fallback_blocks(result: dict[str, Any]) -> list[dict[str, Any]]:
    items_raw = result.get("items", [])
    items = [item for item in items_raw if isinstance(item, dict)]
    total_raw = result.get("total")
    total = int(total_raw) if isinstance(total_raw, int) else len(items)
    summary = _summarize_product_search(result) or "No encontré productos disponibles con ese criterio."

    tracked_stock = 0
    priced_items = 0
    for item in items:
        if bool(item.get("track_stock")):
            tracked_stock += 1
        if isinstance(item.get("price"), int | float):
            priced_items += 1

    return [
        build_insight_card_block(
            title="Productos",
            summary=summary,
            scope="Catálogo",
            highlights=[
                {"label": "Productos", "value": _format_count(total)},
                {"label": "Con stock", "value": _format_count(tracked_stock)},
                {"label": "Con precio", "value": _format_count(priced_items)},
            ],
        ),
        build_kpi_group_block(
            title="KPIs clave",
            items=[
                {"label": "Productos disponibles", "value": _format_count(total)},
                {"label": "Con seguimiento de stock", "value": _format_count(tracked_stock)},
                {"label": "Con precio definido", "value": _format_count(priced_items)},
            ],
        ),
        build_table_block(
            title="Productos",
            columns=["Producto", "SKU", "Precio"],
            rows=[
                [
                    str(item.get("name", "")).strip() or "-",
                    str(item.get("sku", "")).strip() or "-",
                    _format_currency(item.get("price") if isinstance(item.get("price"), int | float) else 0.0),
                ]
                for item in items[:10]
            ],
            empty_state="No hay productos para mostrar.",
        ),
    ]


def _build_product_low_stock_fallback_blocks(result: dict[str, Any]) -> list[dict[str, Any]]:
    items_raw = result.get("items", [])
    items = [item for item in items_raw if isinstance(item, dict)]
    total_raw = result.get("total")
    total = int(total_raw) if isinstance(total_raw, int) else len(items)
    summary = _summarize_low_stock_products(result) or "No veo productos con stock bajo en este momento."

    return [
        build_insight_card_block(
            title="Productos",
            summary=summary,
            scope="Stock bajo",
            highlights=[
                {"label": "Alertas", "value": _format_count(total)},
                {"label": "Productos visibles", "value": _format_count(len(items[:10]))},
            ],
        ),
        build_kpi_group_block(
            title="KPIs clave",
            items=[
                {"label": "Productos con stock bajo", "value": _format_count(total)},
                {"label": "Resultados visibles", "value": _format_count(len(items[:10]))},
            ],
        ),
        build_table_block(
            title="Alertas de stock",
            columns=["Producto", "Stock", "Mínimo"],
            rows=[
                [
                    str(item.get("product_name") or item.get("name") or "").strip() or "-",
                    _format_count(item.get("quantity") if isinstance(item.get("quantity"), int | float) else 0),
                    _format_count(item.get("min_quantity") if isinstance(item.get("min_quantity"), int | float) else 0),
                ]
                for item in items[:10]
            ],
            empty_state="No hay alertas de stock para mostrar.",
        ),
    ]


def _scope_label_for_agent(routed_agent: str) -> str:
    labels = {
        CUSTOMERS_DOMAIN_AGENT_NAME: "Clientes",
        PRODUCTS_DOMAIN_AGENT_NAME: "Productos",
        SALES_DOMAIN_AGENT_NAME: "Ventas",
        COLLECTIONS_DOMAIN_AGENT_NAME: "Cobros",
        PURCHASES_DOMAIN_AGENT_NAME: "Compras",
    }
    return labels.get(routed_agent, "Negocio")


async def _collect_internal_domain_snapshot(
    *,
    registry: Any,
    routed_agent: str,
    org_id: str,
    user_message: str,
    mode: Literal["read", "analysis"],
) -> _InternalDomainSnapshot | None:
    agent = registry.get(routed_agent)
    if agent is None:
        return None

    if routed_agent == CUSTOMERS_DOMAIN_AGENT_NAME:
        should_run = mode == "analysis" or _looks_like_customer_summary_request(user_message, assume_domain_context=True)
        if should_run:
            handler = agent.tool_handlers.get("search_customers")
            if handler is None:
                return None
            result = await handler(org_id=org_id, query="", limit=100)
            if isinstance(result, dict):
                return _InternalDomainSnapshot(
                    routed_agent=routed_agent,
                    scope="Clientes",
                    summary=_summarize_customer_search(result) or "Hoy no veo clientes cargados para esta organización.",
                    tool_calls=["search_customers"],
                    blocks=_build_customer_fallback_blocks(result),
                    raw_result=result,
                )

    if routed_agent == SALES_DOMAIN_AGENT_NAME:
        should_run = mode == "analysis" or _looks_like_sales_summary_request(user_message, assume_domain_context=True)
        if should_run:
            handler = agent.tool_handlers.get("get_recent_sales")
            if handler is None:
                return None
            result = await handler(org_id=org_id, limit=20)
            if isinstance(result, dict):
                return _InternalDomainSnapshot(
                    routed_agent=routed_agent,
                    scope="Ventas",
                    summary=_summarize_recent_sales(result) or "No veo ventas registradas en el período consultado.",
                    tool_calls=["get_recent_sales"],
                    blocks=_build_sales_fallback_blocks(result),
                    raw_result=result,
                )

    if routed_agent == PRODUCTS_DOMAIN_AGENT_NAME:
        if _looks_like_product_low_stock_request(user_message):
            handler = agent.tool_handlers.get("get_low_stock")
            if handler is None:
                return None
            result = await handler(org_id=org_id)
            if isinstance(result, dict):
                return _InternalDomainSnapshot(
                    routed_agent=routed_agent,
                    scope="Productos · Stock bajo",
                    summary=_summarize_low_stock_products(result) or "No veo productos con stock bajo en este momento.",
                    tool_calls=["get_low_stock"],
                    blocks=_build_product_low_stock_fallback_blocks(result),
                    raw_result=result,
                )

        should_run = mode == "analysis" or _looks_like_product_catalog_request(user_message, assume_domain_context=True)
        if should_run:
            handler = agent.tool_handlers.get("search_products")
            if handler is None:
                return None
            result = await handler(org_id=org_id, query="", limit=20)
            if isinstance(result, dict):
                return _InternalDomainSnapshot(
                    routed_agent=routed_agent,
                    scope="Productos · Catálogo",
                    summary=_summarize_product_search(result) or "No encontré productos disponibles con ese criterio.",
                    tool_calls=["search_products"],
                    blocks=_build_product_catalog_fallback_blocks(result),
                    raw_result=result,
                )

    if routed_agent == COLLECTIONS_DOMAIN_AGENT_NAME:
        should_run = mode == "analysis" or _looks_like_collections_summary_request(user_message, assume_domain_context=True)
        if should_run:
            handler = agent.tool_handlers.get("get_account_balances")
            if handler is None:
                return None
            result = await handler(org_id=org_id)
            if isinstance(result, dict):
                return _InternalDomainSnapshot(
                    routed_agent=routed_agent,
                    scope="Cobros",
                    summary=_summarize_account_balances(result) or "No veo cuentas corrientes con saldo abierto en este momento.",
                    tool_calls=["get_account_balances"],
                    blocks=_build_collections_fallback_blocks(result),
                    raw_result=result,
                )

    if routed_agent == PURCHASES_DOMAIN_AGENT_NAME:
        should_run = mode == "analysis" or _looks_like_procurement_summary_request(user_message, assume_domain_context=True)
        if should_run:
            handler = agent.tool_handlers.get("list_procurement_requests")
            if handler is None:
                return None
            result = await handler(org_id=org_id, limit=20, archived=False)
            if isinstance(result, dict):
                return _InternalDomainSnapshot(
                    routed_agent=routed_agent,
                    scope="Compras",
                    summary=_summarize_procurement_requests(result) or "No veo solicitudes de compra activas en este momento.",
                    tool_calls=["list_procurement_requests"],
                    blocks=_build_procurement_fallback_blocks(result),
                    raw_result=result,
                )

    return None


def _build_internal_analysis_settings() -> _InternalAnalysisCompletionSettings:
    settings = get_settings()
    provider = settings.llm_provider.strip().lower() or "stub"
    if provider == "echo":
        provider = "stub"

    model: str | None = None
    api_key: str | None = None
    base_url: str | None = None

    if provider == "gemini":
        model = settings.gemini_model
        api_key = settings.gemini_api_key
    elif provider == "ollama":
        model = settings.ollama_model
        base_url = settings.ollama_base_url

    return _InternalAnalysisCompletionSettings(
        llm_provider=provider,
        llm_model=model,
        llm_api_key=api_key,
        llm_base_url=base_url,
        llm_timeout_ms=int(min(float(settings.assistant_total_timeout_seconds), 30.0) * 1000),
        llm_max_retries=1,
        llm_max_output_tokens=700,
        llm_max_calls_per_request=1,
        llm_budget_tokens_per_request=4000,
        llm_rate_limit_rps=2.0,
    )


def _build_internal_analysis_user_prompt(*, snapshot: _InternalDomainSnapshot, user_message: str) -> str:
    payload = {
        "category": _scope_label_for_agent(snapshot.routed_agent),
        "scope": snapshot.scope,
        "question": user_message,
        "factual_summary": snapshot.summary,
        "evidence": snapshot.raw_result,
        "fallback_blocks": snapshot.blocks,
    }
    return json.dumps(payload, ensure_ascii=False)


def _build_internal_analysis_blocks(payload: _AnalysisCompletion) -> list[dict[str, Any]]:
    return [
        build_insight_card_block(
            title="Analisis",
            summary=payload.summary,
            scope=payload.scope,
            highlights=[item.model_dump(mode="json") for item in payload.highlights[:3]],
            recommendations=[item for item in payload.recommendations[:3] if item.strip()],
        ),
        build_kpi_group_block(
            title="KPIs clave",
            items=[item.model_dump(mode="json") for item in payload.kpis[:4]],
        ),
        build_table_block(
            title=payload.table.title,
            columns=payload.table.columns,
            rows=payload.table.rows[:5],
            empty_state=payload.table.empty_state,
        ),
    ]


async def _run_internal_analysis_fallback(
    *,
    registry: Any,
    routed_agent: str,
    org_id: str,
    user_message: str,
) -> tuple[str | None, list[str], list[dict[str, Any]]]:
    snapshot = await _collect_internal_domain_snapshot(
        registry=registry,
        routed_agent=routed_agent,
        org_id=org_id,
        user_message=user_message,
        mode="analysis",
    )
    if snapshot is None:
        return None, [], []

    try:
        client = build_llm_client(_build_internal_analysis_settings(), logger_name="pymes.internal_analysis")
        completion = await asyncio.to_thread(
            client.complete_json,
            system_prompt=_ANALYTICS_SYSTEM_PROMPT,
            user_prompt=_build_internal_analysis_user_prompt(snapshot=snapshot, user_message=user_message),
        )
        payload = validate_json_completion(completion.content, _AnalysisCompletion)
        return payload.reply, snapshot.tool_calls, _build_internal_analysis_blocks(payload)
    except (LLMError, ValueError) as exc:
        logger.warning(
            "internal_analysis_fallback_failed",
            routed_agent=routed_agent,
            error=str(exc),
        )
        return snapshot.summary, snapshot.tool_calls, snapshot.blocks


def _apply_internal_route_hint(*, routed_agent: str, user_message: str) -> str:
    if routed_agent != PRODUCT_AGENT_NAME:
        return routed_agent
    if _looks_like_product_domain_request(user_message):
        return PRODUCTS_DOMAIN_AGENT_NAME
    if _looks_like_sales_summary_request(user_message):
        return SALES_DOMAIN_AGENT_NAME
    if _looks_like_collections_summary_request(user_message):
        return COLLECTIONS_DOMAIN_AGENT_NAME
    if _looks_like_customer_summary_request(user_message):
        return CUSTOMERS_DOMAIN_AGENT_NAME
    if _looks_like_procurement_summary_request(user_message):
        return PURCHASES_DOMAIN_AGENT_NAME
    return routed_agent


def _normalize_explicit_route_hint(route_hint: str | None) -> str | None:
    if route_hint is None:
        return None
    normalized = str(route_hint).strip().lower()
    if not normalized or normalized == PRODUCT_AGENT_NAME:
        return None
    if not is_known_routed_agent(normalized):
        return None
    return normalized


def _override_explicit_route_hint(*, explicit_route_hint: str, user_message: str) -> str | None:
    explicit_matchers: tuple[tuple[str, Any], ...] = (
        (PRODUCTS_DOMAIN_AGENT_NAME, _looks_like_product_domain_request),
        (SALES_DOMAIN_AGENT_NAME, _looks_like_sales_domain_request),
        (COLLECTIONS_DOMAIN_AGENT_NAME, _looks_like_collections_domain_request),
        (CUSTOMERS_DOMAIN_AGENT_NAME, _looks_like_customer_domain_request),
        (PURCHASES_DOMAIN_AGENT_NAME, _looks_like_procurement_domain_request),
    )
    for candidate, matcher in explicit_matchers:
        if candidate == explicit_route_hint:
            continue
        if matcher(user_message):
            return candidate
    if _looks_like_ambiguous_internal_query(user_message):
        return explicit_route_hint
    if _looks_like_contextual_follow_up_request(user_message):
        return explicit_route_hint
    for candidate, matcher in explicit_matchers:
        if candidate == explicit_route_hint and matcher(user_message):
            return explicit_route_hint
    if _looks_like_internal_analysis_request(user_message, assume_domain_context=True):
        return explicit_route_hint
    return None


def _extract_pending_confirmation(chunk: Any) -> str | None:
    tool_result = getattr(chunk, "tool_result", None)
    if isinstance(tool_result, dict) and tool_result.get("pending_confirmation"):
        required_action = str(tool_result.get("required_action", "")).strip()
        return required_action or None

    tool_call = getattr(chunk, "tool_call", None)
    arguments = getattr(tool_call, "arguments", None)
    if isinstance(arguments, dict) and arguments.get("pending_confirmation"):
        required_action = str(arguments.get("required_action", "")).strip()
        return required_action or None
    return None


async def _run_internal_read_fallback(
    *,
    registry: Any,
    routed_agent: str,
    org_id: str,
    user_message: str,
) -> tuple[str | None, list[str], list[dict[str, Any]]]:
    snapshot = await _collect_internal_domain_snapshot(
        registry=registry,
        routed_agent=routed_agent,
        org_id=org_id,
        user_message=user_message,
        mode="read",
    )
    if snapshot is None:
        return None, [], []
    return snapshot.summary, snapshot.tool_calls, snapshot.blocks


def _wrap_internal_registry_handlers(
    *,
    registry: Any,
    repo: AIRepository,
    org_id: str,
    conversation_id: str,
    auth: AuthContext,
    confirmed_actions: set[str],
) -> None:
    for agent_name in registry.names():
        agent = registry.get(agent_name)
        if agent is None:
            continue

        wrapped_handlers = {}
        for tool_name, handler in agent.tool_handlers.items():
            wrapped_handlers[tool_name] = _wrap_internal_tool_handler(
                tool_name=tool_name,
                handler=handler,
                repo=repo,
                org_id=org_id,
                conversation_id=conversation_id,
                actor_id=auth.actor,
                confirmed_actions=confirmed_actions,
                agent_name=agent_name,
            )
        agent.tool_handlers = wrapped_handlers


def _wrap_internal_tool_handler(
    *,
    tool_name: str,
    handler: Any,
    repo: AIRepository,
    org_id: str,
    conversation_id: str,
    actor_id: str,
    confirmed_actions: set[str],
    agent_name: str,
):
    async def wrapped_handler(**kwargs: Any) -> dict[str, Any]:
        is_confirmed = tool_name.lower() in confirmed_actions
        if tool_name in _INTERNAL_SENSITIVE_TOOLS and not is_confirmed:
            result = _build_internal_pending_confirmation(tool_name)
            await record_agent_event(
                repo,
                org_id=org_id,
                conversation_id=conversation_id,
                agent_mode=agent_name,
                channel=_INTERNAL_ASSISTANT_CHANNEL,
                actor_id=actor_id,
                actor_type="internal_user",
                action=f"tool.{tool_name}",
                result="confirmation_required",
                confirmed=False,
                tool_name=tool_name,
                metadata={"required_action": tool_name},
            )
            return result

        result = await handler(**kwargs)
        outcome = "success"
        if isinstance(result, dict) and result.get("error"):
            outcome = "error"
        await record_agent_event(
            repo,
            org_id=org_id,
            conversation_id=conversation_id,
            agent_mode=agent_name,
            channel=_INTERNAL_ASSISTANT_CHANNEL,
            actor_id=actor_id,
            actor_type="internal_user",
            action=f"tool.{tool_name}",
            result=outcome,
            confirmed=is_confirmed,
            tool_name=tool_name,
        )
        return result

    return wrapped_handler


async def _run_direct_internal_agent(
    *,
    llm: LLMProvider,
    agent: Any,
    history: list[Message],
    org_id: str,
    user_message: str,
) -> tuple[str, list[str], list[str]]:
    assistant_parts: list[str] = []
    tool_calls: list[str] = []
    pending_confirmations: list[str] = []
    llm_messages: list[Message] = [
        Message(role="system", content=agent.system_prompt),
        *history,
        Message(role="user", content=user_message),
    ]

    async for chunk in orchestrate(
        llm=llm,
        messages=llm_messages,
        tools=agent.tools,
        tool_handlers=agent.tool_handlers,
        context={"org_id": org_id},
        limits=agent.limits or _build_internal_general_limits(),
    ):
        if chunk.type == "text" and chunk.text:
            assistant_parts.append(chunk.text)
            continue
        if chunk.type == "tool_call" and chunk.tool_call:
            tool_name = str(chunk.tool_call.name).strip()
            if tool_name:
                tool_calls.append(tool_name)
            continue
        if chunk.type == "tool_result":
            required_action = _extract_pending_confirmation(chunk)
            if required_action and required_action not in pending_confirmations:
                pending_confirmations.append(required_action)

    reply = "".join(assistant_parts).strip() or _default_internal_reply(agent.descriptor.name)
    return reply, tool_calls, pending_confirmations


async def run_internal_orchestrated_chat(
    *,
    repo: AIRepository,
    llm: LLMProvider,
    backend_client: BackendClient,
    org_id: str,
    message: str,
    conversation_id: str | None,
    auth: AuthContext,
    confirmed_actions: list[str] | None = None,
    route_hint: str | None = None,
    preferred_language: str | None = None,
) -> CommercialChatResult:
    """Punto de entrada canónico del assistant interno de Pymes."""
    sanitized_message = sanitize_message(message)
    conversation = await _load_internal_conversation(repo, auth, conversation_id, sanitized_message)
    confirmed = {item.strip().lower() for item in (confirmed_actions or []) if item.strip()}
    tokens_in = estimate_tokens(sanitized_message)

    assistant_parts: list[str] = []
    tool_calls: list[str] = []
    pending_confirmations: list[str] = []
    routed_agent = PRODUCT_AGENT_NAME
    routing_source = ROUTING_SOURCE_ORCHESTRATOR
    blocks: list[dict[str, Any]] = []
    explicit_route_hint = _normalize_explicit_route_hint(route_hint)
    registry = build_registry(backend_client, auth)
    _wrap_internal_registry_handlers(
        registry=registry,
        repo=repo,
        org_id=org_id,
        conversation_id=conversation.id,
        auth=auth,
        confirmed_actions=confirmed,
    )

    if explicit_route_hint != COPILOT_AGENT_NAME and _looks_like_menu_request(sanitized_message):
        reply, blocks = _build_internal_route_menu(sanitized_message)
        explicit_route_hint = None
        tool_calls = []
        pending_confirmations = []
        tokens_out = estimate_tokens(reply)
        now = datetime.now(UTC).isoformat()
        user_message = {"role": "user", "content": sanitized_message, "ts": now}
        assistant_message = {
            "role": "assistant",
            "content": reply,
            "ts": now,
            "tool_calls": [],
            "pending_confirmations": [],
            "blocks": blocks,
            "routed_agent": routed_agent,
            "routed_mode": routed_agent,
            "routing_source": routing_source,
        }
        await repo.append_messages(
            org_id=org_id,
            conversation_id=conversation.id,
            new_messages=[user_message, assistant_message],
            tool_calls_count=0,
            tokens_input=tokens_in,
            tokens_output=tokens_out,
        )
        await repo.track_usage(org_id, tokens_in, tokens_out)
        await record_agent_event(
            repo,
            org_id=org_id,
            conversation_id=conversation.id,
            agent_mode=routed_agent,
            channel=_INTERNAL_ASSISTANT_CHANNEL,
            actor_id=auth.actor,
            actor_type="internal_user",
            action="chat.completed",
            result="success",
            confirmed=True,
            metadata={
                "routing_source": routing_source,
                "tool_calls": [],
                "pending_confirmations": [],
            },
        )
        return CommercialChatResult(
            conversation_id=conversation.id,
            reply=reply,
            tokens_input=tokens_in,
            tokens_output=tokens_out,
            tool_calls=[],
            pending_confirmations=[],
            blocks=blocks,
            routed_agent=routed_agent,
            routing_source=routing_source,
        )

    if explicit_route_hint is None and _looks_like_ambiguous_internal_query(sanitized_message):
        reply, blocks = _build_internal_route_clarification(sanitized_message)
        tokens_out = estimate_tokens(reply)
        now = datetime.now(UTC).isoformat()
        user_message = {"role": "user", "content": sanitized_message, "ts": now}
        assistant_message = {
            "role": "assistant",
            "content": reply,
            "ts": now,
            "tool_calls": [],
            "pending_confirmations": [],
            "blocks": blocks,
            "routed_agent": routed_agent,
            "routed_mode": routed_agent,
            "routing_source": routing_source,
        }
        await repo.append_messages(
            org_id=org_id,
            conversation_id=conversation.id,
            new_messages=[user_message, assistant_message],
            tool_calls_count=0,
            tokens_input=tokens_in,
            tokens_output=tokens_out,
        )
        await repo.track_usage(org_id, tokens_in, tokens_out)
        await record_agent_event(
            repo,
            org_id=org_id,
            conversation_id=conversation.id,
            agent_mode=routed_agent,
            channel=_INTERNAL_ASSISTANT_CHANNEL,
            actor_id=auth.actor,
            actor_type="internal_user",
            action="chat.completed",
            result="success",
            confirmed=bool(confirmed),
            metadata={
                "routing_source": routing_source,
                "tool_calls": [],
                "pending_confirmations": [],
            },
        )
        return CommercialChatResult(
            conversation_id=conversation.id,
            reply=reply,
            tokens_input=tokens_in,
            tokens_output=tokens_out,
            tool_calls=[],
            pending_confirmations=[],
            blocks=blocks,
            routed_agent=routed_agent,
            routing_source=routing_source,
        )

    if explicit_route_hint not in {None, COPILOT_AGENT_NAME}:
        overridden_route_hint = _override_explicit_route_hint(
            explicit_route_hint=explicit_route_hint,
            user_message=sanitized_message,
        )
        if overridden_route_hint != explicit_route_hint:
            logger.info(
                "internal_assistant_route_hint_overridden",
                org_id=org_id,
                conversation_id=conversation.id,
                explicit_route_hint=explicit_route_hint,
                overridden_route_hint=overridden_route_hint,
            )
            explicit_route_hint = overridden_route_hint

    if explicit_route_hint not in {None, COPILOT_AGENT_NAME} and _looks_like_internal_analysis_request(
        sanitized_message,
        assume_domain_context=True,
    ):
        analysis_reply, analysis_tool_calls, analysis_blocks = await _run_internal_analysis_fallback(
            registry=registry,
            routed_agent=explicit_route_hint,
            org_id=org_id,
            user_message=sanitized_message,
        )
        if analysis_reply:
            routed_agent = explicit_route_hint
            routing_source = ROUTING_SOURCE_UI_HINT
            reply = analysis_reply
            tool_calls.extend(analysis_tool_calls)
            blocks = analysis_blocks or _build_internal_blocks(reply, [])
            tokens_out = estimate_tokens(reply)
            now = datetime.now(UTC).isoformat()
            user_message = {"role": "user", "content": sanitized_message, "ts": now}
            assistant_message = {
                "role": "assistant",
                "content": reply,
                "ts": now,
                "tool_calls": sorted(set(tool_calls)),
                "routed_agent": routed_agent,
                "routed_mode": routed_agent,
                "agent_mode": routed_agent,
                "channel": _INTERNAL_ASSISTANT_CHANNEL,
                "routing_source": routing_source,
                "pending_confirmations": [],
                "blocks": copy.deepcopy(blocks),
            }
            await repo.append_messages(
                org_id=org_id,
                conversation_id=conversation.id,
                new_messages=[user_message, assistant_message],
                tool_calls_count=len(tool_calls),
                tokens_input=tokens_in,
                tokens_output=tokens_out,
            )
            await repo.track_usage(org_id, tokens_in=tokens_in, tokens_out=tokens_out)
            await record_agent_event(
                repo,
                org_id=org_id,
                conversation_id=conversation.id,
                agent_mode=routed_agent,
                channel=_INTERNAL_ASSISTANT_CHANNEL,
                actor_id=auth.actor,
                actor_type="internal_user",
                action="chat.completed",
                result="success",
                confirmed=bool(confirmed),
                metadata={
                    "routing_source": routing_source,
                    "tool_calls": sorted(set(tool_calls)),
                    "pending_confirmations": [],
                },
            )
            return CommercialChatResult(
                conversation_id=conversation.id,
                reply=reply,
                tokens_input=tokens_in,
                tokens_output=tokens_out,
                tool_calls=sorted(set(tool_calls)),
                pending_confirmations=[],
                blocks=blocks,
                routed_agent=routed_agent,
                routing_source=routing_source,
            )

    if explicit_route_hint not in {None, COPILOT_AGENT_NAME}:
        fallback_reply, fallback_tool_calls, fallback_blocks = await _run_internal_read_fallback(
            registry=registry,
            routed_agent=explicit_route_hint,
            org_id=org_id,
            user_message=sanitized_message,
        )
        if fallback_reply:
            routed_agent = explicit_route_hint
            routing_source = ROUTING_SOURCE_UI_HINT
            reply = fallback_reply
            tool_calls.extend(fallback_tool_calls)
            blocks = fallback_blocks or _build_internal_blocks(reply, [])
            tokens_out = estimate_tokens(reply)
            now = datetime.now(UTC).isoformat()
            user_message = {"role": "user", "content": sanitized_message, "ts": now}
            assistant_message = {
                "role": "assistant",
                "content": reply,
                "ts": now,
                "tool_calls": sorted(set(tool_calls)),
                "routed_agent": routed_agent,
                "routed_mode": routed_agent,
                "agent_mode": routed_agent,
                "channel": _INTERNAL_ASSISTANT_CHANNEL,
                "routing_source": routing_source,
                "pending_confirmations": [],
                "blocks": copy.deepcopy(blocks),
            }
            await repo.append_messages(
                org_id=org_id,
                conversation_id=conversation.id,
                new_messages=[user_message, assistant_message],
                tool_calls_count=len(tool_calls),
                tokens_input=tokens_in,
                tokens_output=tokens_out,
            )
            await repo.track_usage(org_id, tokens_in=tokens_in, tokens_out=tokens_out)
            await record_agent_event(
                repo,
                org_id=org_id,
                conversation_id=conversation.id,
                agent_mode=routed_agent,
                channel=_INTERNAL_ASSISTANT_CHANNEL,
                actor_id=auth.actor,
                actor_type="internal_user",
                action="chat.completed",
                result="success",
                confirmed=bool(confirmed),
                metadata={
                    "routing_source": routing_source,
                    "tool_calls": sorted(set(tool_calls)),
                    "pending_confirmations": [],
                },
            )
            return CommercialChatResult(
                conversation_id=conversation.id,
                reply=reply,
                tokens_input=tokens_in,
                tokens_output=tokens_out,
                tool_calls=sorted(set(tool_calls)),
                pending_confirmations=[],
                blocks=blocks,
                routed_agent=routed_agent,
                routing_source=routing_source,
            )

    if explicit_route_hint is None:
        analysis_routed_agent = _infer_internal_analysis_route(sanitized_message)
        if analysis_routed_agent is not None:
            analysis_reply, analysis_tool_calls, analysis_blocks = await _run_internal_analysis_fallback(
                registry=registry,
                routed_agent=analysis_routed_agent,
                org_id=org_id,
                user_message=sanitized_message,
            )
            if analysis_reply:
                routed_agent = analysis_routed_agent
                routing_source = ROUTING_SOURCE_READ_FALLBACK
                reply = analysis_reply
                tool_calls.extend(analysis_tool_calls)
                blocks = analysis_blocks or _build_internal_blocks(reply, [])
                tokens_out = estimate_tokens(reply)
                now = datetime.now(UTC).isoformat()
                user_message = {"role": "user", "content": sanitized_message, "ts": now}
                assistant_message = {
                    "role": "assistant",
                    "content": reply,
                    "ts": now,
                    "tool_calls": sorted(set(tool_calls)),
                    "routed_agent": routed_agent,
                    "routed_mode": routed_agent,
                    "agent_mode": routed_agent,
                    "channel": _INTERNAL_ASSISTANT_CHANNEL,
                    "routing_source": routing_source,
                    "pending_confirmations": [],
                    "blocks": copy.deepcopy(blocks),
                }
                await repo.append_messages(
                    org_id=org_id,
                    conversation_id=conversation.id,
                    new_messages=[user_message, assistant_message],
                    tool_calls_count=len(tool_calls),
                    tokens_input=tokens_in,
                    tokens_output=tokens_out,
                )
                await repo.track_usage(org_id, tokens_in=tokens_in, tokens_out=tokens_out)
                await record_agent_event(
                    repo,
                    org_id=org_id,
                    conversation_id=conversation.id,
                    agent_mode=routed_agent,
                    channel=_INTERNAL_ASSISTANT_CHANNEL,
                    actor_id=auth.actor,
                    actor_type="internal_user",
                    action="chat.completed",
                    result="success",
                    confirmed=bool(confirmed),
                    metadata={
                        "routing_source": routing_source,
                        "tool_calls": sorted(set(tool_calls)),
                        "pending_confirmations": [],
                    },
                )
                return CommercialChatResult(
                    conversation_id=conversation.id,
                    reply=reply,
                    tokens_input=tokens_in,
                    tokens_output=tokens_out,
                    tool_calls=sorted(set(tool_calls)),
                    pending_confirmations=[],
                    blocks=blocks,
                    routed_agent=routed_agent,
                    routing_source=routing_source,
                )

    if explicit_route_hint is None:
        fastpath_routed_agent = _infer_internal_read_route(sanitized_message)
        if fastpath_routed_agent is not None:
            fallback_reply, fallback_tool_calls, fallback_blocks = await _run_internal_read_fallback(
                registry=registry,
                routed_agent=fastpath_routed_agent,
                org_id=org_id,
                user_message=sanitized_message,
            )
            if fallback_reply:
                routed_agent = fastpath_routed_agent
                routing_source = ROUTING_SOURCE_READ_FALLBACK
                reply = fallback_reply
                tool_calls.extend(fallback_tool_calls)
                blocks = fallback_blocks or _build_internal_blocks(reply, pending_confirmations)
                tokens_out = estimate_tokens(reply)
                now = datetime.now(UTC).isoformat()
                user_message = {"role": "user", "content": sanitized_message, "ts": now}
                assistant_message = {
                    "role": "assistant",
                    "content": reply,
                    "ts": now,
                    "tool_calls": sorted(set(tool_calls)),
                    "routed_agent": routed_agent,
                    "routed_mode": routed_agent,
                    "agent_mode": routed_agent,
                    "channel": _INTERNAL_ASSISTANT_CHANNEL,
                    "routing_source": routing_source,
                    "pending_confirmations": [],
                    "blocks": copy.deepcopy(blocks),
                }
                await repo.append_messages(
                    org_id=org_id,
                    conversation_id=conversation.id,
                    new_messages=[user_message, assistant_message],
                    tool_calls_count=len(tool_calls),
                    tokens_input=tokens_in,
                    tokens_output=tokens_out,
                )
                await repo.track_usage(org_id, tokens_in=tokens_in, tokens_out=tokens_out)
                await record_agent_event(
                    repo,
                    org_id=org_id,
                    conversation_id=conversation.id,
                    agent_mode=routed_agent,
                    channel=_INTERNAL_ASSISTANT_CHANNEL,
                    actor_id=auth.actor,
                    actor_type="internal_user",
                    action="chat.completed",
                    result="success",
                    confirmed=bool(confirmed),
                    metadata={
                        "routing_source": routing_source,
                        "tool_calls": sorted(set(tool_calls)),
                        "pending_confirmations": [],
                    },
                )
                return CommercialChatResult(
                    conversation_id=conversation.id,
                    reply=reply,
                    tokens_input=tokens_in,
                    tokens_output=tokens_out,
                    tool_calls=sorted(set(tool_calls)),
                    pending_confirmations=[],
                    blocks=blocks,
                    routed_agent=routed_agent,
                    routing_source=routing_source,
                )

    handled_by_explicit_copilot = False
    if explicit_route_hint == COPILOT_AGENT_NAME:
        copilot_response = await maybe_build_copilot_response(
            backend_client=backend_client,
            auth=auth,
            user_message=sanitized_message,
            preferred_language=preferred_language,
        )
        if copilot_response is not None:
            routed_agent = COPILOT_AGENT_NAME
            routing_source = ROUTING_SOURCE_UI_HINT
            reply = copilot_response.reply
            blocks = copy.deepcopy(copilot_response.blocks)
            logger.info(
                "internal_copilot_routed",
                org_id=org_id,
                conversation_id=conversation.id,
                routed_agent=routed_agent,
                route_hint=explicit_route_hint,
            )
            handled_by_explicit_copilot = True
        else:
            explicit_route_hint = None
    if handled_by_explicit_copilot:
        pass
    else:
        history = history_to_messages(list(conversation.messages))

        try:
            if explicit_route_hint is not None:
                forced_agent = registry.get(explicit_route_hint)
                if forced_agent is None:
                    explicit_route_hint = None
                else:
                    routed_agent = explicit_route_hint
                    routing_source = ROUTING_SOURCE_UI_HINT
                    logger.info(
                        "internal_assistant_route_hint_requested",
                        org_id=org_id,
                        conversation_id=conversation.id,
                        routed_agent=routed_agent,
                    )
                    try:
                        reply, tool_calls, pending_confirmations = await _run_direct_internal_agent(
                            llm=llm,
                            agent=forced_agent,
                            history=history,
                            org_id=org_id,
                            user_message=sanitized_message,
                        )
                    except Exception as exc:  # noqa: BLE001
                        logger.warning(
                            "internal_assistant_route_hint_direct_failed",
                            org_id=org_id,
                            conversation_id=conversation.id,
                            routed_agent=routed_agent,
                            error=str(exc),
                        )
                        reply = _default_internal_reply(routed_agent)
            if explicit_route_hint is None:
                async for chunk in run_routed_agent(
                    llm=llm,
                    registry=registry,
                    user_message=sanitized_message,
                    history=history,
                    context={"org_id": org_id},
                    general_system_prompt=_INTERNAL_GENERAL_SYSTEM_PROMPT,
                    general_limits=_build_internal_general_limits(),
                ):
                    if chunk.type == "route" and chunk.text:
                        routed_agent = normalize_routed_agent(chunk.text)
                        logger.info(
                            "internal_assistant_routed",
                            org_id=org_id,
                            conversation_id=conversation.id,
                            routed_agent=routed_agent,
                        )
                    elif chunk.type == "text" and chunk.text:
                        assistant_parts.append(chunk.text)
                    elif chunk.type == "tool_call" and chunk.tool_call:
                        tool_name = str(chunk.tool_call.name).strip()
                        if tool_name:
                            tool_calls.append(tool_name)
                    elif chunk.type == "tool_result":
                        required_action = _extract_pending_confirmation(chunk)
                        if required_action and required_action not in pending_confirmations:
                            pending_confirmations.append(required_action)
        except Exception as exc:  # noqa: BLE001
            logger.exception("internal_assistant_failed", org_id=org_id, conversation_id=conversation.id, error=str(exc))
            raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail="ai unavailable") from exc

        if explicit_route_hint is None:
            hinted_routed_agent = _apply_internal_route_hint(routed_agent=routed_agent, user_message=sanitized_message)
            if hinted_routed_agent != routed_agent:
                logger.info(
                    "internal_assistant_route_hint_applied",
                    org_id=org_id,
                    conversation_id=conversation.id,
                    routed_agent=routed_agent,
                    hinted_routed_agent=hinted_routed_agent,
                )
                routed_agent = hinted_routed_agent
            reply = "".join(assistant_parts).strip() or _default_internal_reply(routed_agent)
        blocks = _build_internal_blocks(reply, pending_confirmations)
        if not pending_confirmations and not tool_calls:
            fallback_reply, fallback_tool_calls, fallback_blocks = await _run_internal_read_fallback(
                registry=registry,
                routed_agent=routed_agent,
                org_id=org_id,
                user_message=sanitized_message,
            )
            if fallback_reply:
                reply = fallback_reply
                if routing_source != ROUTING_SOURCE_UI_HINT:
                    routing_source = ROUTING_SOURCE_READ_FALLBACK
                tool_calls.extend(fallback_tool_calls)
                if fallback_blocks:
                    blocks = fallback_blocks
                logger.info(
                    "internal_assistant_read_fallback_applied",
                    org_id=org_id,
                    conversation_id=conversation.id,
                    routed_agent=routed_agent,
                )
            if not fallback_blocks:
                blocks = _build_internal_blocks(reply, pending_confirmations)
        if (
            explicit_route_hint is None
            and routed_agent == PRODUCT_AGENT_NAME
            and not pending_confirmations
            and not tool_calls
            and _looks_like_menu_request(sanitized_message)
        ):
            reply, blocks = _build_internal_route_menu(sanitized_message)
        elif (
            explicit_route_hint is None
            and routed_agent == PRODUCT_AGENT_NAME
            and not pending_confirmations
            and not tool_calls
            and _looks_like_ambiguous_internal_query(sanitized_message)
        ):
            reply, blocks = _build_internal_route_clarification(sanitized_message)
    if pending_confirmations:
        reply = (
            "Necesito confirmación explícita para continuar con: "
            + ", ".join(pending_confirmations)
            + ". Reenviame la solicitud incluyendo esas acciones en confirmed_actions."
        )
        blocks = _build_internal_blocks(reply, pending_confirmations)
    tokens_out = estimate_tokens(reply)
    now = datetime.now(UTC).isoformat()
    user_message = {"role": "user", "content": sanitized_message, "ts": now}
    if confirmed:
        user_message["confirmed_actions"] = sorted(confirmed)
    assistant_message = {
        "role": "assistant",
        "content": reply,
        "ts": now,
        "tool_calls": sorted(set(tool_calls)),
        "routed_agent": routed_agent,
        "routed_mode": routed_agent,
        "agent_mode": routed_agent,
        "channel": _INTERNAL_ASSISTANT_CHANNEL,
        "routing_source": routing_source,
        "pending_confirmations": list(pending_confirmations),
        "blocks": copy.deepcopy(blocks),
    }

    await repo.append_messages(
        org_id=org_id,
        conversation_id=conversation.id,
        new_messages=[
            user_message,
            assistant_message,
        ],
        tool_calls_count=len(tool_calls),
        tokens_input=tokens_in,
        tokens_output=tokens_out,
    )
    await repo.track_usage(org_id, tokens_in=tokens_in, tokens_out=tokens_out)
    await record_agent_event(
        repo,
        org_id=org_id,
        conversation_id=conversation.id,
        agent_mode=routed_agent,
        channel=_INTERNAL_ASSISTANT_CHANNEL,
        actor_id=auth.actor,
        actor_type="internal_user",
        action="chat.completed",
        result="confirmation_required" if pending_confirmations else "success",
        confirmed=bool(confirmed),
        metadata={
            "routing_source": routing_source,
            "tool_calls": sorted(set(tool_calls)),
            "pending_confirmations": list(pending_confirmations),
        },
    )

    return CommercialChatResult(
        conversation_id=conversation.id,
        reply=reply,
        tokens_input=tokens_in,
        tokens_output=tokens_out,
        tool_calls=sorted(set(tool_calls)),
        pending_confirmations=list(pending_confirmations),
        blocks=blocks,
        routed_agent=routed_agent,
        routing_source=routing_source,
    )


async def run_commercial_chat(
    *,
    repo: AIRepository,
    llm: LLMProvider,
    backend_client: BackendClient,
    org_id: str,
    message: str,
    agent_mode: str,
    channel: str,
    conversation_id: str | None = None,
    auth: AuthContext | None = None,
    external_contact: str = "",
    confirmed_actions: list[str] | None = None,
    user_metadata: dict[str, Any] | None = None,
    assistant_metadata: dict[str, Any] | None = None,
) -> CommercialChatResult:
    sanitized_message = sanitize_message(message)
    confirmed = {item.strip().lower() for item in (confirmed_actions or []) if item.strip()}
    state = CommercialRunState()

    if agent_mode == "external_sales":
        policy = build_external_sales_policy(channel=channel)  # type: ignore[arg-type]
        conversation = await get_external_conversation(
            repo=repo,
            org_id=org_id,
            external_contact=external_contact,
            message=sanitized_message,
            conversation_id=conversation_id,
            reuse_latest=bool(external_contact),
        )
        actor_id = external_contact or "external"
        actor_type = "external_contact"
    else:
        if auth is None:
            raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="unauthorized")
        dossier = await repo.get_or_create_dossier(org_id)
        modules_active = dossier.get("modules_active", []) if isinstance(dossier, dict) else []
        if agent_mode == "internal_sales":
            policy = build_internal_sales_policy(auth, modules_active, channel=channel)  # type: ignore[arg-type]
        else:
            policy = build_internal_procurement_policy(auth, modules_active, channel=channel)  # type: ignore[arg-type]
        if not policy.allowed_tools:
            raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="commercial role has no enabled tools")
        conversation = await _load_internal_conversation(repo, auth, conversation_id, sanitized_message)
        actor_id = auth.actor
        actor_type = "internal_user"

    dossier = await repo.get_or_create_dossier(org_id)
    dossier_snapshot = copy.deepcopy(dossier)

    if agent_mode == "external_sales":
        declarations, handlers = await _build_external_sales_tools(
            client=backend_client,
            repo=repo,
            org_id=org_id,
            conversation_id=conversation.id,
            policy=policy,
            state=state,
            confirmed_actions=confirmed,
            external_contact=external_contact,
        )
    elif agent_mode == "internal_sales":
        declarations, handlers = await _build_internal_sales_tools(
            client=backend_client,
            auth=auth,
            repo=repo,
            org_id=org_id,
            conversation_id=conversation.id,
            policy=policy,
            state=state,
            confirmed_actions=confirmed,
        )
    else:
        declarations, handlers = await _build_procurement_tools(
            client=backend_client,
            auth=auth,
            repo=repo,
            org_id=org_id,
            conversation_id=conversation.id,
            policy=policy,
            state=state,
            confirmed_actions=confirmed,
        )

    history = history_to_messages(list(conversation.messages))
    llm_messages: list[Message] = [
        Message(role="system", content=build_commercial_prompt(agent_mode, channel, auth, dossier)),
        Message(role="system", content=f"Dossier: {summarize_dossier_for_context(dossier)}"),
        *history,
        Message(role="user", content=sanitized_message),
    ]

    assistant_parts: list[str] = []
    tool_calls: list[str] = []
    tokens_in = estimate_tokens("\n".join(m.content for m in llm_messages))
    limits = OrchestratorLimits(
        max_tool_calls=policy.max_tool_calls,
        tool_timeout_seconds=policy.tool_timeout_seconds,
        total_timeout_seconds=policy.total_timeout_seconds,
    )

    try:
        async for chunk in orchestrate(
            llm=llm,
            messages=llm_messages,
            tools=declarations,
            tool_handlers=handlers,
            context={"org_id": org_id},
            limits=limits,
        ):
            if chunk.type == "text" and chunk.text:
                assistant_parts.append(chunk.text)
            elif chunk.type == "tool_call" and chunk.tool_call:
                tool_name = str(chunk.tool_call.name).strip()
                if tool_name:
                    tool_calls.append(tool_name)
                    if tool_name not in handlers:
                        state.add_guardrail(f"La accion {tool_name} no esta habilitada para este agente.")
                        await record_agent_event(
                            repo,
                            org_id=org_id,
                            conversation_id=conversation.id,
                            agent_mode=agent_mode,
                            channel=channel,
                            actor_id=actor_id,
                            actor_type=actor_type,
                            action=f"tool.{tool_name}",
                            result="blocked",
                            confirmed=False,
                            tool_name=tool_name,
                            metadata={"reason": "tool_not_declared"},
                        )
    except Exception as exc:  # noqa: BLE001
        logger.exception("commercial_chat_failed", org_id=org_id, agent_mode=agent_mode, error=str(exc))
        raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail="ai unavailable") from exc

    assistant_text = "".join(assistant_parts).strip()
    if state.pending_confirmations:
        assistant_text = (
            "Necesito confirmacion explicita para continuar con: "
            + ", ".join(state.pending_confirmations)
            + ". Reenviame la solicitud incluyendo la accion confirmada."
        )
    elif state.guardrail_messages:
        assistant_text = state.guardrail_messages[0]
    elif not assistant_text:
        assistant_text = "No pude generar una respuesta util en este momento."

    tokens_out = estimate_tokens(assistant_text)
    now = datetime.now(UTC).isoformat()
    user_message = {
        "role": "user",
        "content": sanitized_message,
        "ts": now,
        "agent_mode": agent_mode,
        "channel": channel,
        "confirmed_actions": sorted(confirmed),
    }
    if user_metadata:
        user_message.update(user_metadata)
    assistant_message = {
        "role": "assistant",
        "content": assistant_text,
        "ts": now,
        "tool_calls": sorted(set(tool_calls)),
        "agent_mode": agent_mode,
        "channel": channel,
        "pending_confirmations": list(state.pending_confirmations),
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
    if dossier != dossier_snapshot:
        await repo.update_dossier(org_id, dossier)

    await record_agent_event(
        repo,
        org_id=org_id,
        conversation_id=conversation.id,
        agent_mode=agent_mode,
        channel=channel,
        actor_id=actor_id,
        actor_type=actor_type,
        action="chat.completed",
        result="success" if not state.guardrail_messages else "guardrail",
        confirmed=bool(confirmed),
        metadata={
            "tool_calls": sorted(set(tool_calls)),
            "pending_confirmations": list(state.pending_confirmations),
        },
    )

    return CommercialChatResult(
        conversation_id=conversation.id,
        reply=assistant_text,
        tokens_input=tokens_in,
        tokens_output=tokens_out,
        tool_calls=sorted(set(tool_calls)),
        pending_confirmations=list(state.pending_confirmations),
    )


async def process_contract(
    *,
    repo: AIRepository,
    backend_client: BackendClient,
    org_id: str,
    envelope: CommercialContractEnvelope,
    actor_id: str,
) -> dict[str, Any]:
    contract = envelope.contract
    if contract.org_id.strip() != org_id:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="org mismatch")
    if contract.channel not in {"api", "embedded", "web_public", "whatsapp"}:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="invalid channel")
    if await has_processed_request(repo, org_id, contract.request_id):
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail="request_id already processed")

    response_payload: dict[str, Any]
    status_label = "processed"

    if contract.intent == "availability_request":
        date_value = str(contract.metadata.get("date", "")).strip()
        duration = int(contract.metadata.get("duration", 60) or 60)
        response_payload = {
            "intent": "availability_response",
            "request_id": contract.request_id,
            "availability": await scheduling.check_availability(backend_client, org_id=org_id, date=date_value, duration=duration),
        }
    elif contract.intent == "request_quote":
        preview = await _build_quote_preview(
            backend_client,
            org_id,
            items=[item.model_dump() for item in contract.items],
            customer_name=envelope.contact_name or str(contract.metadata.get("customer_name", "")),
            notes=str(contract.metadata.get("notes", "")),
        )
        response_payload = {
            "intent": "quote_response",
            "request_id": contract.request_id,
            "quote_preview": preview,
        }
    elif contract.intent == "payment_request":
        quote_id = str(contract.metadata.get("quote_id", "")).strip()
        if not quote_id:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="quote_id is required for payment_request")
        response_payload = {
            "intent": "payment_request",
            "request_id": contract.request_id,
            "payment": await payments.get_public_quote_payment_link(backend_client, org_id=org_id, quote_id=quote_id),
        }
    elif contract.intent == "reservation_request":
        if "book_scheduling" not in envelope.confirmed_actions:
            status_label = "confirmation_required"
            response_payload = {
                "intent": "reservation_request",
                "request_id": contract.request_id,
                "status": "confirmation_required",
                "required_action": "book_scheduling",
                "message": "La reserva requiere confirmacion explicita antes de escribir en el backend.",
            }
        else:
            response_payload = {
                "intent": "reservation_request",
                "request_id": contract.request_id,
                "reservation": await scheduling.book_scheduling(
                    backend_client,
                    org_id=org_id,
                    customer_name=envelope.contact_name or str(contract.metadata.get("customer_name", "")),
                    customer_phone=envelope.contact_phone or str(contract.metadata.get("customer_phone", "")),
                    title=str(contract.metadata.get("title", "Reserva")),
                    start_at=str(contract.metadata.get("start_at", "")),
                    duration=int(contract.metadata.get("duration", 60) or 60),
                ),
            }
    else:
        status_label = "accepted_for_review"
        response_payload = {
            "intent": contract.intent,
            "request_id": contract.request_id,
            "status": status_label,
            "message": "La propuesta estructurada fue recibida y queda marcada para revision controlada.",
        }

    await record_agent_event(
        repo,
        org_id=org_id,
        conversation_id=None,
        agent_mode="external_sales",
        channel=contract.channel,
        actor_id=actor_id,
        actor_type="external_agent",
        action=f"contract.{contract.intent}",
        result=status_label,
        confirmed="book_scheduling" in envelope.confirmed_actions,
        request_id=contract.request_id,
        metadata={
            "counterparty_id": contract.counterparty_id,
            "intent": contract.intent,
            "signature_present": bool(contract.signature),
        },
    )
    return response_payload
