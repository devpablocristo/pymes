#!/usr/bin/env bash
# Load demo seeds for the remote MedLab tenant in GCP Cloud SQL.
#
# This script is intentionally separate from load-gcp-pymes-single-tenant.sh so
# the generic GCP loader remains unchanged. It only targets the pymes DB and does
# not touch Governance/Nexus.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

GCP_PROJECT_ID="${GCP_PROJECT_ID:-pymes-dev-352318}"
PYMES_SEED_DEMO_TENANT_EXTERNAL_ID="${PYMES_SEED_DEMO_TENANT_EXTERNAL_ID:-org_3DW6a7d4dFd0o3ntPWcS6RKRfas}"
PYMES_SEED_DEMO_TENANT_NAME="${PYMES_SEED_DEMO_TENANT_NAME:-MedLab}"
PYMES_SEED_DEMO_TENANT_SLUG="${PYMES_SEED_DEMO_TENANT_SLUG:-medlab}"
PYMES_GCP_INCLUDE_MEDICAL_SEEDS="${PYMES_GCP_INCLUDE_MEDICAL_SEEDS:-1}"
PYMES_SEEDS_SKIP_DOTENV=1

export GCP_PROJECT_ID
export PYMES_SEED_DEMO_TENANT_EXTERNAL_ID
export PYMES_SEED_DEMO_TENANT_NAME
export PYMES_SEED_DEMO_TENANT_SLUG
export PYMES_GCP_INCLUDE_MEDICAL_SEEDS
export PYMES_SEEDS_SKIP_DOTENV

started_proxy=0
proxy_pid=""

cleanup() {
  if [[ "$started_proxy" == "1" && -n "${proxy_pid:-}" ]]; then
    kill "$proxy_pid" 2>/dev/null || true
    wait "$proxy_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT

require_command() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd" >&2
    exit 1
  fi
}

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
  return 1
}

require_command gcloud
require_command psql
require_command python3

PROXY_BIN="$(resolve_proxy_bin)" || {
  echo "Install cloud-sql-proxy v2 or set CLOUD_SQL_PROXY_BIN." >&2
  exit 1
}

active_account="$(gcloud auth list --filter=status:ACTIVE --format='value(account)' | head -n 1)"
if [[ -z "$active_account" ]]; then
  echo "No active gcloud account. Run: gcloud auth login" >&2
  exit 1
fi

active_project="$(gcloud config get-value project 2>/dev/null || true)"
if [[ "$active_project" != "$GCP_PROJECT_ID" && "${ALLOW_GCLOUD_PROJECT_MISMATCH:-}" != "1" ]]; then
  echo "Active gcloud project is '$active_project', expected '$GCP_PROJECT_ID'." >&2
  echo "Run: gcloud config set project $GCP_PROJECT_ID" >&2
  exit 1
fi

db_raw="$(gcloud secrets versions access latest --secret=DATABASE_URL --project="$GCP_PROJECT_ID")"

mapfile -t db_parts < <(DB_RAW="$db_raw" python3 - <<'PY'
import os
import re
from urllib.parse import unquote

raw = os.environ["DB_RAW"].strip()
m = re.match(r"postgres://([^:]+):([^@]*)@/([^?]+)\?(?:.*host=/cloudsql/([^&]+))", raw)
if not m:
    raise SystemExit("DATABASE_URL secret has an unexpected format")
print(m.group(4))
print(m.group(1))
print(unquote(m.group(2)))
print(m.group(3))
PY
)

cloudsql_instance="${db_parts[0]}"
pg_user="${db_parts[1]}"
pg_password="${db_parts[2]}"
pg_database="${db_parts[3]}"

conn_project="${cloudsql_instance%%:*}"
if [[ "$conn_project" != "$GCP_PROJECT_ID" ]]; then
  echo "DATABASE_URL points to Cloud SQL project '$conn_project', expected '$GCP_PROJECT_ID'." >&2
  exit 2
fi

LOCAL_PORT="${LOCAL_PORT:-15435}"
"$PROXY_BIN" --address 127.0.0.1 --port "$LOCAL_PORT" "$cloudsql_instance" >/tmp/pymes-medlab-cloud-sql-proxy.log 2>&1 &
proxy_pid=$!
started_proxy=1
sleep 3

export PGUSER="$pg_user"
export PGPASSWORD="$pg_password"
export PGDATABASE="$pg_database"
export PGHOST="127.0.0.1"
export PGPORT="$LOCAL_PORT"
export PYMES_SEED_DATABASE_URI="postgresql://127.0.0.1:${LOCAL_PORT}/${pg_database}?sslmode=disable"

# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

resolve_target_tenant_uuid() {
  local external_id="${PYMES_SEED_DEMO_TENANT_EXTERNAL_ID:-}"
  local external_id_sql="${external_id//\'/\'\'}"
  local slug="${PYMES_SEED_DEMO_TENANT_SLUG:-}"
  local slug_sql="${slug//\'/\'\'}"
  local tenant_uuid

  tenant_uuid="$(
    psql "$PYMES_SEED_DATABASE_URI" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT cast(id as text) FROM orgs WHERE external_id = '$external_id_sql' AND slug = '$slug_sql';"
  )"
  tenant_uuid="$(printf '%s' "$tenant_uuid" | tr -d '[:space:]')"
  if [[ -z "$tenant_uuid" ]]; then
    echo "Could not resolve orgs.id for external_id='$external_id' and slug='$slug'." >&2
    exit 1
  fi
  printf '%s\n' "$tenant_uuid"
}

run_remote_seed_sql_file() {
  local file="$1"
  echo "-> $file"
  render_seed_sql "$file" | psql "$PYMES_SEED_DATABASE_URI" -v ON_ERROR_STOP=1
}

require_table() {
  local table_ref="$1"
  local found
  found="$(
    psql "$PYMES_SEED_DATABASE_URI" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT to_regclass('$table_ref') IS NOT NULL;"
  )"
  found="$(printf '%s' "$found" | tr -d '[:space:]')"
  if [[ "$found" != "t" ]]; then
    echo "Required table '$table_ref' does not exist in remote DB." >&2
    exit 1
  fi
}

TARGET_TENANT_UUID="$(resolve_target_tenant_uuid)"
SEED_TENANT_EXTERNAL_ID="$PYMES_SEED_DEMO_TENANT_EXTERNAL_ID"
SEED_TENANT_NAME="$PYMES_SEED_DEMO_TENANT_NAME"
SEED_TENANT_SLUG="$PYMES_SEED_DEMO_TENANT_SLUG"
export TARGET_TENANT_UUID SEED_TENANT_EXTERNAL_ID SEED_TENANT_NAME SEED_TENANT_SLUG

files=(
  "pymes-core/backend/seeds/01_clerk_prereqs.sql"
  "pymes-core/backend/seeds/02_core_business.sql"
  "pymes-core/backend/seeds/03_rbac.sql"
  "pymes-core/backend/seeds/04_full_demo.sql"
  "pymes-core/backend/seeds/05_scheduling_demo.sql"
)

if [[ "$PYMES_GCP_INCLUDE_MEDICAL_SEEDS" == "1" ]]; then
  require_table "medical.occupational_health_exams"
  files+=("medical/backend/seeds/occupational_health_demo.sql")
fi

echo "Remote seed target:"
echo "  Project: $GCP_PROJECT_ID"
echo "  Active gcloud account: $active_account"
echo "  Cloud SQL: $cloudsql_instance"
echo "  Database: $pg_database"
echo "  Tenant name: $SEED_TENANT_NAME"
echo "  Tenant slug: $SEED_TENANT_SLUG"
echo "  Tenant external_id: $SEED_TENANT_EXTERNAL_ID"
echo "  Tenant UUID: $TARGET_TENANT_UUID"
echo "  Seeds: core + scheduling$([[ "$PYMES_GCP_INCLUDE_MEDICAL_SEEDS" == "1" ]] && printf ' + medical')"
echo "  Governance/Nexus: no"
echo "  Workshops: no"
echo "SQL files:"
printf '  - %s\n' "${files[@]}"

for sql_file in "${files[@]}"; do
  run_remote_seed_sql_file "$sql_file"
done

if [[ "$PYMES_GCP_INCLUDE_MEDICAL_SEEDS" == "1" ]]; then
  medical_count="$(
    psql "$PYMES_SEED_DATABASE_URI" -Atq -v ON_ERROR_STOP=1 \
      -c "SELECT count(*) FROM medical.occupational_health_exams WHERE org_id = '$TARGET_TENANT_UUID' AND deleted_at IS NULL;"
  )"
  medical_count="$(printf '%s' "$medical_count" | tr -d '[:space:]')"
  echo "Medical occupational health exams for tenant: $medical_count"
  if (( medical_count < 10 )); then
    echo "Expected at least 10 medical occupational health exams, got $medical_count." >&2
    exit 1
  fi
fi

echo "OK - remote MedLab seeds loaded. Governance/Nexus was not touched."
