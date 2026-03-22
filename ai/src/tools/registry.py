from __future__ import annotations

from collections.abc import Awaitable, Callable
from typing import Any

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.core.dossier import add_learned_context, set_preference, update_business_field
from src.core.onboarding import BUSINESS_PROFILES, apply_profile, complete_step, skip_step
from pymes_core_shared.ai_runtime import ToolDeclaration
from src.tools import (
    accounts,
    appointments,
    cashflow,
    currency,
    customers,
    help,
    inventory,
    payments,
    products,
    purchases,
    quotes,
    recurring,
    sales,
    settings,
)

ToolHandler = Callable[..., Awaitable[dict[str, Any]]]

ROLE_TOOL_ACCESS: dict[str, str | list[str]] = {
    "admin": "*",
    "vendedor": [
        "search_customers",
        "search_products",
        "get_appointments",
        "check_availability",
        "book_appointment",
        "get_quotes",
        "create_quote",
        "create_sale",
        "get_account_balances",
        "get_low_stock",
        "generate_payment_link",
        "get_payment_status",
        "send_payment_info",
        "search_help",
    ],
    "cajero": [
        "get_recent_sales",
        "create_sale",
        "get_cashflow_summary",
        "create_cash_movement",
        "search_customers",
        "get_account_balances",
        "get_appointments",
        "generate_payment_link",
        "get_payment_status",
        "send_payment_info",
        "search_help",
    ],
    "contador": [
        "get_sales_summary",
        "get_cashflow_summary",
        "get_account_balances",
        "get_purchases",
        "get_recurring_expenses",
        "get_debtors",
        "get_exchange_rates",
        "get_payment_status",
        "search_help",
    ],
    "almacenero": [
        "search_products",
        "get_low_stock",
        "get_stock_level",
        "get_purchases",
        "search_help",
    ],
}

TOOL_MODULES: dict[str, set[str]] = {
    "get_sales_summary": {"sales"},
    "get_recent_sales": {"sales"},
    "get_top_customers": {"customers", "sales"},
    "search_customers": {"customers"},
    "search_products": {"products"},
    "get_low_stock": {"inventory", "products"},
    "get_stock_level": {"inventory", "products"},
    "get_cashflow_summary": {"cashflow"},
    "get_account_balances": {"accounts"},
    "get_debtors": {"accounts"},
    "get_appointments": {"appointments"},
    "check_availability": {"appointments"},
    "get_quotes": {"quotes"},
    "get_purchases": {"purchases"},
    "get_recurring_expenses": {"recurring"},
    "get_exchange_rates": {"currency"},
    "create_quote": {"quotes"},
    "create_sale": {"sales"},
    "book_appointment": {"appointments"},
    "create_cash_movement": {"cashflow"},
    "generate_payment_link": {"sales"},
    "get_payment_status": {"sales"},
    "send_payment_info": {"sales"},
    "complete_onboarding_step": set(),
    "skip_onboarding_step": set(),
    "apply_business_profile": set(),
    "update_business_info": set(),
    "get_tenant_settings": set(),
    "remember_fact": set(),
    "search_help": set(),
}


def _tool(name: str, description: str, parameters: dict[str, Any]) -> ToolDeclaration:
    return ToolDeclaration(name=name, description=description, parameters=parameters)


def _is_allowed_by_role(role: str, tool_name: str) -> bool:
    allowed = ROLE_TOOL_ACCESS.get((role or "").strip().lower(), ["search_help"])
    if allowed == "*":
        return True
    return tool_name in allowed


def _is_allowed_by_modules(modules_active: list[str], tool_name: str) -> bool:
    if not modules_active:
        return True
    required = TOOL_MODULES.get(tool_name)
    if not required:
        return True
    active = {m.strip().lower() for m in modules_active if isinstance(m, str)}
    return bool(required.intersection(active))


def _maybe_add(
    declarations: list[ToolDeclaration],
    handlers: dict[str, ToolHandler],
    role: str,
    modules_active: list[str],
    declaration: ToolDeclaration,
    handler: ToolHandler,
) -> None:
    if not _is_allowed_by_role(role, declaration.name):
        return
    if not _is_allowed_by_modules(modules_active, declaration.name):
        return
    declarations.append(declaration)
    handlers[declaration.name] = handler


