from __future__ import annotations

import copy
import unicodedata
from dataclasses import dataclass, field
from typing import Any, Literal

from src.internal_chat.evidence import EvidenceCall, EvidencePacket
from src.internal_chat.routing import InternalRouteDecision

AnswerMode = Literal["facts_only", "analysis"]


@dataclass(frozen=True)
class DashboardLink:
    label: str
    url: str
    kind: str = "dashboard"

    def as_dict(self) -> dict[str, str]:
        return {"label": self.label, "url": self.url, "kind": self.kind}


@dataclass(frozen=True)
class DeterministicFactPack:
    used: bool
    summary: str = ""
    blocks: list[dict[str, Any]] = field(default_factory=list)
    dashboard_links: list[DashboardLink] = field(default_factory=list)

    def metadata(self) -> dict[str, Any]:
        return {
            "used": self.used,
            "summary": self.summary,
            "blocks": copy.deepcopy(self.blocks),
        }


_ANALYSIS_HINTS = (
    "accion",
    "acciones",
    "analiza",
    "analizame",
    "analizar",
    "conviene",
    "deberia",
    "explica",
    "explicame",
    "mejor",
    "oportunidad",
    "oportunidades",
    "por que",
    "prioridad",
    "prioriz",
    "que hago",
    "recomend",
    "riesgo",
    "seguimiento",
)


def classify_answer_mode(message: str, decision: InternalRouteDecision) -> AnswerMode:
    """Classify whether deterministic facts are enough or Gemini should interpret them."""

    if decision.scope == "general":
        return "analysis"
    text = _normalize(message)
    if any(hint in text for hint in _ANALYSIS_HINTS):
        return "analysis"
    return "facts_only"


def build_fact_pack(
    *,
    evidence: EvidencePacket,
    decision: InternalRouteDecision,
) -> DeterministicFactPack:
    if decision.scope == "general":
        return DeterministicFactPack(used=False)
    builders = {
        "sales_collections": _build_sales_collections_pack,
        "customers": _build_customers_pack,
        "products": _build_inventory_pack,
        "services": _build_services_pack,
        "purchases": _build_purchases_pack,
        "scheduling": _build_agenda_pack,
        "operations": _build_operations_pack,
        "employees": _build_employees_pack,
    }
    builder = builders.get(decision.scope)
    if builder is None:
        return DeterministicFactPack(used=False)
    return builder(evidence)


def _build_sales_collections_pack(evidence: EvidencePacket) -> DeterministicFactPack:
    sales_summary = _dict_data(_call_data(evidence, "get_sales_summary"))
    customer_rows = _items(_call_data(evidence, "get_sales_by_customer"))
    payment_rows = _items(_call_data(evidence, "get_sales_by_payment"))
    debtors = _items(_call_data(evidence, "get_debtors"))
    accounts = _items(_call_data(evidence, "get_account_balances"))

    total_sales = _number_from(sales_summary, "total_sales", "total", "amount", "sales_total")
    count_sales = _number_from(sales_summary, "count_sales", "count", "sales_count", "transactions")
    average_ticket = _number_from(sales_summary, "average_ticket", "avg_ticket", "average")
    debtor_rows = debtors or accounts
    debtor_total = sum(_balance(row) for row in debtor_rows)
    top_debtor = max(debtor_rows, key=_balance, default=None)

    summary_parts = [
        f"Ventas { _period_label(evidence) }: {_format_money(total_sales)}",
        f"{_format_count(count_sales)} ventas",
    ]
    if average_ticket is not None:
        summary_parts.append(f"ticket promedio {_format_money(average_ticket)}")
    if debtor_rows:
        summary_parts.append(f"{len(debtor_rows)} deudores por {_format_money(debtor_total)}")
    if top_debtor:
        summary_parts.append(f"mayor deuda: {_name(top_debtor)} ({_format_money(_balance(top_debtor))})")
    summary = ". ".join(summary_parts) + "."

    blocks: list[dict[str, Any]] = [
        {
            "type": "kpi_group",
            "title": "Resumen operativo",
            "items": [
                {"label": "Ventas", "value": _format_money(total_sales), "trend": "unknown", "context": _period_label(evidence)},
                {"label": "Operaciones", "value": _format_count(count_sales), "trend": "unknown", "context": "ventas registradas"},
                {
                    "label": "Saldo pendiente",
                    "value": _format_money(debtor_total),
                    "trend": "unknown",
                    "context": f"{len(debtor_rows)} cuentas con deuda",
                },
            ],
        },
        _table_block(
            title="Clientes con ventas",
            columns=["Cliente", "Total", "Ventas"],
            rows=[
                [
                    _name(row),
                    _format_money(_number_from(row, "total", "total_sales", "amount", "sales_total")),
                    _format_count(_number_from(row, "count", "count_sales", "sales_count", "transactions")),
                ]
                for row in customer_rows[:5]
            ],
            empty_state="No hay ventas por cliente para el periodo.",
        ),
        _table_block(
            title="Deudores",
            columns=["Cliente", "Saldo"],
            rows=[
                [_name(row), _format_money(_balance(row))]
                for row in sorted(debtor_rows, key=_balance, reverse=True)[:5]
            ],
            empty_state="No hay saldos pendientes en la evidencia.",
        ),
    ]
    if payment_rows:
        blocks.append(
            _table_block(
                title="Medios de pago",
                columns=["Medio", "Total", "Operaciones"],
                rows=[
                    [
                        _first_text(row, "payment_method", "method", "name", default="Sin medio"),
                        _format_money(_number_from(row, "total", "amount", "total_sales")),
                        _format_count(_number_from(row, "count", "transactions", "count_sales")),
                    ]
                    for row in payment_rows[:5]
                ],
                empty_state="No hay medios de pago para el periodo.",
            )
        )
    links = [
        DashboardLink("Ver dashboard", "dashboard", "dashboard"),
        DashboardLink("Ver reportes", "reports", "reports"),
    ]
    blocks.append(_links_block(links))
    return DeterministicFactPack(used=True, summary=summary, blocks=blocks, dashboard_links=links)


