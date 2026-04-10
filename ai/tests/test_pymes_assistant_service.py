from __future__ import annotations

from datetime import UTC, datetime
from types import SimpleNamespace

import pytest

from src.agents.insight_chat_service import InternalInsightEvidence, InsightEvidencePeriod
from src.agents.service import (
    _InternalDomainSnapshot,
    _build_internal_general_system_prompt,
    _build_internal_analysis_user_prompt,
    _default_internal_reply,
    _hydrate_dossier_from_backend_settings,
    _summarize_procurement_requests,
    run_internal_orchestrated_chat,
)
from src.backend_client.auth import AuthContext
from src.core.dossier import build_operating_context_for_prompt, capture_turn_memory, consolidate_memory


class StubRepo:
    def __init__(self) -> None:
        self.append_calls: list[dict[str, object]] = []
        self.track_calls: list[dict[str, int | str]] = []
        self.agent_events: list[dict[str, object]] = []
        self.dossier: dict[str, object] = {
            "business": {"name": "", "vertical": "", "profile": "", "type": "", "description": "", "currency": "ARS", "tax_rate": 21.0},
            "onboarding": {"status": "pending", "current_step": "welcome", "steps_completed": [], "steps_skipped": []},
            "modules_active": [],
            "modules_inactive": [],
            "preferences": {},
            "team": [],
            "learned_context": [],
            "memory": {
                "business_facts": [],
                "stable_business_facts": [],
                "open_loops": [],
                "decisions": [],
                "recent_threads": [],
                "user_profiles": {},
            },
            "kpis_baseline": {},
        }

    async def append_messages(self, **kwargs):
        self.append_calls.append(kwargs)
        return SimpleNamespace(id=kwargs["conversation_id"])

    async def get_or_create_dossier(self, _org_id: str):
        return self.dossier

    async def update_dossier(self, _org_id: str, patch):
        self.dossier = patch
        return self.dossier

    async def track_usage(self, org_id: str, tokens_in: int, tokens_out: int) -> None:
        self.track_calls.append({"org_id": org_id, "tokens_in": tokens_in, "tokens_out": tokens_out})

    async def record_agent_event(self, **kwargs) -> None:
        self.agent_events.append(kwargs)


class StubRegistry:
    def names(self) -> list[str]:
        return []

    def get(self, _name: str):
        return None


def _async_return(value):
    async def _inner(*_args, **_kwargs):
        return value

    return _inner


class CapturingLogger:
    def __init__(self) -> None:
        self.info_calls: list[tuple[str, dict[str, object]]] = []
        self.warning_calls: list[tuple[str, dict[str, object]]] = []

    def info(self, event: str, **kwargs) -> None:
        self.info_calls.append((event, kwargs))

    def warning(self, event: str, **kwargs) -> None:
        self.warning_calls.append((event, kwargs))

    def exception(self, event: str, **kwargs) -> None:
        self.warning_calls.append((event, kwargs))


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


def test_default_internal_reply_presents_business_advisor_role() -> None:
    reply = _default_internal_reply("general")
    assert "asesor del negocio" in reply.lower()
    assert "ventas" in reply.lower()


def test_build_operating_context_for_prompt_infers_vertical_and_pymes_playbook() -> None:
    dossier = {
        "business": {"name": "Taller Norte", "profile": "servicio_profesional"},
        "modules_active": ["customers", "scheduling", "products", "sales"],
    }

    context = build_operating_context_for_prompt(dossier)

    assert "Vertical principal: servicios profesionales." in context
    assert "Cómo funciona Pymes:" in context
    assert "Antes que listar registros" not in context
    assert "priorizá análisis, riesgos, prioridades y acciones" in context


def test_internal_general_system_prompt_embeds_operating_context() -> None:
    dossier = {
        "business": {"name": "Bici Centro", "vertical": "bike_shop"},
        "modules_active": ["products", "inventory", "sales", "scheduling"],
    }

    prompt = _build_internal_general_system_prompt(dossier)

    assert "asesor del negocio" in prompt.lower()
    assert "Vertical principal: bicicletería." in prompt
    assert "Cómo pensar esta vertical:" in prompt


