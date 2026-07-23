#!/usr/bin/env bash

set -euo pipefail

app_id=""
instance="dev"

while (($# > 0)); do
  case "$1" in
    --app)
      app_id="${2:-}"
      shift 2
      ;;
    --instance)
      instance="${2:-}"
      shift 2
      ;;
    *)
      echo "Uso: $0 [--app <app_id>] [--instance <dev|prod|instance_id>]" >&2
      exit 2
      ;;
  esac
done

command -v clerk >/dev/null 2>&1 || {
  echo "Falta Clerk CLI." >&2
  exit 1
}
command -v jq >/dev/null 2>&1 || {
  echo "Falta jq." >&2
  exit 1
}

target_args=(--instance "$instance")
if [[ -n "$app_id" ]]; then
  target_args+=(--app "$app_id")
fi

doctor_json="$(clerk doctor --json)"
if ! jq -e 'all(.[]; .status != "fail")' >/dev/null <<<"$doctor_json"; then
  echo "Clerk CLI no está listo para consultar la instancia." >&2
  jq -r '.[] | select(.status == "fail") | "- \(.name): \(.message)"' \
    <<<"$doctor_json" >&2
  exit 1
fi

config_json="$(
  clerk config pull "${target_args[@]}" --keys session organization_settings
)"
templates_json="$(clerk api /jwt_templates "${target_args[@]}")"

failures=()
jq -e '.session.claims.aud == "pymes-v2-api"' >/dev/null <<<"$config_json" ||
  failures+=("session.claims.aud debe ser pymes-v2-api")
jq -e '.organization_settings.force_organization_selection == false' \
  >/dev/null <<<"$config_json" ||
  failures+=("force_organization_selection debe ser false")
jq -e '.organization_settings.slug_disabled == false' >/dev/null <<<"$config_json" ||
  failures+=("slug_disabled debe ser false")
jq -e 'type == "array" and length == 0' >/dev/null <<<"$templates_json" ||
  failures+=("no debe haber JWT templates; Pymes usa el session token estándar")

if ((${#failures[@]} > 0)); then
  echo "Configuración Clerk incompatible con Pymes v2:" >&2
  printf '  - %s\n' "${failures[@]}" >&2
  exit 1
fi

echo "Configuración Clerk compatible con Pymes v2."