def _build_customers_pack(evidence: EvidencePacket) -> DeterministicFactPack:
    customers = _items(_call_data(evidence, "search_customers"))
    sales_by_customer = _items(_call_data(evidence, "get_sales_by_customer"))
    total_sales = sum(_number_from(row, "total", "total_sales", "amount") or 0 for row in sales_by_customer)
    summary = (
        f"Clientes: {len(customers)} registros leidos. "
        f"Ventas por cliente { _period_label(evidence) }: {_format_money(total_sales)} en {len(sales_by_customer)} clientes."
    )
    blocks = [
        {
            "type": "kpi_group",
            "title": "Clientes",
            "items": [
                {"label": "Clientes", "value": _format_count(len(customers)), "trend": "unknown", "context": "registros"},
                {"label": "Clientes con ventas", "value": _format_count(len(sales_by_customer)), "trend": "unknown", "context": _period_label(evidence)},
                {"label": "Ventas", "value": _format_money(total_sales), "trend": "unknown", "context": _period_label(evidence)},
            ],
        },
        _table_block(
            title="Top clientes por ventas",
            columns=["Cliente", "Total", "Ventas"],
            rows=[
                [
                    _name(row),
                    _format_money(_number_from(row, "total", "total_sales", "amount")),
                    _format_count(_number_from(row, "count", "count_sales", "transactions")),
                ]
                for row in sorted(sales_by_customer, key=lambda item: _number_from(item, "total", "total_sales", "amount") or 0, reverse=True)[:5]
            ],
            empty_state="No hay ventas por cliente para el periodo.",
        ),
    ]
    links = [DashboardLink("Ver clientes", "customers", "module"), DashboardLink("Ver reportes", "reports", "reports")]
    blocks.append(_links_block(links))
    return DeterministicFactPack(used=True, summary=summary, blocks=blocks, dashboard_links=links)


def _build_inventory_pack(evidence: EvidencePacket) -> DeterministicFactPack:
    products = _items(_call_data(evidence, "search_products"))
    low_stock = _items(_call_data(evidence, "get_low_stock"))
    sales_by_product = _items(_call_data(evidence, "get_sales_by_product"))
    summary = (
        f"Inventario: {len(products)} productos leidos. "
        f"Stock bajo: {len(low_stock)} productos. "
        f"Productos con ventas { _period_label(evidence) }: {len(sales_by_product)}."
    )
    blocks = [
        {
            "type": "kpi_group",
            "title": "Inventario",
            "items": [
                {"label": "Productos", "value": _format_count(len(products)), "trend": "unknown", "context": "registros"},
                {"label": "Stock bajo", "value": _format_count(len(low_stock)), "trend": "unknown", "context": "alertas"},
                {"label": "Con ventas", "value": _format_count(len(sales_by_product)), "trend": "unknown", "context": _period_label(evidence)},
            ],
        },
        _table_block(
            title="Stock bajo",
            columns=["Producto", "Stock", "Minimo"],
            rows=[
                [
                    _name(row, default="Producto sin nombre"),
                    _format_count(_number_from(row, "quantity", "stock", "current_stock")),
                    _format_count(_number_from(row, "min_quantity", "minimum_stock", "min_stock")),
                ]
                for row in low_stock[:5]
            ],
            empty_state="No hay productos con stock bajo en la evidencia.",
        ),
    ]
    links = [DashboardLink("Ver dashboard", "dashboard", "dashboard"), DashboardLink("Ver productos", "products", "module")]
    blocks.append(_links_block(links))
    return DeterministicFactPack(used=True, summary=summary, blocks=blocks, dashboard_links=links)