def test_internal_analysis_user_prompt_includes_operating_context() -> None:
    snapshot = _InternalDomainSnapshot(
        routed_agent="sales",
        scope="Ventas",
        summary="Tenés 2 ventas registradas.",
        tool_calls=["get_recent_sales"],
        blocks=[],
        raw_result={"total": 2, "items": [{"customer_name": "Acme", "total": 1500.0}]},
    )

    payload = _build_internal_analysis_user_prompt(
        snapshot=snapshot,
        user_message="Resumime cómo viene el negocio",
        dossier={
            "business": {"name": "Resto Demo", "vertical": "restaurants"},
            "modules_active": ["products", "sales"],
        },
    )

    assert '"operating_context"' in payload
    assert "gastronomía" in payload


def test_capture_turn_memory_stores_business_user_and_open_loops() -> None:
    dossier = {
        "business": {"name": "Demo"},
        "modules_active": ["sales"],
    }

    capture_turn_memory(
        dossier,
        user_id="user-1",
        user_message="Recordá que nuestro negocio vende al por mayor y respondeme breve.",
        assistant_reply="Entendido. Voy a priorizar ese contexto.",
        routed_agent="general",
        tool_calls=[],
        pending_confirmations=["create_sale"],
        confirmed_actions=set(),
    )

    context = build_operating_context_for_prompt(dossier, "user-1")

    assert "Memoria del negocio:" in context
    assert "nuestro negocio vende al por mayor" in context.lower()
    assert "Memoria del usuario interno:" in context
    assert "respuestas breves" in context.lower()
    assert "Temas abiertos recientes:" in context
    assert "create_sale" in context


def test_consolidate_memory_curates_stable_facts_preferences_and_decisions() -> None:
    dossier = {
        "business": {"name": "Demo"},
        "modules_active": ["sales"],
        "memory": {
            "business_facts": [
                {"text": "Vendemos al por mayor", "kind": "explicit_business_fact", "times_seen": 2},
                {"text": "Vendemos al por mayor", "kind": "explicit_business_fact", "times_seen": 1},
                {"text": "Tenemos atención personalizada", "kind": "explicit_business_fact", "times_seen": 1},
            ],
            "decisions": [
                {"action": "create_sale", "summary": "Confirmar venta urgente", "agent": "sales"},
                {"action": "create_sale", "summary": "Confirmar venta urgente", "agent": "sales"},
            ],
            "open_loops": [],
            "recent_threads": [],
            "user_profiles": {
                "user-1": {
                    "preferences": [
                        "El usuario prefiere respuestas breves.",
                        "El usuario prefiere respuestas breves.",
                        "El usuario prefiere respuestas con tablas cuando agregan valor.",
                    ],
                    "recent_topics": ["sales: revisar ventas", "sales: revisar ventas"],
                }
            },
        },
    }

    consolidate_memory(dossier)
    context = build_operating_context_for_prompt(dossier, "user-1")

    assert "Vendemos al por mayor" in dossier["memory"]["stable_business_facts"]
    assert len(dossier["memory"]["decisions"]) == 1
    assert dossier["memory"]["user_profiles"]["user-1"]["active_preferences"] == [
        "El usuario prefiere respuestas breves.",
        "El usuario prefiere respuestas con tablas cuando agregan valor.",
    ]
    assert "Decisiones recientes:" in context
    assert "create_sale: Confirmar venta urgente" in context


