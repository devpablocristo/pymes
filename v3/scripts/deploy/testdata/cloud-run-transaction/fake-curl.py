#!/usr/bin/env python3
"""Resolve fake Cloud Run tag URLs against the transaction state."""

from __future__ import annotations

import json
import os
import re
import stat
import sys
from pathlib import Path


state = json.loads(Path(os.environ["FAKE_GCLOUD_STATE"]).read_text(encoding="utf-8"))
call_log = os.environ.get("FAKE_CURL_CALL_LOG")
if call_log:
    with Path(call_log).open("a", encoding="utf-8") as stream:
        stream.write(json.dumps(sys.argv[1:]) + "\n")
config_path = ""
provided_token = ""
for index, argument in enumerate(sys.argv[1:]):
    if argument.startswith("--config="):
        config_path = argument.split("=", 1)[1]
    elif argument == "--config" and index + 2 <= len(sys.argv[1:]):
        config_path = sys.argv[index + 2]
if config_path:
    path = Path(config_path)
    mode = stat.S_IMODE(path.stat().st_mode)
    config = path.read_text(encoding="utf-8")
    if mode != 0o600:
        print(f"curl config must have mode 0600, got {mode:04o}", file=sys.stderr)
        raise SystemExit(2)
    if 'header = "X-Pymes-Preflight-Token: ' not in config:
        print("curl config omitted the preflight capability header", file=sys.stderr)
        raise SystemExit(2)
    token_match = re.search(
        r'^header = "X-Pymes-Preflight-Token: ([0-9a-f]{64})"$',
        config,
        re.MULTILINE,
    )
    if not token_match:
        print("curl config contains an invalid preflight capability", file=sys.stderr)
        raise SystemExit(2)
    provided_token = token_match.group(1)
url = next((item for item in reversed(sys.argv[1:]) if item.startswith("https://")), "")
match = re.fullmatch(
    r"https://([a-z0-9-]+)---([a-z0-9-]+)\.([a-z0-9-]+)\.run\.fake(?:/.*)?",
    url,
)
status = 404
if match:
    tag, service_name, _region = match.groups()
    service = state["services"].get(service_name, {})
    tagged_entries = [
        entry
        for entry in service.get("traffic", [])
        if entry.get("tag") == tag
    ]
    tagged = len(tagged_entries) == 1
    expected_token = ""
    if tagged:
        revision_name = tagged_entries[0].get("revisionName", "")
        expected_token = (
            service.get("revisions", {})
            .get(revision_name, {})
            .get("environment", {})
            .get("PYMES_PREFLIGHT_TOKEN", "")
        )
    capability_valid = (
        not url.endswith("/readyz")
        or (provided_token and provided_token == expected_token)
    )
    public = (
        not service.get("invoker_iam_check", True)
        or "allUsers" in service.get("invokers", [])
    )
    if service.get("exists") and tagged:
        if not public:
            status = 403
        elif not capability_valid:
            status = 401
        else:
            status = 200

write_out = ""
for index, argument in enumerate(sys.argv[1:]):
    if argument.startswith("--write-out="):
        write_out = argument.split("=", 1)[1]
    elif argument == "--write-out" and index + 2 <= len(sys.argv[1:]):
        write_out = sys.argv[index + 2]
if write_out:
    sys.stdout.write(
        write_out.replace("%{http_code}", str(status))
        .replace("%{url_effective}", url)
        .replace("%{redirect_url}", "")
    )
else:
    sys.stdout.write(str(status))
raise SystemExit(0)
