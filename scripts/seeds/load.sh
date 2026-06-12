#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

ensure_seed_dbs_ready
require_seed_tenant_selector

TARGET_TENANT_UUID="$(resolve_target_tenant_uuid)"
SEED_TENANT_EXTERNAL_ID="${PYMES_SEED_DEMO_TENANT_EXTERNAL_ID:-}"
SEED_TENANT_NAME="${PYMES_SEED_DEMO_TENANT_NAME:-Pymes Demo Tenant}"
SEED_TENANT_SLUG="${PYMES_SEED_DEMO_TENANT_SLUG:-$(derive_seed_tenant_slug "$SEED_TENANT_EXTERNAL_ID")}"
export TARGET_TENANT_UUID SEED_TENANT_EXTERNAL_ID SEED_TENANT_NAME SEED_TENANT_SLUG

for sql_file in \
  "core/backend/seeds/01_clerk_prereqs.sql" \
  "core/backend/seeds/02_core_business.sql" \
  "core/backend/seeds/03_rbac.sql" \
  "core/backend/seeds/04_full_demo.sql" \
  "core/backend/seeds/05_scheduling_demo.sql" \
  "core/backend/seeds/06_bulk_demo.sql" \
  "workshops/backend/seeds/auto_repair_demo.sql" \
  "workshops/backend/seeds/bike_shop_demo.sql" \
  "professionals/backend/seeds/demo.sql" \
  "restaurants/backend/seeds/demo.sql" \
  "medical/backend/seeds/occupational_health_demo.sql"
do
  run_pymes_sql_file "$sql_file"
done

run_governance_sql_file "scripts/seeds/governance_demo.sql"
