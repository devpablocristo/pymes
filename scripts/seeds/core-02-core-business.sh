#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

TARGET_ORG_UUID="$(resolve_target_org_uuid)"
run_pymes_sql_file "pymes-core/backend/seeds/02_core_business.sql"
