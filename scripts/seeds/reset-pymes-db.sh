#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

stop_services_if_running cp-backend work-backend prof-backend beauty-backend restaurants-backend ai
ensure_services_up postgres
wait_for_pg postgres "$PYMES_DB_ADMIN_NAME" "$PYMES_DB_USER"

run_pymes_sql_inline "
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = '${PYMES_DB_NAME}'
  AND pid <> pg_backend_pid();
DROP DATABASE IF EXISTS ${PYMES_DB_NAME};
CREATE DATABASE ${PYMES_DB_NAME};
"
