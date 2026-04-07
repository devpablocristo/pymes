from __future__ import annotations

from typing import Any

from src.backend_client.auth import AuthContext
from src.backend_client.http_client import HTTPBackendClient


class BikeShopBackendClient(HTTPBackendClient):
    async def list_bicycles(self, auth: AuthContext, search: str = "") -> dict[str, Any]:
        params = {"search": search} if search else {}
        return await self.request("GET", "/v1/bike-shop/bicycles", auth=auth, params=params)

    async def get_bicycle(self, auth: AuthContext, bicycle_id: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/bike-shop/bicycles/{bicycle_id}", auth=auth)

    async def list_services(self, auth: AuthContext, search: str = "") -> dict[str, Any]:
        params = {"search": search} if search else {}
        return await self.request("GET", "/v1/bike-shop/workshop-services", auth=auth, params=params)

    async def get_service(self, auth: AuthContext, service_id: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/bike-shop/workshop-services/{service_id}", auth=auth)

    async def list_work_orders(self, auth: AuthContext, status: str = "", search: str = "") -> dict[str, Any]:
        params: dict[str, str] = {}
        if status:
            params["status"] = status
        if search:
            params["search"] = search
        return await self.request("GET", "/v1/bike-shop/work-orders", auth=auth, params=params)

    async def get_work_order(self, auth: AuthContext, work_order_id: str) -> dict[str, Any]:
        return await self.request("GET", f"/v1/bike-shop/work-orders/{work_order_id}", auth=auth)

    async def create_booking(self, auth: AuthContext, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("POST", "/v1/bike-shop/workshop-bookings", auth=auth, json=data)

    async def create_quote(self, auth: AuthContext, work_order_id: str) -> dict[str, Any]:
        return await self.request("POST", f"/v1/bike-shop/work-orders/{work_order_id}/quote", auth=auth)

    async def create_sale(self, auth: AuthContext, work_order_id: str) -> dict[str, Any]:
        return await self.request("POST", f"/v1/bike-shop/work-orders/{work_order_id}/sale", auth=auth)

    async def create_payment_link(self, auth: AuthContext, work_order_id: str) -> dict[str, Any]:
        return await self.request("POST", f"/v1/bike-shop/work-orders/{work_order_id}/payment-link", auth=auth)

    async def get_public_services(self, org_slug: str, search: str = "") -> dict[str, Any]:
        params = {"search": search} if search else {}
        return await self.request("GET", f"/v1/public/{org_slug}/bike-shop/services", include_internal=True, params=params)

    async def public_book_scheduling(self, org_slug: str, data: dict[str, Any]) -> dict[str, Any]:
        return await self.request("POST", f"/v1/public/{org_slug}/bike-shop/bookings", include_internal=True, json=data)
