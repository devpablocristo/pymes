from __future__ import annotations

from typing import Any

from src.backend_client.auth import AuthContext
from src.backend_client.http_client import HTTPBackendClient


class AutoRepairBackendClient(HTTPBackendClient):
    async def list_vehicles(self, auth: AuthContext, search: str = "") -> dict[str, Any]:
        params = {"search": search} if search else {}
        return await self.request("GET", "/v1/auto-repair/vehicles", auth=auth, params=params)

    async def get_vehicle(self, auth: AuthContext, vehicle_id: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/auto-repair/vehicles/{vehicle_id}", auth=auth)

    async def list_work_orders(self, auth: AuthContext, status: str = "", search: str = "") -> dict[str, Any]:
        params: dict[str, str] = {"target_type": "vehicle"}
        if status:
            params["status"] = status
        if search:
            params["search"] = search
        return await self.request("GET", "/v1/work-orders", auth=auth, params=params)

    async def get_work_order(self, auth: AuthContext, work_order_id: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/work-orders/{work_order_id}", auth=auth)

    async def create_vehicle(self, auth: AuthContext, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("POST", "/v1/auto-repair/vehicles", auth=auth, json=data)

    async def update_vehicle(self, auth: AuthContext, vehicle_id: str, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("PUT", f"/v1/auto-repair/vehicles/{vehicle_id}", auth=auth, json=data)

    async def update_work_order(self, auth: AuthContext, work_order_id: str, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("PATCH", f"/v1/work-orders/{work_order_id}", auth=auth, json=data)

    async def create_booking(self, auth: AuthContext, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("POST", "/v1/workshop-bookings", auth=auth, json=data)

    async def create_quote(self, auth: AuthContext, work_order_id: str) -> dict[str, Any]:
        return await self.request("POST", f"/v1/work-orders/{work_order_id}/quote", auth=auth)

    async def create_sale(self, auth: AuthContext, work_order_id: str) -> dict[str, Any]:
        return await self.request("POST", f"/v1/work-orders/{work_order_id}/sale", auth=auth)

    async def create_payment_link(self, auth: AuthContext, work_order_id: str) -> dict[str, Any]:
        return await self.request("POST", f"/v1/work-orders/{work_order_id}/payment-link", auth=auth)

    async def public_book_scheduling(self, org_slug: str, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("POST", f"/v1/public/{org_slug}/auto-repair/bookings", include_internal=True, json=data)
