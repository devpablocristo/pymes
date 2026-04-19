from __future__ import annotations

from types import SimpleNamespace

from runtime.contexts import AuthContext

from src.agents.policy import build_internal_procurement_policy, build_internal_sales_policy
from src.tools.registry import build_internal_tools


def _auth(*, role: str) -> AuthContext:
    return AuthContext(
        tenant_id="11111111-1111-1111-1111-111111111111",
        actor="22222222-2222-2222-2222-222222222222",
        role=role,
        scopes=[],
        mode="internal",
    )


def test_internal_sales_policy_filters_tools_by_role_and_modules() -> None:
    policy = build_internal_sales_policy(_auth(role="seller"), ["products", "sales"])

    assert "search_products" in policy.allowed_tools
    assert "create_sale" in policy.allowed_tools
    assert "create_quote" not in policy.allowed_tools
    assert "get_account_balances" not in policy.allowed_tools


def test_internal_procurement_policy_uses_same_shared_matrix() -> None:
    policy = build_internal_procurement_policy(_auth(role="accountant"), ["purchases", "inventory"])

    assert policy.allowed_tools == frozenset(
        {
            "list_procurement_requests",
            "get_procurement_request",
            "get_purchases",
        }
    )


def test_registry_legacy_internal_tools_use_shared_access_rules() -> None:
    declarations, _handlers = build_internal_tools(
        SimpleNamespace(),
        _auth(role="seller"),
        {"modules_active": ["sales", "products", "customers"]},
    )

    names = {item.name for item in declarations}
    assert "search_customers" in names
    assert "search_products" in names
    assert "create_sale" in names
    assert "search_help" in names
    assert "get_cashflow_summary" not in names
    assert "create_quote" not in names
