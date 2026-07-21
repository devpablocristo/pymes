#!/usr/bin/env python3
from __future__ import annotations

import subprocess
import sys
import re
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[2]
ECOSYSTEM_ROOT = ROOT.parent
AXIS_ROOT = Path(__import__("os").environ.get("AXIS_ROOT", ECOSYSTEM_ROOT / "axis")).resolve()

COMPANION_SPEC = AXIS_ROOT / "companion" / "openapi.yaml"
NEXUS_SPEC = AXIS_ROOT / "nexus" / "openapi.yaml"
PYMES_COMPANION_SPEC = ROOT / "ui" / "src" / "generated" / "companion.openapi.yaml"


COMPANION_ROUTES = [
    ("POST", "/v1/chat"),
    ("GET", "/v1/chat/conversations"),
    ("GET", "/v1/chat/conversations/{id}"),
    ("POST", "/v1/notifications"),
    ("POST", "/v1/customer-messaging/inbound"),
    ("GET", "/v1/watchers"),
    ("POST", "/v1/watchers"),
    ("PATCH", "/v1/watchers/{id}"),
]

NEXUS_ROUTES = [
    ("POST", "/v1/requests"),
    ("POST", "/v1/requests/simulate"),
    ("GET", "/v1/requests/{id}"),
    ("GET", "/v1/policies"),
    ("POST", "/v1/policies"),
    ("GET", "/v1/policies/{id}"),
    ("PATCH", "/v1/policies/{id}"),
    ("DELETE", "/v1/policies/{id}"),
    ("GET", "/v1/approvals/pending"),
    ("POST", "/v1/approvals/{id}/approve"),
    ("POST", "/v1/approvals/{id}/reject"),
    ("GET", "/v1/action-types"),
    ("POST", "/v1/action-types"),
    ("GET", "/v1/delegations"),
    ("POST", "/v1/delegations"),
]

OPERATIONAL_SCAN_PATHS = [
    ".github",
    ".env.example",
    "Makefile",
    "PROJECT_CONTEXT.md",
    "docker-compose.yml",
    ".github/ci-infra/docker-compose.yml",
    "scripts/seeds/lib.sh",
    "infra/verticals.yml",
    "firebase.json",
    "firebase.generated.json",
    "ui/cloudbuild.yaml",
]

FORBIDDEN_OPERATIONAL_REFERENCES = [
    re.compile(re.escape("../nexus")),
    re.compile(re.escape("devpablocristo/nexus")),
    re.compile(re.escape("governance-postgres")),
    re.compile(r"(?<!axis-)companion-dev"),
    re.compile(re.escape("GOVERNANCE_APPROVAL_")),
    re.compile(re.escape("GOVERNANCE_PORT")),
    re.compile(re.escape("nexus_governance")),
    re.compile(re.escape("pymes/ai")),
]

OLD_COMPANION_INTERNAL_CUSTOMER_MESSAGING_ROUTE = "/v1/" + "internal/customer-messaging/inbound"


def fail(message: str) -> int:
    print(f"ERROR: {message}", file=sys.stderr)
    return 1


def load_openapi(path: Path) -> dict:
    if not path.exists():
        raise FileNotFoundError(path)
    with path.open(encoding="utf-8") as fh:
        return yaml.safe_load(fh) or {}


def check_routes(spec: dict, expected: list[tuple[str, str]], label: str) -> list[str]:
    paths = spec.get("paths", {})
    missing: list[str] = []
    for method, path in expected:
        operations = {key.lower() for key in paths.get(path, {}) if not key.startswith("x-")}
        if method.lower() not in operations:
            missing.append(f"{label}: missing {method} {path}")
    return missing


def iter_operational_files() -> list[Path]:
    files: list[Path] = []
    for item in OPERATIONAL_SCAN_PATHS:
        path = ROOT / item
        if not path.exists():
            continue
        if path.is_file():
            files.append(path)
            continue
        files.extend(p for p in path.rglob("*") if p.is_file())
    return files


def main() -> int:
    errors: list[str] = []

    if not AXIS_ROOT.exists():
        errors.append(f"AXIS_ROOT does not exist: {AXIS_ROOT}")
    if COMPANION_SPEC.exists() and PYMES_COMPANION_SPEC.exists():
        companion_bytes = COMPANION_SPEC.read_bytes().replace(b"\r\n", b"\n")
        pymes_bytes = PYMES_COMPANION_SPEC.read_bytes().replace(b"\r\n", b"\n")
        if companion_bytes != pymes_bytes:
            errors.append("ui/src/generated/companion.openapi.yaml is not synchronized with Axis Companion")
    else:
        errors.append("Companion OpenAPI spec missing in Axis or Pymes")

    try:
        companion = load_openapi(COMPANION_SPEC)
        nexus = load_openapi(NEXUS_SPEC)
    except FileNotFoundError as exc:
        errors.append(f"OpenAPI file not found: {exc.filename}")
    else:
        errors.extend(check_routes(companion, COMPANION_ROUTES, "Axis Companion"))
        errors.extend(check_routes(nexus, NEXUS_ROUTES, "Axis Nexus"))

    tracked_ai = subprocess.run(
        ["git", "ls-files", "ai"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if tracked_ai.stdout.strip():
        errors.append("tracked pymes/ai files still exist")

    for path in iter_operational_files():
        rel = path.relative_to(ROOT)
        text = path.read_text(encoding="utf-8", errors="ignore")
        for pattern in FORBIDDEN_OPERATIONAL_REFERENCES:
            if pattern.search(text):
                errors.append(f"{rel}: forbidden operational reference {pattern.pattern!r}")

    tracked = subprocess.run(
        ["git", "ls-files"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    for raw_rel in tracked.stdout.splitlines():
        if not raw_rel or raw_rel == "scripts/audit/axis_contracts_check.py":
            continue
        path = ROOT / raw_rel
        if not path.is_file():
            continue
        text = path.read_text(encoding="utf-8", errors="ignore")
        if OLD_COMPANION_INTERNAL_CUSTOMER_MESSAGING_ROUTE in text:
            errors.append(f"{raw_rel}: forbidden Companion internal customer-messaging route")

    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        return 1
    print("OK: Pymes is aligned with Axis contracts")
    return 0


if __name__ == "__main__":
    sys.exit(main())