def _build_services_pack(evidence: EvidencePacket) -> DeterministicFactPack:
    services = _items(_call_data(evidence, "search_services"))
    sales_by_service = _items(_call_data(evidence, "get_sales_by_service"))
    total_sales = sum(_number_from(row, "total", "total_sales", "amount") or 0 for row in sales_by_service)
    summary = (
        f"Servicios: {len(services)} registros leidos. "
        f"Ventas por servicio { _period_label(evidence) }: {_format_money(total_sales)} en {len(sales_by_service)} servicios."
    )
    blocks = [
        {
            "type": "kpi_group",
            "title": "Servicios",
            "items": [
                {"label": "Servicios", "value": _format_count(len(services)), "trend": "unknown", "context": "registros"},
                {"label": "Con ventas", "value": _format_count(len(sales_by_service)), "trend": "unknown", "context": _period_label(evidence)},
                {"label": "Ventas", "value": _format_money(total_sales), "trend": "unknown", "context": _period_label(evidence)},
            ],
        },
        _table_block(
            title="Servicios vendidos",
            columns=["Servicio", "Total", "Ventas"],
            rows=[
                [
                    _name(row, default="Servicio sin nombre"),
                    _format_money(_number_from(row, "total", "total_sales", "amount")),
                    _format_count(_number_from(row, "count", "count_sales", "transactions")),
                ]
                for row in sales_by_service[:5]
            ],
            empty_state="No hay ventas por servicio para el periodo.",
        ),
    ]
    links = [DashboardLink("Ver servicios", "services", "module"), DashboardLink("Ver reportes", "reports", "reports")]
    blocks.append(_links_block(links))
    return DeterministicFactPack(used=True, summary=summary, blocks=blocks, dashboard_links=links)


def _build_purchases_pack(evidence: EvidencePacket) -> DeterministicFactPack:
    purchases = _items(_call_data(evidence, "get_purchases_summary"))
    requests = _items(_call_data(evidence, "list_procurement_requests"))
    suppliers = _items(_call_data(evidence, "search_suppliers"))
    total_purchases = sum(_number_from(row, "total", "amount", "total_amount") or 0 for row in purchases)
    summary = (
        f"Compras: {len(purchases)} registros por {_format_money(total_purchases)}. "
        f"Solicitudes de compra: {len(requests)}. Proveedores: {len(suppliers)}."
    )
    blocks = [
        {
            "type": "kpi_group",
            "title": "Compras",
            "items": [
                {"label": "Compras", "value": _format_count(len(purchases)), "trend": "unknown", "context": "registros"},
                {"label": "Total", "value": _format_money(total_purchases), "trend": "unknown", "context": "compras leidas"},
                {"label": "Solicitudes", "value": _format_count(len(requests)), "trend": "unknown", "context": "procurement"},
            ],
        },
        _table_block(
            title="Solicitudes de compra",
            columns=["Solicitud", "Estado", "Prioridad"],
            rows=[
                [
                    _first_text(row, "title", "name", "description", default="Solicitud"),
                    _first_text(row, "status", default="Sin estado"),
                    _first_text(row, "priority", default="-"),
                ]
                for row in requests[:5]
            ],
            empty_state="No hay solicitudes de compra en la evidencia.",
        ),
    ]
    links = [DashboardLink("Ver compras", "purchases", "module")]
    blocks.append(_links_block(links))
    return DeterministicFactPack(used=True, summary=summary, blocks=blocks, dashboard_links=links)


