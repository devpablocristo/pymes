from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Any

from fastapi.testclient import TestClient
from httpx import HTTPStatusError, Request, Response

from src.api import deps
from pymes_control_plane_shared.ai_runtime import ChatChunk, Message, ToolDeclaration
from src.main import app


class _ToolCallingLLM:
    def __init__(self, tool_name: str, arguments: dict[str, Any]) -> None:
        self.tool_name = tool_name
        self.arguments = arguments
        self.calls = 0

    async def chat(
        self,
        messages: list[Message],
        tools: list[ToolDeclaration] | None = None,
        temperature: float = 0.3,
        max_tokens: int = 2048,
    ) -> AsyncIterator[ChatChunk]:
        _ = (messages, tools, temperature, max_tokens)
        if self.calls == 0:
            self.calls += 1
            yield ChatChunk(type="tool_call", tool_call={"name": self.tool_name, "arguments": self.arguments})
            return
        yield ChatChunk(type="done")


class _SilentLLM:
    async def chat(
        self,
        messages: list[Message],
        tools: list[ToolDeclaration] | None = None,
        temperature: float = 0.3,
        max_tokens: int = 2048,
    ) -> AsyncIterator[ChatChunk]:
        _ = (messages, tools, temperature, max_tokens)
        yield ChatChunk(type="done")


class _Conversation:
    def __init__(self, mode: str) -> None:
        self.id = "conv-1"
        self.mode = mode
        self.user_id: str | None = None
        self.messages: list[dict[str, Any]] = []


class _CommercialRepo:
    def __init__(self) -> None:
        self.internal = _Conversation("internal")
        self.external = _Conversation("external")
        self.appended: list[dict[str, Any]] = []
        self.recorded_events: list[dict[str, Any]] = []
        self.processed_request_ids: set[str] = set()

    async def get_plan_code(self, org_id: str) -> str:
        _ = org_id
        return "growth"

    async def get_month_usage(self, org_id: str, year: int, month: int) -> dict[str, int]:
        _ = (org_id, year, month)
        return {"queries": 0, "tokens_input": 0, "tokens_output": 0}

    async def count_external_conversations_in_month(self, org_id: str, year: int, month: int) -> int:
        _ = (org_id, year, month)
        return 0

    async def get_latest_external_conversation(self, org_id: str, external_contact: str):
        _ = (org_id, external_contact)
        return None

    async def get_conversation(self, org_id: str, conversation_id: str):
        _ = org_id
        if conversation_id == self.internal.id:
            return self.internal
        if conversation_id == self.external.id:
            return self.external
        return None

    async def create_conversation(self, org_id: str, mode: str, user_id: str | None = None, external_contact: str = "", title: str = ""):
        _ = (org_id, user_id, external_contact, title)
        conv = _Conversation(mode)
        if mode == "internal":
            conv.user_id = user_id
            self.internal = conv
            return conv
        self.external = conv
        return conv

    async def get_or_create_dossier(self, org_id: str) -> dict[str, Any]:
        _ = org_id
        return {"business": {"name": "Acme"}, "modules_active": ["sales", "quotes", "products", "inventory", "suppliers", "purchases"], "preferences": {}, "learned_context": []}

    async def append_messages(
        self,
        org_id: str,
        conversation_id: str,
        new_messages: list[dict[str, Any]],
        tool_calls_count: int,
        tokens_input: int,
        tokens_output: int,
    ):
        _ = (org_id, conversation_id, tool_calls_count, tokens_input, tokens_output)
        self.appended = new_messages

    async def track_usage(self, org_id: str, tokens_in: int, tokens_out: int) -> None:
        _ = (org_id, tokens_in, tokens_out)

    async def record_agent_event(self, **kwargs: Any) -> None:
        self.recorded_events.append(kwargs)
        request_id = str(kwargs.get("external_request_id") or "").strip()
        if request_id:
            self.processed_request_ids.add(request_id)

    async def has_agent_request(self, org_id: str, request_id: str) -> bool:
        _ = org_id
        return request_id in self.processed_request_ids


class _BackendClient:
    async def request(self, method: str, path: str, auth=None, include_internal: bool = False, **kwargs: Any) -> dict[str, Any]:
        _ = (auth, include_internal, kwargs)
        if path.endswith("/info"):
            return {"org_id": "org-1", "business_name": "Acme", "name": "Acme", "org_secret": "hidden"}
        if "availability" in path:
            return {"date": "2026-03-06", "slots": [{"start_at": "2026-03-06T10:00:00Z", "end_at": "2026-03-06T11:00:00Z", "remaining": 1}]}
        if path.endswith("/services"):
            return {"items": [{"id": "prod-1", "name": "Servicio A", "price": 10, "currency": "ARS", "unit": "unidad", "description": "Demo", "type": "service"}]}
        return {"ok": True}


class _FailingBackendClient(_BackendClient):
    async def request(self, method: str, path: str, auth=None, include_internal: bool = False, **kwargs: Any) -> dict[str, Any]:
        _ = (auth, include_internal, kwargs)
        raise HTTPStatusError("boom", request=Request(method, f"http://backend{path}"), response=Response(503))


