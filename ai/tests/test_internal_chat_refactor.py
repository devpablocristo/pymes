from __future__ import annotations

from types import SimpleNamespace
from typing import Any

import pytest

from runtime.types import ChatChunk, Message
from src.backend_client.auth import AuthContext
from src.internal_chat.evidence import (
    MUTATING_INTERNAL_TOOLS,
    READ_ONLY_INTERNAL_TOOLS,
    EvidenceCall,
    EvidencePacket,
    EvidencePeriod,
)
from src.internal_chat.facts import build_fact_pack, classify_answer_mode
from src.internal_chat.routing import InternalRouteDecision, route_internal_message
from src.internal_chat.service import InternalChatError, run_internal_orchestrated_chat


class StubRepo:
    def __init__(self) -> None:
        self.created_conversation = SimpleNamespace(id="conv-1", mode="internal", user_id=None, messages=[], title="")
        self.append_calls: list[dict[str, Any]] = []
        self.track_calls: list[dict[str, Any]] = []
        self.agent_events: list[dict[str, Any]] = []
        self.dossier = {"business": {"name": "Bicimax"}}

    async def create_conversation(self, **kwargs):
        self.created_conversation = SimpleNamespace(
            id="conv-1",
            mode=kwargs.get("mode", "internal"),
            user_id=kwargs.get("user_id"),
            messages=[],
            title=kwargs.get("title", ""),
        )
        return self.created_conversation

    async def get_conversation(self, _org_id: str, _conversation_id: str):
        return self.created_conversation

    async def get_or_create_dossier(self, _org_id: str):
        return self.dossier

    async def append_messages(self, **kwargs):
        self.append_calls.append(kwargs)
        return self.created_conversation

    async def track_usage(self, org_id: str, tokens_in: int, tokens_out: int) -> None:
        self.track_calls.append({"org_id": org_id, "tokens_in": tokens_in, "tokens_out": tokens_out})

    async def record_agent_event(self, **kwargs) -> None:
        self.agent_events.append(kwargs)


class FakeBackend:
    def __init__(self) -> None:
        self.calls: list[dict[str, Any]] = []

    async def request(self, method: str, path: str, *, auth, params=None, **_kwargs):  # noqa: ANN001
        self.calls.append({"method": method, "path": path, "params": params or {}})
        if path == "/v1/reports/sales-summary":
            return {"from": params["from"], "to": params["to"], "data": {"total_sales": 100000, "count_sales": 4, "average_ticket": 25000}}
        if path == "/v1/reports/sales-by-customer":
            return {"items": [{"customer_name": "Taller Beta", "total": 70000, "count": 2}]}
        if path == "/v1/reports/sales-by-payment":
            return {"items": [{"payment_method": "cash", "total": 60000, "count": 2}]}
        if path == "/v1/sales":
            return {"items": [{"customer_name": "Taller Beta", "total": 40000}]}
        if path == "/v1/accounts/debtors":
            return {"items": [{"customer_name": "Taller Beta", "balance": 18000}]}
        if path == "/v1/accounts":
            return {"items": [{"customer_name": "Taller Beta", "balance": 18000}]}
        if path == "/v1/parties" and (params or {}).get("role") == "employee":
            return {
                "items": [
                    {
                        "display_name": "Valentina Acosta",
                        "person": {"first_name": "Valentina", "last_name": "Acosta"},
                        "position": "Soporte",
                        "roles": [{"role": "employee", "is_active": True}],
                        "email": "valentina@example.test",
                    },
                    {
                        "display_name": "Nicolas Herrera",
                        "person": {"first_name": "Nicolas", "last_name": "Herrera"},
                        "position": "Operaciones",
                        "roles": [{"role": "employee", "is_active": True}],
                        "email": "nicolas@example.test",
                    },
                ],
                "total": 2,
            }
        if path == "/v1/admin/tenant-settings":
            return {"business_name": "Bicimax"}
        return {"items": []}


class FakeGemini:
    model = "gemini-test"

    def __init__(self) -> None:
        self.messages: list[Message] = []

    async def chat(self, messages, **_kwargs):  # noqa: ANN001
        self.messages = list(messages)
        yield ChatChunk(
            type="text",
            text="Según los datos cargados, Taller Beta concentra ventas y saldo pendiente. Priorizá cobrar ese saldo y hacer seguimiento comercial.",
        )
        yield ChatChunk(type="done")


