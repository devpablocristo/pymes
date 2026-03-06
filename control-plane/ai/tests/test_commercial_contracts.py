from __future__ import annotations

from datetime import UTC, datetime

import pytest
from pydantic import ValidationError

from src.agents.contracts import CommercialContractEnvelope


def _base_contract() -> dict:
    return {
        "contract": {
            "request_id": "req-commercial-001",
            "org_id": "org-1",
            "counterparty_id": "buyer-agent-9",
            "intent": "request_quote",
            "items": [{"name": "Servicio A", "quantity": 2, "unit_price": 10, "currency": "ARS"}],
            "quantities": {"Servicio A": 2},
            "currency": "ARS",
            "price_terms": "lista",
            "payment_terms": "contado",
            "delivery_terms": "retiro",
            "metadata": {},
            "channel": "api",
            "timestamp": datetime.now(UTC).isoformat(),
        }
    }


def test_contract_schema_rejects_extra_fields() -> None:
    payload = _base_contract()
    payload["contract"]["unexpected"] = "boom"

    with pytest.raises(ValidationError):
        CommercialContractEnvelope.model_validate(payload)


def test_contract_schema_requires_items_for_quote_intents() -> None:
    payload = _base_contract()
    payload["contract"]["items"] = []

    with pytest.raises(ValidationError):
        CommercialContractEnvelope.model_validate(payload)


def test_contract_schema_normalizes_confirmed_actions() -> None:
    payload = _base_contract()
    payload["confirmed_actions"] = [" book_appointment ", "BOOK_APPOINTMENT", ""]

    envelope = CommercialContractEnvelope.model_validate(payload)

    assert envelope.confirmed_actions == ["book_appointment"]
