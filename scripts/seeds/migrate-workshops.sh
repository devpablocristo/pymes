#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

ensure_services_up postgres
wait_for_pg postgres "$PYMES_DB_ADMIN_NAME" "$PYMES_DB_USER"

dc run --rm -T --entrypoint go -e PYMES_SEED_DEMO=false work-backend run ./cmd/migrate
