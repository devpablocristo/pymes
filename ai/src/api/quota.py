from __future__ import annotations

from datetime import UTC, datetime

from fastapi import HTTPException, status

from src.config import get_settings
from src.db.repository import AIRepository

PLAN_LIMITS: dict[str, dict[str, int | bool]] = {
    "starter": {"queries": 50, "external": False, "external_limit": 0},
    "growth": {"queries": 500, "external": True, "external_limit": 200},
    "enterprise": {"queries": -1, "external": True, "external_limit": -1},
}


async def check_quota(repo: AIRepository, org_id: str, mode: str) -> str:
    settings = get_settings()
    now = datetime.now(UTC)
    plan = await repo.get_plan_code(org_id)
    if not settings.ai_enforce_plan_limits:
        return plan
    limits = PLAN_LIMITS.get(plan, PLAN_LIMITS["starter"])
    usage = await repo.get_month_usage(org_id, now.year, now.month)

    if mode == "external" and not bool(limits["external"]):
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="AI externo no disponible para este plan")

    query_limit = int(limits["queries"])
    if query_limit != -1 and usage["queries"] >= query_limit:
        raise HTTPException(
            status_code=status.HTTP_429_TOO_MANY_REQUESTS,
            detail=f"Limite mensual alcanzado ({query_limit} consultas)",
        )

    return plan