def test_external_sales_chat_requires_confirmation_for_booking() -> None:
    repo = _CommercialRepo()
    app.dependency_overrides[deps.get_repository] = lambda: repo
    app.dependency_overrides[deps.get_backend_client] = lambda: _BackendClient()
    app.dependency_overrides[deps.get_llm_provider] = lambda: _ToolCallingLLM(
        "book_appointment",
        {
            "customer_name": "Juan",
            "customer_phone": "+5491111111111",
            "title": "Consulta",
            "start_at": "2026-03-06T10:00:00Z",
        },
    )

    client = TestClient(app)
    response = client.post("/v1/public/acme/sales-agent/chat", json={"message": "Reservame un turno", "phone": "+54 9 11 1111-1111"})

    app.dependency_overrides.clear()

    assert response.status_code == 200
    payload = response.json()
    assert payload["pending_confirmations"] == ["book_appointment"]
    assert "confirmacion explicita" in payload["reply"].lower()
    assert any(event["result"] == "confirmation_required" for event in repo.recorded_events)


def test_internal_sales_chat_requires_confirmation_and_audits() -> None:
    repo = _CommercialRepo()
    app.dependency_overrides[deps.get_repository] = lambda: repo
    app.dependency_overrides[deps.get_backend_client] = lambda: _BackendClient()
    app.dependency_overrides[deps.get_llm_provider] = lambda: _ToolCallingLLM(
        "create_quote",
        {"customer_name": "Cliente Demo", "items": [{"description": "Servicio", "quantity": 1, "unit_price": 100}]},
    )

    client = TestClient(app)
    response = client.post(
        "/v1/chat/commercial/sales",
        headers={"X-API-KEY": "test", "X-Org-ID": "org-1", "X-Actor": "seller-1", "X-Role": "vendedor"},
        json={"message": "Armame un presupuesto"},
    )

    app.dependency_overrides.clear()

    assert response.status_code == 200
    payload = response.json()
    assert payload["pending_confirmations"] == ["create_quote"]
    assert repo.recorded_events[0]["tool_name"] == "create_quote"
    assert repo.recorded_events[0]["result"] == "confirmation_required"


def test_internal_sales_chat_handles_empty_model_response() -> None:
    repo = _CommercialRepo()
    app.dependency_overrides[deps.get_repository] = lambda: repo
    app.dependency_overrides[deps.get_backend_client] = lambda: _BackendClient()
    app.dependency_overrides[deps.get_llm_provider] = lambda: _SilentLLM()

    client = TestClient(app)
    response = client.post(
        "/v1/chat/commercial/sales",
        headers={"X-API-KEY": "test", "X-Org-ID": "org-1", "X-Actor": "seller-1", "X-Role": "admin"},
        json={"message": "Hola"},
    )

    app.dependency_overrides.clear()

    assert response.status_code == 200
    assert "no pude generar" in response.json()["reply"].lower()


def test_external_contract_success_and_idempotency() -> None:
    repo = _CommercialRepo()
    app.dependency_overrides[deps.get_repository] = lambda: repo
    app.dependency_overrides[deps.get_backend_client] = lambda: _BackendClient()

    client = TestClient(app)
    payload = {
        "contract": {
            "request_id": "req-dup-1",
            "org_id": "org-1",
            "counterparty_id": "buyer-agent-1",
            "intent": "availability_request",
            "items": [],
            "quantities": {},
            "currency": "ARS",
            "metadata": {"date": "2026-03-06", "duration": 60},
            "channel": "api",
            "timestamp": "2026-03-06T12:00:00Z",
        }
    }
    first = client.post("/v1/public/acme/sales-agent/contracts", json=payload)
    second = client.post("/v1/public/acme/sales-agent/contracts", json=payload)

    app.dependency_overrides.clear()

    assert first.status_code == 200
    assert first.json()["intent"] == "availability_response"
    assert second.status_code == 409


def test_external_contract_rejects_invalid_payload() -> None:
    repo = _CommercialRepo()
    app.dependency_overrides[deps.get_repository] = lambda: repo
    app.dependency_overrides[deps.get_backend_client] = lambda: _BackendClient()

    client = TestClient(app)
    payload = {
        "contract": {
            "request_id": "req-invalid-1",
            "org_id": "org-1",
            "counterparty_id": "buyer-agent-1",
            "intent": "request_quote",
            "items": [{"name": "Servicio A", "quantity": 1, "unit_price": 10, "currency": "ARS"}],
            "quantities": {"Servicio A": 1},
            "currency": "ARS",
            "metadata": {},
            "channel": "api",
            "timestamp": "2026-03-06T12:00:00Z",
            "unexpected": True,
        }
    }
    response = client.post("/v1/public/acme/sales-agent/contracts", json=payload)

    app.dependency_overrides.clear()

    assert response.status_code == 422


def test_internal_sales_backend_error_is_audited() -> None:
    repo = _CommercialRepo()
    app.dependency_overrides[deps.get_repository] = lambda: repo
    app.dependency_overrides[deps.get_backend_client] = lambda: _FailingBackendClient()
    app.dependency_overrides[deps.get_llm_provider] = lambda: _ToolCallingLLM(
        "generate_payment_link",
        {"reference_type": "sale", "reference_id": "sale-1"},
    )

    client = TestClient(app)
    response = client.post(
        "/v1/chat/commercial/sales",
        headers={"X-API-KEY": "test", "X-Org-ID": "org-1", "X-Actor": "seller-1", "X-Role": "admin"},
        json={"message": "Generame un link", "confirmed_actions": ["generate_payment_link"]},
    )

    app.dependency_overrides.clear()

    assert response.status_code == 200
    assert any(event["result"] == "backend_error" for event in repo.recorded_events)
