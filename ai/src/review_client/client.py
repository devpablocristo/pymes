"""Cliente HTTP para Nexus Review API."""
from __future__ import annotations

import uuid
import logging
from dataclasses import dataclass, field
from typing import Any

import httpx

logger = logging.getLogger(__name__)

_TIMEOUT = 10.0
_FALLBACK_DECISION = "require_approval"


@dataclass(frozen=True)
class SubmitResponse:
    request_id: str
    decision: str
    risk_level: str
    decision_reason: str
    status: str
    approval_id: str | None = None
    approval_expires_at: str | None = None


@dataclass(frozen=True)
class RequestStatus:
    id: str
    decision: str
    status: str
    decided_by: str | None = None
    decision_note: str | None = None


@dataclass(frozen=True)
class PolicyInfo:
    id: str
    name: str
    action_type: str
    expression: str
    effect: str
    mode: str
    created_at: str = ""
    updated_at: str = ""


@dataclass(frozen=True)
class ActionTypeInfo:
    name: str
    risk_class: str
    enabled: bool = True


@dataclass(frozen=True)
class ApprovalInfo:
    id: str
    request_id: str
    status: str
    action_type: str = ""
    target_resource: str = ""
    reason: str = ""
    risk_level: str = ""
    ai_summary: str | None = None
    created_at: str = ""
    expires_at: str | None = None


