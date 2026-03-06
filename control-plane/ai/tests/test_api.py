from __future__ import annotations

from typing import Any
from unittest.mock import AsyncMock, patch

from fastapi.testclient import TestClient

from src.main import app
from src.api import deps
from src.llm.base import EchoProvider


def test_healthz() -> None:
    client = TestClient(app)
    response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
    assert response.headers["X-Request-ID"].startswith("req_")


def test_readyz() -> None:
    client = TestClient(app)
    with patch("src.main.ping_database", new=AsyncMock()) as ping_mock:
        response = client.get("/readyz")
    assert response.status_code == 200
    assert response.json() == {"status": "ready"}
    ping_mock.assert_awaited_once()


def test_readyz_returns_500_on_database_error() -> None:
    client = TestClient(app, raise_server_exceptions=False)
    with patch("src.main.ping_database", new=AsyncMock(side_effect=RuntimeError("db unavailable"))):
        response = client.get("/readyz")
    assert response.status_code == 500
    payload = response.json()
    assert payload["error"]["code"] == "internal_error"


def test_chat_requires_auth() -> None:
    client = TestClient(app)
    response = client.post("/v1/chat", json={"message": "hola"})
    assert response.status_code == 401
    payload = response.json()
    assert payload["error"]["code"] == "unauthorized"
    assert payload["error"]["request_id"]


def test_public_identify() -> None:
    client = TestClient(app)
    response = client.post("/v1/public/acme/chat/identify", json={"name": "Juan", "phone": "+54 9 11 1234-5678"})
    assert response.status_code == 200
    payload = response.json()
    assert payload["status"] == "identified"
    assert payload["phone"].startswith("+549")


class _FakeConversation:
    def __init__(self) -> None:
        self.id = "conv-1"
        self.mode = "external"
        self.messages: list[dict[str, Any]] = []


class _FakeRepo:
    def __init__(self, external_conversations: int = 0) -> None:
        self.conversation = _FakeConversation()
        self.appended: list[dict[str, Any]] = []
        self.external_conversations = external_conversations

    async def get_plan_code(self, org_id: str) -> str:
        _ = org_id
        return "growth"

    async def get_month_usage(self, org_id: str, year: int, month: int) -> dict[str, int]:
        _ = (org_id, year, month)
        return {"queries": 0, "tokens_input": 0, "tokens_output": 0}

    async def count_external_conversations_in_month(self, org_id: str, year: int, month: int) -> int:
        _ = (org_id, year, month)
        return self.external_conversations

    async def get_latest_external_conversation(self, org_id: str, external_contact: str):
        _ = (org_id, external_contact)
        return None

    async def get_conversation(self, org_id: str, conversation_id: str):
        _ = (org_id, conversation_id)
        return None

    async def create_conversation(self, org_id: str, mode: str, user_id: str | None = None, external_contact: str = "", title: str = ""):
        _ = (org_id, mode, user_id, external_contact, title)
        return self.conversation

    async def get_or_create_dossier(self, org_id: str) -> dict[str, Any]:
        _ = org_id
        return {"business": {"name": "Acme"}, "modules_active": [], "preferences": {}, "learned_context": []}

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
        return self.conversation

    async def track_usage(self, org_id: str, tokens_in: int, tokens_out: int) -> None:
        _ = (org_id, tokens_in, tokens_out)


class _FakeBackendClient:
    async def request(self, method: str, path: str, auth=None, include_internal: bool = False, **kwargs: Any) -> dict[str, Any]:
        _ = (method, path, auth, include_internal, kwargs)
        return {"ok": True}


def test_internal_whatsapp_requires_internal_token() -> None:
    client = TestClient(app)
    response = client.post(
        "/v1/internal/whatsapp/message",
        json={
            "org_id": "00000000-0000-0000-0000-000000000001",
            "phone_number_id": "123456",
            "from_phone": "+5491111111111",
            "message": "hola",
        },
    )
    assert response.status_code == 401
    assert response.json()["error"]["code"] == "http_error"


def test_internal_whatsapp_message() -> None:
    repo = _FakeRepo()
    app.dependency_overrides[deps.get_repository] = lambda: repo
    app.dependency_overrides[deps.get_backend_client] = lambda: _FakeBackendClient()
    app.dependency_overrides[deps.get_llm_provider] = lambda: EchoProvider()

    client = TestClient(app)
    response = client.post(
        "/v1/internal/whatsapp/message",
        headers={"X-Internal-Service-Token": "local-internal-token"},
        json={
            "org_id": "00000000-0000-0000-0000-000000000001",
            "phone_number_id": "123456",
            "from_phone": "+54 9 11 1234-5678",
            "message": "hola",
            "message_id": "wamid-1",
            "profile_name": "Juan",
        },
    )

    app.dependency_overrides.clear()

    assert response.status_code == 200
    payload = response.json()
    assert payload["conversation_id"] == "conv-1"
    assert "hola" in payload["reply"].lower()
    assert repo.appended[0]["channel"] == "whatsapp"
    assert repo.appended[0]["message_id"] == "wamid-1"


def test_internal_whatsapp_message_respects_external_conversation_limit() -> None:
    repo = _FakeRepo(external_conversations=200)
    app.dependency_overrides[deps.get_repository] = lambda: repo
    app.dependency_overrides[deps.get_backend_client] = lambda: _FakeBackendClient()
    app.dependency_overrides[deps.get_llm_provider] = lambda: EchoProvider()

    client = TestClient(app)
    response = client.post(
        "/v1/internal/whatsapp/message",
        headers={"X-Internal-Service-Token": "local-internal-token"},
        json={
            "org_id": "00000000-0000-0000-0000-000000000001",
            "phone_number_id": "123456",
            "from_phone": "+54 9 11 1234-5678",
            "message": "hola",
        },
    )

    app.dependency_overrides.clear()

    assert response.status_code == 429
    payload = response.json()
    assert payload["error"]["code"] == "http_error"
    assert "conversaciones externas" in payload["error"]["message"].lower()
