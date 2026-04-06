from __future__ import annotations

from typing import Any

from src.backend_client.auth import AuthContext
from src.backend_client.http_client import HTTPBackendClient


class TeachersBackendClient(HTTPBackendClient):
    async def get_teachers(self, auth: AuthContext) -> dict[str, Any]:
        return await self.request("GET", "/v1/teachers/professionals", auth=auth)

    async def get_teacher(self, auth: AuthContext, profile_id: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/teachers/professionals/{profile_id}", auth=auth)

    async def get_specialties(self, auth: AuthContext) -> dict[str, Any]:
        return await self.request("GET", "/v1/teachers/specialties", auth=auth)

    async def get_teacher_services(self, auth: AuthContext, profile_id: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/teachers/professionals/{profile_id}/services", auth=auth)

    async def create_intake(self, auth: AuthContext, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("POST", "/v1/teachers/intakes", auth=auth, json=data)

    async def update_intake(self, auth: AuthContext, intake_id: str, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("PUT", f"/v1/teachers/intakes/{intake_id}", auth=auth, json=data)

    async def get_sessions(self, auth: AuthContext, filters: dict[str, str] | None = None) -> dict[str, Any]:
        return await self.request("GET", "/v1/teachers/sessions", auth=auth, params=filters or {})

    async def get_session(self, auth: AuthContext, session_id: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/teachers/sessions/{session_id}", auth=auth)

    async def book_scheduling(self, auth: AuthContext, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("POST", "/v1/teachers/bookings", auth=auth, json=data)

    async def prepare_quote(self, auth: AuthContext, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("POST", "/v1/teachers/quotes", auth=auth, json=data)

    async def get_payment_link(self, auth: AuthContext, sale_id: str) -> dict[str, Any]:
        return await self.request("POST", f"/v1/teachers/payments/{sale_id}/link", auth=auth)

    async def get_public_teachers(self, org_slug: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/public/{org_slug}/teachers", include_internal=True)

    async def get_public_catalog(self, org_slug: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/public/{org_slug}/teachers/catalog", include_internal=True)

    async def get_public_availability(self, org_slug: str, date: str, professional_id: str | None = None) -> dict[str, Any]:
        params: dict[str, str] = {"date": date}
        if professional_id:
            params["professional_id"] = professional_id
        return await self.request("GET", f"/v1/public/{org_slug}/teachers/availability", include_internal=True, params=params)

    async def public_book_scheduling(self, org_slug: str, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("POST", f"/v1/public/{org_slug}/teachers/bookings", include_internal=True, json=data)