def build_internal_tools(
    client: BackendClient,
    auth: AuthContext,
    dossier: dict[str, Any],
) -> tuple[list[ToolDeclaration], dict[str, ToolHandler]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, ToolHandler] = {}
    role = auth.role
    modules_active = dossier.get("modules_active", []) if isinstance(dossier, dict) else []

    async def _get_sales_summary(org_id: str, period: str = "today") -> dict[str, Any]:
        _ = org_id
        return await sales.get_sales_summary(client, auth, period=period)

    async def _get_recent_sales(org_id: str, limit: int = 10) -> dict[str, Any]:
        _ = org_id
        return await sales.get_recent_sales(client, auth, limit=limit)

    async def _get_top_customers(org_id: str, from_date: str, to_date: str) -> dict[str, Any]:
        _ = org_id
        return await customers.get_top_customers(client, auth, from_date=from_date, to_date=to_date)

    async def _search_customers(org_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        _ = org_id
        return await customers.search_customers(client, auth, query=query, limit=limit)

    async def _search_products(org_id: str, query: str, limit: int = 10) -> dict[str, Any]:
        _ = org_id
        return await products.search_products(client, auth, query=query, limit=limit)

    async def _get_low_stock(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await inventory.get_low_stock(client, auth)

    async def _get_stock_level(org_id: str, product_id: str) -> dict[str, Any]:
        _ = org_id
        return await inventory.get_stock_level(client, auth, product_id=product_id)

    async def _get_cashflow_summary(org_id: str, from_date: str, to_date: str) -> dict[str, Any]:
        _ = org_id
        return await cashflow.get_cashflow_summary(client, auth, from_date=from_date, to_date=to_date)

    async def _get_account_balances(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await accounts.get_account_balances(client, auth)

    async def _get_debtors(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await accounts.get_debtors(client, auth)

    async def _get_appointments(org_id: str, from_date: str | None = None, to_date: str | None = None) -> dict[str, Any]:
        _ = org_id
        return await appointments.get_appointments(client, auth, from_date=from_date, to_date=to_date)

    async def _check_availability(org_id: str, date: str, duration: int = 60) -> dict[str, Any]:
        return await appointments.check_availability(client, org_id=org_id, date=date, duration=duration)

    async def _get_quotes(org_id: str, status: str | None = None) -> dict[str, Any]:
        _ = org_id
        return await quotes.get_quotes(client, auth, status=status)

    async def _get_purchases(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await purchases.get_purchases_summary(client, auth)

    async def _get_recurring_expenses(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await recurring.get_recurring_expenses(client, auth)

    async def _get_exchange_rates(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await currency.get_exchange_rates(client, auth)

    async def _create_quote(org_id: str, customer_name: str, items: list[dict[str, Any]], notes: str = "") -> dict[str, Any]:
        _ = org_id
        return await quotes.create_quote(client, auth, customer_name=customer_name, items=items, notes=notes)

    async def _create_sale(
        org_id: str,
        customer_name: str,
        items: list[dict[str, Any]],
        payment_method: str = "cash",
        notes: str = "",
    ) -> dict[str, Any]:
        _ = org_id
        return await sales.create_sale(
            client,
            auth,
            customer_name=customer_name,
            items=items,
            payment_method=payment_method,
            notes=notes,
        )

    async def _generate_payment_link(org_id: str, sale_id: str) -> dict[str, Any]:
        _ = org_id
        return await payments.generate_payment_link(client, auth, sale_id=sale_id)

    async def _get_payment_status(org_id: str, sale_id: str) -> dict[str, Any]:
        _ = org_id
        return await payments.get_payment_status(client, auth, sale_id=sale_id)

    async def _send_payment_info(org_id: str, sale_id: str) -> dict[str, Any]:
        _ = org_id
        return await payments.send_payment_info(client, auth, sale_id=sale_id)

    async def _book_appointment(
        org_id: str,
        customer_name: str,
        customer_phone: str,
        title: str,
        start_at: str,
        duration: int = 60,
    ) -> dict[str, Any]:
        return await appointments.book_appointment(
            client,
            org_id=org_id,
            customer_name=customer_name,
            customer_phone=customer_phone,
            title=title,
            start_at=start_at,
            duration=duration,
        )

    async def _create_cash_movement(
        org_id: str,
        movement_type: str,
        amount: float,
        category: str = "other",
        description: str = "",
    ) -> dict[str, Any]:
        _ = org_id
        return await cashflow.create_cash_movement(
            client,
            auth,
            movement_type=movement_type,
            amount=amount,
            category=category,
            description=description,
        )

    async def _complete_onboarding_step(org_id: str, step: str) -> dict[str, Any]:
        _ = org_id
        complete_step(dossier, step)
        current = dossier.get("onboarding", {}).get("current_step", "")
        return {"ok": True, "current_step": current, "completed": dossier.get("onboarding", {}).get("steps_completed", [])}

    async def _skip_onboarding_step(org_id: str, step: str) -> dict[str, Any]:
        _ = org_id
        skip_step(dossier, step)
        current = dossier.get("onboarding", {}).get("current_step", "")
        return {"ok": True, "current_step": current, "skipped": dossier.get("onboarding", {}).get("steps_skipped", [])}

    async def _apply_business_profile(org_id: str, profile: str) -> dict[str, Any]:
        _ = org_id
        if profile not in BUSINESS_PROFILES:
            available = list(BUSINESS_PROFILES.keys())
            return {"error": f"Perfil desconocido. Opciones: {available}"}
        apply_profile(dossier, profile)
        return {"ok": True, "profile": profile, "modules_active": dossier.get("modules_active", [])}

    async def _update_business_info(
        org_id: str,
        business_name: str | None = None,
        business_tax_id: str | None = None,
        business_address: str | None = None,
        business_phone: str | None = None,
        default_currency: str | None = None,
        default_tax_rate: float | None = None,
        appointments_enabled: bool | None = None,
    ) -> dict[str, Any]:
        _ = org_id
        field_map = {
            "name": business_name, "tax_id": business_tax_id,
            "address": business_address, "phone": business_phone,
            "currency": default_currency, "tax_rate": default_tax_rate,
        }
        for key, val in field_map.items():
            if val is not None:
                update_business_field(dossier, key, val)
        if appointments_enabled is not None:
            set_preference(dossier, "appointments_enabled", appointments_enabled)
        result = await settings.update_business_info(
            client, auth,
            business_name=business_name, business_tax_id=business_tax_id,
            business_address=business_address, business_phone=business_phone,
            default_currency=default_currency, default_tax_rate=default_tax_rate,
            appointments_enabled=appointments_enabled,
        )
        return result

    async def _get_tenant_settings(org_id: str) -> dict[str, Any]:
        _ = org_id
        return await settings.get_tenant_settings(client, auth)

    async def _remember_fact(org_id: str, fact: str) -> dict[str, Any]:
        _ = org_id
        add_learned_context(dossier, fact)
        return {"ok": True, "total_facts": len(dossier.get("learned_context", []))}

    async def _search_help(org_id: str, query: str) -> dict[str, Any]:
        _ = org_id
        return await help.search_help_docs(query)

    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "complete_onboarding_step",
            "Marcar un paso del onboarding como completado",
            {
                "type": "object",
                "properties": {
                    "step": {
                        "type": "string",
                        "description": "welcome, business_type, business_info, currency_setup, tax_setup, modules_setup, first_record, feature_tips",
                    }
                },
                "required": ["step"],
            },
        ),
        _complete_onboarding_step,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "skip_onboarding_step",
            "Saltar un paso del onboarding",
            {
                "type": "object",
                "properties": {
                    "step": {
                        "type": "string",
                        "description": "welcome, business_type, business_info, currency_setup, tax_setup, modules_setup, first_record, feature_tips",
                    }
                },
                "required": ["step"],
            },
        ),
        _skip_onboarding_step,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "apply_business_profile",
            "Aplicar perfil de negocio predefinido que configura modulos y preferencias",
            {
                "type": "object",
                "properties": {
                    "profile": {
                        "type": "string",
                        "description": "comercio_minorista, servicio_profesional, gastronomia, distribuidora, freelancer, otro",
                    }
                },
                "required": ["profile"],
            },
        ),
        _apply_business_profile,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "update_business_info",
            "Actualizar datos del negocio (nombre, CUIT, direccion, telefono, moneda, impuesto, turnos)",
            {
                "type": "object",
                "properties": {
                    "business_name": {"type": "string"},
                    "business_tax_id": {"type": "string"},
                    "business_address": {"type": "string"},
                    "business_phone": {"type": "string"},
                    "default_currency": {"type": "string", "description": "ARS, USD, etc"},
                    "default_tax_rate": {"type": "number", "description": "21.0 para IVA standard"},
                    "appointments_enabled": {"type": "boolean"},
                },
            },
        ),
        _update_business_info,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool("get_tenant_settings", "Obtener configuracion actual del negocio", {"type": "object", "properties": {}}),
        _get_tenant_settings,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "remember_fact",
            "Guardar un dato aprendido sobre el negocio para recordarlo en futuras conversaciones",
            {
                "type": "object",
                "properties": {"fact": {"type": "string", "description": "Dato a recordar"}},
                "required": ["fact"],
            },
        ),
        _remember_fact,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "get_sales_summary",
            "Ventas por periodo",
            {
                "type": "object",
                "properties": {"period": {"type": "string", "description": "today, week, month"}},
            },
        ),
        _get_sales_summary,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "get_recent_sales",
            "Ultimas ventas",
            {
                "type": "object",
                "properties": {"limit": {"type": "integer", "description": "max 50"}},
            },
        ),
        _get_recent_sales,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "get_top_customers",
            "Top clientes por facturacion",
            {
                "type": "object",
                "properties": {
                    "from_date": {"type": "string", "description": "YYYY-MM-DD"},
                    "to_date": {"type": "string", "description": "YYYY-MM-DD"},
                },
                "required": ["from_date", "to_date"],
            },
        ),
        _get_top_customers,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "search_customers",
            "Buscar clientes",
            {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "texto a buscar"},
                    "limit": {"type": "integer", "description": "max 100"},
                },
                "required": ["query"],
            },
        ),
        _search_customers,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "search_products",
            "Buscar productos",
            {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "texto a buscar"},
                    "limit": {"type": "integer", "description": "max 100"},
                },
                "required": ["query"],
            },
        ),
        _search_products,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool("get_low_stock", "Productos con stock bajo", {"type": "object", "properties": {}}),
        _get_low_stock,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "get_stock_level",
            "Stock de un producto",
            {
                "type": "object",
                "properties": {"product_id": {"type": "string", "description": "UUID del producto"}},
                "required": ["product_id"],
            },
        ),
        _get_stock_level,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "get_cashflow_summary",
            "Resumen de caja por rango",
            {
                "type": "object",
                "properties": {
                    "from_date": {"type": "string", "description": "YYYY-MM-DD"},
                    "to_date": {"type": "string", "description": "YYYY-MM-DD"},
                },
                "required": ["from_date", "to_date"],
            },
        ),
        _get_cashflow_summary,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool("get_account_balances", "Resumen de cuentas corrientes", {"type": "object", "properties": {}}),
        _get_account_balances,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool("get_debtors", "Clientes deudores", {"type": "object", "properties": {}}),
        _get_debtors,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "get_appointments",
            "Listar turnos",
            {
                "type": "object",
                "properties": {
                    "from_date": {"type": "string", "description": "YYYY-MM-DD"},
                    "to_date": {"type": "string", "description": "YYYY-MM-DD"},
                },
            },
        ),
        _get_appointments,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "check_availability",
            "Consultar disponibilidad de turnos",
            {
                "type": "object",
                "properties": {
                    "date": {"type": "string", "description": "YYYY-MM-DD"},
                    "duration": {"type": "integer", "description": "duracion en minutos"},
                },
                "required": ["date"],
            },
        ),
        _check_availability,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "get_quotes",
            "Listar presupuestos",
            {
                "type": "object",
                "properties": {
                    "status": {
                        "type": "string",
                        "description": "draft, sent, accepted, rejected, expired",
                    }
                },
            },
        ),
        _get_quotes,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool("get_purchases", "Resumen de compras", {"type": "object", "properties": {}}),
        _get_purchases,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool("get_recurring_expenses", "Gastos recurrentes", {"type": "object", "properties": {}}),
        _get_recurring_expenses,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool("get_exchange_rates", "Cotizaciones del dia", {"type": "object", "properties": {}}),
        _get_exchange_rates,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "create_quote",
            "Crear presupuesto",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "items": {"type": "array", "items": {"type": "object"}},
                    "notes": {"type": "string"},
                },
                "required": ["customer_name", "items"],
            },
        ),
        _create_quote,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "create_sale",
            "Crear venta",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "items": {"type": "array", "items": {"type": "object"}},
                    "payment_method": {"type": "string"},
                    "notes": {"type": "string"},
                },
                "required": ["customer_name", "items"],
            },
        ),
        _create_sale,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "generate_payment_link",
            "Generar link de pago para una venta",
            {
                "type": "object",
                "properties": {"sale_id": {"type": "string", "description": "UUID de la venta"}},
                "required": ["sale_id"],
            },
        ),
        _generate_payment_link,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "get_payment_status",
            "Consultar estado de link de pago de una venta",
            {
                "type": "object",
                "properties": {"sale_id": {"type": "string", "description": "UUID de la venta"}},
                "required": ["sale_id"],
            },
        ),
        _get_payment_status,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "send_payment_info",
            "Obtener link de WhatsApp con datos de transferencia de una venta",
            {
                "type": "object",
                "properties": {"sale_id": {"type": "string", "description": "UUID de la venta"}},
                "required": ["sale_id"],
            },
        ),
        _send_payment_info,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "book_appointment",
            "Reservar turno",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "customer_phone": {"type": "string"},
                    "title": {"type": "string"},
                    "start_at": {"type": "string", "description": "RFC3339"},
                    "duration": {"type": "integer"},
                },
                "required": ["customer_name", "customer_phone", "title", "start_at"],
            },
        ),
        _book_appointment,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "create_cash_movement",
            "Crear movimiento de caja",
            {
                "type": "object",
                "properties": {
                    "movement_type": {"type": "string", "description": "income o expense"},
                    "amount": {"type": "number"},
                    "category": {"type": "string"},
                    "description": {"type": "string"},
                },
                "required": ["movement_type", "amount"],
            },
        ),
        _create_cash_movement,
    )
    _maybe_add(
        declarations,
        handlers,
        role,
        modules_active,
        _tool(
            "search_help",
            "Buscar ayuda funcional",
            {
                "type": "object",
                "properties": {"query": {"type": "string"}},
                "required": ["query"],
            },
        ),
        _search_help,
    )

    return declarations, handlers


