#!/usr/bin/env python3
"""Audit CRUD contract drift without changing application behavior.

The goal is to make the current state visible before refactoring. The audit is
static and conservative: it inspects the visible CRUD handlers and frontend
resource ids, then reports route, handler, list response and error-shape drift.
It intentionally does not call runtime services or mutate data.
"""

from __future__ import annotations

import argparse
import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]


CANONICAL_METHODS = (
    "List",
    "ListArchived",
    "Create",
    "Get",
    "Update",
    "Delete",
    "Archive",
    "Restore",
    "HardDelete",
)

CRUDPATHS = {
    "crudpaths.SegmentArchived": "archived",
    "crudpaths.SegmentArchive": "archive",
    "crudpaths.SegmentRestore": "restore",
    "crudpaths.SegmentHard": "hard",
}

CANONICAL_ROUTES = (
    ("List", "GET", "{base}"),
    ("ListArchived", "GET", "{base}/archived"),
    ("Create", "POST", "{base}"),
    ("Get", "GET", "{base}/:id"),
    ("Update", "PATCH", "{base}/:id"),
    ("Delete", "DELETE", "{base}/:id"),
    ("Archive", "POST", "{base}/:id/archive"),
    ("Restore", "POST", "{base}/:id/restore"),
    ("HardDelete", "DELETE", "{base}/:id/hard"),
)


@dataclass(frozen=True)
class CrudResource:
    name: str
    group: str
    handler: str
    base_path: str
    frontend_ids: tuple[str, ...] = ()
    expected: tuple[str, ...] = CANONICAL_METHODS
    note: str = ""


RESOURCES: tuple[CrudResource, ...] = (
    CrudResource("customers", "core", "pymes-core/backend/internal/customers/handler.go", "/customers", ("customers",)),
    CrudResource("suppliers", "core", "pymes-core/backend/internal/suppliers/handler.go", "/suppliers", ("suppliers",)),
    CrudResource("products", "core", "pymes-core/backend/internal/products/handler.go", "/products", ("products",)),
    CrudResource("services", "core", "pymes-core/backend/internal/services/handler.go", "/services", ("services",)),
    CrudResource("quotes", "core", "pymes-core/backend/internal/quotes/handler.go", "/quotes", ("quotes",)),
    CrudResource(
        "sales",
        "core",
        "pymes-core/backend/internal/sales/handler.go",
        "/sales",
        ("sales",),
        ("List", "Create", "Get", "Update"),
        "void replaces archive semantics",
    ),
    CrudResource("purchases", "core", "pymes-core/backend/internal/purchases/handler.go", "/purchases", ("purchases",)),
    CrudResource("invoices", "core", "pymes-core/backend/internal/invoices/handler.go", "/invoices", ("invoices",)),
    CrudResource("employees", "core", "pymes-core/backend/internal/employees/handler.go", "/employees", ("employees",)),
    CrudResource(
        "inventory",
        "core",
        "pymes-core/backend/internal/inventory/handler.go",
        "/inventory",
        ("inventory",),
        ("List", "Adjust", "ListMovements", "LowStock"),
        "stock view with adjustment/movement actions, not entity CRUD",
    ),
    CrudResource("cashflow", "core", "pymes-core/backend/internal/cashflow/handler.go", "/cashflow", ("cashflow",)),
    CrudResource(
        "returns",
        "core",
        "pymes-core/backend/internal/returns/handler.go",
        "/returns",
        ("returns",),
        ("List", "ListArchived", "Get", "Update", "Delete", "Archive", "Restore", "HardDelete"),
        "create is sale-scoped via POST /sales/:id/return",
    ),
    CrudResource(
        "credit-notes",
        "core",
        "pymes-core/backend/internal/returns/handler.go",
        "/credit-notes",
        ("creditNotes",),
        ("ListCreditNotes", "CreateCreditNote", "GetCreditNote"),
        "sub-resource under returns",
    ),
    CrudResource(
        "recurring",
        "core",
        "pymes-core/backend/internal/recurring/handler.go",
        "/recurring-expenses",
        ("recurring",),
    ),
    CrudResource(
        "payments",
        "core",
        "pymes-core/backend/internal/payments/handler.go",
        "/payments",
        ("payments",),
        ("List", "Get", "Update", "Delete", "Archive", "Restore", "HardDelete"),
        "currently sale-scoped/global-list compatibility",
    ),
    CrudResource(
        "auto-vehicles",
        "workshops",
        "workshops/backend/internal/auto_repair/vehicles/handler.go",
        "/vehicles",
        ("workshopVehicles",),
    ),
    CrudResource(
        "bike-bicycles",
        "workshops",
        "workshops/backend/internal/bike_shop/bicycles/handler.go",
        "/bicycles",
    ),
    CrudResource(
        "work-orders",
        "workshops",
        "workshops/backend/internal/workorders/handler.go",
        "/work-orders",
        ("carWorkOrders", "bikeWorkOrders"),
    ),
    CrudResource(
        "restaurant-areas",
        "restaurants",
        "restaurants/backend/internal/dining/areas/handler.go",
        "/dining-areas",
        ("restaurantDiningAreas",),
    ),
    CrudResource(
        "restaurant-tables",
        "restaurants",
        "restaurants/backend/internal/dining/tables/handler.go",
        "/dining-tables",
        ("restaurantDiningTables",),
    ),
    CrudResource(
        "restaurant-table-sessions",
        "restaurants",
        "restaurants/backend/internal/dining/sessions/handler.go",
        "/table-sessions",
        (),
        ("List", "Open", "Close"),
        "open/close lifecycle, not pure archive CRUD",
    ),
    CrudResource(
        "professional-profiles",
        "professionals",
        "professionals/backend/internal/teachers/professional_profiles/handler.go",
        "/professionals",
        ("professionals", "teachers"),
    ),
    CrudResource(
        "professional-specialties",
        "professionals",
        "professionals/backend/internal/teachers/specialties/handler.go",
        "/specialties",
        ("specialties",),
    ),
    CrudResource(
        "professional-intakes",
        "professionals",
        "professionals/backend/internal/teachers/intakes/handler.go",
        "/intakes",
        ("intakes",),
    ),
    CrudResource(
        "professional-sessions",
        "professionals",
        "professionals/backend/internal/teachers/sessions/handler.go",
        "/sessions",
        ("sessions",),
    ),
)


