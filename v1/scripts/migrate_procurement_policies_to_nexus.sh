#!/usr/bin/env bash
set -euo pipefail

# Migrates local Pymes procurement policies to Nexus before migration 0076 drops
# the local procurement_policies table.
#
# Default mode is dry-run. Use --apply to POST missing policies to Nexus and
# verify the Nexus tenant-scoped policy count matches the local source rows.
#
# Required env:
#   DATABASE_URL
#   GOVERNANCE_URL
#   GOVERNANCE_API_KEY

apply=false
if [[ "${1:-}" == "--apply" ]]; then
  apply=true
elif [[ "${1:-}" != "" ]]; then
  echo "usage: $0 [--apply]" >&2
  exit 2
fi

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${GOVERNANCE_URL:?GOVERNANCE_URL is required}"
: "${GOVERNANCE_API_KEY:?GOVERNANCE_API_KEY is required}"

tmp_json="$(mktemp)"
trap 'rm -f "$tmp_json"' EXIT

if [[ "$(psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atc "SELECT to_regclass('public.procurement_policies') IS NOT NULL")" != "t" ]]; then
  echo "[]" >"$tmp_json"
else
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -At <<'SQL' >"$tmp_json"
WITH source_rows AS (
  SELECT
    tenant_id::text AS tenant_id,
    jsonb_build_object(
      'name', name,
      'description', '',
      'action_type', NULLIF(action_filter, ''),
      'target_system', NULLIF(system_filter, ''),
      'expression', expression,
      'effect', effect,
      'priority', priority,
      'mode', CASE mode WHEN 'enforce' THEN 'enforced' ELSE mode END,
      'enabled', enabled
    ) AS body
  FROM procurement_policies
  ORDER BY tenant_id, priority, name
)
SELECT COALESCE(
  jsonb_agg(jsonb_build_object('tenant_id', tenant_id, 'body', body))::text,
  '[]'
)
FROM source_rows;
SQL
fi

python3 - "$tmp_json" "$GOVERNANCE_URL" "$GOVERNANCE_API_KEY" "$apply" <<'PY'
import json
import sys
import urllib.error
import urllib.request
from collections import Counter, defaultdict

path, base_url, api_key, apply_raw = sys.argv[1:]
apply = apply_raw == "true"
base_url = base_url.rstrip("/")
tenant_header = "X-Org-ID"  # Nexus wire contract; Pymes product code uses tenant naming.

with open(path, "r", encoding="utf-8") as fh:
    raw = fh.read().strip() or "[]"
rows = json.loads(raw.splitlines()[-1])

def policy_key(policy):
    return (
        policy.get("name") or "",
        policy.get("expression") or "",
        policy.get("effect") or "",
        policy.get("action_type") or "",
        policy.get("target_system") or "",
    )

def request_json(method, url, tenant_id, body=None):
    data = None
    headers = {
        "X-API-Key": api_key,
        tenant_header: tenant_id,
        "Accept": "application/json",
    }
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            payload = resp.read().decode("utf-8")
            return resp.status, json.loads(payload or "{}")
    except urllib.error.HTTPError as exc:
        payload = exc.read().decode("utf-8", errors="replace")
        raise SystemExit(f"{method} {url} failed status={exc.code} body={payload}") from exc

by_tenant = defaultdict(list)
for row in rows:
    by_tenant[row["tenant_id"]].append(row["body"])

total = sum(len(items) for items in by_tenant.values())
print(f"local procurement policies: {total}")
if not rows:
    print("nothing to migrate")
    sys.exit(0)

for tenant_id, policies in sorted(by_tenant.items()):
    status, envelope = request_json("GET", f"{base_url}/v1/policies", tenant_id)
    if status >= 400:
        raise SystemExit(f"list policies failed for tenant {tenant_id}: status={status}")
    existing = envelope.get("data") or envelope.get("items") or []
    existing_counts = Counter(policy_key(p) for p in existing)
    local_counts = Counter(policy_key(p) for p in policies)

    missing = []
    simulated_counts = existing_counts.copy()
    for policy in policies:
        key = policy_key(policy)
        if simulated_counts[key] > 0:
            simulated_counts[key] -= 1
            continue
        missing.append(policy)

    print(f"tenant {tenant_id}: local={len(policies)} existing_matches={len(policies)-len(missing)} missing={len(missing)}")
    if not apply:
        continue

    for policy in missing:
        request_json("POST", f"{base_url}/v1/policies", tenant_id, policy)

    _, after = request_json("GET", f"{base_url}/v1/policies", tenant_id)
    after_items = after.get("data") or after.get("items") or []
    after_counts = Counter(policy_key(p) for p in after_items)
    for key, count in local_counts.items():
        if after_counts[key] < count:
            raise SystemExit(
                f"verify failed tenant={tenant_id} key={key!r}: expected>={count} got={after_counts[key]}"
            )
    print(f"tenant {tenant_id}: verify ok")

if not apply:
    print("dry-run only; rerun with --apply before migration 0076")
else:
    print("migration export to Nexus completed and verified")
PY
