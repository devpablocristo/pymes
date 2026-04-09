from __future__ import annotations

from types import SimpleNamespace

import pytest

from src.agents.service import (
    _InternalDomainSnapshot,
    _build_internal_analysis_user_prompt,
    _summarize_procurement_requests,
    run_internal_orchestrated_chat,
)
from src.backend_client.auth import AuthContext


class StubRepo:
    def __init__(self) -> None:
        self.append_calls: list[dict[str, object]] = []
        self.track_calls: list[dict[str, int | str]] = []
        self.agent_events: list[dict[str, object]] = []

    async def append_messages(self, **kwargs):
        self.append_calls.append(kwargs)
        return SimpleNamespace(id=kwargs["conversation_id"])

    async def track_usage(self, org_id: str, tokens_in: int, tokens_out: int) -> None:
        self.track_calls.append({"org_id": org_id, "tokens_in": tokens_in, "tokens_out": tokens_out})

    async def record_agent_event(self, **kwargs) -> None:
        self.agent_events.append(kwargs)


class StubRegistry:
    def names(self) -> list[str]:
        return []

    def get(self, _name: str):
        return None


class CustomersAgent:
    def __init__(self) -> None:
        self.tool_handlers = {"search_customers": self.search_customers}

    async def search_customers(self, **_kwargs):
        return {
            "total": 2,
            "items": [
                {"name": "Acme"},
                {"name": "Beta"},
            ],
        }


class CustomersRegistry:
    def __init__(self) -> None:
        self.agent = CustomersAgent()

    def names(self) -> list[str]:
        return ["customers"]

    def get(self, name: str):
        if name == "customers":
            return self.agent
        return None


class ProcurementAgent:
    def __init__(self) -> None:
        self.descriptor = SimpleNamespace(name="purchases")
        self.tools = []
        self.tool_handlers = {"list_procurement_requests": self.list_procurement_requests}
        self.system_prompt = "Sos el agente de compras."
        self.limits = None

    async def list_procurement_requests(self, **_kwargs):
        return {
            "total": 3,
            "items": [
                {"id": "pr-1", "title": "Reposición filtros", "status": "draft"},
                {"id": "pr-2", "title": "Compra de insumos", "status": "pending_approval"},
                {"id": "pr-3", "title": "Herramientas taller", "status": "pending_approval"},
            ],
        }


class ProcurementRegistry:
    def __init__(self) -> None:
        self.agent = ProcurementAgent()

    def names(self) -> list[str]:
        return ["purchases"]

    def get(self, name: str):
        if name == "purchases":
            return self.agent
        return None


class SalesAgent:
    def __init__(self) -> None:
        self.descriptor = SimpleNamespace(name="sales")
        self.tools = []
        self.tool_handlers = {"get_recent_sales": self.get_recent_sales}
        self.system_prompt = "Sos el agente de ventas."
        self.limits = None

    async def get_recent_sales(self, **_kwargs):
        return {
            "total": 2,
            "items": [
                {"id": "sale-1", "customer_name": "Acme", "total": 1500.0},
                {"id": "sale-2", "customer_name": "Beta", "total": 2500.0},
            ],
        }


class SalesRegistry:
    def __init__(self) -> None:
        self.agent = SalesAgent()

    def names(self) -> list[str]:
        return ["sales"]

    def get(self, name: str):
        if name == "sales":
            return self.agent
        return None


class CollectionsAgent:
    def __init__(self) -> None:
        self.descriptor = SimpleNamespace(name="collections")
        self.tools = []
        self.tool_handlers = {"get_account_balances": self.get_account_balances}
        self.system_prompt = "Sos el agente de cobros."
        self.limits = None

    async def get_account_balances(self, **_kwargs):
        return {
            "items": [
                {"entity_name": "Acme", "balance": 1200.0},
                {"entity_name": "Beta", "balance": 800.0},
            ],
        }


class CollectionsRegistry:
    def __init__(self) -> None:
        self.agent = CollectionsAgent()

    def names(self) -> list[str]:
        return ["collections"]

    def get(self, name: str):
        if name == "collections":
            return self.agent
        return None


