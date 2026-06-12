#!/usr/bin/env bash
# Limpia estados "dirty" conocidos de las migraciones de verticales en DEV.
# Solo repara el caso seguro: version=1 dirty=true sin tablas propias creadas,
# donde se puede borrar la tabla de migrations para que 0001 corra desde cero.
#
# Uso:
#   PROJECT_ID=pymes-dev-352318 ./scripts/fix-pymes-vertical-migration-dirty-gcp.sh status
#   PROJECT_ID=pymes-dev-352318 ./scripts/fix-pymes-vertical-migration-dirty-gcp.sh repair-known-dev-dirty
#   PROJECT_ID=pymes-dev-352318 ./scripts/fix-pymes-vertical-migration-dirty-gcp.sh check-clean
#
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-}"
DATABASE_URL_SECRET_NAME="${DATABASE_URL_SECRET_NAME:-DATABASE_URL}"
MODE="${1:-}"

if [[ -z "$PROJECT_ID" ]]; then
  echo "Defini PROJECT_ID (ej. pymes-dev-352318)." >&2
  exit 1
fi

PROXY_BIN="${CLOUD_SQL_PROXY_BIN:-}"
if [[ -z "$PROXY_BIN" ]]; then
  if command -v cloud-sql-proxy >/dev/null 2>&1; then
    PROXY_BIN="$(command -v cloud-sql-proxy)"
  elif [[ -x "/tmp/cloud-sql-proxy" ]]; then
    PROXY_BIN="/tmp/cloud-sql-proxy"
  fi
fi
if [[ -z "$PROXY_BIN" || ! -x "$PROXY_BIN" ]]; then
  echo "Instala cloud-sql-proxy v2 o defini CLOUD_SQL_PROXY_BIN." >&2
  exit 1
fi

DBRAW="$(gcloud secrets versions access latest --secret="$DATABASE_URL_SECRET_NAME" --project="$PROJECT_ID")"
export DBRAW

eval "$(python3 <<'PY'
import os
import re
import shlex
from urllib.parse import unquote

raw = os.environ["DBRAW"].strip().replace("\n", "")
m = re.match(r"postgres://([^:]+):([^@]*)@/([^?]+)\?(?:.*host=/cloudsql/([^&]+))", raw)
if not m:
    raise SystemExit("DATABASE_URL inesperado (se espera ...@/DB?...host=/cloudsql/PROJECT:REGION:INSTANCE)")
user, pw, db, conn = m.group(1), unquote(m.group(2)), m.group(3), m.group(4)
print(f"export PGINSTANCE_CONN={shlex.quote(conn)}")
print(f"export PGUSER={shlex.quote(user)}")
print(f"export PGPASSWORD={shlex.quote(pw)}")
print(f"export PGDATABASE={shlex.quote(db)}")
PY
)"

LOCAL_PORT="${LOCAL_PORT:-15433}"

echo "Instancia: $PGINSTANCE_CONN"
echo "Base de datos: $PGDATABASE (usuario $PGUSER)"

conn_project="${PGINSTANCE_CONN%%:*}"
if [[ "$conn_project" != "$PROJECT_ID" ]]; then
  echo "ERROR: $DATABASE_URL_SECRET_NAME apunta a Cloud SQL del proyecto '$conn_project', distinto de PROJECT_ID='$PROJECT_ID'." >&2
  if [[ "${PYMES_SQL_FIX_ALLOW_FOREIGN_INSTANCE:-}" != "yes" ]]; then
    exit 2
  fi
  echo "Advertencia: PYMES_SQL_FIX_ALLOW_FOREIGN_INSTANCE=yes; continuando contra instancia foranea." >&2
fi

"$PROXY_BIN" --address 127.0.0.1 --port "$LOCAL_PORT" "$PGINSTANCE_CONN" &
proxy_pid=$!
cleanup() { kill "$proxy_pid" 2>/dev/null || true; wait "$proxy_pid" 2>/dev/null || true; }
trap cleanup EXIT
sleep 2

export PGHOST=127.0.0.1
export PGPORT="$LOCAL_PORT"

psql_cmd=(psql -v ON_ERROR_STOP=1)

vertical_specs=(
  "professionals:schema_migrations_professionals:professionals.professional_profiles"
  "restaurants:schema_migrations_restaurant:restaurant.dining_areas"
  "workshops:schema_migrations_workshops:workshops.vehicles"
  "beauty:schema_migrations_beauty:"
  "medical:schema_migrations_medical:"
)

sql_value() {
  "${psql_cmd[@]}" -Atq -c "$1"
}

table_exists() {
  local table="$1"
  [[ "$(sql_value "SELECT to_regclass('public.${table}') IS NOT NULL;")" == "t" ]]
}

relation_exists() {
  local relation="$1"
  [[ "$(sql_value "SELECT to_regclass('${relation}') IS NOT NULL;")" == "t" ]]
}

show_status() {
  local spec vertical table sentinel version dirty
  for spec in "${vertical_specs[@]}"; do
    IFS=: read -r vertical table sentinel <<<"$spec"
    if table_exists "$table"; then
      IFS='|' read -r version dirty < <(sql_value "SELECT version, dirty FROM public.${table} LIMIT 1;")
      echo "$vertical $table version=$version dirty=$dirty"
    else
      echo "$vertical $table missing"
    fi
  done
}

repair_known_dev_dirty() {
  local spec vertical table sentinel version dirty
  for spec in "${vertical_specs[@]}"; do
    IFS=: read -r vertical table sentinel <<<"$spec"
    if ! table_exists "$table"; then
      echo "OK: $vertical no tiene $table."
      continue
    fi

    IFS='|' read -r version dirty < <(sql_value "SELECT version, dirty FROM public.${table} LIMIT 1;")
    if [[ "$dirty" != "t" ]]; then
      echo "OK: $vertical version=$version no dirty."
      continue
    fi

    if [[ "$version" == "1" ]]; then
      if [[ -n "$sentinel" ]] && relation_exists "$sentinel"; then
        echo "ERROR: $vertical version=1 dirty pero $sentinel existe; no se repara automaticamente." >&2
        exit 4
      fi
      echo "Reparando $vertical: version=1 dirty sin tablas propias; se borra $table para reintentar 0001."
      "${psql_cmd[@]}" -c "DROP TABLE IF EXISTS public.${table};"
      continue
    fi

    echo "ERROR: dirty state no reconocido para $vertical: version=$version dirty=$dirty." >&2
    exit 5
  done
}

check_clean() {
  local spec vertical table dirty dirty_rows=""
  for spec in "${vertical_specs[@]}"; do
    IFS=: read -r vertical table _ <<<"$spec"
    if table_exists "$table"; then
      dirty="$(sql_value "SELECT dirty FROM public.${table} LIMIT 1;")"
      if [[ "$dirty" == "t" ]]; then
        dirty_rows="${dirty_rows}${vertical}:${table}"$'\n'
      fi
    fi
  done

  dirty_rows="$(printf '%s' "$dirty_rows" | sed '/^[[:space:]]*$/d')"
  if [[ -n "$dirty_rows" ]]; then
    echo "ERROR: migraciones dirty detectadas:" >&2
    printf '%s\n' "$dirty_rows" >&2
    show_status >&2
    exit 3
  fi
  echo "OK: no hay migraciones dirty en verticales."
}

case "$MODE" in
  status)
    show_status
    ;;
  repair-known-dev-dirty)
    repair_known_dev_dirty
    ;;
  check-clean)
    check_clean
    ;;
  *)
    echo "Modos: status | repair-known-dev-dirty | check-clean" >&2
    exit 1
    ;;
esac
