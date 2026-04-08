#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

TARGET_ORG_UUID="$(resolve_target_org_uuid)"
run_pymes_sql_file "../modules/scheduling/go/seeds/0001_demo.sql"
run_pymes_sql_file "../modules/scheduling/go/seeds/0002_catchall_service.sql"
run_pymes_sql_file "../modules/scheduling/go/seeds/0003_demo_bookings.sql"