def read(path: str) -> str:
    full_path = ROOT / path
    if not full_path.exists():
        return ""
    return full_path.read_text(encoding="utf-8")


def has_method(source: str, method: str) -> bool:
    return re.search(rf"func \(h \*Handler\) {re.escape(method)}\(", source) is not None


def has_error_shape_drift(source: str) -> bool:
    return 'gin.H{"error"' in source or 'gin.H{ "error"' in source


def has_canonical_list_shape(source: str) -> bool:
    return (
        ("has_more" in source and "next_cursor" in source)
        or ("HasMore" in source and "NextCursor" in source)
        or "WriteListResponse" in source
        or "WriteOffsetListResponse" in source
    )


def extract_consts(source: str) -> dict[str, str]:
    consts: dict[str, str] = {}
    for name, value in re.findall(r"^\s*const\s+(\w+)\s*=\s*([^\n]+)", source, re.MULTILINE):
        parsed = eval_go_path_expr(value, consts)
        if parsed is not None:
            consts[name] = parsed
    return consts


def eval_go_path_expr(expr: str, consts: dict[str, str]) -> str | None:
    expr = expr.strip()
    if not expr:
        return ""
    parts = [part.strip() for part in expr.split("+")]
    out = ""
    for part in parts:
        if not part:
            continue
        if part.startswith('"') and part.endswith('"'):
            out += part[1:-1]
            continue
        if part in consts:
            out += consts[part]
            continue
        if part in CRUDPATHS:
            out += CRUDPATHS[part]
            continue
        return None
    return out


def extract_group_prefixes(source: str, consts: dict[str, str]) -> dict[str, str]:
    prefixes: dict[str, str] = {}
    for var_name, parent, expr in re.findall(r'(\w+)\s*:=\s*(\w+)\.Group\(([^)]*)\)', source):
        parent_prefix = prefixes.get(parent, "")
        path = eval_go_path_expr(expr, consts)
        if path is not None:
            prefixes[var_name] = normalize_path(parent_prefix + path)
    return prefixes


def normalize_path(path: str) -> str:
    path = re.sub(r"/+", "/", path.strip())
    if not path.startswith("/"):
        path = "/" + path
    if len(path) > 1 and path.endswith("/"):
        path = path[:-1]
    return path


def extract_routes(source: str) -> list[dict[str, str]]:
    consts = extract_consts(source)
    groups = extract_group_prefixes(source, consts)
    routes: list[dict[str, str]] = []
    route_pattern = re.compile(
        r'(?P<receiver>\w+)\.(?P<method>GET|POST|PATCH|DELETE)\((?P<path>[^,\n]+).*?h\.(?P<handler>\w+)',
        re.DOTALL,
    )
    for match in route_pattern.finditer(source):
        path = eval_go_path_expr(match.group("path"), consts)
        if path is None:
            path = "<dynamic>"
        prefix = groups.get(match.group("receiver"), "")
        routes.append(
            {
                "method": match.group("method"),
                "path": normalize_path(prefix + path) if path != "<dynamic>" else path,
                "handler": match.group("handler"),
            }
        )
    return routes


