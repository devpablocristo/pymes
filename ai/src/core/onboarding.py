from __future__ import annotations

from typing import Any

STEPS = [
    "welcome",
    "business_type",
    "business_info",
    "currency_setup",
    "tax_setup",
    "modules_setup",
    "first_record",
    "feature_tips",
    "completed",
]

BUSINESS_PROFILES: dict[str, dict[str, Any]] = {
    "comercio_minorista": {
        "modules": ["customers", "products", "inventory", "sales", "cashflow", "suppliers"],
        "settings": {"track_stock": True, "scheduling_enabled": False},
        "description": "Kiosco, almacen, tienda de ropa, ferreteria",
    },
    "servicio_profesional": {
        "modules": ["customers", "scheduling", "sales", "cashflow"],
        "settings": {"track_stock": False, "scheduling_enabled": True},
        "description": "Peluqueria, consultorio, estudio juridico, taller",
    },
    "gastronomia": {
        "modules": ["products", "sales", "cashflow", "suppliers", "recurring"],
        "settings": {"track_stock": False, "scheduling_enabled": False},
        "description": "Restaurante, bar, delivery, cafeteria",
    },
    "distribuidora": {
        "modules": ["customers", "suppliers", "products", "inventory", "purchases", "sales", "accounts", "cashflow"],
        "settings": {"track_stock": True, "scheduling_enabled": False},
        "description": "Mayorista, distribuidor, deposito",
    },
    "freelancer": {
        "modules": ["customers", "quotes", "sales", "cashflow"],
        "settings": {"track_stock": False, "scheduling_enabled": False},
        "description": "Disenador, programador, consultor, contador",
    },
    "otro": {
        "modules": ["customers", "sales", "cashflow"],
        "settings": {},
        "description": "Otro tipo de negocio",
    },
}


def get_profile(profile_name: str) -> dict[str, Any]:
    return BUSINESS_PROFILES.get(profile_name, BUSINESS_PROFILES["otro"])


def apply_profile(dossier: dict[str, Any], profile_name: str) -> dict[str, Any]:
    profile = get_profile(profile_name)
    dossier["business"]["profile"] = profile_name
    dossier["modules_active"] = list(profile["modules"])
    dossier["preferences"] = {**dossier.get("preferences", {}), **profile["settings"]}
    return dossier


def next_step(current_step: str) -> str:
    if current_step not in STEPS:
        return "welcome"
    idx = STEPS.index(current_step)
    if idx >= len(STEPS) - 1:
        return "completed"
    return STEPS[idx + 1]


def complete_step(dossier: dict[str, Any], step: str) -> dict[str, Any]:
    onboarding = dossier.setdefault("onboarding", {})
    completed = onboarding.setdefault("steps_completed", [])
    if step not in completed:
        completed.append(step)
    onboarding["current_step"] = next_step(step)
    if onboarding["current_step"] == "completed":
        onboarding["status"] = "completed"
    return dossier


def skip_step(dossier: dict[str, Any], step: str) -> dict[str, Any]:
    onboarding = dossier.setdefault("onboarding", {})
    skipped = onboarding.setdefault("steps_skipped", [])
    if step not in skipped:
        skipped.append(step)
    onboarding["current_step"] = next_step(step)
    return dossier