def _build_agenda_pack(evidence: EvidencePacket) -> DeterministicFactPack:
    bookings = _items(_call_data(evidence, "get_bookings"))
    summary = f"Agenda { _period_label(evidence) }: {len(bookings)} turnos o reservas en la evidencia."
    blocks = [
        {
            "type": "kpi_group",
            "title": "Agenda",
            "items": [
                {"label": "Turnos", "value": _format_count(len(bookings)), "trend": "unknown", "context": _period_label(evidence)}
            ],
        },
        _table_block(
            title="Turnos",
            columns=["Cliente", "Servicio", "Horario", "Estado"],
            rows=[
                [
                    _name(row),
                    _first_text(row, "service_name", "service", "resource_name", default="-"),
                    _first_text(row, "start_at", "scheduled_at", "date", "time", default="-"),
                    _first_text(row, "status", default="-"),
                ]
                for row in bookings[:5]
            ],
            empty_state="No hay turnos en el periodo consultado.",
        ),
    ]
    links = [DashboardLink("Ver agenda", "agenda", "module")]
    blocks.append(_links_block(links))
    return DeterministicFactPack(used=True, summary=summary, blocks=blocks, dashboard_links=links)


def _build_employees_pack(evidence: EvidencePacket) -> DeterministicFactPack:
    employees = _items(_call_data(evidence, "search_employees"))
    active_count = sum(1 for row in employees if _employee_status(row) == "active")
    inactive_count = sum(1 for row in employees if _employee_status(row) == "inactive")
    terminated_count = sum(1 for row in employees if _employee_status(row) == "terminated")
    summary = (
        f"Empleados: {len(employees)} registros leidos. "
        f"Activos: {active_count}. Inactivos: {inactive_count}. Baja: {terminated_count}."
    )
    blocks = [
        {
            "type": "kpi_group",
            "title": "Empleados",
            "items": [
                {"label": "Total", "value": _format_count(len(employees)), "trend": "unknown", "context": "registros"},
                {"label": "Activos", "value": _format_count(active_count), "trend": "unknown", "context": "status active"},
                {"label": "Inactivos", "value": _format_count(inactive_count + terminated_count), "trend": "unknown", "context": "inactive/terminated"},
            ],
        },
        _table_block(
            title="Listado de empleados",
            columns=["Empleado", "Puesto", "Estado", "Email"],
            rows=[
                [
                    _employee_name(row),
                    _first_text(row, "position", default="-"),
                    _employee_status(row) or "-",
                    _first_text(row, "email", default="-"),
                ]
                for row in employees[:10]
            ],
            empty_state="No hay empleados cargados en la evidencia.",
        ),
    ]
    links = [DashboardLink("Ver empleados", "employees", "module")]
    blocks.append(_links_block(links))
    return DeterministicFactPack(used=True, summary=summary, blocks=blocks, dashboard_links=links)


def _build_operations_pack(evidence: EvidencePacket) -> DeterministicFactPack:
    sales = _build_sales_collections_pack(evidence)
    inventory = _build_inventory_pack(evidence)
    requests = _items(_call_data(evidence, "list_procurement_requests"))
    summary = f"{sales.summary} Stock bajo: {len(_items(_call_data(evidence, 'get_low_stock')))}. Solicitudes de compra: {len(requests)}."
    links = [DashboardLink("Ver dashboard", "dashboard", "dashboard"), DashboardLink("Ver reportes", "reports", "reports")]
    blocks = [
        *(sales.blocks[:3]),
        *(inventory.blocks[1:2]),
        _table_block(
            title="Solicitudes de compra",
            columns=["Solicitud", "Estado", "Prioridad"],
            rows=[
                [
                    _first_text(row, "title", "name", "description", default="Solicitud"),
                    _first_text(row, "status", default="Sin estado"),
                    _first_text(row, "priority", default="-"),
                ]
                for row in requests[:5]
            ],
            empty_state="No hay solicitudes de compra en la evidencia.",
        ),
        _links_block(links),
    ]
    return DeterministicFactPack(used=True, summary=summary, blocks=blocks, dashboard_links=links)


def _links_block(links: list[DashboardLink]) -> dict[str, Any]:
    return {
        "type": "actions",
        "actions": [
            {
                "id": f"open_{link.kind}_{index}",
                "kind": "open_url",
                "label": link.label,
                "url": link.url,
                "style": "ghost",
                "confirmed_actions": [],
            }
            for index, link in enumerate(links)
        ],
    }


def _table_block(*, title: str, columns: list[str], rows: list[list[str]], empty_state: str) -> dict[str, Any]:
    return {"type": "table", "title": title, "columns": columns, "rows": rows, "empty_state": empty_state}


def _call_data(evidence: EvidencePacket, name: str) -> Any:
    for call in evidence.calls:
        if call.name == name:
            return call.data
    return None


