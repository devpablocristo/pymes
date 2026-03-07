from __future__ import annotations

from typing import Any


def summarize_dossier_for_context(dossier: dict[str, Any]) -> str:
    business = dossier.get("business", {})
    onboarding = dossier.get("onboarding", {})
    modules = dossier.get("modules_active", [])
    return (
        f"business={business}; onboarding={onboarding}; modules_active={modules}; "
        f"team={dossier.get('team', [])}"
    )


def update_business_field(dossier: dict[str, Any], key: str, value: Any) -> dict[str, Any]:
    business = dossier.setdefault("business", {})
    business[key] = value
    return dossier


def add_learned_context(dossier: dict[str, Any], fact: str) -> dict[str, Any]:
    learned = dossier.setdefault("learned_context", [])
    if fact not in learned:
        learned.append(fact)
    if len(learned) > 100:
        dossier["learned_context"] = learned[-100:]
    return dossier


def set_modules(dossier: dict[str, Any], active: list[str]) -> dict[str, Any]:
    dossier["modules_active"] = list(active)
    return dossier


def set_preference(dossier: dict[str, Any], key: str, value: Any) -> dict[str, Any]:
    prefs = dossier.setdefault("preferences", {})
    prefs[key] = value
    return dossier


def sync_business_from_settings(dossier: dict[str, Any], settings: dict[str, Any]) -> dict[str, Any]:
    business = dossier.setdefault("business", {})
    mapping = {
        "business_name": "name",
        "business_tax_id": "tax_id",
        "business_address": "address",
        "business_phone": "phone",
        "default_currency": "currency",
        "default_tax_rate": "tax_rate",
    }
    for settings_key, dossier_key in mapping.items():
        val = settings.get(settings_key)
        if val is not None and val != "":
            business[dossier_key] = val
    return dossier
