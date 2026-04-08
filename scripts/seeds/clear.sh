#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

ensure_seed_dbs_ready

bash "$ROOT_DIR/scripts/seeds/clear-review.sh"
bash "$ROOT_DIR/scripts/seeds/clear-workshops.sh"
bash "$ROOT_DIR/scripts/seeds/clear-core.sh"
