#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

stop_services_if_running review
ensure_services_up review-postgres
wait_for_pg review-postgres "$REVIEW_DB_ADMIN_NAME" "$REVIEW_DB_USER"

run_review_sql_inline "
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = '${REVIEW_DB_NAME}'
  AND pid <> pg_backend_pid();
DROP DATABASE IF EXISTS ${REVIEW_DB_NAME};
CREATE DATABASE ${REVIEW_DB_NAME};
"
