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

# Tenant local que reciben `make seed`, `make seed-verify` y `make seed-reset`.
# Cambiar acá si querés que `make seed` apunte a otro tenant por defecto.
DEFAULT_SEED_TENANT_SLUG="${DEFAULT_SEED_TENANT_SLUG:-medlab}"

# Owner local del tenant semilla. Estos defaults evitan que `make seed`
# deje como owner a usuarios placeholder de Clerk.
DEFAULT_SEED_OWNER_EXTERNAL_ID="${DEFAULT_SEED_OWNER_EXTERNAL_ID:-user_3AXavi5Algpygf3F8NxLWf5r88I}"
DEFAULT_SEED_OWNER_EMAIL="${DEFAULT_SEED_OWNER_EMAIL:-devpablocristo@gmail.com}"
DEFAULT_SEED_OWNER_GIVEN_NAME="${DEFAULT_SEED_OWNER_GIVEN_NAME:-Pablo}"
DEFAULT_SEED_OWNER_FAMILY_NAME="${DEFAULT_SEED_OWNER_FAMILY_NAME:-Cristo}"
export DEFAULT_SEED_OWNER_EXTERNAL_ID DEFAULT_SEED_OWNER_EMAIL DEFAULT_SEED_OWNER_GIVEN_NAME DEFAULT_SEED_OWNER_FAMILY_NAME

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

derive_seed_tenant_slug() {
  local external_id="$1"
  local cleaned="${external_id#org_}"
  cleaned="$(printf '%s' "$cleaned" | tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '-' | sed 's/^-*//;s/-*$//')"
  cleaned="${cleaned:-demo}"
  printf 'demo-%s\n' "$(printf '%s' "$cleaned" | cut -c1-40)"
}

seed_tenant_slug_exists() {
  local slug="$1"
  local slug_sql="${slug//\'/\'\'}"
  local found=""

  if command -v psql >/dev/null 2>&1; then
    found="$(
      host_pymes_psql \
        -Atq -v ON_ERROR_STOP=1 \
        -c "SELECT 1 FROM tenants WHERE slug = '$slug_sql' LIMIT 1;" \
        2>/dev/null || true
    )"
    found="$(printf '%s' "$found" | tr -d '[:space:]')"
    [[ "$found" == "1" ]] && return 0
  fi

  found="$(
    dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT 1 FROM tenants WHERE slug = '$slug_sql' LIMIT 1;" \
      2>/dev/null || true
  )"
  found="$(printf '%s' "$found" | tr -d '[:space:]')"
  [[ "$found" == "1" ]]
}

require_seed_tenant_selector() {
  if [[ -n "${PYMES_SEED_DEMO_TENANT_SLUG:-}" || -n "${PYMES_SEED_DEMO_TENANT_EXTERNAL_ID:-}" ]]; then
    return
  fi

  # Local developer default: seed the tenant people are actively testing.
  # CI/GCP should pass either PYMES_SEED_DEMO_TENANT_SLUG or PYMES_SEED_DEMO_TENANT_EXTERNAL_ID explicitly.
  if seed_tenant_slug_exists "$DEFAULT_SEED_TENANT_SLUG"; then
    PYMES_SEED_DEMO_TENANT_SLUG="$DEFAULT_SEED_TENANT_SLUG"
    export PYMES_SEED_DEMO_TENANT_SLUG
    return
  fi
  if seed_tenant_slug_exists "bicimax"; then
    PYMES_SEED_DEMO_TENANT_SLUG="bicimax"
    export PYMES_SEED_DEMO_TENANT_SLUG
    return
  fi

  PYMES_SEED_DEMO_TENANT_EXTERNAL_ID="org_local_demo"
  export PYMES_SEED_DEMO_TENANT_EXTERNAL_ID
}

require_seed_tenant_external_id() {
  if [[ -z "${PYMES_SEED_DEMO_TENANT_EXTERNAL_ID:-}" ]]; then
    PYMES_SEED_DEMO_TENANT_EXTERNAL_ID="org_local_demo"
    export PYMES_SEED_DEMO_TENANT_EXTERNAL_ID
  fi
}

resolve_target_tenant_uuid_by_slug() {
  local slug="$1"
  local slug_sql="${slug//\'/\'\'}"
  local tenant_uuid

  tenant_uuid="$(
    dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT cast(id as text) FROM tenants WHERE slug = '$slug_sql';"
  )"
  tenant_uuid="$(printf '%s' "$tenant_uuid" | tr -d '[:space:]')"
  if [[ -n "$tenant_uuid" ]]; then
    printf '%s\n' "$tenant_uuid"
    return 0
  fi

  if command -v psql >/dev/null 2>&1; then
    tenant_uuid="$(
      host_pymes_psql \
        -Atq -v ON_ERROR_STOP=1 \
        -c "SELECT cast(id as text) FROM tenants WHERE slug = '$slug_sql';" \
        2>/dev/null || true
    )"
    tenant_uuid="$(printf '%s' "$tenant_uuid" | tr -d '[:space:]')"
    if [[ -n "$tenant_uuid" ]]; then
      printf '%s\n' "$tenant_uuid"
      return 0
    fi
  fi

  tenant_uuid="$(
    dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT cast(uuid_generate_v5(uuid_ns_url(), 'pymes-seed/tenant-slug/' || '$slug_sql') as text);"
  )"
  tenant_uuid="$(printf '%s' "$tenant_uuid" | tr -d '[:space:]')"
  if [[ -z "$tenant_uuid" ]] && command -v python3 >/dev/null 2>&1; then
    tenant_uuid="$(
      python3 - "$slug" <<'PY'
