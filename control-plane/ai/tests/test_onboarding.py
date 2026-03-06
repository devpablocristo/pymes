from __future__ import annotations

from src.core.onboarding import apply_profile, complete_step, get_profile, skip_step, BUSINESS_PROFILES


def test_complete_step_advances_state() -> None:
    dossier = {"onboarding": {"status": "pending", "current_step": "welcome", "steps_completed": [], "steps_skipped": []}}
    updated = complete_step(dossier, "welcome")
    assert updated["onboarding"]["current_step"] == "business_type"
    assert "welcome" in updated["onboarding"]["steps_completed"]


def test_skip_step_registers_skip() -> None:
    dossier = {"onboarding": {"status": "pending", "current_step": "tax_setup", "steps_completed": [], "steps_skipped": []}}
    updated = skip_step(dossier, "tax_setup")
    assert updated["onboarding"]["current_step"] == "modules_setup"
    assert "tax_setup" in updated["onboarding"]["steps_skipped"]


def test_apply_profile_sets_modules() -> None:
    dossier = {"business": {}, "modules_active": [], "preferences": {}}
    updated = apply_profile(dossier, "comercio_minorista")
    assert updated["business"]["profile"] == "comercio_minorista"
    assert "sales" in updated["modules_active"]
    assert "inventory" in updated["modules_active"]
    assert updated["preferences"]["track_stock"] is True


def test_get_profile_unknown_falls_back_to_otro() -> None:
    profile = get_profile("inexistente")
    assert profile == BUSINESS_PROFILES["otro"]


def test_complete_all_steps_marks_completed() -> None:
    dossier = {"onboarding": {"status": "pending", "current_step": "welcome", "steps_completed": [], "steps_skipped": []}}
    for step in ["welcome", "business_type", "business_info", "currency_setup", "tax_setup", "modules_setup", "first_record", "feature_tips"]:
        dossier = complete_step(dossier, step)
    assert dossier["onboarding"]["status"] == "completed"
    assert dossier["onboarding"]["current_step"] == "completed"
