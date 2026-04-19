#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

ensure_seed_dbs_ready
require_seed_org_external_id

TARGET_ORG_UUID="$(resolve_target_org_uuid)"
SEED_ORG_EXTERNAL_ID="${PYMES_SEED_DEMO_ORG_EXTERNAL_ID}"
SEED_ORG_NAME="${PYMES_SEED_DEMO_ORG_NAME:-Pymes Demo Org}"
SEED_ORG_SLUG="${PYMES_SEED_DEMO_ORG_SLUG:-$(derive_seed_org_slug "$SEED_ORG_EXTERNAL_ID")}"
export TARGET_ORG_UUID SEED_ORG_EXTERNAL_ID SEED_ORG_NAME SEED_ORG_SLUG

for sql_file in \
  "pymes-core/backend/seeds/01_clerk_prereqs.sql" \
  "pymes-core/backend/seeds/02_core_business.sql" \
  "pymes-core/backend/seeds/03_rbac.sql" \
  "pymes-core/backend/seeds/04_full_demo.sql" \
  "pymes-core/backend/seeds/05_scheduling_demo.sql" \
  "workshops/backend/seeds/auto_repair_demo.sql" \
  "workshops/backend/seeds/bike_shop_demo.sql" \
  "professionals/backend/seeds/demo.sql" \
  "restaurants/backend/seeds/demo.sql"
do
  run_pymes_sql_file "$sql_file"
done

run_review_sql_file "scripts/seeds/review_demo.sql"
