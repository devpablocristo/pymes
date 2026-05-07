#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_LOCAL_INFRA_DIR="$(cd "$ROOT_DIR/.." && pwd)/local-infra"

# `make seed` y los scripts hijo corren en bash sin pasar por docker compose: leen `.env` de la raíz del monorepo.
# No usar `source .env`: placeholders tipo `https://<clerk-host>/...` interpretan `<` como redirección en bash.
# Para cargar seeds contra GCP sin tocar `.env` local, exportá PYMES_SEEDS_SKIP_DOTENV=1.
if [[ "${PYMES_SEEDS_SKIP_DOTENV:-}" != "1" ]] && [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1090
  eval "$(python3 "$ROOT_DIR/scripts/seeds/load_dotenv_exports.py" "$ROOT_DIR/.env")"
  set +a
fi

LOCAL_INFRA_DIR="${LOCAL_INFRA_DIR:-$DEFAULT_LOCAL_INFRA_DIR}"
DOCKER_COMPOSE="${DOCKER_COMPOSE:-docker compose --project-directory $ROOT_DIR -f $ROOT_DIR/docker-compose.yml}"
# Postgres de governance vive en el compose del repo Nexus (levantalo aparte), no en el de Pymes.
NEXUS_ROOT="${NEXUS_ROOT:-$(cd "$ROOT_DIR/.." && pwd)/nexus}"
PYMES_DB_NAME="${PYMES_DB_NAME:-pymes}"
PYMES_DB_USER="${PYMES_DB_USER:-postgres}"
# DB del binario Nexus governance.
GOVERNANCE_DB_NAME="${GOVERNANCE_DB_NAME:-nexus_governance}"
GOVERNANCE_DB_USER="${GOVERNANCE_DB_USER:-postgres}"

dc() {
  (cd "$ROOT_DIR" && ${DOCKER_COMPOSE} "$@")
}

nexus_dc() {
  if [[ ! -f "$NEXUS_ROOT/docker-compose.yml" ]]; then
    echo "NEXUS_ROOT=$NEXUS_ROOT no tiene docker-compose.yml — cloná Nexus o exportá NEXUS_ROOT." >&2
    return 1
  fi
  (cd "$NEXUS_ROOT" && docker compose --project-directory "$NEXUS_ROOT" -f docker-compose.yml "$@")
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

wait_for_nexus_pg() {
  local service="$1"
  local db_name="$2"
  local user_name="$3"
  local tries="${4:-60}"
  local i
  for ((i = 1; i <= tries; i++)); do
    if nexus_dc exec -T "$service" pg_isready -U "$user_name" -d "$db_name" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "Postgres Nexus no quedó listo (servicio=$service db=$db_name). ¿Está el compose de Nexus arriba?" >&2
  return 1
}

host_pymes_psql() {
  PGPASSWORD="${POSTGRES_PASSWORD:-postgres}" psql \
    -h "${POSTGRES_HOST:-localhost}" \
    -p "${POSTGRES_PORT:-5434}" \
    -U "${POSTGRES_USER:-$PYMES_DB_USER}" \
    -d "${POSTGRES_DB:-$PYMES_DB_NAME}" \
    "$@"
}

governance_pg_port() {
  local published_port nexus_env_port
  if [[ -n "${GOVERNANCE_POSTGRES_PORT:-}" ]]; then
    printf '%s\n' "$GOVERNANCE_POSTGRES_PORT"
    return 0
  fi
  published_port="$(docker port nexus-governance-postgres-1 5432/tcp 2>/dev/null | awk -F: 'NR == 1 { print $NF }' || true)"
  if [[ -n "$published_port" ]]; then
    printf '%s\n' "$published_port"
    return 0
  fi
  if [[ -f "$NEXUS_ROOT/.env" ]]; then
    nexus_env_port="$(awk -F= '$1 == "GOVERNANCE_POSTGRES_PORT" { print $2; exit }' "$NEXUS_ROOT/.env" | tr -d '"'\''[:space:]')"
    if [[ -n "$nexus_env_port" ]]; then
      printf '%s\n' "$nexus_env_port"
      return 0
    fi
  fi
  printf '%s\n' "${GOVERNANCE_POSTGRES_PORT:-15434}"
}

host_governance_psql() {
  PGPASSWORD="${GOVERNANCE_DB_PASSWORD:-postgres}" psql \
    -h "${GOVERNANCE_DB_HOST:-localhost}" \
    -p "$(governance_pg_port)" \
    -U "$GOVERNANCE_DB_USER" \
    -d "$GOVERNANCE_DB_NAME" \
    "$@"
}

wait_for_host_pymes_pg() {
  local tries="${1:-60}"
  local i
  if ! command -v pg_isready >/dev/null 2>&1; then
    return 1
  fi
  for ((i = 1; i <= tries; i++)); do
    if PGPASSWORD="${POSTGRES_PASSWORD:-postgres}" pg_isready \
      -h "${POSTGRES_HOST:-localhost}" \
      -p "${POSTGRES_PORT:-5434}" \
      -U "${POSTGRES_USER:-$PYMES_DB_USER}" \
      -d "${POSTGRES_DB:-$PYMES_DB_NAME}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_for_host_governance_pg() {
  local tries="${1:-60}"
  local i
  if ! command -v pg_isready >/dev/null 2>&1; then
    return 1
  fi
  for ((i = 1; i <= tries; i++)); do
    if PGPASSWORD="${GOVERNANCE_DB_PASSWORD:-postgres}" pg_isready \
      -h "${GOVERNANCE_DB_HOST:-localhost}" \
      -p "$(governance_pg_port)" \
      -U "$GOVERNANCE_DB_USER" \
      -d "$GOVERNANCE_DB_NAME" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

ensure_pymes_seed_db_ready() {
  dc up -d postgres
  wait_for_host_pymes_pg || wait_for_pg postgres "$PYMES_DB_NAME" "$PYMES_DB_USER"
}

ensure_governance_seed_db_ready() {
  nexus_dc up -d governance-postgres
  wait_for_host_governance_pg || wait_for_nexus_pg governance-postgres "$GOVERNANCE_DB_NAME" "$GOVERNANCE_DB_USER"
}

ensure_seed_dbs_ready() {
  ensure_pymes_seed_db_ready
  ensure_governance_seed_db_ready
}

require_seed_org_external_id() {
  if [[ -z "${PYMES_SEED_DEMO_ORG_EXTERNAL_ID:-}" ]]; then
    PYMES_SEED_DEMO_ORG_EXTERNAL_ID="org_local_demo"
    export PYMES_SEED_DEMO_ORG_EXTERNAL_ID
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
  local external_id_sql="${external_id//\'/\'\'}"

  local org_uuid
  org_uuid="$(
    dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT cast(id as text) FROM tenants WHERE external_id = '$external_id_sql';"
  )"
  org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
  if [[ -n "$org_uuid" ]]; then
    printf '%s\n' "$org_uuid"
    return 0
  fi

  if command -v psql >/dev/null 2>&1; then
    org_uuid="$(
      host_pymes_psql \
        -Atq -v ON_ERROR_STOP=1 \
        -c "SELECT cast(id as text) FROM tenants WHERE external_id = '$external_id_sql';" \
        2>/dev/null || true
    )"
    org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
    if [[ -n "$org_uuid" ]]; then
      printf '%s\n' "$org_uuid"
      return 0
    fi
  fi

  org_uuid="$(
    dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT cast(uuid_generate_v5(uuid_ns_url(), 'pymes-seed/org/' || '$external_id_sql') as text);"
  )"
  org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
  if [[ -z "$org_uuid" ]] && command -v python3 >/dev/null 2>&1; then
    org_uuid="$(
      python3 - "$external_id" <<'PY'
import sys
import uuid

print(uuid.uuid5(uuid.NAMESPACE_URL, "pymes-seed/org/" + sys.argv[1]))
PY
    )"
    org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
  fi
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
  local tmp_name tmp_path
  tmp_name="pymes-seed-$RANDOM-$(basename "$file")"
  tmp_path="/tmp/$tmp_name"
  render_seed_sql "$file" > "$tmp_path"
  if command -v psql >/dev/null 2>&1; then
    host_pymes_psql -v ON_ERROR_STOP=1 -f "$tmp_path"
  else
    dc cp "$tmp_path" "postgres:/tmp/$tmp_name"
    dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -v ON_ERROR_STOP=1 -f "/tmp/$tmp_name"
    dc exec -T postgres rm -f "/tmp/$tmp_name" >/dev/null 2>&1 || true
  fi
  rm -f "$tmp_path"
}

