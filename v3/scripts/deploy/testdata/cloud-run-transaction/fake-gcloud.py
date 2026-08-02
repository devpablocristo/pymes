#!/usr/bin/env python3
"""Small stateful Cloud Run fake used only by cloud-run-transaction-test.sh."""

from __future__ import annotations

import json
import os
import stat
import sys
from pathlib import Path
from typing import Any


STATE_PATH = Path(os.environ["FAKE_GCLOUD_STATE"])
CALL_LOG = Path(os.environ["FAKE_GCLOUD_CALL_LOG"])


def load_state() -> dict[str, Any]:
    return json.loads(STATE_PATH.read_text(encoding="utf-8"))


def save_state(state: dict[str, Any]) -> None:
    temporary = STATE_PATH.with_suffix(".tmp")
    temporary.write_text(
        json.dumps(state, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    temporary.replace(STATE_PATH)


def append_call(arguments: list[str]) -> None:
    with CALL_LOG.open("a", encoding="utf-8") as stream:
        stream.write(json.dumps(arguments) + "\n")


def option(arguments: list[str], name: str, default: str = "") -> str:
    prefix = f"{name}="
    for index, argument in enumerate(arguments):
        if argument.startswith(prefix):
            return argument[len(prefix) :]
        if argument == name and index + 1 < len(arguments):
            return arguments[index + 1]
    return default


def has_option(arguments: list[str], name: str) -> bool:
    return name in arguments or any(argument.startswith(f"{name}=") for argument in arguments)


def active_revision(service: dict[str, Any]) -> str:
    active = [
        entry
        for entry in service["traffic"]
        if int(entry.get("percent", 0)) > 0
    ]
    if len(active) != 1 or int(active[0].get("percent", 0)) != 100:
        return ""
    return str(active[0]["revisionName"])


def revision_url(service_name: str, tag: str, region: str) -> str:
    return f"https://{tag}---{service_name}.{region}.run.fake"


def environment_from_flag(value: str) -> dict[str, str]:
    if value.startswith("^|^"):
        value = value[3:]
        delimiter = "|"
    else:
        delimiter = ","
    result: dict[str, str] = {}
    if not value:
        return result
    for assignment in value.split(delimiter):
        if not assignment:
            continue
        key, separator, item = assignment.partition("=")
        if not separator:
            raise ValueError(f"invalid environment assignment: {assignment}")
        result[key] = item
    return result


def environment_from_file(path: str) -> dict[str, str]:
    if not path:
        return {}
    environment_path = Path(path)
    mode = stat.S_IMODE(environment_path.stat().st_mode)
    if mode != 0o600:
        raise ValueError(
            f"Cloud Run environment file must have mode 0600, got {mode:04o}"
        )
    text = environment_path.read_text(encoding="utf-8")
    if text.lstrip().startswith("{"):
        parsed = json.loads(text)
        return {str(key): str(value) for key, value in parsed.items()}
    result: dict[str, str] = {}
    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        key, separator, value = line.partition(":")
        if not separator:
            raise ValueError(f"invalid env vars file line: {raw_line}")
        result[key.strip()] = value.strip().strip("'\"")
    return result


def labels_from_flag(value: str) -> dict[str, str]:
    result: dict[str, str] = {}
    for assignment in value.split(","):
        if not assignment:
            continue
        key, separator, item = assignment.partition("=")
        if separator:
            result[key] = item
    return result


def service_json(name: str, service: dict[str, Any]) -> dict[str, Any]:
    latest = service["latest_created"]
    revision = service["revisions"][latest]
    scaling = service["scaling"]
    return {
        "metadata": {
            "name": name,
            "annotations": {
                "run.googleapis.com/ingress": service["ingress"],
                "run.googleapis.com/scalingMode": scaling["mode"],
                "run.googleapis.com/manualInstanceCount": str(scaling["count"]),
                "run.googleapis.com/invoker-iam-disabled": str(
                    not service.get("invoker_iam_check", True)
                ).lower(),
            },
        },
        "spec": {
            "template": {
                "metadata": {
                    "labels": revision.get("labels", {}),
                    "annotations": {
                        "autoscaling.knative.dev/minScale": "0",
                    },
                },
                "spec": {
                    "serviceAccountName": revision.get("service_account", ""),
                    "containers": [
                        {
                            "name": "main",
                            "image": revision.get("image", "test.invalid/unknown"),
                            "env": [
                                {"name": key, "value": value}
                                for key, value in sorted(
                                    revision.get("environment", {}).items()
                                )
                            ],
                        }
                    ],
                },
            }
        },
        "status": {
            "latestCreatedRevisionName": service["latest_created"],
            "latestReadyRevisionName": service["latest_ready"],
            "conditions": [{"type": "Ready", "status": "True"}],
            "traffic": service["traffic"],
        },
    }


def fault_for_call(state: dict[str, Any], arguments: list[str]) -> str:
    fault = state.get("fault")
    if not fault or not fault.get("enabled", False):
        return ""
    rendered = " ".join(arguments)
    if str(fault.get("match", "")) not in rendered:
        return ""
    fault["seen"] = int(fault.get("seen", 0)) + 1
    save_state(state)
    mode = str(fault.get("mode", "before"))
    nth = int(fault.get("nth", 1))
    if mode.startswith("always-"):
        if fault["seen"] < nth:
            return ""
        return mode.removeprefix("always-")
    if fault["seen"] != nth:
        return ""
    return mode


def fail(arguments: list[str]) -> None:
    print(f"fake gcloud injected failure: {' '.join(arguments)}", file=sys.stderr)
    raise SystemExit(42)


def ensure_service(state: dict[str, Any], name: str) -> dict[str, Any]:
    service = state["services"].get(name)
    if not service or not service.get("exists", False):
        print(f"service not found: {name}", file=sys.stderr)
        raise SystemExit(1)
    return service


def set_fault(state: dict[str, Any], arguments: list[str]) -> None:
    if len(arguments) != 4:
        raise SystemExit(
            "__fake__ set-fault MATCH NTH before|after|always-before|always-after"
        )
    if arguments[3] not in {
        "before",
        "after",
        "always-before",
        "always-after",
    }:
        raise SystemExit("unsupported fake fault mode")
    state["fault"] = {
        "enabled": True,
        "match": arguments[1],
        "nth": int(arguments[2]),
        "mode": arguments[3],
        "seen": 0,
    }
    save_state(state)


def clear_fault(state: dict[str, Any]) -> None:
    state["fault"] = {"enabled": False}
    save_state(state)


def set_worker_signal(state: dict[str, Any], arguments: list[str]) -> None:
    if len(arguments) != 3:
        raise SystemExit("__fake__ set-worker-signal SERVICE REVISION")
    service = ensure_service(state, arguments[1])
    service["worker_signal_revision"] = arguments[2]
    save_state(state)


def deploy_service(
    state: dict[str, Any], name: str, arguments: list[str]
) -> None:
    region = state["region"]
    service = state["services"].setdefault(
        name,
        {
            "exists": True,
            "next_revision": 1,
            "latest_created": "",
            "latest_ready": "",
            "ingress": "internal",
            "invoker_iam_check": True,
            "invokers": [],
            "scaling": {"mode": "automatic", "count": 0},
            "traffic": [],
            "revisions": {},
        },
    )
    service["exists"] = True
    revision_number = int(service.get("next_revision", 1))
    service["next_revision"] = revision_number + 1
    revision_name = f"{name}-candidate-{revision_number}"
    environment_file = option(arguments, "--env-vars-file")
    if environment_file:
        environment = environment_from_file(environment_file)
    else:
        environment = environment_from_flag(option(arguments, "--set-env-vars"))
    service["revisions"][revision_name] = {
        "image": option(arguments, "--image"),
        "service_account": option(arguments, "--service-account"),
        "environment": environment,
        "labels": labels_from_flag(option(arguments, "--labels")),
    }
    service["latest_created"] = revision_name
    service["latest_ready"] = revision_name
    service["ingress"] = option(arguments, "--ingress", service["ingress"])
    if has_option(arguments, "--allow-unauthenticated"):
        service["invokers"] = sorted(set(service["invokers"]) | {"allUsers"})
    if has_option(arguments, "--no-allow-unauthenticated"):
        service["invokers"] = [
            member for member in service["invokers"] if member != "allUsers"
        ]
    if has_option(arguments, "--invoker-iam-check"):
        service["invoker_iam_check"] = True
    if has_option(arguments, "--no-invoker-iam-check"):
        service["invoker_iam_check"] = False
    scaling = option(arguments, "--scaling")
    if scaling:
        if scaling == "auto":
            service["scaling"] = {"mode": "automatic", "count": 0}
        else:
            service["scaling"] = {"mode": "manual", "count": int(scaling)}
    tag = option(arguments, "--tag")
    if tag:
        service["traffic"] = [
            entry for entry in service["traffic"] if entry.get("tag") != tag
        ]
        service["traffic"].append(
            {
                "revisionName": revision_name,
                "percent": 0,
                "tag": tag,
                "url": revision_url(name, tag, region),
            }
        )
    save_state(state)


def update_traffic(
    state: dict[str, Any], name: str, arguments: list[str]
) -> None:
    service = ensure_service(state, name)
    region = state["region"]
    to_revisions = option(arguments, "--to-revisions")
    if to_revisions:
        revision_name, separator, raw_percent = to_revisions.partition("=")
        if not separator or int(raw_percent) != 100:
            raise SystemExit("fake supports only one revision at 100 percent")
        for entry in service["traffic"]:
            entry["percent"] = 0
        untagged = [
            entry
            for entry in service["traffic"]
            if entry.get("revisionName") == revision_name and "tag" not in entry
        ]
        if untagged:
            untagged[0]["percent"] = 100
        else:
            service["traffic"].append(
                {"revisionName": revision_name, "percent": 100}
            )
    if has_option(arguments, "--clear-tags"):
        service["traffic"] = [
            entry for entry in service["traffic"] if "tag" not in entry
        ]
    set_tags = option(arguments, "--set-tags")
    if set_tags:
        service["traffic"] = [
            entry for entry in service["traffic"] if "tag" not in entry
        ]
        for assignment in set_tags.split(","):
            tag, revision_name = assignment.split("=", 1)
            service["traffic"].append(
                {
                    "revisionName": revision_name,
                    "percent": 0,
                    "tag": tag,
                    "url": revision_url(name, tag, region),
                }
            )
    remove_tags = option(arguments, "--remove-tags")
    if remove_tags:
        removed = set(remove_tags.split(","))
        service["traffic"] = [
            entry
            for entry in service["traffic"]
            if entry.get("tag") not in removed
        ]
    update_tags = option(arguments, "--update-tags")
    if update_tags:
        replacements: dict[str, str] = {}
        for assignment in update_tags.split(","):
            tag, revision_name = assignment.split("=", 1)
            replacements[tag] = revision_name
        service["traffic"] = [
            entry
            for entry in service["traffic"]
            if entry.get("tag") not in replacements
        ]
        for tag, revision_name in replacements.items():
            service["traffic"].append(
                {
                    "revisionName": revision_name,
                    "percent": 0,
                    "tag": tag,
                    "url": revision_url(name, tag, region),
                }
            )
    save_state(state)


def handle_run(state: dict[str, Any], arguments: list[str]) -> None:
    if arguments[:2] == ["services", "list"]:
        exact = option(arguments[2:], "--filter").removeprefix("metadata.name=")
        resources = []
        for name, service in state["services"].items():
            if not service.get("exists", False):
                continue
            if exact and name != exact:
                continue
            resources.append({"metadata": {"name": name}})
        print(json.dumps(resources))
        return
    if arguments[:2] == ["services", "describe"]:
        service = ensure_service(state, arguments[2])
        print(json.dumps(service_json(arguments[2], service)))
        return
    if arguments[:2] == ["services", "update-traffic"]:
        update_traffic(state, arguments[2], arguments[3:])
        return
    if arguments[:2] == ["services", "update"]:
        service = ensure_service(state, arguments[2])
        scaling = option(arguments[3:], "--scaling")
        if scaling:
            service["scaling"] = {"mode": "manual", "count": int(scaling)}
        ingress = option(arguments[3:], "--ingress")
        if ingress:
            service["ingress"] = ingress
        if has_option(arguments[3:], "--invoker-iam-check"):
            service["invoker_iam_check"] = True
        if has_option(arguments[3:], "--no-invoker-iam-check"):
            service["invoker_iam_check"] = False
        save_state(state)
        return
    if arguments[:2] == ["services", "get-iam-policy"]:
        service = ensure_service(state, arguments[2])
        if option(arguments[3:], "--format") == "json":
            bindings = []
            if service["invokers"]:
                bindings.append(
                    {
                        "role": "roles/run.invoker",
                        "members": service["invokers"],
                    }
                )
            print(json.dumps({"bindings": bindings, "etag": "fake"}))
        else:
            print("\n".join(service["invokers"]))
        return
    if arguments[:2] == ["services", "set-iam-policy"]:
        service = ensure_service(state, arguments[2])
        policy = json.loads(Path(arguments[3]).read_text(encoding="utf-8"))
        service["invokers"] = sorted(
            {
                member
                for binding in policy.get("bindings", [])
                if binding.get("role") == "roles/run.invoker"
                for member in binding.get("members", [])
            }
        )
        save_state(state)
        return
    if arguments[:2] == ["services", "add-iam-policy-binding"]:
        service = ensure_service(state, arguments[2])
        member = option(arguments[3:], "--member")
        service["invokers"] = sorted(set(service["invokers"]) | {member})
        save_state(state)
        return
    if arguments[:2] == ["services", "remove-iam-policy-binding"]:
        service = ensure_service(state, arguments[2])
        member = option(arguments[3:], "--member")
        service["invokers"] = [
            existing for existing in service["invokers"] if existing != member
        ]
        save_state(state)
        return
    if arguments[:2] == ["services", "delete"]:
        service = ensure_service(state, arguments[2])
        service["exists"] = False
        service["traffic"] = []
        service["revisions"] = {}
        service["latest_created"] = ""
        service["latest_ready"] = ""
        save_state(state)
        return
    if arguments[:1] == ["deploy"]:
        deploy_service(state, arguments[1], arguments[2:])
        return
    if arguments[:2] == ["revisions", "describe"]:
        revision_name = arguments[2]
        for service in state["services"].values():
            if service.get("exists") and revision_name in service["revisions"]:
                revision = service["revisions"][revision_name]
                print(
                    json.dumps(
                        {
                            "metadata": {"name": revision_name},
                            "spec": {
                                "containers": [
                                    {
                                        "env": [
                                            {"name": key, "value": value}
                                            for key, value in sorted(
                                                revision.get(
                                                    "environment", {}
                                                ).items()
                                            )
                                        ]
                                    }
                                ]
                            },
                        }
                    )
                )
                return
        raise SystemExit(1)
    if arguments[:2] == ["revisions", "list"]:
        exact = option(arguments[2:], "--filter").removeprefix("metadata.name=")
        resources = []
        for service in state["services"].values():
            if not service.get("exists", False):
                continue
            for revision_name in service.get("revisions", {}):
                if exact and revision_name != exact:
                    continue
                resources.append({"metadata": {"name": revision_name}})
        print(json.dumps(resources))
        return
    if arguments[:2] == ["revisions", "delete"]:
        revision_name = arguments[2]
        for service in state["services"].values():
            if revision_name in service.get("revisions", {}):
                del service["revisions"][revision_name]
                service["traffic"] = [
                    entry
                    for entry in service["traffic"]
                    if entry.get("revisionName") != revision_name
                ]
                remaining = list(service["revisions"])
                active = active_revision(service)
                fallback = active or (remaining[-1] if remaining else "")
                if service.get("latest_created") == revision_name:
                    service["latest_created"] = fallback
                if service.get("latest_ready") == revision_name:
                    service["latest_ready"] = fallback
                save_state(state)
                return
        raise SystemExit(1)
    raise SystemExit(f"unsupported fake gcloud run command: {' '.join(arguments)}")


def main() -> None:
    arguments = sys.argv[1:]
    append_call(arguments)
    state = load_state()
    if arguments[:2] == ["__fake__", "set-fault"]:
        set_fault(state, arguments[1:])
        return
    if arguments[:2] == ["__fake__", "clear-fault"]:
        clear_fault(state)
        return
    if arguments[:2] == ["__fake__", "set-worker-signal"]:
        set_worker_signal(state, arguments[1:])
        return
    if arguments[:2] == ["__fake__", "worker-signal"]:
        service = ensure_service(state, arguments[2])
        expected = arguments[3]
        if service.get("worker_signal_revision") != expected:
            raise SystemExit(1)
        return
    fault_mode = fault_for_call(state, arguments)
    if fault_mode == "before":
        fail(arguments)
    if arguments[:1] == ["run"]:
        handle_run(state, arguments[1:])
    elif arguments[:2] == ["logging", "read"]:
        query = arguments[2] if len(arguments) > 2 else ""
        revision_match = ""
        marker = 'resource.labels.revision_name="'
        if marker in query:
            revision_match = query.split(marker, 1)[1].split('"', 1)[0]
        worker = state["services"].get("pymes-v3-stg-worker", {})
        if worker.get("worker_signal_revision") == revision_match:
            print(json.dumps([{"insertId": f"ready-{revision_match}"}]))
        else:
            print("[]")
    else:
        raise SystemExit(f"unsupported fake gcloud command: {' '.join(arguments)}")
    if fault_mode == "after":
        fail(arguments)


if __name__ == "__main__":
    main()
