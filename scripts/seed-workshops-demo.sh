#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LEGACY_DEMO_ORG_UUID="00000000-0000-0000-0000-000000000001"

resolve_target_org_uuid() {
  local external_id="${PYMES_SEED_DEMO_ORG_EXTERNAL_ID:-}"
  if [[ -z "$external_id" ]]; then
    printf '%s\n' "$LEGACY_DEMO_ORG_UUID"
    return
  fi

  local org_uuid
  org_uuid="$(
    docker compose exec -T postgres \
      psql -U postgres -d pymes -Atq -v ON_ERROR_STOP=1 -v external_id="$external_id" \
      -c "SELECT id::text FROM orgs WHERE external_id = :'external_id';"
  )"
  org_uuid="$(printf '%s' "$org_uuid" | tr -d '[:space:]')"
  if [[ -z "$org_uuid" ]]; then
    echo "No existe org con external_id=$external_id para aplicar seeds demo" >&2
    exit 1
  fi
  printf '%s\n' "$org_uuid"
}

render_sql() {
  local file="$1"
  python3 - "$ROOT_DIR/$file" "$TARGET_ORG_UUID" "$LEGACY_DEMO_ORG_UUID" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
target_org_uuid = sys.argv[2]
legacy_org_uuid = sys.argv[3]
body = path.read_text()
body = body.replace(legacy_org_uuid, target_org_uuid)
sys.stdout.write(body)
PY
}

run_sql() {
  local file="$1"
  render_sql "$file" | docker compose exec -T postgres psql -U postgres -d pymes -v ON_ERROR_STOP=1
}

TARGET_ORG_UUID="$(resolve_target_org_uuid)"

run_sql "workshops/backend/seeds/auto_repair_demo.sql"