import sys
import uuid

print(uuid.uuid5(uuid.NAMESPACE_URL, "pymes-seed/tenant-slug/" + sys.argv[1]))
PY
    )"
    tenant_uuid="$(printf '%s' "$tenant_uuid" | tr -d '[:space:]')"
  fi
  if [[ -z "$tenant_uuid" ]]; then
    echo "No se pudo resolver tenant uuid para slug=$slug" >&2
    exit 1
  fi
  printf '%s\n' "$tenant_uuid"
}

resolve_target_tenant_uuid() {
  require_seed_tenant_selector

  if [[ -n "${PYMES_SEED_DEMO_TENANT_SLUG:-}" ]]; then
    resolve_target_tenant_uuid_by_slug "$PYMES_SEED_DEMO_TENANT_SLUG"
    return 0
  fi

  local external_id="${PYMES_SEED_DEMO_TENANT_EXTERNAL_ID:-}"
  local external_id_sql="${external_id//\'/\'\'}"

  local tenant_uuid
  tenant_uuid="$(
    dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT cast(id as text) FROM tenants WHERE external_id = '$external_id_sql';"
  )"
  tenant_uuid="$(printf '%s' "$tenant_uuid" | tr -d '[:space:]')"
  if [[ -n "$tenant_uuid" ]]; then
    printf '%s\n' "$tenant_uuid"
    return 0
  fi

  if command -v psql >/dev/null 2>&1; then
    tenant_uuid="$(
      host_pymes_psql \
        -Atq -v ON_ERROR_STOP=1 \
        -c "SELECT cast(id as text) FROM tenants WHERE external_id = '$external_id_sql';" \
        2>/dev/null || true
    )"
    tenant_uuid="$(printf '%s' "$tenant_uuid" | tr -d '[:space:]')"
    if [[ -n "$tenant_uuid" ]]; then
      printf '%s\n' "$tenant_uuid"
      return 0
    fi
  fi

  tenant_uuid="$(
    dc exec -T postgres psql -U "$PYMES_DB_USER" -d "$PYMES_DB_NAME" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT cast(uuid_generate_v5(uuid_ns_url(), 'pymes-seed/tenant/' || '$external_id_sql') as text);"
  )"
  tenant_uuid="$(printf '%s' "$tenant_uuid" | tr -d '[:space:]')"
  if [[ -z "$tenant_uuid" ]] && command -v python3 >/dev/null 2>&1; then
    tenant_uuid="$(
      python3 - "$external_id" <<'PY'
import sys
import uuid

print(uuid.uuid5(uuid.NAMESPACE_URL, "pymes-seed/tenant/" + sys.argv[1]))
PY
    )"
    tenant_uuid="$(printf '%s' "$tenant_uuid" | tr -d '[:space:]')"
  fi
  if [[ -z "$tenant_uuid" ]]; then
    echo "No se pudo resolver tenant uuid para external_id=$external_id" >&2
    exit 1
  fi
  printf '%s\n' "$tenant_uuid"
}

render_seed_sql() {
  local file="$1"
  local fullpath
  if [[ "$file" == /* ]]; then
    fullpath="$file"
  else
    fullpath="$ROOT_DIR/$file"
  fi
  python3 - "$fullpath" "$TARGET_TENANT_UUID" <<'PY'
import os
from pathlib import Path
import sys

path = Path(sys.argv[1])
target_tenant_uuid = sys.argv[2]
body = path.read_text()

def sql_escape(value: str) -> str:
    return value.replace("'", "''")

replacements = {
    "__SEED_TENANT_ID__": target_tenant_uuid,
    "__SEED_TENANT_EXTERNAL_ID__": sql_escape(os.environ.get("SEED_TENANT_EXTERNAL_ID", "")),
    "__SEED_TENANT_NAME__": sql_escape(os.environ.get("SEED_TENANT_NAME", "")),
    "__SEED_TENANT_SLUG__": sql_escape(os.environ.get("SEED_TENANT_SLUG", "")),
    "__SEED_OWNER_EXTERNAL_ID__": sql_escape(os.environ.get("PYMES_SEED_OWNER_EXTERNAL_ID", os.environ.get("DEFAULT_SEED_OWNER_EXTERNAL_ID", ""))),
    "__SEED_OWNER_EMAIL__": sql_escape(os.environ.get("PYMES_SEED_OWNER_EMAIL", os.environ.get("DEFAULT_SEED_OWNER_EMAIL", ""))),
    "__SEED_OWNER_GIVEN_NAME__": sql_escape(os.environ.get("PYMES_SEED_OWNER_GIVEN_NAME", os.environ.get("DEFAULT_SEED_OWNER_GIVEN_NAME", ""))),
    "__SEED_OWNER_FAMILY_NAME__": sql_escape(os.environ.get("PYMES_SEED_OWNER_FAMILY_NAME", os.environ.get("DEFAULT_SEED_OWNER_FAMILY_NAME", ""))),
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
