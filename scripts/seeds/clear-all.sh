#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

bash "$ROOT_DIR/scripts/seeds/reset-pymes-db.sh"
bash "$ROOT_DIR/scripts/seeds/reset-review-db.sh"

bash "$ROOT_DIR/scripts/seeds/migrate-core.sh"
bash "$ROOT_DIR/scripts/seeds/migrate-workshops.sh"

cd "$ROOT_DIR"
docker compose up -d review cp-backend work-backend prof-backend beauty-backend restaurants-backend frontend ai
