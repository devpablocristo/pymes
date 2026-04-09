#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

TARGET_ORG_UUID="$(resolve_target_org_uuid)"
# Misma ruta que MODULES_REPO_PATH en docker-compose (.env); por defecto repo hermano `modules`.
if [[ -n "${MODULES_REPO_PATH:-}" ]]; then
  _mods_root="$(cd "$ROOT_DIR" && cd "$MODULES_REPO_PATH" && pwd)"
else
  _mods_root="$(cd "$ROOT_DIR/../modules" && pwd)"
fi
run_pymes_sql_file "$_mods_root/scheduling/go/seeds/0001_demo.sql"
run_pymes_sql_file "$_mods_root/scheduling/go/seeds/0002_catchall_service.sql"
run_pymes_sql_file "$_mods_root/scheduling/go/seeds/0003_demo_bookings.sql"