class FailingGemini:
    model = "gemini-test"

    async def chat(self, messages, **_kwargs):  # noqa: ANN001
        _ = messages
        raise RuntimeError("vertex auth failed")
        yield ChatChunk(type="done")


class UnexpectedGemini:
    model = "gemini-test"

    async def chat(self, messages, **_kwargs):  # noqa: ANN001
        _ = messages
        raise AssertionError("facts_only should not call Gemini")
        yield ChatChunk(type="done")


def _auth() -> AuthContext:
    return AuthContext(
        tenant_id="org-123",
        actor="user-1",
        role="admin",
        scopes=["admin:console:write"],
        mode="jwt",
    )


def test_routes_sales_customer_collections_question_to_sales_collections() -> None:
    decision = route_internal_message(
        "¿Qué clientes tuvieron ventas esta semana y cuál debería priorizar para cobrar o hacer seguimiento?",
        route_hint="sales",
    )

    assert decision.scope == "sales_collections"
    assert decision.routed_agent == "collections"


def test_classifies_factual_sales_question_as_facts_only() -> None:
    message = "¿Cuánto vendí hoy?"
    decision = route_internal_message(message, route_hint="sales")

    assert decision.scope == "sales_collections"
    assert classify_answer_mode(message, decision) == "facts_only"


def test_routes_unaccented_sales_question_to_facts_only_without_hint() -> None:
    message = "cuanto vendi hoy?"
    decision = route_internal_message(message)

    assert decision.scope == "sales_collections"
    assert decision.routed_agent == "sales"
    assert classify_answer_mode(message, decision) == "facts_only"


def test_routes_employee_question_over_sticky_sales_hint() -> None:
    message = "cuantos y cuales empleados tengo?"
    decision = route_internal_message(message, route_hint="sales")

    assert decision.scope == "employees"
    assert decision.routed_agent == "employees"
    assert classify_answer_mode(message, decision) == "facts_only"


def test_classifies_collection_priority_question_as_analysis() -> None:
    message = "¿Qué clientes priorizo para cobrar?"
    decision = route_internal_message(message)

    assert decision.scope == "sales_collections"
    assert classify_answer_mode(message, decision) == "analysis"


def test_classifies_debtors_question_as_facts_only() -> None:
    message = "¿Quién me debe?"
    decision = route_internal_message(message)

    assert decision.scope == "sales_collections"
    assert classify_answer_mode(message, decision) == "facts_only"


def test_internal_read_only_registry_excludes_mutating_tools() -> None:
    assert READ_ONLY_INTERNAL_TOOLS
    assert READ_ONLY_INTERNAL_TOOLS.isdisjoint(MUTATING_INTERNAL_TOOLS)
    assert "create_sale" not in READ_ONLY_INTERNAL_TOOLS
    assert "generate_payment_link" not in READ_ONLY_INTERNAL_TOOLS
    assert "submit_procurement_request" not in READ_ONLY_INTERNAL_TOOLS


def test_sales_collections_fact_pack_builds_deterministic_blocks_from_readonly_evidence() -> None:
    evidence = EvidencePacket(
        scope="sales_collections",
        period=EvidencePeriod(label="hoy", from_date="2026-05-05", to_date="2026-05-05"),
        calls=[
            EvidenceCall(
                "get_sales_summary",
                {"data": {"total_sales": 100000, "count_sales": 4, "average_ticket": 25000}},
            ),
            EvidenceCall("get_sales_by_customer", {"items": [{"customer_name": "Taller Beta", "total": 70000, "count": 2}]}),
            EvidenceCall("get_sales_by_payment", {"items": [{"payment_method": "cash", "total": 60000, "count": 2}]}),
            EvidenceCall("get_debtors", {"items": [{"customer_name": "Taller Beta", "balance": 18000}]}),
            EvidenceCall("get_account_balances", {"items": [{"customer_name": "Taller Beta", "balance": 18000}]}),
        ],
    )
    decision = InternalRouteDecision(scope="sales_collections", routed_agent="sales", reason="test")

    fact_pack = build_fact_pack(evidence=evidence, decision=decision)

    assert fact_pack.used is True
    assert "Ventas hoy: $100.000" in fact_pack.summary
    assert any(block["type"] == "kpi_group" for block in fact_pack.blocks)
    assert any(block["type"] == "table" and block["title"] == "Deudores" for block in fact_pack.blocks)
    assert {call.name for call in evidence.calls}.issubset(READ_ONLY_INTERNAL_TOOLS)


