#!/usr/bin/env python3
"""Strict postcondition assertions for the Cloud Run transaction fake."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any


def active(service: dict[str, Any]) -> list[dict[str, Any]]:
    return [
        entry
        for entry in service.get("traffic", [])
        if int(entry.get("percent", 0)) > 0
    ]


CURRENT_CANDIDATE_TAG = "c-2222222222222222"


def release_candidate_entries(service: dict[str, Any]) -> list[dict[str, Any]]:
    return [
        entry
        for entry in service.get("traffic", [])
        if entry.get("tag") == CURRENT_CANDIDATE_TAG
    ]


def require(condition: bool, message: str, failures: list[str]) -> None:
    if not condition:
        failures.append(message)


def assert_established(state: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    services = state["services"]
    expected_invokers = {
        "pymes-v3-stg-accounting": [
            "serviceAccount:pymes-v3-worker-stg@pymes-dev-352318.iam.gserviceaccount.com"
        ],
        "pymes-v3-stg-accounting-admin": [
            "serviceAccount:pymes-v3-provision-stg@pymes-dev-352318.iam.gserviceaccount.com"
        ],
        "pymes-v3-stg-api": ["allUsers"],
        "pymes-v3-stg-fiscal": [
            "serviceAccount:pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com",
            "serviceAccount:pymes-v3-worker-stg@pymes-dev-352318.iam.gserviceaccount.com",
        ],
        "pymes-v3-stg-web": ["allUsers"],
        "pymes-v3-stg-worker": [],
    }
    expected_ingress = {
        "pymes-v3-stg-accounting": "internal",
        "pymes-v3-stg-accounting-admin": "internal",
        "pymes-v3-stg-api": "all",
        "pymes-v3-stg-fiscal": "internal",
        "pymes-v3-stg-web": "all",
        "pymes-v3-stg-worker": "internal",
    }
    expected_scaling = {
        name: {"count": 0, "mode": "automatic"} for name in services
    }
    expected_scaling["pymes-v3-stg-worker"] = {"count": 1, "mode": "manual"}
    for name, service in services.items():
        require(service.get("exists", False), f"{name}: service is absent", failures)
        expected = f"{name}-old"
        current = active(service)
        require(
            len(current) == 1
            and current[0].get("revisionName") == expected
            and int(current[0].get("percent", 0)) == 100,
            f"{name}: previous revision is not exactly active at 100%",
            failures,
        )
        require(
            not release_candidate_entries(service),
            f"{name}: candidate tag remains reachable",
            failures,
        )
        require(
            f"{name}-candidate-1" not in service.get("revisions", {}),
            f"{name}: candidate revision was not retired",
            failures,
        )
        require(
            service.get("invoker_iam_check") is True,
            f"{name}: invoker IAM check is disabled",
            failures,
        )
        require(
            service.get("invokers", []) == expected_invokers[name],
            f"{name}: invoker policy drifted: {service.get('invokers')}",
            failures,
        )
        require(
            service.get("ingress") == expected_ingress[name],
            f"{name}: ingress drifted: {service.get('ingress')}",
            failures,
        )
        require(
            service.get("scaling") == expected_scaling[name],
            f"{name}: scaling drifted: {service.get('scaling')}",
            failures,
        )
    api = services["pymes-v3-stg-api"]
    api_tags = {
        entry.get("tag"): entry.get("revisionName")
        for entry in api["traffic"]
        if entry.get("tag")
    }
    require(
        api_tags
        == {
            "c-1111111111111111":
            "pymes-v3-stg-api-old"
        },
        f"api: previous tag was not restored exactly: {api_tags}",
        failures,
    )
    web = services["pymes-v3-stg-web"]
    old_web = web["revisions"]["pymes-v3-stg-web-old"]
    require(
        old_web.get("environment", {}).get("PYMES_API_UPSTREAM")
        == "https://c-1111111111111111---pymes-v3-stg-api.us-central1.run.fake",
        "web: previous revision no longer points to the previous API tag",
        failures,
    )
    old_token = "a" * 64
    require(
        old_web.get("environment", {}).get("PYMES_PREFLIGHT_TOKEN") == old_token,
        "web: previous preflight capability changed",
        failures,
    )
    old_api = api["revisions"]["pymes-v3-stg-api-old"]
    require(
        old_api.get("environment", {}).get("PYMES_PREFLIGHT_TOKEN") == old_token,
        "api: previous preflight capability changed",
        failures,
    )
    worker = services["pymes-v3-stg-worker"]
    require(
        worker.get("scaling") == {"count": 1, "mode": "manual"},
        f"worker: previous manual scaling was not restored: {worker.get('scaling')}",
        failures,
    )
    return failures


def assert_inert(state: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    for name, service in state["services"].items():
        require(
            service.get("exists", False) is False,
            f"{name}: first-release rollback did not remove the inert service",
            failures,
        )
        require(
            not release_candidate_entries(service),
            f"{name}: rollback left a candidate tag",
            failures,
        )
    return failures


def assert_iam_incomplete(state: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    services = state["services"]
    for name, service in services.items():
        require(service.get("exists") is True, f"{name}: service disappeared", failures)
        require(not active(service), f"{name}: active traffic remains", failures)
        require(
            not release_candidate_entries(service),
            f"{name}: candidate tag remains",
            failures,
        )
        require(
            f"{name}-candidate-1" not in service.get("revisions", {}),
            f"{name}: candidate revision remains",
            failures,
        )
        require(
            service.get("invoker_iam_check") is True,
            f"{name}: invoker IAM check is disabled",
            failures,
        )
        if name == "pymes-v3-stg-api":
            require(
                service.get("ingress") == "internal",
                "api: fail-close did not restrict ingress",
                failures,
            )
            require(
                service.get("invokers") == ["allUsers"],
                f"api: expected unresolved allUsers binding, got {service.get('invokers')}",
                failures,
            )
        else:
            require(
                service.get("invokers") == [],
                f"{name}: unexpected invokers {service.get('invokers')}",
                failures,
            )
    return failures


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit(
            "usage: assert-state.py established|inert|iam-incomplete STATE.json"
        )
    state = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
    if sys.argv[1] == "established":
        failures = assert_established(state)
    elif sys.argv[1] == "inert":
        failures = assert_inert(state)
    elif sys.argv[1] == "iam-incomplete":
        failures = assert_iam_incomplete(state)
    else:
        raise SystemExit(f"unknown state assertion: {sys.argv[1]}")
    if failures:
        for failure in failures:
            print(failure, file=sys.stderr)
        raise SystemExit(1)


if __name__ == "__main__":
    main()