def expected_routes(resource: CrudResource) -> list[dict[str, str]]:
    expected = set(resource.expected)
    routes = []
    for handler, method, path_template in CANONICAL_ROUTES:
        if handler not in expected:
            continue
        routes.append(
            {
                "handler": handler,
                "method": method,
                "path": path_template.format(base=resource.base_path),
            }
        )
    return routes


def load_frontend_resource_ids() -> set[str]:
    ids: set[str] = set()
    for path in (ROOT / "frontend/src/crud").glob("resourceConfigs*.tsx"):
        text = path.read_text(encoding="utf-8")
        for match in re.finditer(r"^\s*([A-Za-z][A-Za-z0-9_]*)\s*:", text, re.MULTILINE):
            ids.add(match.group(1))
    return ids


def route_key(route: dict[str, str]) -> tuple[str, str, str]:
    return route["handler"], route["method"], route["path"]


def audit_resource(resource: CrudResource, frontend_ids: set[str]) -> dict[str, Any]:
    source = read(resource.handler)
    if not source:
        return {
            "name": resource.name,
            "group": resource.group,
            "handler": resource.handler,
            "base_path": resource.base_path,
            "missing_handlers": list(resource.expected),
            "missing_routes": [f"{r['method']} {r['path']} -> {r['handler']}" for r in expected_routes(resource)],
            "extra_routes": [],
            "error_drift": "?",
            "list_shape": "?",
            "frontend": "missing" if resource.frontend_ids else "n/a",
            "note": "handler not found",
        }
    present = {method for method in resource.expected if has_method(source, method)}
    routes = extract_routes(source)
    expected = expected_routes(resource)
    route_keys = {route_key(route) for route in routes}
    expected_keys = {route_key(route) for route in expected}
    missing_routes = [route for route in expected if route_key(route) not in route_keys]
    extra_routes = [
        route
        for route in routes
        if route["path"].startswith(resource.base_path) and route_key(route) not in expected_keys
    ]
    frontend_status = "n/a"
    if resource.frontend_ids:
        missing_frontend = [resource_id for resource_id in resource.frontend_ids if resource_id not in frontend_ids]
        frontend_status = "ok" if not missing_frontend else "missing:" + ",".join(missing_frontend)
    return {
        "name": resource.name,
        "group": resource.group,
        "base_path": resource.base_path,
        "handler": resource.handler,
        "frontend_ids": list(resource.frontend_ids),
        "missing_handlers": [method for method in resource.expected if method not in present],
        "missing_routes": [f"{r['method']} {r['path']} -> {r['handler']}" for r in missing_routes],
        "extra_routes": [f"{r['method']} {r['path']} -> {r['handler']}" for r in extra_routes],
        "error_drift": "yes" if has_error_shape_drift(source) else "no",
        "list_shape": "canonical" if has_canonical_list_shape(source) else "unknown",
        "frontend": frontend_status,
        "note": resource.note,
    }


def print_markdown(rows: list[dict[str, Any]]) -> None:
    print("# CRUD contract audit")
    print()
    print("This audit is informational by default. Use `make audit-crud-strict` to fail on missing handlers/routes.")
    print()
    print("| resource | group | base | frontend | missing handlers | missing routes | error drift | list shape | note |")
    print("|---|---|---|---|---|---|---|---|---|")
    for row in rows:
        missing_handlers = ", ".join(row["missing_handlers"]) if row["missing_handlers"] else "-"
        missing_routes = "<br>".join(row["missing_routes"]) if row["missing_routes"] else "-"
        print(
            f"| {row['name']} | {row['group']} | `{row['base_path']}` | {row['frontend']} | "
            f"{missing_handlers} | {missing_routes} | {row['error_drift']} | {row['list_shape']} | {row['note']} |"
        )
    missing_handler_count = sum(1 for row in rows if row["missing_handlers"])
    missing_route_count = sum(1 for row in rows if row["missing_routes"])
    drift_count = sum(1 for row in rows if row["error_drift"] == "yes")
    frontend_missing_count = sum(1 for row in rows if str(row["frontend"]).startswith("missing:"))
    print()
    print(
        "Summary: "
        f"resources={len(rows)} "
        f"missing_handlers={missing_handler_count} "
        f"missing_routes={missing_route_count} "
        f"frontend_missing={frontend_missing_count} "
        f"error_shape_drift={drift_count}"
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--format", choices=("markdown", "json"), default="markdown")
    parser.add_argument("--strict", action="store_true", help="Exit 1 when a resource misses expected handlers/routes.")
    args = parser.parse_args()

    frontend_ids = load_frontend_resource_ids()
    rows = [audit_resource(resource, frontend_ids) for resource in RESOURCES]
    if args.format == "json":
        print(json.dumps({"resources": rows}, indent=2, sort_keys=True))
    else:
        print_markdown(rows)
    if args.strict and any(row["missing_handlers"] or row["missing_routes"] for row in rows):
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