def build_external_tools(client: BackendClient) -> tuple[list[ToolDeclaration], dict[str, ToolHandler]]:
    declarations: list[ToolDeclaration] = []
    handlers: dict[str, ToolHandler] = {}

    async def _check_availability(org_id: str, date: str, duration: int = 60) -> dict[str, Any]:
        return await appointments.check_availability(client, org_id=org_id, date=date, duration=duration)

    async def _book_appointment(
        org_id: str,
        customer_name: str,
        customer_phone: str,
        title: str,
        start_at: str,
        duration: int = 60,
    ) -> dict[str, Any]:
        return await appointments.book_appointment(
            client,
            org_id=org_id,
            customer_name=customer_name,
            customer_phone=customer_phone,
            title=title,
            start_at=start_at,
            duration=duration,
        )

    async def _get_public_services(org_id: str, limit: int = 20) -> dict[str, Any]:
        return await products.get_public_services(client, org_id=org_id, limit=limit)

    async def _get_business_info(org_id: str) -> dict[str, Any]:
        return await client.request("GET", f"/v1/public/{org_id}/info", include_internal=True)

    async def _get_my_appointments(org_id: str, phone: str) -> dict[str, Any]:
        return await appointments.get_my_appointments(client, org_id=org_id, phone=phone)

    async def _get_payment_link(org_id: str, quote_id: str) -> dict[str, Any]:
        return await payments.get_public_quote_payment_link(client, org_id=org_id, quote_id=quote_id)

    declarations.append(
        _tool(
            "check_availability",
            "Consultar slots disponibles",
            {
                "type": "object",
                "properties": {
                    "date": {"type": "string", "description": "YYYY-MM-DD"},
                    "duration": {"type": "integer", "description": "duracion en minutos"},
                },
                "required": ["date"],
            },
        )
    )
    handlers["check_availability"] = _check_availability

    declarations.append(
        _tool(
            "book_appointment",
            "Reservar turno",
            {
                "type": "object",
                "properties": {
                    "customer_name": {"type": "string"},
                    "customer_phone": {"type": "string"},
                    "title": {"type": "string"},
                    "start_at": {"type": "string", "description": "RFC3339"},
                    "duration": {"type": "integer"},
                },
                "required": ["customer_name", "customer_phone", "title", "start_at"],
            },
        )
    )
    handlers["book_appointment"] = _book_appointment

    declarations.append(
        _tool(
            "get_public_services",
            "Listar servicios/productos publicos",
            {
                "type": "object",
                "properties": {"limit": {"type": "integer", "description": "max 100"}},
            },
        )
    )
    handlers["get_public_services"] = _get_public_services

    declarations.append(_tool("get_business_info", "Informacion del negocio", {"type": "object", "properties": {}}))
    handlers["get_business_info"] = _get_business_info

    declarations.append(
        _tool(
            "get_my_appointments",
            "Consultar turnos de un cliente por telefono",
            {
                "type": "object",
                "properties": {"phone": {"type": "string"}},
                "required": ["phone"],
            },
        )
    )
    handlers["get_my_appointments"] = _get_my_appointments

    declarations.append(
        _tool(
            "get_payment_link",
            "Obtener link de pago de un presupuesto",
            {
                "type": "object",
                "properties": {"quote_id": {"type": "string", "description": "UUID del presupuesto"}},
                "required": ["quote_id"],
            },
        )
    )
    handlers["get_payment_link"] = _get_payment_link

    return declarations, handlers
