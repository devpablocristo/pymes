#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKER_COMPOSE="${DOCKER_COMPOSE:-docker compose}"
PYMES_DB_NAME="${PYMES_DB_NAME:-pymes}"
PYMES_DB_USER="${PYMES_DB_USER:-postgres}"
REVIEW_DB_NAME="${REVIEW_DB_NAME:-nexus_review}"
REVIEW_DB_USER="${REVIEW_DB_USER:-postgres}"
LEGACY_DEMO_ORG_UUID="00000000-0000-0000-0000-000000000001"

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

resolve_target_org_uuid() {
  local external_id="${PYMES_SEED_DEMO_ORG_EXTERNAL_ID:-}"
  if [[ -z "$external_id" ]]; then
    printf '%s\n' "$LEGACY_DEMO_ORG_UUID"
    return
  fi

  local org_uuid
  org_uuid="$(
    dc exec -T postgres \
      psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 -v external_id="$external_id" \
      -c "SELECT id::text FROM orgs WHERE external_id = :'external_id';"
  )"
  org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
  if [[ -z "$org_uuid" ]]; then
    echo "No existe org con external_id=$external_id para aplicar seeds demo" >&2
    exit 1
  fi
  printf '%s\n' "$org_uuid"
}

render_seed_sql() {
  local file="$1"
  python3 - "$ROOT_DIR/$file" "$TARGET_ORG_UUID" "$LEGACY_DEMO_ORG_UUID" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
target_org_uuid = sys.argv[2]
legacy_org_uuid = sys.argv[3]
body = path.read_text()
body = body.replace("__SEED_ORG_ID__", target_org_uuid)
body = body.replace(legacy_org_uuid, target_org_uuid)
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

export ROOT_DIR DOCKER_COMPOSE PYMES_DB_NAME PYMES_DB_USER
export REVIEW_DB_NAME REVIEW_DB_USER LEGACY_DEMO_ORG_UUID