class ReviewClient:
    """Cliente asíncrono para Nexus Review API."""

    def __init__(self, base_url: str, api_key: str) -> None:
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._http = httpx.AsyncClient(
            base_url=self._base_url,
            timeout=_TIMEOUT,
            headers={"X-API-Key": self._api_key, "Content-Type": "application/json"},
        )

    async def close(self) -> None:
        await self._http.aclose()

    # --- Requests ---

    async def submit_request(
        self,
        *,
        action_type: str,
        target_system: str = "pymes",
        target_resource: str = "",
        params: dict[str, Any] | None = None,
        reason: str = "",
        context: str = "",
    ) -> SubmitResponse:
        """Envía una solicitud de acción a Review para evaluación."""
        idempotency_key = str(uuid.uuid4())
        body = {
            "requester_type": "service",
            "requester_id": "pymes-ai",
            "requester_name": "Pymes AI Service",
            "action_type": action_type,
            "target_system": target_system,
            "target_resource": target_resource,
            "params": params or {},
            "reason": reason,
            "context": context,
        }
        try:
            resp = await self._http.post(
                "/v1/requests",
                json=body,
                headers={"Idempotency-Key": idempotency_key},
            )
            resp.raise_for_status()
            data = resp.json()
            approval = data.get("approval") or {}
            return SubmitResponse(
                request_id=str(data.get("request_id", "")),
                decision=str(data.get("decision", "")),
                risk_level=str(data.get("risk_level", "")),
                decision_reason=str(data.get("decision_reason", "")),
                status=str(data.get("status", "")),
                approval_id=str(approval.get("id", "")) or None,
                approval_expires_at=str(approval.get("expires_at", "")) or None,
            )
        except Exception:
            logger.exception("review_submit_failed", extra={"action_type": action_type})
            return SubmitResponse(
                request_id="",
                decision=_FALLBACK_DECISION,
                risk_level="unknown",
                decision_reason="Review service unavailable — fallback to require_approval",
                status="fallback",
            )

    async def get_request(self, request_id: str) -> RequestStatus:
        """Consulta el estado actual de una solicitud."""
        try:
            resp = await self._http.get(f"/v1/requests/{request_id}")
            resp.raise_for_status()
            data = resp.json()
            return RequestStatus(
                id=str(data.get("id", "")),
                decision=str(data.get("decision", "")),
                status=str(data.get("status", "")),
                decided_by=data.get("decided_by"),
                decision_note=data.get("decision_note"),
            )
        except Exception:
            logger.exception("review_get_request_failed", extra={"request_id": request_id})
            return RequestStatus(id=request_id, decision="", status="unknown")

    async def report_result(
        self,
        request_id: str,
        *,
        success: bool,
        duration_ms: int = 0,
        details: str = "",
    ) -> None:
        """Reporta el resultado de la ejecución al Review."""
        try:
            resp = await self._http.post(
                f"/v1/requests/{request_id}/result",
                json={"success": success, "duration_ms": duration_ms, "details": details},
            )
            resp.raise_for_status()
        except Exception:
            logger.warning("review_report_result_failed", extra={"request_id": request_id})

    # --- Policies ---

    async def list_policies(self) -> list[PolicyInfo]:
        try:
            resp = await self._http.get("/v1/policies")
            resp.raise_for_status()
            items = resp.json() if isinstance(resp.json(), list) else resp.json().get("policies", [])
            return [
                PolicyInfo(
                    id=str(p.get("id", "")),
                    name=str(p.get("name", "")),
                    action_type=str(p.get("action_type", "")),
                    expression=str(p.get("expression", "")),
                    effect=str(p.get("effect", "")),
                    mode=str(p.get("mode", "enforced")),
                    created_at=str(p.get("created_at", "")),
                    updated_at=str(p.get("updated_at", "")),
                )
                for p in items
            ]
        except Exception:
            logger.exception("review_list_policies_failed")
            return []

    async def create_policy(
        self,
        *,
        name: str,
        action_type: str,
        expression: str,
        effect: str,
        mode: str = "enforced",
    ) -> PolicyInfo | None:
        try:
            resp = await self._http.post(
                "/v1/policies",
                json={
                    "name": name,
                    "action_type": action_type,
                    "expression": expression,
                    "effect": effect,
                    "mode": mode,
                },
            )
            resp.raise_for_status()
            p = resp.json()
            return PolicyInfo(
                id=str(p.get("id", "")),
                name=str(p.get("name", "")),
                action_type=str(p.get("action_type", "")),
                expression=str(p.get("expression", "")),
                effect=str(p.get("effect", "")),
                mode=str(p.get("mode", "enforced")),
                created_at=str(p.get("created_at", "")),
                updated_at=str(p.get("updated_at", "")),
            )
        except Exception:
            logger.exception("review_create_policy_failed")
            return None

    async def update_policy(self, policy_id: str, **kwargs: Any) -> PolicyInfo | None:
        try:
            resp = await self._http.patch(f"/v1/policies/{policy_id}", json=kwargs)
            resp.raise_for_status()
            p = resp.json()
            return PolicyInfo(
                id=str(p.get("id", "")),
                name=str(p.get("name", "")),
                action_type=str(p.get("action_type", "")),
                expression=str(p.get("expression", "")),
                effect=str(p.get("effect", "")),
                mode=str(p.get("mode", "enforced")),
            )
        except Exception:
            logger.exception("review_update_policy_failed")
            return None

    async def delete_policy(self, policy_id: str) -> bool:
        try:
            resp = await self._http.delete(f"/v1/policies/{policy_id}")
            return resp.status_code in (200, 204)
        except Exception:
            logger.exception("review_delete_policy_failed")
            return False

    # --- Action Types ---

    async def list_action_types(self) -> list[ActionTypeInfo]:
        try:
            resp = await self._http.get("/v1/action-types")
            resp.raise_for_status()
            items = resp.json() if isinstance(resp.json(), list) else resp.json().get("action_types", [])
            return [
                ActionTypeInfo(
                    name=str(a.get("name", "")),
                    risk_class=str(a.get("risk_class", "low")),
                    enabled=bool(a.get("enabled", True)),
                )
                for a in items
            ]
        except Exception:
            logger.exception("review_list_action_types_failed")
            return []

    # --- Approvals ---

    async def list_pending_approvals(self) -> list[ApprovalInfo]:
        try:
            resp = await self._http.get("/v1/approvals/pending")
            resp.raise_for_status()
            items = resp.json() if isinstance(resp.json(), list) else resp.json().get("approvals", [])
            return [
                ApprovalInfo(
                    id=str(a.get("id", "")),
                    request_id=str(a.get("request_id", "")),
                    status=str(a.get("status", "")),
                    action_type=str(a.get("action_type", "")),
                    target_resource=str(a.get("target_resource", "")),
                    reason=str(a.get("reason", "")),
                    risk_level=str(a.get("risk_level", "")),
                    ai_summary=a.get("ai_summary"),
                    created_at=str(a.get("created_at", "")),
                    expires_at=a.get("expires_at"),
                )
                for a in items
            ]
        except Exception:
            logger.exception("review_list_pending_approvals_failed")
            return []

    async def approve(self, approval_id: str, decided_by: str, note: str = "") -> bool:
        try:
            resp = await self._http.post(
                f"/v1/approvals/{approval_id}/approve",
                json={"decided_by": decided_by, "note": note},
            )
            return resp.status_code in (200, 204)
        except Exception:
            logger.exception("review_approve_failed")
            return False

    async def reject(self, approval_id: str, decided_by: str, note: str = "") -> bool:
        try:
            resp = await self._http.post(
                f"/v1/approvals/{approval_id}/reject",
                json={"decided_by": decided_by, "note": note},
            )
            return resp.status_code in (200, 204)
        except Exception:
            logger.exception("review_reject_failed")
            return False
