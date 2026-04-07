#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

ensure_services_up postgres review-postgres review
wait_for_pg postgres "$PYMES_DB_ADMIN_NAME" "$PYMES_DB_USER"
wait_for_pg review-postgres "$REVIEW_DB_ADMIN_NAME" "$REVIEW_DB_USER"
wait_for_http review "http://127.0.0.1:8080/readyz"

dc run --rm -T --entrypoint go -e PYMES_SEED_DEMO=false cp-backend run ./cmd/migrate