class ProductsAgent:
    def __init__(self) -> None:
        self.descriptor = SimpleNamespace(name="products")
        self.tools = []
        self.tool_handlers = {
            "search_products": self.search_products,
            "get_low_stock": self.get_low_stock,
        }
        self.system_prompt = "Sos el agente de productos."
        self.limits = None

    async def search_products(self, **_kwargs):
        return {
            "total": 3,
            "items": [
                {"id": "prod-1", "name": "Filtro de aceite"},
                {"id": "prod-2", "name": "Bujía NGK"},
                {"id": "prod-3", "name": "Pastillas de freno"},
            ],
        }

    async def get_low_stock(self, **_kwargs):
        return {
            "total": 2,
            "items": [
                {"product_name": "Bujía NGK"},
                {"product_name": "Pastillas de freno"},
            ],
        }


class ProductsRegistry:
    def __init__(self) -> None:
        self.agent = ProductsAgent()

    def names(self) -> list[str]:
        return ["products"]

    def get(self, name: str):
        if name == "products":
            return self.agent
        return None


def _registry_for_agent(name: str):
    registries = {
        "customers": CustomersRegistry,
        "sales": SalesRegistry,
        "collections": CollectionsRegistry,
        "purchases": ProcurementRegistry,
        "products": ProductsRegistry,
    }
    return registries[name]()


def test_summarize_procurement_requests_uses_singular_copy() -> None:
    result = _summarize_procurement_requests(
        {
            "total": 1,
            "items": [
                {
                    "id": "req-1",
                    "title": "Repuestos taller",
                    "status": "draft",
                }
            ],
        }
    )

    assert result == "Tenés 1 solicitud de compra activa: 1 en borrador. Es: Repuestos taller."


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("route_hint", "message", "expected_tool", "expected_text"),
    [
        ("customers", "dame la lista", "search_customers", "Tenés 2 clientes registrados"),
        ("customers", "quiero toda la info disponible", "search_customers", "Tenés 2 clientes registrados"),
        ("sales", "dame la lista", "get_recent_sales", "Tenés 2 ventas registradas"),
        ("sales", "dame toda la info disponible", "get_recent_sales", "Tenés 2 ventas registradas"),
        ("collections", "dame la lista", "get_account_balances", "Tenés 2 cuentas con saldo abierto"),
        ("collections", "quiero toda la info disponible", "get_account_balances", "Tenés 2 cuentas con saldo abierto"),
        ("purchases", "dame la lista", "list_procurement_requests", "Tenés 3 solicitudes de compra activas"),
        ("purchases", "cuáles fueron?", "list_procurement_requests", "Tenés 3 solicitudes de compra activas"),
        ("purchases", "dame toda la info disponible", "list_procurement_requests", "Tenés 3 solicitudes de compra activas"),
        ("products", "dame la lista", "search_products", "Tenés 3 productos disponibles"),
        ("products", "dame toda la info disponible", "search_products", "Tenés 3 productos disponibles"),
    ],
)
async def test_run_internal_orchestrated_chat_uses_selected_category_context_for_generic_list_request(
    monkeypatch, route_hint: str, message: str, expected_tool: str, expected_text: str
) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general cuando llega route_hint")

    async def fake_orchestrate(**_kwargs):
        if False:
            yield None

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.orchestrate", fake_orchestrate)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: _registry_for_agent(route_hint))

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message=message,
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint=route_hint,
    )

    assert result.routed_agent == route_hint
    assert result.routing_source == "ui_hint"
    assert result.tool_calls == [expected_tool]
    assert expected_text in result.reply
    assert result.blocks[0]["type"] == "insight_card"
    assert result.blocks[1]["type"] == "kpi_group"
    assert result.blocks[2]["type"] == "table"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("message", "expected_agent", "expected_tool", "expected_text"),
    [
        ("dame toda info disponible de compras", "purchases", "list_procurement_requests", "Tenés 3 solicitudes de compra activas"),
        ("dame toda la info disponible de ventas", "sales", "get_recent_sales", "Tenés 2 ventas registradas"),
    ],
)
async def test_run_internal_orchestrated_chat_routes_broad_info_requests_without_llm(
    monkeypatch, message: str, expected_agent: str, expected_tool: str, expected_text: str
) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: _registry_for_agent(expected_agent))

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message=message,
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == expected_agent
    assert result.routing_source == "read_fallback"
    assert result.tool_calls == [expected_tool]
    assert expected_text in result.reply
    assert result.blocks[0]["type"] == "insight_card"
    assert result.blocks[1]["type"] == "kpi_group"
    assert result.blocks[2]["type"] == "table"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_bypasses_llm_for_clear_read_request(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia invocar el router LLM para una lectura obvia")

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: CustomersRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="listame los clientes",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "customers"
    assert result.routing_source == "read_fallback"
    assert result.tool_calls == ["search_customers"]
    assert "Tenés 2 clientes registrados" in result.reply


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "message",
    [
        "creá una solicitud de compra para filtros y aceite",
        "creá una solicitud interna para filtros y aceite",
    ],
)
async def test_run_internal_orchestrated_chat_does_not_treat_procurement_create_request_as_read_summary(monkeypatch, message: str) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="purchases", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: ProcurementRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message=message,
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "purchases"
    assert result.routing_source == "orchestrator"
    assert result.tool_calls == []
    assert "Tenés 3 solicitudes de compra activas" not in result.reply


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_uses_structured_analysis_for_active_category(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general para un analisis con categoria activa")

    class StubAnalysisClient:
        def complete_json(self, *, system_prompt: str, user_prompt: str):
            assert "Sos un analista operacional para PyMEs." in system_prompt
            assert '"category": "Ventas"' in user_prompt
            return SimpleNamespace(
                content=(
                    '{"reply":"Ventas firmes este mes.","summary":"Las ventas se sostienen con buen volumen.",'
                    '"scope":"Ventas · este mes","highlights":[{"label":"Operacion","value":"Estable"}],'
                    '"recommendations":["Revisar ticket promedio semanal."],'
                    '"kpis":[{"label":"Ventas","value":"2","trend":"up","context":"sobre la muestra reciente"}],'
                    '"table":{"title":"Ventas recientes","columns":["Cliente","Total"],"rows":[["Acme","$1,500.00"]]}}'
                )
            )

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_llm_client", lambda *_args, **_kwargs: StubAnalysisClient())
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: SalesRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="dame un resumen",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint="sales",
    )

    assert result.routed_agent == "sales"
    assert result.routing_source == "ui_hint"
    assert result.tool_calls == ["get_recent_sales"]
    assert result.reply == "Ventas firmes este mes."
    assert result.blocks[0]["type"] == "insight_card"
    assert result.blocks[1]["type"] == "kpi_group"
    assert result.blocks[2]["type"] == "table"


