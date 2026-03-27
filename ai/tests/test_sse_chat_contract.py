from __future__ import annotations

import json
from types import SimpleNamespace

from fastapi import FastAPI
from fastapi.testclient import TestClient

from src.api import chat_stream
from src.api.deps import get_auth_context, get_backend_client, get_llm_provider, get_repository
from src.api.pymes_assistant_router import router as pymes_assistant_router
from src.api.public_router import router as public_chat_router
from src.api.router import router as internal_chat_router
from src.agents.service_support import CommercialChatResult
import src.api.public_router as public_router_module
import src.api.pymes_assistant_router as pymes_assistant_router_module
import src.api.router as router_module


class StubAuthContext:
    def __init__(self, tenant_id: str, actor: str, role: str, scopes: list[str], mode: str) -> None:
        self.tenant_id = tenant_id
        self.actor = actor
        self.role = role
        self.scopes = scopes
        self.mode = mode

    @property
    def org_id(self) -> str:
        return self.tenant_id


class StubRepo:
    def __init__(self, *, plan_code: str = "starter") -> None:
        self.plan_code = plan_code
        self.append_calls: list[dict[str, object]] = []
        self.track_calls: list[dict[str, int]] = []
        self.update_calls: list[dict[str, object]] = []
        self.created_conversation = SimpleNamespace(
            id="conv-1",
            mode="internal",
            user_id=None,
            messages=[],
            title="hola",
        )

    async def get_plan_code(self, _org_id: str) -> str:
        return self.plan_code

    async def get_month_usage(self, _org_id: str, _year: int, _month: int) -> dict[str, int]:
        return {"queries": 0, "tokens_input": 0, "tokens_output": 0}

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

    async def get_or_create_dossier(self, _org_id: str) -> dict[str, object]:
        return {"business": {"name": "Demo"}}

    async def append_messages(self, **kwargs):
        self.append_calls.append(kwargs)
        return self.created_conversation

    async def track_usage(self, org_id: str, tokens_in: int, tokens_out: int) -> None:
        self.track_calls.append({"org_id": org_id, "tokens_in": tokens_in, "tokens_out": tokens_out})

    async def update_dossier(self, org_id: str, patch: dict[str, object]) -> dict[str, object]:
        self.update_calls.append({"org_id": org_id, "patch": patch})
        return patch


def chunk(kind: str, *, text: str | None = None, tool_name: str | None = None):
    tool_call = {"name": tool_name} if tool_name is not None else None
    return SimpleNamespace(type=kind, text=text, tool_call=tool_call)


def make_orchestrate(script):
    async def fake_orchestrate(**_kwargs):
        for item in script:
            if isinstance(item, Exception):
                raise item
            yield item

    return fake_orchestrate


def parse_sse_events(body: str) -> list[tuple[str, dict[str, object]]]:
    events: list[tuple[str, dict[str, object]]] = []
    current_event = "message"
    data_lines: list[str] = []

    for line in body.splitlines():
        if not line.strip():
            if data_lines:
                payload = json.loads("\n".join(data_lines))
                events.append((current_event, payload))
            current_event = "message"
            data_lines = []
            continue
        if line.startswith("event: "):
            current_event = line.removeprefix("event: ").strip()
            continue
        if line.startswith("data: "):
            data_lines.append(line.removeprefix("data: "))

    if data_lines:
        payload = json.loads("\n".join(data_lines))
        events.append((current_event, payload))

    return events


def create_internal_client(repo: StubRepo) -> TestClient:
    app = FastAPI()
    app.include_router(internal_chat_router)
    app.dependency_overrides[get_repository] = lambda: repo
    app.dependency_overrides[get_auth_context] = lambda: StubAuthContext(
        tenant_id="00000000-0000-0000-0000-000000000123",
        actor="00000000-0000-0000-0000-000000000999",
        role="admin",
        scopes=["admin:console:write"],
        mode="jwt",
    )
    app.dependency_overrides[get_llm_provider] = lambda: object()
    app.dependency_overrides[get_backend_client] = lambda: object()
    return TestClient(app)


def create_pymes_assistant_client(repo: StubRepo) -> TestClient:
    app = FastAPI()
    app.include_router(pymes_assistant_router)
    app.dependency_overrides[get_repository] = lambda: repo
    app.dependency_overrides[get_auth_context] = lambda: StubAuthContext(
        tenant_id="00000000-0000-0000-0000-000000000123",
        actor="00000000-0000-0000-0000-000000000999",
        role="admin",
        scopes=["admin:console:write"],
        mode="jwt",
    )
    app.dependency_overrides[get_llm_provider] = lambda: object()
    app.dependency_overrides[get_backend_client] = lambda: object()
    return TestClient(app)


def create_public_client(repo: StubRepo, monkeypatch) -> TestClient:
    app = FastAPI()
    app.include_router(public_chat_router)
    app.dependency_overrides[get_repository] = lambda: repo
    app.dependency_overrides[get_llm_provider] = lambda: object()
    app.dependency_overrides[get_backend_client] = lambda: object()

    async def fake_resolve_org_id(*_args, **_kwargs) -> str:
        return "org-public-123"

    async def fake_get_external_conversation(**_kwargs):
        return await repo.create_conversation(org_id="org-public-123", mode="external", title="hola")

    monkeypatch.setattr(public_router_module, "resolve_org_id", fake_resolve_org_id)
    monkeypatch.setattr(public_router_module, "get_external_conversation", fake_get_external_conversation)
    return TestClient(app)


