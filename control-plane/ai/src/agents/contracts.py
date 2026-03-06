from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

ContractIntent = Literal[
    "request_quote",
    "quote_response",
    "counter_offer",
    "offer_acceptance",
    "offer_rejection",
    "availability_request",
    "availability_response",
    "payment_request",
    "reservation_request",
]
ContractChannel = Literal["api", "web_public", "whatsapp", "embedded"]


class ContractItem(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    product_id: str | None = Field(default=None, max_length=64)
    sku: str | None = Field(default=None, max_length=64)
    name: str = Field(min_length=1, max_length=200)
    quantity: float = Field(gt=0)
    unit_price: float | None = Field(default=None, ge=0)
    currency: str | None = Field(default=None, min_length=3, max_length=8)
    metadata: dict[str, Any] = Field(default_factory=dict)


class CommercialContractPayload(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    request_id: str = Field(min_length=8, max_length=120)
    org_id: str = Field(min_length=1, max_length=64)
    counterparty_id: str = Field(min_length=1, max_length=120)
    intent: ContractIntent
    items: list[ContractItem] = Field(default_factory=list)
    quantities: dict[str, float] = Field(default_factory=dict)
    currency: str = Field(min_length=3, max_length=8)
    price_terms: str | None = Field(default=None, max_length=255)
    payment_terms: str | None = Field(default=None, max_length=255)
    delivery_terms: str | None = Field(default=None, max_length=255)
    valid_until: datetime | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)
    signature: str | None = Field(default=None, max_length=512)
    channel: ContractChannel = "api"
    timestamp: datetime = Field(default_factory=lambda: datetime.now(UTC))

    @field_validator("request_id")
    @classmethod
    def validate_request_id(cls, value: str) -> str:
        allowed = set("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_:.")
        if any(ch not in allowed for ch in value):
            raise ValueError("request_id contains invalid characters")
        return value

    @field_validator("quantities")
    @classmethod
    def validate_quantities(cls, value: dict[str, float]) -> dict[str, float]:
        for key, quantity in value.items():
            if not str(key).strip():
                raise ValueError("quantities keys cannot be empty")
            if quantity <= 0:
                raise ValueError("quantities must be greater than zero")
        return value

    @field_validator("timestamp")
    @classmethod
    def validate_timestamp(cls, value: datetime) -> datetime:
        now = datetime.now(UTC)
        ts = value.astimezone(UTC) if value.tzinfo else value.replace(tzinfo=UTC)
        if ts > now + timedelta(minutes=5):
            raise ValueError("timestamp cannot be more than 5 minutes in the future")
        if ts < now - timedelta(hours=24):
            raise ValueError("timestamp is too old")
        return ts

    @model_validator(mode="after")
    def validate_semantics(self) -> "CommercialContractPayload":
        intents_requiring_items = {"request_quote", "quote_response", "counter_offer"}
        intents_requiring_schedule = {"reservation_request", "availability_request"}
        if self.intent in intents_requiring_items and not self.items:
            raise ValueError("items are required for this intent")
        if self.intent in intents_requiring_schedule and "date" not in self.metadata and "start_at" not in self.metadata:
            raise ValueError("metadata.date or metadata.start_at is required for this intent")
        return self


class CommercialContractEnvelope(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)

    contract: CommercialContractPayload
    confirmed_actions: list[str] = Field(default_factory=list)
    contact_name: str | None = Field(default=None, max_length=120)
    contact_phone: str | None = Field(default=None, max_length=32)

    @field_validator("confirmed_actions")
    @classmethod
    def normalize_actions(cls, value: list[str]) -> list[str]:
        normalized: list[str] = []
        seen: set[str] = set()
        for item in value:
            action = str(item).strip().lower()
            if not action or action in seen:
                continue
            seen.add(action)
            normalized.append(action)
        return normalized
