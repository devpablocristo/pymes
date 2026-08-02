#!/usr/bin/env bash
set -euo pipefail

compose_json=$(docker compose --profile fakes config --format json)
violations=$(jq -r '
  .services
  | to_entries[]
  | .key as $service
  | .value.ports[]?
  | select(.host_ip != "127.0.0.1")
  | "\($service):\(.published)->\(.target) host_ip=\(.host_ip // "all")"
' <<<"$compose_json")
if [[ -n "$violations" ]]; then
  echo "Compose must publish every local port on IPv4 loopback only:" >&2
  printf '%s\n' "$violations" >&2
  exit 1
fi

if ! jq -e '
  .services.web.environment.PYMES_PREFLIGHT_TAG == "local-compose-disabled"
  and (.services.web.environment.PYMES_PREFLIGHT_TOKEN | type == "string")
  and (.services.web.environment.PYMES_PREFLIGHT_TOKEN | test("^[0-9a-f]{64}$"))
' >/dev/null <<<"$compose_json"; then
  echo "Compose Web must resolve a disabled local preflight tag and a 64-hex string token" >&2
  exit 1
fi

echo "Compose published ports are loopback-only and Web preflight configuration is typed"