@pytest.mark.asyncio
async def test_hydrate_dossier_from_backend_settings_refreshes_business_context() -> None:
    repo = StubRepo()

    class SettingsBackend:
        async def request(self, method: str, path: str, auth=None, **_kwargs):
            assert method == "GET"
            assert path == "/v1/tenant-settings"
            assert auth is not None
            return {
                "business_name": "Bici Norte",
                "vertical": "bike_shop",
                "business_type": "retail",
                "business_description": "Taller y tienda de ciclismo",
            }

    dossier, snapshot = await _hydrate_dossier_from_backend_settings(
        repo=repo,  # type: ignore[arg-type]
        backend_client=SettingsBackend(),  # type: ignore[arg-type]
        org_id="org-123",
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert dossier["business"]["name"] == "Bici Norte"
    assert dossier["business"]["vertical"] == "bike_shop"
    assert snapshot["business"]["name"] == "Bici Norte"


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
async def test_run_internal_orchestrated_chat_prioritizes_sales_analysis_for_commercial_growth_prompt(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general para un pedido comercial ejecutivo claro")

    class StubAnalysisClient:
        def complete_json(self, *, system_prompt: str, user_prompt: str):
            assert '"category": "Ventas"' in user_prompt
            return SimpleNamespace(
                content=(
                    '{"reply":"Semana floja con oportunidad comercial clara.",'
                    '"summary":"El volumen vendido es bajo para la semana y conviene activar palancas comerciales.",'
                    '"scope":"Ventas · semanal","highlights":[{"label":"Lectura","value":"Comercial"}],'
                    '"recommendations":["Reactivar clientes recientes.","Promover productos con stock.","Ofrecer combo de ticket medio."],'
                    '"kpis":[{"label":"Ventas","value":"2","trend":"flat","context":"muestra actual"}],'
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
        message=(
            "Analizá el negocio con foco comercial. No quiero un listado de productos. "
            "Quiero un resumen ejecutivo de esta semana y 3 acciones concretas para vender más."
        ),
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
    assert "Semana floja con oportunidad comercial clara." in result.reply
    assert result.blocks[0]["type"] == "insight_card"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_does_not_offer_clarification_for_executive_business_request(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia pedir clarificacion para una consulta ejecutiva clara")

    class StubAnalysisClient:
        def complete_json(self, *, system_prompt: str, user_prompt: str):
            assert '"category": "Ventas"' in user_prompt
            return SimpleNamespace(
                content=(
                    '{"reply":"Necesitás foco comercial esta semana.",'
                    '"summary":"El negocio necesita empuje comercial y seguimiento de ventas.",'
                    '"scope":"Ventas · semanal","highlights":[{"label":"Lectura","value":"Ejecutiva"}],'
                    '"recommendations":["Reactivar clientes recientes."],'
                    '"kpis":[{"label":"Ventas","value":"2","trend":"flat","context":"muestra actual"}],'
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
        message="Quiero una mirada de dueño del negocio y 3 acciones concretas para vender más esta semana.",
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
    assert "Necesitás foco comercial esta semana." in result.reply


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_overrides_service_route_hint_for_executive_business_request(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general cuando un route_hint ejecutivo se puede resolver por analysis fallback")

    class StubAnalysisClient:
        def complete_json(self, *, system_prompt: str, user_prompt: str):
            assert '"category": "Ventas"' in user_prompt
            return SimpleNamespace(
                content=(
                    '{"reply":"La prioridad es mover ventas, no listar servicios.",'
                    '"summary":"La consulta es ejecutiva y debe resolverse con foco comercial.",'
                    '"scope":"Ventas · semanal","highlights":[{"label":"Ruteo","value":"Ventas"}],'
                    '"recommendations":["Promover servicios de mayor margen."],'
                    '"kpis":[{"label":"Ventas","value":"2","trend":"flat","context":"muestra actual"}],'
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
        message="Quiero una mirada de dueño del negocio. Decime 3 acciones concretas para vender más.",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint="services",
    )

    assert result.routed_agent == "sales"
    assert result.routing_source == "ui_hint"
    assert "La prioridad es mover ventas, no listar servicios." in result.reply


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
    assert "asesor del negocio" in result.reply.lower()
    assert "clientes, productos, ventas, cobros, servicios y compras" in result.reply.lower()
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
    assert result.reply == "Elegí una categoría para que pueda ayudarte mejor."
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
    assert result.reply == "Elegí una categoría para que pueda ayudarte mejor."
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
async def test_run_internal_orchestrated_chat_routes_to_explicit_insight_chat_handoff(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("el product agent no deberia ejecutar routing LLM cuando llega un handoff explicito")

    async def fake_build_insight_chat_response_for_scope(**kwargs):
        assert kwargs["scope"] == "sales_collections"
        assert kwargs["period"] == "month"
        return SimpleNamespace(
            reply="Ventas arriba 12% este mes.",
            blocks=[
                {"type": "insight_card", "title": "Ventas y cobranzas", "summary": "Ventas arriba 12% este mes.", "scope": "Ventas y cobranzas · este mes", "highlights": [], "recommendations": ["Mantener seguimiento semanal."]},
                {"type": "kpi_group", "title": "KPIs clave", "items": [{"label": "Ventas", "value": "$120,000.00", "trend": "up", "context": "+12.0% vs período anterior"}]},
            ],
            insight_evidence=None,
        )

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.build_insight_chat_response_for_scope", fake_build_insight_chat_response_for_scope)

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
        route_hint="insight_chat",
    )

    assert result.routed_agent == "insight_chat"
    assert result.reply == "Ventas arriba 12% este mes."
    assert result.blocks[0]["type"] == "insight_card"
    assert result.blocks[1]["type"] == "kpi_group"
    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert assistant_message["routed_agent"] == "insight_chat"
    assert assistant_message["agent_mode"] == "insight_chat"
    assert assistant_message["routing_source"] == "ui_hint"
    assert repo.agent_events[-1]["metadata"]["routing_source"] == "ui_hint"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_prioritizes_structured_handoff_before_legacy_insight_chat_match(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])
    captured: dict[str, object] = {}

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general cuando el handoff estructurado ya alcanza para entrar al carril insight")

    async def fake_build_insight_chat_response_for_scope(**kwargs):
        captured.update(kwargs)
        return SimpleNamespace(
            reply="Ventas arriba 12% esta semana.",
            blocks=[{"type": "insight_card", "title": "Ventas y cobranzas", "summary": "Ventas arriba 12% esta semana.", "scope": "Ventas y cobranzas · esta semana", "highlights": [], "recommendations": []}],
            insight_evidence=None,
        )

    async def fake_validate_internal_insight_handoff_with_reason(**_kwargs):
        return True, "validated"

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.build_insight_chat_response_for_scope", fake_build_insight_chat_response_for_scope)
    monkeypatch.setattr(
        "src.agents.service._validate_internal_insight_handoff_with_reason",
        fake_validate_internal_insight_handoff_with_reason,
    )

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
        handoff=SimpleNamespace(
            source="in_app_notification",
            notification_id="notif-123",
            insight_scope="sales_collections",
            period="week",
            compare=True,
            top_limit=5,
        ),
    )

    assert result.routed_agent == "insight_chat"
    assert result.reply == "Ventas arriba 12% esta semana."
    assert captured["scope"] == "sales_collections"
    assert captured["period"] == "week"
    assert captured["compare"] is True
    assert captured["top_limit"] == 5


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_persists_insight_evidence_in_assistant_message(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_build_insight_chat_response_for_scope(**_kwargs):
        return SimpleNamespace(
            reply="Ventas arriba 12% esta semana.",
            blocks=[{"type": "insight_card", "title": "Ventas y cobranzas", "summary": "Ventas arriba 12% esta semana.", "scope": "Ventas y cobranzas · esta semana", "highlights": [], "recommendations": []}],
            insight_evidence=InternalInsightEvidence(
                source="insight_handoff",
                notification_id="notif-123",
                scope="sales_collections",
                period="week",
                compare=True,
                top_limit=5,
                computed_at="2026-04-10T12:00:00Z",
                summary="Ventas arriba 12% esta semana.",
                current_period=InsightEvidencePeriod(
                    label="Esta semana",
                    from_date="2026-04-07",
                    to_date="2026-04-10",
                ),
                comparison_period=None,
            ),
        )

    async def fake_validate_internal_insight_handoff_with_reason(**_kwargs):
        return True, "validated"

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.build_insight_chat_response_for_scope", fake_build_insight_chat_response_for_scope)
    monkeypatch.setattr(
        "src.agents.service._validate_internal_insight_handoff_with_reason",
        fake_validate_internal_insight_handoff_with_reason,
    )

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
        handoff=SimpleNamespace(
            source="in_app_notification",
            notification_id="notif-123",
            insight_scope="sales_collections",
            period="week",
            compare=True,
            top_limit=5,
        ),
    )

    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert result.routed_agent == "insight_chat"
    assert assistant_message["insight_evidence"]["notification_id"] == "notif-123"
    assert assistant_message["insight_evidence"]["scope"] == "sales_collections"
    assert assistant_message["insight_evidence"]["period"] == "week"
    assert assistant_message["insight_evidence"]["summary"] == "Ventas arriba 12% esta semana."


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_falls_back_to_legacy_insight_chat_when_handoff_is_invalid(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general cuando el fallback legacy de insight_chat alcanza")

    async def fake_build_insight_chat_response_for_scope(**kwargs):
        assert kwargs["scope"] == "sales_collections"
        assert kwargs["period"] == "month"
        assert kwargs["evidence_source"] == "insight_chat_legacy_match"
        return SimpleNamespace(
            reply="Ventas arriba 12% este mes.",
            blocks=[{"type": "insight_card", "title": "Ventas y cobranzas", "summary": "Ventas arriba 12% este mes.", "scope": "Ventas y cobranzas · este mes", "highlights": [], "recommendations": []}],
            insight_evidence=None,
        )

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.build_insight_chat_response_for_scope", fake_build_insight_chat_response_for_scope)

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="Como viene el negocio este mes?",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint="insight_chat",
        handoff=SimpleNamespace(
            source="in_app_notification",
            notification_id="notif-123",
            insight_scope="unknown_scope",
            period="month",
            compare=True,
            top_limit=5,
        ),
    )

    assert result.routed_agent == "insight_chat"
    assert result.reply == "Ventas arriba 12% este mes."
    assert result.blocks[0]["type"] == "insight_card"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_falls_back_to_legacy_insight_chat_when_handoff_resolution_fails(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError("no deberia usar el router general cuando el fallback legacy de insight_chat alcanza")

    calls: list[dict[str, object]] = []

    async def fake_build_insight_chat_response_for_scope(**kwargs):
        calls.append(kwargs)
        if kwargs["evidence_source"] == "insight_handoff":
            return None
        return SimpleNamespace(
            reply="Ventas arriba 12% este mes.",
            blocks=[{"type": "insight_card", "title": "Ventas y cobranzas", "summary": "Ventas arriba 12% este mes.", "scope": "Ventas y cobranzas · este mes", "highlights": [], "recommendations": []}],
            insight_evidence=None,
        )

    async def fake_validate_internal_insight_handoff_with_reason(**_kwargs):
        return True, "validated"

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.build_insight_chat_response_for_scope", fake_build_insight_chat_response_for_scope)
    monkeypatch.setattr(
        "src.agents.service._validate_internal_insight_handoff_with_reason",
        fake_validate_internal_insight_handoff_with_reason,
    )

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="Como viene el negocio este mes?",
        conversation_id=None,
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
        route_hint="insight_chat",
        handoff=SimpleNamespace(
            source="in_app_notification",
            notification_id="notif-123",
            insight_scope="sales_collections",
            period="month",
            compare=True,
            top_limit=5,
        ),
    )

    assert result.routed_agent == "insight_chat"
    assert result.reply == "Ventas arriba 12% este mes."
    assert result.blocks[0]["type"] == "insight_card"
    assert [call["evidence_source"] for call in calls] == ["insight_handoff", "insight_chat_legacy_match"]


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_logs_handoff_resolved(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])
    logger = CapturingLogger()

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_build_insight_chat_response_for_scope(**_kwargs):
        return SimpleNamespace(
            reply="Ventas arriba 12% esta semana.",
            blocks=[{"type": "insight_card", "title": "Ventas y cobranzas", "summary": "Ventas arriba 12% esta semana.", "scope": "Ventas y cobranzas · esta semana", "highlights": [], "recommendations": []}],
            insight_evidence=None,
        )

    async def fake_validate_internal_insight_handoff_with_reason(**_kwargs):
        return True, "validated"

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.build_insight_chat_response_for_scope", fake_build_insight_chat_response_for_scope)
    monkeypatch.setattr("src.agents.service._validate_internal_insight_handoff_with_reason", fake_validate_internal_insight_handoff_with_reason)
    monkeypatch.setattr("src.agents.service.logger", logger)

    await run_internal_orchestrated_chat(
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
        handoff=SimpleNamespace(
            source="in_app_notification",
            notification_id="notif-123",
            insight_scope="sales_collections",
            period="week",
            compare=True,
            top_limit=5,
        ),
    )

    resolved_logs = [entry for entry in logger.info_calls if entry[0] == "handoff_resolved"]
    assert len(resolved_logs) == 1
    assert resolved_logs[0][1]["notification_id"] == "notif-123"
    assert resolved_logs[0][1]["handoff_scope"] == "sales_collections"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_logs_handoff_failed(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])
    logger = CapturingLogger()

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)
        yield SimpleNamespace(type="text", text="Hola.", tool_call=None)

    async def fake_validate_internal_insight_handoff_with_reason(**_kwargs):
        return False, "notification_not_found"

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service._validate_internal_insight_handoff_with_reason", fake_validate_internal_insight_handoff_with_reason)
    monkeypatch.setattr("src.agents.service.logger", logger)

    await run_internal_orchestrated_chat(
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
        handoff=SimpleNamespace(
            source="in_app_notification",
            notification_id="notif-404",
            insight_scope="sales_collections",
            period="week",
            compare=True,
            top_limit=5,
        ),
    )

    assert logger.warning_calls[0][0] == "handoff_failed"
    assert logger.warning_calls[0][1]["notification_id"] == "notif-404"
    assert logger.warning_calls[0][1]["reason"] == "notification_not_found"


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_logs_routing_decision_for_structured_handoff(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])
    logger = CapturingLogger()

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_build_insight_chat_response_for_scope(**_kwargs):
        return SimpleNamespace(
            reply="Ventas arriba 12% esta semana.",
            blocks=[{"type": "insight_card", "title": "Ventas y cobranzas", "summary": "Ventas arriba 12% esta semana.", "scope": "Ventas y cobranzas · esta semana", "highlights": [], "recommendations": []}],
            insight_evidence=None,
        )

    async def fake_validate_internal_insight_handoff_with_reason(**_kwargs):
        return True, "validated"

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.build_insight_chat_response_for_scope", fake_build_insight_chat_response_for_scope)
    monkeypatch.setattr("src.agents.service._validate_internal_insight_handoff_with_reason", fake_validate_internal_insight_handoff_with_reason)
    monkeypatch.setattr("src.agents.service.logger", logger)

    await run_internal_orchestrated_chat(
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
        handoff=SimpleNamespace(
            source="in_app_notification",
            notification_id="notif-123",
            insight_scope="sales_collections",
            period="week",
            compare=True,
            top_limit=5,
        ),
    )

    decision_logs = [entry for entry in logger.info_calls if entry[0] == "internal_turn_routing_decision"]
    assert len(decision_logs) == 1
    event, payload = decision_logs[0]
    assert event == "internal_turn_routing_decision"
    assert payload["handler_kind"] == "insight_lane"
    assert payload["routing_target"] == "sales_collections"
    assert payload["routing_reason"] == "structured_handoff"
    assert payload["handoff_source"] == "in_app_notification"
    assert payload["handoff_scope"] == "sales_collections"
    assert payload["handoff_valid"] is True


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_logs_routing_decision_for_orchestrator_fallback(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])
    logger = CapturingLogger()

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="general", tool_call=None)
        yield SimpleNamespace(type="text", text="Hola.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.logger", logger)

    await run_internal_orchestrated_chat(
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

    decision_logs = [entry for entry in logger.info_calls if entry[0] == "internal_turn_routing_decision"]
    assert len(decision_logs) == 1
    event, payload = decision_logs[0]
    assert event == "internal_turn_routing_decision"
    assert payload["handler_kind"] == "orchestrator"
    assert payload["routing_target"] == "general"
    assert payload["routing_reason"] == "no_deterministic_match"
    assert payload["route_hint"] == ""
    assert payload["route_hint_source"] == ""
    assert payload["handoff_source"] == ""
    assert payload["handoff_scope"] == ""
    assert payload["handoff_valid"] is False


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_does_not_persist_insight_evidence_for_regular_route(monkeypatch) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        yield SimpleNamespace(type="route", text="customers", tool_call=None)
        yield SimpleNamespace(type="text", text="Encontré 3 clientes.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    await run_internal_orchestrated_chat(
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

    assistant_message = repo.append_calls[0]["new_messages"][1]
    assert "insight_evidence" not in assistant_message


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("scenario", "route_hint", "message", "handoff", "validation_result", "structured_reply", "legacy_reply", "expected_reply"),
    [
        (
            "valid_handoff",
            None,
            "hola",
            SimpleNamespace(
                source="in_app_notification",
                notification_id="notif-123",
                insight_scope="sales_collections",
                period="week",
                compare=True,
                top_limit=5,
            ),
            (True, "validated"),
            "Ventas arriba 12% esta semana.",
            None,
            "Ventas arriba 12% esta semana.",
        ),
        (
            "no_handoff_legacy_insight_chat",
            "insight_chat",
            "Como viene el negocio este mes?",
            None,
            (False, "unsupported_scope"),
            None,
            "Ventas arriba 12% este mes.",
            "Ventas arriba 12% este mes.",
        ),
        (
            "invalid_handoff_fallback",
            "insight_chat",
            "Como viene el negocio este mes?",
            SimpleNamespace(
                source="in_app_notification",
                notification_id="notif-404",
                insight_scope="sales_collections",
                period="month",
                compare=True,
                top_limit=5,
            ),
            (False, "notification_not_found"),
            None,
            "Ventas arriba 12% este mes.",
            "Ventas arriba 12% este mes.",
        ),
    ],
)
async def test_run_internal_orchestrated_chat_handoff_parity_table(
    monkeypatch,
    scenario: str,
    route_hint: str | None,
    message: str,
    handoff: SimpleNamespace | None,
    validation_result: tuple[bool, str],
    structured_reply: str | None,
    legacy_reply: str | None,
    expected_reply: str,
) -> None:
    repo = StubRepo()
    conversation = SimpleNamespace(id="conv-1", messages=[])

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    async def fake_run_routed_agent(**_kwargs):
        raise AssertionError(f"el caso {scenario} no deberia necesitar el router general")

    async def fake_validate_internal_insight_handoff_with_reason(**_kwargs):
        return validation_result

    async def fake_build_insight_chat_response_for_scope(**kwargs):
        if kwargs["evidence_source"] == "insight_handoff":
            if structured_reply is None:
                return None
            reply = structured_reply
            scope = "Ventas y cobranzas · esta semana"
        else:
            if legacy_reply is None:
                return None
            reply = legacy_reply
            scope = "Ventas y cobranzas · este mes"
        return SimpleNamespace(
            reply=reply,
            blocks=[{"type": "insight_card", "title": "Ventas y cobranzas", "summary": reply, "scope": scope, "highlights": [], "recommendations": []}],
            insight_evidence=None,
        )

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())
    monkeypatch.setattr("src.agents.service._validate_internal_insight_handoff_with_reason", fake_validate_internal_insight_handoff_with_reason)
    monkeypatch.setattr("src.agents.service.build_insight_chat_response_for_scope", fake_build_insight_chat_response_for_scope)

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
        handoff=handoff,
    )

    assert result.routed_agent == "insight_chat"
    assert result.reply == expected_reply
    assert result.blocks[0]["type"] == "insight_card"


