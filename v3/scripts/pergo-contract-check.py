#!/usr/bin/env python3
"""Keep the PerGo delivery webhook vocabulary identical at both boundaries."""

from pathlib import Path
import sys

import yaml


EXPECTED_EVENTS = ["queued", "sent", "delivered", "read", "failed"]


def event_enum(document: dict, schema_name: str) -> list[str]:
    try:
        value = document["components"]["schemas"][schema_name]["properties"]["event"][
            "enum"
        ]
    except (KeyError, TypeError) as error:
        raise SystemExit(f"{schema_name}.event enum is missing: {error}") from error
    if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
        raise SystemExit(f"{schema_name}.event enum must be a string list")
    return value


def load(path: Path) -> dict:
    with path.open(encoding="utf-8") as stream:
        document = yaml.safe_load(stream)
    if not isinstance(document, dict):
        raise SystemExit(f"{path} is not an OpenAPI object")
    return document


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: pergo-contract-check.py PUBLIC_API PERGO_CONTRACT")
    public_path = Path(sys.argv[1])
    private_path = Path(sys.argv[2])
    public_events = event_enum(load(public_path), "PerGoDeliveryEvent")
    private_events = event_enum(load(private_path), "DeliveryEvent")
    if public_events != EXPECTED_EVENTS:
        raise SystemExit(
            f"public PerGo event enum {public_events!r} != {EXPECTED_EVENTS!r}"
        )
    if private_events != EXPECTED_EVENTS:
        raise SystemExit(
            f"private PerGo event enum {private_events!r} != {EXPECTED_EVENTS!r}"
        )


if __name__ == "__main__":
    main()
