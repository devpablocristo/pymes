#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

bash "$ROOT_DIR/scripts/seeds/migrate-core.sh"
bash "$ROOT_DIR/scripts/seeds/migrate-workshops.sh"

if [[ -n "${PYMES_SEED_DEMO_ORG_EXTERNAL_ID:-}" ]]; then
  bash "$ROOT_DIR/scripts/seeds/core-01-clerk-prereqs.sh"
else
  bash "$ROOT_DIR/scripts/seeds/core-01-local-org.sh"
fi

bash "$ROOT_DIR/scripts/seeds/core-02-core-business.sh"
bash "$ROOT_DIR/scripts/seeds/core-03-rbac.sh"
bash "$ROOT_DIR/scripts/seeds/core-04-transversal-modules.sh"
bash "$ROOT_DIR/scripts/seeds/core-05-in-app-notifications.sh"
bash "$ROOT_DIR/scripts/seeds/core-06-scheduling.sh"
bash "$ROOT_DIR/scripts/seeds/workshops-01-auto-repair.sh"

bash "$ROOT_DIR/scripts/seeds/review-01-policies.sh"

cd "$ROOT_DIR"
docker compose up -d review cp-backend work-backend prof-backend beauty-backend restaurants-backend frontend ai