run_pymes_sql_inline() {
  local sql="$1"
  local tmp_name tmp_path
  tmp_name="pymes-seed-inline-$RANDOM.sql"
  tmp_path="/tmp/$tmp_name"
  printf '%s\n' "$sql" > "$tmp_path"
  if command -v psql >/dev/null 2>&1; then
    host_pymes_psql -v ON_ERROR_STOP=1 -f "$tmp_path"
  else
    dc cp "$tmp_path" "postgres:/tmp/$tmp_name"
    dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -v ON_ERROR_STOP=1 -f "/tmp/$tmp_name"
    dc exec -T postgres rm -f "/tmp/$tmp_name" >/dev/null 2>&1 || true
  fi
  rm -f "$tmp_path"
}

run_governance_sql_inline() {
  local sql="$1"
  local tmp_name tmp_path
  tmp_name="governance-seed-inline-$RANDOM.sql"
  tmp_path="/tmp/$tmp_name"
  printf '%s\n' "$sql" > "$tmp_path"
  if command -v psql >/dev/null 2>&1; then
    host_governance_psql -v ON_ERROR_STOP=1 -f "$tmp_path"
  else
    nexus_dc cp "$tmp_path" "governance-postgres:/tmp/$tmp_name"
    nexus_dc exec -T governance-postgres psql -U "$GOVERNANCE_DB_USER" -d "$GOVERNANCE_DB_NAME" -v ON_ERROR_STOP=1 -f "/tmp/$tmp_name"
    nexus_dc exec -T governance-postgres rm -f "/tmp/$tmp_name" >/dev/null 2>&1 || true
  fi
  rm -f "$tmp_path"
}

run_governance_sql_file() {
  local file="$1"
  local fullpath
  if [[ "$file" == /* ]]; then
    fullpath="$file"
  else
    fullpath="$ROOT_DIR/$file"
  fi
  if command -v psql >/dev/null 2>&1; then
    host_governance_psql -v ON_ERROR_STOP=1 -f "$fullpath"
    return
  fi
  local tmp_name
  tmp_name="governance-seed-$RANDOM-$(basename "$file")"
  nexus_dc cp "$fullpath" "governance-postgres:/tmp/$tmp_name"
  nexus_dc exec -T governance-postgres psql -U "$GOVERNANCE_DB_USER" -d "$GOVERNANCE_DB_NAME" -v ON_ERROR_STOP=1 -f "/tmp/$tmp_name"
  nexus_dc exec -T governance-postgres rm -f "/tmp/$tmp_name" >/dev/null 2>&1 || true
}

export ROOT_DIR LOCAL_INFRA_DIR DOCKER_COMPOSE NEXUS_ROOT PYMES_DB_NAME PYMES_DB_USER
export GOVERNANCE_DB_NAME GOVERNANCE_DB_USER 