@pytest.mark.asyncio
async def test_validate_internal_insight_handoff_accepts_notification_visible_for_org_and_actor() -> None:
    auth = AuthContext(
        tenant_id="org-123",
        actor="user-1",
        role="admin",
        scopes=["admin:console:write"],
        mode="jwt",
    )
    backend_client = SimpleNamespace(
        request=_async_return(
            {
                "items": [
                    {
                        "id": "notif-123",
                        "chat_context": {"scope": "sales_collections"},
                    }
                ]
            }
        )
    )

    from src.agents.service import _validate_internal_insight_handoff

    result = await _validate_internal_insight_handoff(
        backend_client=backend_client,  # type: ignore[arg-type]
        auth=auth,
        handoff=SimpleNamespace(
            source="in_app_notification",
            notification_id="notif-123",
            insight_scope="sales_collections",
            period="month",
            compare=True,
            top_limit=5,
        ),
    )

    assert result is True


@pytest.mark.asyncio
async def test_validate_internal_insight_handoff_rejects_notification_scope_mismatch() -> None:
    auth = AuthContext(
        tenant_id="org-123",
        actor="user-1",
        role="admin",
        scopes=["admin:console:write"],
        mode="jwt",
    )
    backend_client = SimpleNamespace(
        request=_async_return(
            {
                "items": [
                    {
                        "id": "notif-123",
                        "chat_context": {"scope": "inventory_profit"},
                    }
                ]
            }
        )
    )

    from src.agents.service import _validate_internal_insight_handoff

    result = await _validate_internal_insight_handoff(
        backend_client=backend_client,  # type: ignore[arg-type]
        auth=auth,
        handoff=SimpleNamespace(
            source="in_app_notification",
            notification_id="notif-123",
            insight_scope="sales_collections",
            period="month",
            compare=True,
            top_limit=5,
        ),
    )

    assert result is False


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_routes_operational_prompt_without_insight_chat(monkeypatch) -> None:
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


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_injects_evidence_on_followup(monkeypatch) -> None:
    """Turn 2 con chat_id y evidencia previa inyecta CONTEXTO INSIGHT PREVIO en history."""
    repo = StubRepo()
    evidence_payload = {
        "source": "insight_handoff",
        "scope": "sales_collections",
        "period": "week",
        "compare": True,
        "top_limit": 5,
        "computed_at": datetime.now(UTC).isoformat(),
        "summary": "Ventas arriba 12% esta semana.",
        "current_period": {"label": "esta semana", "from_date": "2026-04-03", "to_date": "2026-04-10"},
        "kpis": [{"key": "total_sales", "label": "Ventas totales", "unit": "currency", "value": 120000.0, "delta_pct": 12.1, "trend": "up"}],
        "highlights": [{"severity": "positive", "title": "Ventas en alza", "detail": "Crecimiento sostenido."}],
        "recommendations": ["Revisar stock."],
        "entity_ids": ["cust-1"],
    }
    conversation = SimpleNamespace(
        id="conv-1",
        messages=[
            {"role": "user", "content": "Quiero entender ventas de esta semana."},
            {"role": "assistant", "content": "Ventas arriba 12% esta semana.", "insight_evidence": evidence_payload},
        ],
    )

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    captured_history: list = []

    async def fake_run_routed_agent(**kwargs):
        captured_history.extend(kwargs.get("history") or [])
        yield SimpleNamespace(type="route", text="general", tool_call=None)
        yield SimpleNamespace(type="text", text="Eso implica que el negocio viene bien.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    result = await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="que implica eso?",
        conversation_id="conv-1",
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    assert result.reply == "Eso implica que el negocio viene bien."
    # La evidencia debe estar inyectada como primer mensaje del history
    assert len(captured_history) >= 1
    evidence_msg = captured_history[0]
    assert evidence_msg.role == "system"
    assert "CONTEXTO INSIGHT PREVIO" in evidence_msg.content
    assert "Ventas totales" in evidence_msg.content
    assert "120000" in evidence_msg.content
    # entity_ids no debe estar en el compactado
    assert "cust-1" not in evidence_msg.content


@pytest.mark.asyncio
async def test_run_internal_orchestrated_chat_no_evidence_injection_for_regular_conversation(monkeypatch) -> None:
    """Conversación sin insight_evidence no inyecta mensaje extra de contexto."""
    repo = StubRepo()
    conversation = SimpleNamespace(
        id="conv-2",
        messages=[
            {"role": "user", "content": "dame la lista de clientes"},
            {"role": "assistant", "content": "Tenés 3 clientes."},
        ],
    )

    async def fake_load_internal_conversation(*_args, **_kwargs):
        return conversation

    captured_history: list = []

    async def fake_run_routed_agent(**kwargs):
        captured_history.extend(kwargs.get("history") or [])
        yield SimpleNamespace(type="route", text="general", tool_call=None)
        yield SimpleNamespace(type="text", text="Puedo ayudarte.", tool_call=None)

    monkeypatch.setattr("src.agents.service._load_internal_conversation", fake_load_internal_conversation)
    monkeypatch.setattr("src.agents.service.run_routed_agent", fake_run_routed_agent)
    monkeypatch.setattr("src.agents.service.build_registry", lambda *_args, **_kwargs: StubRegistry())

    await run_internal_orchestrated_chat(
        repo=repo,  # type: ignore[arg-type]
        llm=object(),  # type: ignore[arg-type]
        backend_client=object(),  # type: ignore[arg-type]
        org_id="org-123",
        message="otra cosa",
        conversation_id="conv-2",
        auth=AuthContext(
            tenant_id="org-123",
            actor="user-1",
            role="admin",
            scopes=["admin:console:write"],
            mode="jwt",
        ),
    )

    # Sin evidencia, no debe haber mensaje system de contexto insight
    for msg in captured_history:
        assert "CONTEXTO INSIGHT PREVIO" not in getattr(msg, "content", "")