def test_internal_chat_failure_before_first_chunk_does_not_persist(monkeypatch) -> None:
    repo = StubRepo()
    client = create_internal_client(repo)

    async def fake_run_internal_orchestrated_chat(**_kwargs):
        raise RuntimeError("boom")

    monkeypatch.setattr(router_module, "run_internal_orchestrated_chat", fake_run_internal_orchestrated_chat)

    response = client.post("/v1/chat", json={"message": "hola"})

    assert response.status_code == 200
    events = parse_sse_events(response.text)
    assert events == [("error", {"message": "error processing request"})]
    assert repo.append_calls == []
    assert repo.track_calls == []


def test_internal_chat_failure_after_partial_text_does_not_persist(monkeypatch) -> None:
    repo = StubRepo()
    client = create_internal_client(repo)

    async def fake_run_internal_orchestrated_chat(**_kwargs):
        raise RuntimeError("boom")

    monkeypatch.setattr(router_module, "run_internal_orchestrated_chat", fake_run_internal_orchestrated_chat)

    response = client.post("/v1/chat", json={"message": "hola"})

    assert response.status_code == 200
    events = parse_sse_events(response.text)
    assert events == [("error", {"message": "error processing request"})]
    assert repo.append_calls == []
    assert repo.track_calls == []


def test_internal_chat_success_persists_and_finishes(monkeypatch) -> None:
    repo = StubRepo()
    client = create_internal_client(repo)

    async def fake_run_internal_orchestrated_chat(**_kwargs):
        return CommercialChatResult(
            conversation_id="conv-1",
            reply="respuesta final",
            tokens_input=10,
            tokens_output=15,
            tool_calls=["lookup_customer"],
            pending_confirmations=[],
            routed_agent="clientes",
        )

    monkeypatch.setattr(router_module, "run_internal_orchestrated_chat", fake_run_internal_orchestrated_chat)

    response = client.post("/v1/chat", json={"message": "hola"})

    assert response.status_code == 200
    events = parse_sse_events(response.text)
    assert events == [
        ("tool_call", {"tool": "lookup_customer", "status": "done"}),
        ("text", {"content": "respuesta final"}),
        (
            "done",
            {
                "conversation_id": "conv-1",
                "tokens_used": 25,
                "routed_agent": "clientes",
                "routed_mode": "clientes",
            },
        ),
    ]


def test_pymes_assistant_response_includes_routed_agent_and_legacy_alias(monkeypatch) -> None:
    repo = StubRepo()
    client = create_pymes_assistant_client(repo)

    async def fake_run_internal_orchestrated_chat(**_kwargs):
        return CommercialChatResult(
            conversation_id="conv-1",
            reply="respuesta final",
            tokens_input=12,
            tokens_output=18,
            tool_calls=["search_customers"],
            pending_confirmations=[],
            routed_agent="clientes",
        )

    monkeypatch.setattr(
        pymes_assistant_router_module,
        "run_internal_orchestrated_chat",
        fake_run_internal_orchestrated_chat,
    )

    response = client.post("/v1/chat/pymes/", json={"message": "hola"})

    assert response.status_code == 200
    assert response.json() == {
        "conversation_id": "conv-1",
        "reply": "respuesta final",
        "tokens_used": 30,
        "tool_calls": ["search_customers"],
        "pending_confirmations": [],
        "routed_agent": "clientes",
        "routed_mode": "clientes",
    }


def test_public_chat_failure_does_not_persist_or_finish(monkeypatch) -> None:
    repo = StubRepo(plan_code="growth")
    client = create_public_client(repo, monkeypatch)

    monkeypatch.setattr(chat_stream, "orchestrate", make_orchestrate([RuntimeError("boom")]))
    monkeypatch.setattr(public_router_module, "build_external_tools", lambda *_args, **_kwargs: ([], {}))

    response = client.post("/v1/public/demo/chat", json={"message": "hola", "phone": "+54 11 5555 1111"})

    assert response.status_code == 200
    events = parse_sse_events(response.text)
    assert events == [("error", {"message": "error processing request"})]
    assert repo.append_calls == []
    assert repo.track_calls == []


def test_public_chat_success_persists_and_finishes(monkeypatch) -> None:
    repo = StubRepo(plan_code="growth")
    client = create_public_client(repo, monkeypatch)

    monkeypatch.setattr(chat_stream, "orchestrate", make_orchestrate([chunk("text", text="respuesta externa")]))
    monkeypatch.setattr(public_router_module, "build_external_tools", lambda *_args, **_kwargs: ([], {}))

    response = client.post("/v1/public/demo/chat", json={"message": "hola", "phone": "+54 11 5555 1111"})

    assert response.status_code == 200
    events = parse_sse_events(response.text)
    assert events[-1][0] == "done"
    assert events[-1][1]["conversation_id"] == "conv-1"
    assert int(events[-1][1]["tokens_used"]) > 0
    assert repo.append_calls and repo.append_calls[0]["conversation_id"] == "conv-1"
    assert len(repo.track_calls) == 1
    assert repo.track_calls[0]["org_id"] == "org-public-123"
    assert repo.track_calls[0]["tokens_in"] > 0
    assert repo.track_calls[0]["tokens_out"] > 0
