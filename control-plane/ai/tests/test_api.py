from __future__ import annotations

from fastapi.testclient import TestClient

from src.main import app


def test_healthz() -> None:
    client = TestClient(app)
    response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
    assert response.headers["X-Request-ID"].startswith("req_")


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
