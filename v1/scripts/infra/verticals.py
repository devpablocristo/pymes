#!/usr/bin/env python3
from __future__ import annotations

import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import yaml


ROOT = Path(__file__).resolve().parents[2]
REGISTRY_PATH = ROOT / "infra" / "verticals.yml"
FIREBASE_GENERATED_PATH = ROOT / "firebase.generated.json"
FIREBASE_LEGACY_PATH = ROOT / "firebase.json"

VERTICAL_ID_RE = re.compile(r"^[a-z][a-z0-9-]*$")
CLOUD_RUN_SERVICE_RE = re.compile(r"^[a-z][a-z0-9-]{0,61}[a-z0-9]$")
ROUTE_PREFIX_RE = re.compile(r"^/v1/[a-z0-9-]+$")


class RegistryError(ValueError):
    pass


@dataclass(frozen=True)
class Vertical:
    id: str
    enabled: bool
    deploy: bool
    path: str
    dockerfile: str
    service: str
    route_prefix: str
    api_prefixes: tuple[str, ...]
    healthcheck: str

    @property
    def absolute_path(self) -> Path:
        return ROOT / self.path

    @property
    def dockerfile_path(self) -> Path:
        return self.absolute_path / self.dockerfile

    def matrix_item(self) -> dict[str, str]:
        return {
            "id": self.id,
            "path": self.path,
            "dockerfile": self.dockerfile,
            "service": self.service,
            "route_prefix": self.route_prefix,
            "healthcheck": self.healthcheck,
        }


def _fail(message: str) -> None:
    raise RegistryError(message)


