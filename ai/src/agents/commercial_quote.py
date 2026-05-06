from __future__ import annotations

from typing import Any

from src.backend_client.client import BackendClient
from src.tools import products

async def _match_quote_items(client: BackendClient, org_id: str, items: list[dict[str, Any]]) -> dict[str, Any]:
    catalog = await products.get_public_services(client, org_id=org_id, limit=100)
    rows = list(catalog.get("items", [])) if isinstance(catalog, dict) else []
    by_id = {str(row.get("id", "")).strip(): row for row in rows if str(row.get("id", "")).strip()}
    by_name = {str(row.get("name", "")).strip().lower(): row for row in rows if str(row.get("name", "")).strip()}

    matched: list[dict[str, Any]] = []
    missing: list[dict[str, Any]] = []
    currency = "ARS"
    total = 0.0
    for raw in items:
        product_id = str(raw.get("product_id", "")).strip()
        name = str(raw.get("name", "")).strip()
        quantity = float(raw.get("quantity", 0) or 0)
        if quantity <= 0:
            missing.append({"name": name or product_id or "item", "error": "quantity must be greater than zero"})
            continue
        row = None
        if product_id:
            row = by_id.get(product_id)
        if row is None and name:
            row = by_name.get(name.lower())
        if row is None:
            missing.append({"name": name or product_id or "item", "error": "not found in public catalog"})
            continue
        unit_price = float(row.get("price", 0) or 0)
        line_currency = str(row.get("currency", "ARS") or "ARS").upper()
        currency = line_currency
        subtotal = round(unit_price * quantity, 2)
        total = round(total + subtotal, 2)
        matched.append(
            {
                "product_id": str(row.get("id", "")),
                "name": str(row.get("name", "")),
                "quantity": quantity,
                "unit": str(row.get("unit", "unit")),
                "unit_price": unit_price,
                "currency": line_currency,
                "subtotal": subtotal,
            }
        )
    return {"items": matched, "missing": missing, "currency": currency, "total": total}


async def _build_quote_preview(client: BackendClient, org_id: str, items: list[dict[str, Any]], customer_name: str = "", notes: str = "") -> dict[str, Any]:
    matched = await _match_quote_items(client, org_id, items)
    if not matched["items"]:
        return {
            "status": "needs_human_review",
            "message": "No pude armar un presupuesto confiable con los datos recibidos.",
            "missing": matched["missing"],
        }
    return {
        "status": "preview_ready",
        "customer_name": customer_name.strip(),
        "currency": matched["currency"],
        "items": matched["items"],
        "missing": matched["missing"],
        "subtotal": matched["total"],
        "total": matched["total"],
        "notes": notes.strip(),
        "formal_quote": False,
        "next_step": "Si queres convertirlo en presupuesto formal, pedile confirmacion al cliente o deriva a un vendedor.",
    }
