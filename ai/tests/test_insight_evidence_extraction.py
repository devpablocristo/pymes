from __future__ import annotations

import json
from datetime import UTC, datetime, timedelta

from src.api.external_chat_support import (
    _EVIDENCE_TTL_HOURS,
    compact_insight_evidence_for_prompt,
    extract_insight_evidence,
)


def _make_evidence(
    scope: str = "sales_collections",
    period: str = "week",
    computed_at: str | None = None,
) -> dict:
    return {
        "source": "insight_handoff",
        "notification_id": "notif-1",
        "scope": scope,
        "period": period,
        "compare": True,
        "top_limit": 5,
        "computed_at": computed_at or datetime.now(UTC).isoformat(),
        "summary": "Ventas arriba 12% esta semana.",
        "current_period": {"label": "esta semana", "from_date": "2026-04-03", "to_date": "2026-04-10"},
        "comparison_period": {"label": "semana anterior", "from_date": "2026-03-27", "to_date": "2026-04-03"},
        "kpis": [
            {
                "key": "total_sales",
                "label": "Ventas totales",
                "unit": "currency",
                "value": 120000.0,
                "previous_value": 107000.0,
                "delta": 13000.0,
                "delta_pct": 12.1,
                "trend": "up",
            },
        ],
        "highlights": [
            {"severity": "positive", "title": "Ventas en alza", "detail": "Crecimiento sostenido."},
        ],
        "recommendations": ["Revisar stock.", "Contactar top clientes."],
        "entity_ids": ["cust-1", "cust-2", "cust-3"],
    }


def _assistant_msg(evidence: dict | None = None) -> dict:
    msg: dict = {"role": "assistant", "content": "Ventas arriba 12%."}
    if evidence is not None:
        msg["insight_evidence"] = evidence
    return msg


def _user_msg(text: str = "que implica eso?") -> dict:
    return {"role": "user", "content": text}


# --- extract_insight_evidence ---


def test_extract_returns_none_for_empty_messages() -> None:
    assert extract_insight_evidence([]) is None


def test_extract_returns_none_when_no_evidence() -> None:
    messages = [_user_msg(), _assistant_msg(evidence=None), _user_msg("otra cosa")]
    assert extract_insight_evidence(messages) is None


def test_extract_returns_latest_evidence() -> None:
    old_ev = _make_evidence(scope="inventory_profit")
    new_ev = _make_evidence(scope="sales_collections")
    messages = [
        _user_msg(),
        _assistant_msg(evidence=old_ev),
        _user_msg("y ahora?"),
        _assistant_msg(evidence=new_ev),
    ]
    result = extract_insight_evidence(messages)
    assert result is not None
    assert result["scope"] == "sales_collections"


def test_extract_ignores_user_messages() -> None:
    fake_user = {"role": "user", "content": "hola", "insight_evidence": _make_evidence()}
    messages = [fake_user, _assistant_msg(evidence=None)]
    assert extract_insight_evidence(messages) is None


def test_extract_respects_24h_expiration() -> None:
    old_ts = (datetime.now(UTC) - timedelta(hours=_EVIDENCE_TTL_HOURS + 1)).isoformat()
    evidence = _make_evidence(computed_at=old_ts)
    messages = [_assistant_msg(evidence=evidence)]
    assert extract_insight_evidence(messages) is None


# --- compact_insight_evidence_for_prompt ---


def test_compact_excludes_entity_ids() -> None:
    evidence = _make_evidence()
    result = compact_insight_evidence_for_prompt(evidence)
    parsed = json.loads(result)
    assert "entity_ids" not in parsed
    assert "notification_id" not in parsed
    assert "source" not in parsed
    assert "computed_at" not in parsed


def test_compact_includes_kpis_without_key() -> None:
    evidence = _make_evidence()
    result = compact_insight_evidence_for_prompt(evidence)
    parsed = json.loads(result)
    assert len(parsed["kpis"]) == 1
    kpi = parsed["kpis"][0]
    assert "key" not in kpi
    assert "previous_value" not in kpi
    assert kpi["label"] == "Ventas totales"
    assert kpi["value"] == 120000.0
    assert kpi["delta_pct"] == 12.1
    assert kpi["trend"] == "up"


def test_compact_truncates_when_large() -> None:
    evidence = _make_evidence()
    # Agregar muchos highlights y recommendations para superar 500 tokens
    evidence["highlights"] = [
        {"severity": "info", "title": f"Highlight {i}", "detail": f"Detalle extenso del highlight numero {i} con mucho texto adicional para inflar el conteo de tokens."}
        for i in range(20)
    ]
    evidence["recommendations"] = [
        f"Recomendacion numero {i} con texto largo adicional para aumentar el conteo de tokens estimado." for i in range(15)
    ]
    result = compact_insight_evidence_for_prompt(evidence)
    parsed = json.loads(result)
    assert len(parsed.get("recommendations", [])) <= 3
    assert len(parsed.get("highlights", [])) <= 5
