from __future__ import annotations

from collections.abc import Callable
from typing import Any

from src.backend_client.auth import AuthContext
from src.backend_client.client import BackendClient
from src.core.dossier import add_learned_context, set_preference, update_business_field
from src.core.onboarding import BUSINESS_PROFILES, apply_profile, complete_step, skip_step
from src.tools import settings
from src.tools.registry_common import ToolHandler, tool
from runtime.types import ToolDeclaration

AddTool = Callable[[list[ToolDeclaration], dict[str, ToolHandler], str, list[str], ToolDeclaration, ToolHandler], None]


def register_profile_tools(
    *,
    declarations: list[ToolDeclaration],
    handlers: dict[str, ToolHandler],
    role: str,
    modules_active: list[str],
    client: BackendClient,
    auth: AuthContext,
    dossier: dict[str, Any],
    add_tool: AddTool,
) -> None:
    async def _complete_onboarding_step(tenant_id: str, step: str) -> dict[str, Any]:
        _ = tenant_id
        complete_step(dossier, step)
        current = dossier.get("onboarding", {}).get("current_step", "")
        return {"ok": True, "current_step": current, "completed": dossier.get("onboarding", {}).get("steps_completed", [])}

    async def _skip_onboarding_step(tenant_id: str, step: str) -> dict[str, Any]:
        _ = tenant_id
        skip_step(dossier, step)
        current = dossier.get("onboarding", {}).get("current_step", "")
        return {"ok": True, "current_step": current, "skipped": dossier.get("onboarding", {}).get("steps_skipped", [])}

    async def _apply_business_profile(tenant_id: str, profile: str) -> dict[str, Any]:
        _ = tenant_id
        if profile not in BUSINESS_PROFILES:
            available = list(BUSINESS_PROFILES.keys())
            return {"error": f"Perfil desconocido. Opciones: {available}"}
        apply_profile(dossier, profile)
        return {"ok": True, "profile": profile, "modules_active": dossier.get("modules_active", [])}

    async def _update_business_info(
        tenant_id: str,
        business_name: str | None = None,
        business_tax_id: str | None = None,
        business_address: str | None = None,
        business_phone: str | None = None,
        default_currency: str | None = None,
        default_tax_rate: float | None = None,
        scheduling_enabled: bool | None = None,
    ) -> dict[str, Any]:
        _ = tenant_id
        field_map = {
            "name": business_name,
            "tax_id": business_tax_id,
            "address": business_address,
            "phone": business_phone,
            "currency": default_currency,
            "tax_rate": default_tax_rate,
        }
        for key, val in field_map.items():
            if val is not None:
                update_business_field(dossier, key, val)
        if scheduling_enabled is not None:
            set_preference(dossier, "scheduling_enabled", scheduling_enabled)
        return await settings.update_business_info(
            client,
            auth,
            business_name=business_name,
            business_tax_id=business_tax_id,
            business_address=business_address,
            business_phone=business_phone,
            default_currency=default_currency,
            default_tax_rate=default_tax_rate,
            scheduling_enabled=scheduling_enabled,
        )

    async def _get_tenant_settings(tenant_id: str) -> dict[str, Any]:
        _ = tenant_id
        return await settings.get_tenant_settings(client, auth)

    async def _remember_fact(tenant_id: str, fact: str) -> dict[str, Any]:
        _ = tenant_id
        add_learned_context(dossier, fact)
        return {"ok": True, "total_facts": len(dossier.get("learned_context", []))}

    add_tool(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
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
    add_tool(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
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
    add_tool(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
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
    add_tool(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
            "update_business_info",
            "Actualizar datos del negocio (nombre, CUIT, direccion, telefono, moneda, impuesto, scheduling)",
            {
                "type": "object",
                "properties": {
                    "business_name": {"type": "string"},
                    "business_tax_id": {"type": "string"},
                    "business_address": {"type": "string"},
                    "business_phone": {"type": "string"},
                    "default_currency": {"type": "string", "description": "ARS, USD, etc"},
                    "default_tax_rate": {"type": "number", "description": "21.0 para IVA standard"},
                    "scheduling_enabled": {"type": "boolean"},
                },
            },
        ),
        _update_business_info,
    )
    add_tool(
        declarations,
        handlers,
        role,
        modules_active,
        tool("get_tenant_settings", "Obtener configuracion actual del negocio", {"type": "object", "properties": {}}),
        _get_tenant_settings,
    )
    add_tool(
        declarations,
        handlers,
        role,
        modules_active,
        tool(
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
