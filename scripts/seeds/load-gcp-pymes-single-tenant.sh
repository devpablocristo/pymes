#!/usr/bin/env bash
# Carga seeds del monorepo contra SOLO la base PostgreSQL `pymes` en GCP (proyecto Pymes).
# No toca Review/Nexus ni bases de otros proyectos.
#
# Requisitos:
#   - gcloud autenticado con acceso al proyecto y Secret Manager.
#   - cloud-sql-proxy v2 en PATH (https://cloud.google.com/sql/docs/postgres/sql-proxy).
#   - psql cliente.
#
# La org seed debe existir primero con ese external_id (p. ej. primer login Clerk con la org activa),
# salvo que uses un external_id nuevo y aceptes el UUID determinístico uuid_generate_v5 de los seeds locales.
#
# Uso típico (reemplazá org por tu Clerk Organization ID):
#
#   export GCP_PROJECT_ID=pymes-dev-352318
#   export PYMES_SEED_DEMO_ORG_EXTERNAL_ID='org_xxxxxxxx'
#   export PYMES_SEED_DEMO_ORG_NAME='Bicimax QA'
#   export PYMES_SEED_DEMO_ORG_SLUG='bicimax'
#   ./scripts/seeds/load-gcp-pymes-single-tenant.sh
#
# Por defecto: solo seeds de pymes-core (Cloud Run GCP no crea esquema workshops.*).
# Incluir seeds workshops (tablas workshops.*): solo si ese esquema existe en la BD
# (p. ej. migraste también el backend workshops contra esta misma DB).
#   export PYMES_GCP_INCLUDE_WORKSHOPS_SEEDS=1
#
# Incluir también professionals + restaurants (misma org):
#   export PYMES_GCP_SEED_ALL_VERTICALS=1
#   export PYMES_SEED_DATABASE_URI='postgres://pymes_app:PASS@127.0.0.1:15432/pymes?sslmode=disable'
#   export PYMES_SEEDS_USE_EXTERNAL_URI=1
#   ./scripts/seeds/load-gcp-pymes-single-tenant.sh
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

GCP_PROJECT_ID="${GCP_PROJECT_ID:-}"
PYMES_SEEDS_SKIP_DOTENV=1
export PYMES_SEEDS_SKIP_DOTENV

started_proxy=0
proxy_pid=""
cleanup() {
  if [[ "$started_proxy" == "1" ]] && [[ -n "${proxy_pid:-}" ]]; then
    kill "$proxy_pid" 2>/dev/null || true
    wait "$proxy_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT

if [[ -z "$GCP_PROJECT_ID" && "${PYMES_SEEDS_USE_EXTERNAL_URI:-}" != "1" ]]; then
  echo "Definí GCP_PROJECT_ID (ej. export GCP_PROJECT_ID=pymes-dev-352318) o usá PYMES_SEEDS_USE_EXTERNAL_URI=1 con PYMES_SEED_DATABASE_URI." >&2
  exit 1
fi

resolve_proxy_bin() {
  if [[ -n "${CLOUD_SQL_PROXY_BIN:-}" ]]; then
    printf '%s\n' "$CLOUD_SQL_PROXY_BIN"
    return 0
  fi
  if command -v cloud-sql-proxy >/dev/null 2>&1; then
    command -v cloud-sql-proxy
    return 0
  fi
  if [[ -x "/tmp/cloud-sql-proxy" ]]; then
    printf '%s\n' "/tmp/cloud-sql-proxy"
    return 0
  fi
  echo "" >&2
  return 1
}

if [[ "${PYMES_SEEDS_USE_EXTERNAL_URI:-}" == "1" ]]; then
  if [[ -z "${PYMES_SEED_DATABASE_URI:-}" ]]; then
    echo "PYMES_SEEDS_USE_EXTERNAL_URI=1 requiere PYMES_SEED_DATABASE_URI (URI postgres TCP)." >&2
    exit 1
  fi
  export PYMES_SEED_DATABASE_URI
else
  db_raw="$(gcloud secrets versions access latest --secret=DATABASE_URL --project="$GCP_PROJECT_ID")"
  conn="$(printf '%s' "$db_raw" | python3 -c "
import re, sys
raw = sys.stdin.read().strip()
m = re.match(r'postgres://([^:]+):([^@]*)@/([^?]+)\?(?:.*host=/cloudsql/([^&]+))', raw)
if not m:
    raise SystemExit('DATABASE_URL del secreto tiene formato inesperado')
print(m.group(4))
")"
  conn_project="${conn%%:*}"
  if [[ "$conn_project" != "$GCP_PROJECT_ID" ]]; then
    echo "ERROR: DATABASE_URL apunta a proyecto Cloud SQL '$conn_project', distinto de GCP_PROJECT_ID='$GCP_PROJECT_ID'. Aborto." >&2
    exit 2
  fi
  PROXY_BIN="$(resolve_proxy_bin)" || {
    echo "Instalá cloud-sql-proxy v2 o definí CLOUD_SQL_PROXY_BIN." >&2
    exit 1
  }
  LOCAL_PORT="${LOCAL_PORT:-15435}"
  export CLOUD_SQL_PROXY_BIN="$PROXY_BIN"
  "$PROXY_BIN" --address 127.0.0.1 --port "$LOCAL_PORT" "$conn" &
  proxy_pid=$!
  started_proxy=1
  sleep 3

  export PYMES_SEED_DATABASE_URI="$(
    printf '%s' "$db_raw" | python3 -c "
import re, sys
from urllib.parse import unquote
raw = sys.stdin.read().strip()
m = re.match(r'postgres://([^:]+):([^@]*)@/([^?]+)\?', raw)
user, pw, db = m.group(1), unquote(m.group(2)), m.group(3)
port = int(sys.argv[1])
print(f'postgres://{user}:{pw}@127.0.0.1:{port}/{db}?sslmode=disable')
" "$LOCAL_PORT"
  )"
