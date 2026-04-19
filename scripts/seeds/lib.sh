#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_LOCAL_INFRA_DIR="$(cd "$ROOT_DIR/.." && pwd)/local-infra"

# `make seed` y los scripts hijo corren en bash sin pasar por docker compose: leen `.env` de la raíz del monorepo.
if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

LOCAL_INFRA_DIR="${LOCAL_INFRA_DIR:-$DEFAULT_LOCAL_INFRA_DIR}"
DOCKER_COMPOSE="${DOCKER_COMPOSE:-docker compose --project-directory $ROOT_DIR -f $LOCAL_INFRA_DIR/docker-compose.yml -f $ROOT_DIR/docker-compose.yml}"
PYMES_DB_NAME="${PYMES_DB_NAME:-pymes}"
PYMES_DB_USER="${PYMES_DB_USER:-postgres}"
REVIEW_DB_NAME="${REVIEW_DB_NAME:-nexus_review}"
REVIEW_DB_USER="${REVIEW_DB_USER:-postgres}"

dc() {
  (cd "$ROOT_DIR" && ${DOCKER_COMPOSE} "$@")
}

wait_for_pg() {
  local service="$1"
  local db_name="$2"
  local user_name="$3"
  local tries="${4:-60}"
  local i
  for ((i = 1; i <= tries; i++)); do
    if dc exec -T "$service" pg_isready -U "$user_name" -d "$db_name" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "Postgres no quedó listo en servicio=$service db=$db_name" >&2
  return 1
}

ensure_seed_dbs_ready() {
  dc up -d postgres review-postgres
  wait_for_pg postgres "$PYMES_DB_NAME" "$PYMES_DB_USER"
  wait_for_pg review-postgres "$REVIEW_DB_NAME" "$REVIEW_DB_USER"
}

require_seed_org_external_id() {
  if [[ -z "${PYMES_SEED_DEMO_ORG_EXTERNAL_ID:-}" ]]; then
    echo "PYMES_SEED_DEMO_ORG_EXTERNAL_ID is required" >&2
    exit 1
  fi
}

derive_seed_org_slug() {
  local external_id="$1"
  local cleaned="${external_id#org_}"
  cleaned="$(printf '%s' "$cleaned" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '-' | sed 's/^-*//;s/-*$//')"
  cleaned="${cleaned:-demo}"
  printf 'demo-%s\n' "$(printf '%s' "$cleaned" | cut -c1-40)"
}

resolve_target_org_uuid() {
  require_seed_org_external_id

  local external_id="${PYMES_SEED_DEMO_ORG_EXTERNAL_ID:-}"

  # psql no sustituye :'var' en -c de modo no interactivo; por stdin sí (PG 16 en imagen oficial).
  local org_uuid
  org_uuid="$(
    printf '%s\n' "SELECT cast(id as text) FROM orgs WHERE external_id = :'external_id';" \
      | dc exec -T postgres \
        psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 -v "external_id=$external_id"
  )"
  org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
  if [[ -n "$org_uuid" ]]; then
    printf '%s\n' "$org_uuid"
    return 0
  fi

  org_uuid="$(
    printf '%s\n' "SELECT cast(uuid_generate_v5(uuid_ns_url(), 'pymes-seed/org/' || :'external_id') as text);" \
      | dc exec -T postgres \
        psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 -v "external_id=$external_id"
  )"
  org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
  if [[ -z "$org_uuid" ]]; then
    echo "No se pudo resolver org uuid para external_id=$external_id" >&2
    exit 1
  fi
  printf '%s\n' "$org_uuid"
}

render_seed_sql() {
  local file="$1"
  local fullpath
  if [[ "$file" == /* ]]; then
    fullpath="$file"
  else
    fullpath="$ROOT_DIR/$file"
  fi
  python3 - "$fullpath" "$TARGET_ORG_UUID" <<'PY'
import os
from pathlib import Path
import sys

path = Path(sys.argv[1])
target_org_uuid = sys.argv[2]
body = path.read_text()

def sql_escape(value: str) -> str:
    return value.replace("'", "''")

replacements = {
    "__SEED_ORG_ID__": target_org_uuid,
    "__SEED_ORG_EXTERNAL_ID__": sql_escape(os.environ.get("SEED_ORG_EXTERNAL_ID", "")),
    "__SEED_ORG_NAME__": sql_escape(os.environ.get("SEED_ORG_NAME", "")),
    "__SEED_ORG_SLUG__": sql_escape(os.environ.get("SEED_ORG_SLUG", "")),
}
for placeholder, value in replacements.items():
    body = body.replace(placeholder, value)
sys.stdout.write(body)
PY
}

run_pymes_sql_file() {
  local file="$1"
  render_seed_sql "$file" | dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -v ON_ERROR_STOP=1
}

run_pymes_sql_inline() {
  local sql="$1"
  printf '%s\n' "$sql" | dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -v ON_ERROR_STOP=1
}

run_review_sql_inline() {
  local sql="$1"
  printf '%s\n' "$sql" | dc exec -T review-postgres psql -U "$REVIEW_DB_USER" -d "$REVIEW_DB_NAME" -v ON_ERROR_STOP=1
}

run_review_sql_file() {
  local file="$1"
  local fullpath
  if [[ "$file" == /* ]]; then
    fullpath="$file"
  else
    fullpath="$ROOT_DIR/$file"
  fi
  cat "$fullpath" | dc exec -T review-postgres psql -U "$REVIEW_DB_USER" -d "$REVIEW_DB_NAME" -v ON_ERROR_STOP=1
}

export ROOT_DIR LOCAL_INFRA_DIR DOCKER_COMPOSE PYMES_DB_NAME PYMES_DB_USER
export REVIEW_DB_NAME REVIEW_DB_USER