def _items(data: Any) -> list[dict[str, Any]]:
    if isinstance(data, list):
        return [item for item in data if isinstance(item, dict)]
    if not isinstance(data, dict):
        return []
    items = data.get("items")
    if isinstance(items, list):
        return [item for item in items if isinstance(item, dict)]
    nested = data.get("data")
    if isinstance(nested, list):
        return [item for item in nested if isinstance(item, dict)]
    if isinstance(nested, dict):
        nested_items = nested.get("items")
        if isinstance(nested_items, list):
            return [item for item in nested_items if isinstance(item, dict)]
    return []


def _dict_data(data: Any) -> dict[str, Any]:
    if isinstance(data, dict) and isinstance(data.get("data"), dict):
        return data["data"]
    if isinstance(data, dict):
        return data
    return {}


def _number_from(row: dict[str, Any], *keys: str) -> float | None:
    for key in keys:
        value = row.get(key)
        parsed = _to_number(value)
        if parsed is not None:
            return parsed
    return None


def _balance(row: dict[str, Any]) -> float:
    return _number_from(row, "balance", "debt", "total_debt", "amount_due", "outstanding", "saldo") or 0


def _to_number(value: Any) -> float | None:
    if isinstance(value, bool) or value is None:
        return None
    if isinstance(value, (int, float)):
        return float(value)
    if isinstance(value, str):
        cleaned = value.strip().replace("$", "").replace(" ", "")
        if not cleaned:
            return None
        if "," in cleaned and "." in cleaned:
            cleaned = cleaned.replace(".", "").replace(",", ".")
        elif "," in cleaned:
            cleaned = cleaned.replace(",", ".")
        try:
            return float(cleaned)
        except ValueError:
            return None
    return None


def _name(row: dict[str, Any], *, default: str = "Sin nombre") -> str:
    return _first_text(
        row,
        "customer_name",
        "customer",
        "client_name",
        "entity_name",
        "display_name",
        "full_name",
        "product_name",
        "service_name",
        "supplier_name",
        "name",
        "title",
        default=default,
    )


def _employee_name(row: dict[str, Any]) -> str:
    person = row.get("person")
    if isinstance(person, dict):
        first_name = _first_text(person, "first_name", default="")
        last_name = _first_text(person, "last_name", default="")
        full_name = " ".join(part for part in (first_name, last_name) if part).strip()
        if full_name:
            return full_name
    first_name = _first_text(row, "first_name", default="")
    last_name = _first_text(row, "last_name", default="")
    full_name = " ".join(part for part in (first_name, last_name) if part).strip()
    return full_name or _name(row, default="Empleado sin nombre")


def _employee_status(row: dict[str, Any]) -> str:
    explicit = _first_text(row, "status", default="").lower()
    if explicit:
        return explicit
    roles = row.get("roles")
    if isinstance(roles, list):
        for role in roles:
            if not isinstance(role, dict) or role.get("role") != "employee":
                continue
            return "active" if role.get("is_active") is True else "inactive"
    return ""


def _first_text(row: dict[str, Any], *keys: str, default: str) -> str:
    for key in keys:
        value = row.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
        if isinstance(value, (int, float)) and not isinstance(value, bool):
            return str(value)
        if isinstance(value, dict):
            nested_name = _name(value, default="")
            if nested_name:
                return nested_name
    return default


def _format_money(value: float | int | None) -> str:
    amount = float(value or 0)
    sign = "-" if amount < 0 else ""
    amount = abs(amount)
    if amount.is_integer():
        formatted = f"{amount:,.0f}".replace(",", ".")
    else:
        formatted = f"{amount:,.2f}".replace(",", "X").replace(".", ",").replace("X", ".")
    return f"{sign}${formatted}"


def _format_count(value: float | int | None) -> str:
    amount = float(value or 0)
    if amount.is_integer():
        return f"{amount:,.0f}".replace(",", ".")
    return f"{amount:,.1f}".replace(",", "X").replace(".", ",").replace("X", ".")


def _period_label(evidence: EvidencePacket) -> str:
    if evidence.period is None or not evidence.period.label:
        return "actual"
    return evidence.period.label


def _normalize(value: str) -> str:
    decomposed = unicodedata.normalize("NFKD", value or "")
    without_accents = "".join(ch for ch in decomposed if not unicodedata.combining(ch))
    return f" {without_accents.lower()} "


def assert_fact_pack_uses_only_readonly_calls(calls: list[EvidenceCall], allowed_tools: frozenset[str]) -> None:
    """Tiny guard for unit tests and future fact-pack builders."""

    for call in calls:
        if call.name not in allowed_tools:
            raise AssertionError(f"Fact pack received non-read-only evidence call: {call.name}")