fi

# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

ensure_seed_dbs_ready() {
  :
}

resolve_target_org_uuid() {
  require_seed_org_external_id
  local external_id="${PYMES_SEED_DEMO_ORG_EXTERNAL_ID:-}"
  local external_id_sql="${external_id//\'/\'\'}"
  local org_uuid
  org_uuid="$(psql "$PYMES_SEED_DATABASE_URI" -Atq -v ON_ERROR_STOP=1 \
    -c "SELECT cast(id as text) FROM orgs WHERE external_id = '$external_id_sql';")"
  org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
  if [[ -n "$org_uuid" ]]; then
    printf '%s\n' "$org_uuid"
    return 0
  fi
  org_uuid="$(psql "$PYMES_SEED_DATABASE_URI" -Atq -v ON_ERROR_STOP=1 \
    -c "SELECT cast(uuid_generate_v5(uuid_ns_url(), 'pymes-seed/org/' || '$external_id_sql') as text);")"
  org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
  if [[ -z "$org_uuid" ]]; then
    echo "No se pudo resolver org uuid para external_id=$external_id" >&2
    exit 1
  fi
  printf '%s\n' "$org_uuid"
}

run_pymes_sql_file() {
  local file="$1"
  render_seed_sql "$file" | psql "$PYMES_SEED_DATABASE_URI" -v ON_ERROR_STOP=1
}

require_seed_org_external_id
TARGET_ORG_UUID="$(resolve_target_org_uuid)"
SEED_ORG_EXTERNAL_ID="${PYMES_SEED_DEMO_ORG_EXTERNAL_ID}"
SEED_ORG_NAME="${PYMES_SEED_DEMO_ORG_NAME:-Pymes Demo Org}"
SEED_ORG_SLUG="${PYMES_SEED_DEMO_ORG_SLUG:-$(derive_seed_org_slug "$SEED_ORG_EXTERNAL_ID")}"
export TARGET_ORG_UUID SEED_ORG_EXTERNAL_ID SEED_ORG_NAME SEED_ORG_SLUG

echo "→ BD destino (solo URI local vía proxy si aplica); proyecto GCP declarado: ${GCP_PROJECT_ID:-URI externa}"
echo "→ Org externa: $SEED_ORG_EXTERNAL_ID | UUID seed: $TARGET_ORG_UUID | slug: $SEED_ORG_SLUG"

files=(
  "pymes-core/backend/seeds/01_clerk_prereqs.sql"
  "pymes-core/backend/seeds/02_core_business.sql"
  "pymes-core/backend/seeds/03_rbac.sql"
  "pymes-core/backend/seeds/04_full_demo.sql"
  "pymes-core/backend/seeds/05_scheduling_demo.sql"
)
if [[ "${PYMES_GCP_INCLUDE_WORKSHOPS_SEEDS:-}" == "1" ]]; then
  files+=(
    "workshops/backend/seeds/auto_repair_demo.sql"
    "workshops/backend/seeds/bike_shop_demo.sql"
  )
fi
if [[ "${PYMES_GCP_SEED_ALL_VERTICALS:-}" == "1" ]]; then
  files+=(
    "professionals/backend/seeds/demo.sql"
    "restaurants/backend/seeds/demo.sql"
  )
fi

for sql_file in "${files[@]}"; do
  echo "→ $(basename "$sql_file")"
  run_pymes_sql_file "$sql_file"
done

echo "OK — seeds aplicados en una sola org. Review/Nexus no fue tocado."
