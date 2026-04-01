from __future__ import annotations

from contextlib import asynccontextmanager
from types import SimpleNamespace

from fastapi import FastAPI
from fastapi.testclient import TestClient

import src.api.review_callback as review_callback_module
from src.api.review_callback import router


def create_client() -> TestClient:
    app = FastAPI()
    app.include_router(router)
    app.state.settings = SimpleNamespace(review_callback_token="test-token")
    return TestClient(app)


def test_review_callback_ignores_invalid_request_id(monkeypatch) -> None:
    client = create_client()

    class UnexpectedRepository:
        def __init__(self, _session) -> None:
            raise AssertionError("repository should not be created for invalid request ids")

    @asynccontextmanager
    async def fake_session():
        yield object()

    monkeypatch.setattr(review_callback_module, "AIRepository", UnexpectedRepository)
    monkeypatch.setattr(review_callback_module, "get_session", fake_session)

    response = client.post(
        "/v1/internal/review-callback",
        headers={"X-Internal-Service-Token": "test-token"},
        json={
            "request_id": "req-not-a-uuid",
            "decision": "approved",
            "decided_by": "admin",
        },
    )

    assert response.status_code == 200
    assert response.json() == {"status": "ignored", "reason": "invalid request_id"}
