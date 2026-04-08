#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

ensure_seed_dbs_ready

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
bash "$ROOT_DIR/scripts/seeds/load-review.sh"
