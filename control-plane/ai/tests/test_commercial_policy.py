from __future__ import annotations

from src.agents.policy import build_external_sales_policy, build_internal_procurement_policy, build_internal_sales_policy
from src.backend_client.auth import AuthContext


def _auth(role: str) -> AuthContext:
    return AuthContext(org_id="org-1", actor="user-1", role=role, scopes=[], mode="internal", api_key="test")


def test_external_sales_policy_blocks_internal_tools() -> None:
    policy = build_external_sales_policy(channel="web_public")

    assert policy.allows("get_business_info")
    assert policy.allows("request_quote")
    assert not policy.allows("search_customers")
    assert not policy.allows("create_sale")
    assert policy.requires_confirmation("book_appointment")


def test_internal_sales_policy_filters_by_role_and_modules() -> None:
    policy = build_internal_sales_policy(_auth("vendedor"), ["sales", "quotes"], channel="internal_ui")

    assert policy.allows("create_quote")
    assert policy.allows("create_sale")
    assert not policy.allows("search_customers")
    assert policy.requires_confirmation("create_quote")
    assert policy.requires_confirmation("generate_payment_link")


def test_internal_procurement_policy_supports_supply_tools() -> None:
    policy = build_internal_procurement_policy(_auth("almacenero"), ["products", "inventory", "purchases", "suppliers"], channel="internal_ui")

    assert policy.allows("search_suppliers")
    assert policy.allows("prepare_purchase_draft")
    assert not policy.allows("create_sale")