def test_build_internal_analysis_user_prompt_compacts_evidence_and_omits_blocks() -> None:
    prompt = _build_internal_analysis_user_prompt(
        snapshot=_InternalDomainSnapshot(
            routed_agent="products",
            scope="Productos · Catalogo",
            summary="Hay productos cargados.",
            tool_calls=["search_products"],
            blocks=[
                {
                    "type": "actions",
                    "actions": [{"id": "clarify_route_sales", "label": "Ventas"}],
                }
            ],
            raw_result={
                "total": 12,
                "items": [
                    {
                        "id": f"prod-{idx}",
                        "name": f"Producto {idx}",
                        "sku": f"SKU-{idx}",
                        "price": 100 + idx,
                        "quantity": 5 + idx,
                        "description": "texto largo que no hace falta en el prompt",
                    }
                    for idx in range(12)
                ],
            },
        ),
        user_message="Resumime cómo viene el negocio esta semana.",
    )

    assert '"fallback_blocks"' not in prompt
    assert '"items_total": 12' in prompt
    assert '"description"' not in prompt
    assert prompt.count('"id": "prod-') == 8


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_allows_explicit_message_to_change_selected_category(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general cuando llega route_hint")

    async def fake_orchestrate(**_kwargs):
        if False:
            yield None

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.orchestrate", fake_orchestrate)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: SalesRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="cuántas ventas se hicieron este mes?",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint="purchases",
    )

    assert result.routed_agent == "sales"
    assert result.routing_source == "ui_hint"
    assert result.tool_calls == ["get_recent_sales"]
    assert "Tenés 2 ventas registradas" in result.reply


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_persists_routed_agent(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="customers", tool_call=None)
        yield SimpleNamespace(type="tool_call", text=None, tool_call=SimpleNamespace(name="search_customers"))
        yield SimpleNamespace(type="text", text="Encontré 3 clientes.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="listame los clientes",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "customers"
    assert result.tool_calls == ["search_customers"]
    assert result.blocks == [{"type": "text", "text": "Encontré 3 clientes."}]
    assert repo.append_calls
    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert assistant_message["routed_agent"] == "customers"
    assert assistant_message["blocks"] == [{"type": "text", "text": "Encontré 3 clientes."}]
    assert repo.track_calls == [{"org_id": "org-123", "tokens_in": result.tokens_input, "tokens_out": result.tokens_output}]
    assert repo.agent_events[-1]["action"] == "chat.completed"
    assert repo.agent_events[-1]["agent_mode"] == "customers"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_uses_general_fallback(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="hola",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "general"
    assert "clientes, productos, ventas, cobros y compras" in result.reply
    assert result.blocks == [{"type": "text", "text": result.reply}]


@pytest.mark.asyncio
@pytest.mark.parametrize("message", ["cuánto hay?", "dame el resumen", "resumen"])
async def test_run_internal_orchestrated_chat_offers_clarification_for_ambiguous_question(monkeypatch, message: str) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message=message,
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "general"
    assert "Necesito un poco más de contexto" in result.reply
    assert result.blocks[0] == {"type": "text", "text": result.reply}
    assert result.blocks[1]["type"] == "actions"
    assert [action["label"] for action in result.blocks[1]["actions"]] == [
        "Ventas",
        "Cobros",
        "Compras",
        "Clientes",
        "Productos",
        "Servicios",
    ]
    assert result.blocks[1]["actions"][0]["message"] == message
    assert result.blocks[1]["actions"][0]["route_hint"] == "sales"
    assert result.blocks[1]["actions"][0]["selection_behavior"] == "route_and_resend"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_offers_clarification_for_menu_request(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="mostrame el menú",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "general"
    assert result.reply == "Elegí una categoría para continuar."
    assert "Necesito un poco más de contexto" not in result.reply
    assert result.blocks[1]["type"] == "actions"
    assert [action["label"] for action in result.blocks[1]["actions"]] == [
        "Ventas",
        "Cobros",
        "Compras",
        "Clientes",
        "Productos",
        "Servicios",
    ]
    assert result.blocks[1]["actions"][0]["message"] == "mostrame el menú"
    assert result.blocks[1]["actions"][0]["selection_behavior"] == "prompt_for_query"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_offers_menu_even_with_active_route_hint(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia intentar enrutar cuando el usuario pide menu")

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="menu",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint="sales",
    )

    assert result.routed_agent == "general"
    assert result.reply == "Elegí una categoría para continuar."
    assert result.blocks[1]["type"] == "actions"
    assert result.blocks[1]["actions"][0]["selection_behavior"] == "prompt_for_query"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_normalizes_unknown_route_to_general(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="inventado", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="hola",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "general"
    assert result.blocks == [{"type": "text", "text": result.reply}]
    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert assistant_message["routed_agent"] == "general"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_routes_to_explicit_copilot_handoff(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("el product agent no deberia ejecutar routing LLM cuando llega un handoff explicito")

    async def fake_maybe_build_copilot_response(**_kwargs):
        return SimpleNamespace(
            reply="Ventas arriba 12% este mes.",
            blocks=[
                {"type": "insight_card", "title": "Ventas y cobranzas", "summary": "Ventas arriba 12% este mes.", "scope": "Ventas y cobranzas · este mes", "highlights": [], "recommendations": ["Mantener seguimiento semanal."]},
                {"type": "kpi_group", "title": "KPIs clave", "items": [{"label": "Ventas", "value": "$120,000.00", "trend": "up", "context": "+12.0% vs período anterior"}]},
            ],
        )

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.maybe_build_copilot_response", fake_maybe_build_copilot_response)

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="Quiero entender insight de ventas y cobranzas de este mes.",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint="copilot",
    )

    assert result.routed_agent == "copilot"
    assert result.reply == "Ventas arriba 12% este mes."
    assert result.blocks[0]["type"] == "insight_card"
    assert result.blocks[1]["type"] == "kpi_group"
    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert assistant_message["routed_agent"] == "copilot"
    assert assistant_message["agent_mode"] == "copilot"
    assert assistant_message["routing_source"] == "ui_hint"
    assert repo.agent_events[-1]["metadata"]["routing_source"] == "ui_hint"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_routes_operational_prompt_without_copilot(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="sales", tool_call=None)
        yield SimpleNamespace(type="text", text="Puedo ayudarte a registrar esa venta.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="registrá una venta",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "sales"
    assert result.reply == "Puedo ayudarte a registrar esa venta."
    assert result.blocks == [{"type": "text", "text": "Puedo ayudarte a registrar esa venta."}]


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_requires_confirmation_for_sensitive_tools(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="sales", tool_call=None)
        yield SimpleNamespace(type="tool_call", text=None, tool_call=SimpleNamespace(name="create_sale"))
        yield SimpleNamespace(
            type="tool_result",
            text=None,
            tool_call=SimpleNamespace(
                name="create_sale",
                arguments={
                    "pending_confirmation": True,
                    "required_action": "create_sale",
                },
            ),
        )

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="registrá una venta",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.pending_confirmations == ["create_sale"]
    assert "confirmed_actions" in result.reply
    assert result.blocks == [
        {"type": "text", "text": result.reply},
        {
            "type": "actions",
            "actions": [
                {
                    "id": "confirm_pending_actions",
                    "label": "Confirmar acciones",
                    "kind": "confirm_action",
                    "message": "Confirmo las acciones pendientes.",
                    "confirmed_actions": ["create_sale"],
                    "style": "primary",
                }
            ],
        },
    ]
    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert assistant_message["pending_confirmations"] == ["create_sale"]
    assert assistant_message["blocks"] == result.blocks
    assert repo.agent_events[-1]["result"] == "confirmation_required"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_marks_read_fallback_source(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="customers", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: CustomersRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="resumí mis clientes",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "customers"
    assert result.reply == "Tenés 2 clientes registrados. Algunos son: Acme, Beta."
    assert result.tool_calls == ["search_customers"]
    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert assistant_message["routing_source"] == "read_fallback"
    assert repo.agent_events[-1]["metadata"]["routing_source"] == "read_fallback"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_applies_customer_hint_when_llm_routes_general(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)
        yield SimpleNamespace(type="text", text="Hola. Puedo ayudarte con clientes, productos, ventas, cobros y compras. Decime qué necesitás.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: CustomersRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="decime cuales son mis clientes",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "customers"
    assert result.routing_source == "read_fallback"
    assert result.reply == "Tenés 2 clientes registrados. Algunos son: Acme, Beta."
    assert result.tool_calls == ["search_customers"]


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_applies_sales_hint_when_llm_routes_general(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)
        yield SimpleNamespace(type="text", text="Hola. Puedo ayudarte con clientes, productos, ventas, cobros y compras. Decime qué necesitás.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: SalesRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="cuántas ventas hay?",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "sales"
    assert result.routing_source == "read_fallback"
    assert result.tool_calls == ["get_recent_sales"]
    assert "Tenés 2 ventas registradas" in result.reply
    assert "$4,000.00" in result.reply


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_applies_collections_hint_when_llm_routes_general(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)
        yield SimpleNamespace(type="text", text="Hola. Puedo ayudarte con clientes, productos, ventas, cobros y compras. Decime qué necesitás.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: CollectionsRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="cuántos cobros hay?",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "collections"
    assert result.routing_source == "read_fallback"
    assert result.tool_calls == ["get_account_balances"]
    assert "Tenés 2 cuentas con saldo abierto" in result.reply
    assert "$2,000.00" in result.reply


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_applies_products_hint_when_llm_routes_general(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)
        yield SimpleNamespace(type="text", text="Hola. Puedo ayudarte con clientes, productos, ventas, cobros y compras. Decime qué necesitás.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: ProductsRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="listame productos disponibles",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "products"
    assert result.routing_source == "read_fallback"
    assert result.tool_calls == ["search_products"]
    assert "Tenés 3 productos disponibles" in result.reply
    assert "Filtro de aceite" in result.reply


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_resolves_explicit_products_hint_with_catalog_fallback(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general cuando llega route_hint")

    async def fake_orchestrate(**_kwargs):
        if False:
            yield None

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.orchestrate", fake_orchestrate)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: ProductsRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="dame la lista disponible",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint="products",
    )

    assert result.routed_agent == "products"
    assert result.routing_source == "ui_hint"
    assert result.tool_calls == ["search_products"]
    assert "Tenés 3 productos disponibles" in result.reply


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_applies_procurement_hint_when_llm_routes_general(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)
        yield SimpleNamespace(type="text", text="Hola. Puedo ayudarte con clientes, productos, ventas, cobros y compras. Decime qué necesitás.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: ProcurementRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="mostrame el estado de las solicitudes de compra pendientes",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.routed_agent == "purchases"
    assert result.routing_source == "read_fallback"
    assert result.tool_calls == ["list_procurement_requests"]
    assert "Tenés 3 solicitudes de compra activas" in result.reply
    assert "Reposición filtros" in result.reply


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_respects_explicit_route_hint(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general cuando llega route_hint")

    async def fake_orchestrate(**_kwargs):
        if False:
            yield None

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.orchestrate", fake_orchestrate)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: ProcurementRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="mostrame el estado de las solicitudes de compra pendientes",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint="purchases",
    )

    assert result.routed_agent == "purchases"
    assert result.routing_source == "ui_hint"
    assert result.tool_calls == ["list_procurement_requests"]
    assert "Tenés 3 solicitudes de compra activas" in result.reply