def _require_mapping(value: Any, name: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        _fail(f"{name} must be a mapping")
    return value


def _require_string(value: Any, name: str) -> str:
    if not isinstance(value, str) or not value.strip():
        _fail(f"{name} must be a non-empty string")
    return value.strip()


def _require_bool(value: Any, name: str) -> bool:
    if not isinstance(value, bool):
        _fail(f"{name} must be a boolean")
    return value


def _optional_string_list(value: Any, name: str) -> list[str] | None:
    if value is None:
        return None
    if not isinstance(value, list) or not value:
        _fail(f"{name} must be a non-empty list when present")
    result: list[str] = []
    for index, item in enumerate(value):
        result.append(_require_string(item, f"{name}[{index}]"))
    return result


def load_registry(path: Path = REGISTRY_PATH) -> dict[str, Any]:
    if not path.exists():
        _fail(f"registry not found: {path.relative_to(ROOT)}")
    try:
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
    except yaml.YAMLError as exc:
        _fail(f"malformed YAML in {path.relative_to(ROOT)}: {exc}")
    return _require_mapping(data, "registry")


def parse_verticals(registry: dict[str, Any]) -> list[Vertical]:
    verticals_raw = _require_mapping(registry.get("verticals"), "verticals")
    if not verticals_raw:
        _fail("verticals must not be empty")

    verticals: list[Vertical] = []
    for vertical_id, raw in verticals_raw.items():
        if not isinstance(vertical_id, str) or not VERTICAL_ID_RE.fullmatch(vertical_id):
            _fail(f"vertical id {vertical_id!r} must match {VERTICAL_ID_RE.pattern}")
        item = _require_mapping(raw, f"verticals.{vertical_id}")
        route_prefix = _require_string(item.get("route_prefix"), f"verticals.{vertical_id}.route_prefix")
        api_prefixes = _optional_string_list(item.get("api_prefixes"), f"verticals.{vertical_id}.api_prefixes")
        if api_prefixes is None:
            api_prefixes = [route_prefix]
        elif route_prefix not in api_prefixes:
            api_prefixes = [route_prefix, *api_prefixes]
        verticals.append(
            Vertical(
                id=vertical_id,
                enabled=_require_bool(item.get("enabled"), f"verticals.{vertical_id}.enabled"),
                deploy=_require_bool(item.get("deploy"), f"verticals.{vertical_id}.deploy"),
                path=_require_string(item.get("path"), f"verticals.{vertical_id}.path"),
                dockerfile=_require_string(item.get("dockerfile"), f"verticals.{vertical_id}.dockerfile"),
                service=_require_string(item.get("service"), f"verticals.{vertical_id}.service"),
                route_prefix=route_prefix,
                api_prefixes=tuple(dict.fromkeys(api_prefixes)),
                healthcheck=_require_string(item.get("healthcheck"), f"verticals.{vertical_id}.healthcheck"),
            )
        )
    return verticals


def firebase_config(registry: dict[str, Any], verticals: list[Vertical]) -> dict[str, Any]:
    platform = _require_mapping(registry.get("platform"), "platform")
    firebase = _require_mapping(platform.get("firebase"), "platform.firebase")
    public = _require_string(firebase.get("public"), "platform.firebase.public")
    region = _require_string(firebase.get("region"), "platform.firebase.region")
    core_service = _require_string(firebase.get("core_service"), "platform.firebase.core_service")
    ignore = firebase.get("ignore")
    if not isinstance(ignore, list) or not all(isinstance(item, str) and item for item in ignore):
        _fail("platform.firebase.ignore must be a list of non-empty strings")
    rewrites_raw = firebase.get("static_rewrites", [])
    if not isinstance(rewrites_raw, list):
        _fail("platform.firebase.static_rewrites must be a list")

    rewrites: list[dict[str, Any]] = []
    for index, item in enumerate(rewrites_raw):
        rewrite = _require_mapping(item, f"platform.firebase.static_rewrites[{index}]")
        source = _require_string(rewrite.get("source"), f"platform.firebase.static_rewrites[{index}].source")
        service = _require_string(rewrite.get("service"), f"platform.firebase.static_rewrites[{index}].service")
        rewrites.append({"source": source, "run": {"serviceId": service, "region": region}})

    for vertical in verticals:
        if vertical.enabled and vertical.deploy:
            for api_prefix in vertical.api_prefixes:
                rewrites.append(
                    {
                        "source": f"{api_prefix}/**",
                        "run": {"serviceId": vertical.service, "region": region},
                    }
                )

    rewrites.append({"source": "/v1/**", "run": {"serviceId": core_service, "region": region}})
    rewrites.append({"source": "**", "destination": "/index.html"})
    return {"hosting": {"public": public, "ignore": ignore, "rewrites": rewrites}}


def render_json(data: dict[str, Any]) -> str:
    return json.dumps(data, indent=2, ensure_ascii=False) + "\n"


def validate_registry(registry: dict[str, Any] | None = None) -> list[Vertical]:
    if registry is None:
        registry = load_registry()
    if registry.get("version") != 1:
        _fail("version must be 1")

    platform = _require_mapping(registry.get("platform"), "platform")
    firebase = _require_mapping(platform.get("firebase"), "platform.firebase")
    _require_string(firebase.get("public"), "platform.firebase.public")
    region = _require_string(firebase.get("region"), "platform.firebase.region")
    if region != "us-central1":
        _fail("platform.firebase.region must be us-central1 for DEV v1")
    core_service = _require_string(firebase.get("core_service"), "platform.firebase.core_service")
    if not CLOUD_RUN_SERVICE_RE.fullmatch(core_service):
        _fail(f"platform.firebase.core_service {core_service!r} is not a valid Cloud Run service name")

    static_sources: set[str] = set()
    for index, item in enumerate(firebase.get("static_rewrites", [])):
        rewrite = _require_mapping(item, f"platform.firebase.static_rewrites[{index}]")
        source = _require_string(rewrite.get("source"), f"platform.firebase.static_rewrites[{index}].source")
        service = _require_string(rewrite.get("service"), f"platform.firebase.static_rewrites[{index}].service")
        if not source.startswith("/"):
            _fail(f"static rewrite {source!r} must start with /")
        if source in static_sources:
            _fail(f"duplicate static rewrite source {source!r}")
        static_sources.add(source)
        if not CLOUD_RUN_SERVICE_RE.fullmatch(service):
            _fail(f"static rewrite service {service!r} is not a valid Cloud Run service name")

    verticals = parse_verticals(registry)
    services: dict[str, str] = {}
    routes: dict[str, str] = {}

    for vertical in verticals:
        base = f"verticals.{vertical.id}"
        if vertical.deploy and not vertical.enabled:
            _fail(f"{base} cannot set deploy=true while enabled=false")
        if not CLOUD_RUN_SERVICE_RE.fullmatch(vertical.service):
            _fail(f"{base}.service {vertical.service!r} is not a valid Cloud Run service name")
        if not ROUTE_PREFIX_RE.fullmatch(vertical.route_prefix):
            _fail(f"{base}.route_prefix must match {ROUTE_PREFIX_RE.pattern}")
        if vertical.route_prefix.endswith("/"):
            _fail(f"{base}.route_prefix must not end with /")
        if vertical.route_prefix == "/v1" or vertical.route_prefix == "/v1/**":
            _fail(f"{base}.route_prefix cannot capture all /v1 traffic")
        for api_prefix in vertical.api_prefixes:
            if not ROUTE_PREFIX_RE.fullmatch(api_prefix):
                _fail(f"{base}.api_prefixes contains {api_prefix!r}, expected {ROUTE_PREFIX_RE.pattern}")
            if api_prefix == "/v1" or api_prefix == "/v1/**":
                _fail(f"{base}.api_prefixes cannot capture all /v1 traffic")
        if not vertical.healthcheck.startswith("/"):
            _fail(f"{base}.healthcheck must start with /")
        if "**" in vertical.healthcheck:
            _fail(f"{base}.healthcheck must be a concrete path")
        if not vertical.absolute_path.exists():
            _fail(f"{base}.path does not exist: {vertical.path}")
        if not vertical.dockerfile_path.exists():
            _fail(f"{base}.dockerfile does not exist: {vertical.path}/{vertical.dockerfile}")

        previous_service = services.get(vertical.service)
        if previous_service:
            _fail(f"duplicate service {vertical.service!r} in {previous_service} and {vertical.id}")
        services[vertical.service] = vertical.id

        for api_prefix in vertical.api_prefixes:
            previous_route = routes.get(api_prefix)
            if previous_route:
                _fail(f"duplicate api prefix {api_prefix!r} in {previous_route} and {vertical.id}")
            routes[api_prefix] = vertical.id

            generated_rewrite = f"{api_prefix}/**"
            if generated_rewrite in static_sources:
                _fail(f"{base}.api_prefixes generates duplicate Firebase rewrite {generated_rewrite!r}")

    sorted_routes = sorted(routes)
    for index, route in enumerate(sorted_routes):
        for other in sorted_routes[index + 1 :]:
            if other.startswith(f"{route}/"):
                _fail(f"overlapping route_prefix values: {route!r} captures {other!r}")

    return verticals


def print_error(exc: Exception) -> int:
    print(f"ERROR: {exc}", file=sys.stderr)
    return 1