@pytest.mark.asyncio
async def test_internal_chat_reads_evidence_before_gemini_and_marks_llm_used() -> None:
    repo = StubRepo()
    backend = FakeBackend()
    llm = FakeGemini()

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=llm,  # type: ignore[arg-type]
        backend_client=backend,  # type: ignore[arg-type]
        org_id="org-123",
        message="¿Qué clientes tuvieron ventas esta semana y cuál debería priorizar para cobrar?",
        conversation_id=None,
        auth=_auth(),
        route_hint="sales",
    )

    assert result.analysis_scope == "sales_collections"
    assert result.answer_mode == "analysis"
    assert result.llm == {"used": True, "provider": "gemini", "model": "gemini-test", "status": "ok"}
    assert result.deterministic["used"] is True
    assert "get_sales_summary" in result.tool_calls
    assert "get_sales_by_customer" in result.tool_calls
    assert "get_debtors" in result.tool_calls
    assert "Taller Beta" in result.reply
    assert any(block["type"] == "kpi_group" for block in result.blocks)
    assert backend.calls[0]["path"] == "/v1/reports/sales-summary"
    assert "Evidencia real del backend" in llm.messages[-1].content
    assert "Resumen determinista" in llm.messages[-1].content
    assert repo.append_calls[0]["new_messages"][1]["llm"]["used"] is True


@pytest.mark.asyncio
async def test_internal_chat_facts_only_uses_fact_pack_without_gemini() -> None:
    repo = StubRepo()
    backend = FakeBackend()

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=UnexpectedGemini(),  # type: ignore[arg-type]
        backend_client=backend,  # type: ignore[arg-type]
        org_id="org-123",
        message="¿Cuánto vendí hoy?",
        conversation_id=None,
        auth=_auth(),
        route_hint="sales",
    )

    assert result.answer_mode == "facts_only"
    assert result.analysis_scope == "sales_collections"
    assert result.llm == {"used": False, "provider": None, "model": None, "status": "unavailable"}
    assert result.deterministic["used"] is True
    assert "Ventas hoy: $100.000" in result.reply
    assert "get_sales_summary" in result.tool_calls
    assert backend.calls[0]["path"] == "/v1/reports/sales-summary"
    assert any(block["type"] == "kpi_group" for block in result.blocks)
    assert any(block["type"] == "table" for block in result.blocks)
    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert assistant_message["answer_mode"] == "facts_only"
    assert assistant_message["llm"]["used"] is False
    assert assistant_message["deterministic"]["used"] is True


@pytest.mark.asyncio
async def test_internal_chat_employee_question_ignores_sticky_sales_hint() -> None:
    repo = StubRepo()
    backend = FakeBackend()

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=UnexpectedGemini(),  # type: ignore[arg-type]
        backend_client=backend,  # type: ignore[arg-type]
        org_id="org-123",
        message="cuantos y cuales empleados tengo?",
        conversation_id=None,
        auth=_auth(),
        route_hint="sales",
    )

    assert result.answer_mode == "facts_only"
    assert result.analysis_scope == "employees"
    assert result.routed_agent == "employees"
    assert result.llm["used"] is False
    assert result.tool_calls == ["search_employees"]
    assert "Empleados: 2 registros leidos" in result.reply
    assert backend.calls[0]["path"] == "/v1/parties"
    assert backend.calls[0]["params"]["role"] == "employee"
    assert any(block["type"] == "table" and block["title"] == "Listado de empleados" for block in result.blocks)


@pytest.mark.asyncio
async def test_internal_chat_returns_visible_error_when_gemini_fails() -> None:
    repo = StubRepo()

    with pytest.raises(InternalChatError) as exc_info:
        await run_internal_orchestrated_chat(
            repo=repo,  # type: ignore[arg-type]
            llm=FailingGemini(),  # type: ignore[arg-type]
            backend_client=FakeBackend(),  # type: ignore[arg-type]
            org_id="org-123",
            message="¿Qué clientes tuvieron ventas esta semana y cuál debería priorizar para cobrar?",
            conversation_id=None,
            auth=_auth(),
            route_hint="sales",
        )

    assert exc_info.value.status_code == 503
    assert exc_info.value.code == "gemini_unavailable"
    assert repo.append_calls == []
    assert repo.track_calls == []
