#!/usr/bin/env bash
# Limpia estado "dirty" de golang-migrate para pymes-core en la BD configurada en Secret Manager.
# Pensado para dev (datos prescindibles). Solo toca la base indicada en DATABASE_URL (ej. .../pymes).
#
# Requisitos: gcloud autenticado, Cloud SQL Client IAM sobre la instancia del secreto,
# curl/psql, y cloud-sql-proxy en PATH o en CLOUD_SQL_PROXY_BIN.
#
# Uso:
#   PROJECT_ID=pymes-dev-352318 ./scripts/fix-pymes-core-migration-dirty-gcp.sh status
#   PROJECT_ID=pymes-dev-352318 ./scripts/fix-pymes-core-migration-dirty-gcp.sh check-clean
#   PROJECT_ID=pymes-dev-352318 ./scripts/fix-pymes-core-migration-dirty-gcp.sh rewind-to 40
#   PROJECT_ID=pymes-dev-352318 ./scripts/fix-pymes-core-migration-dirty-gcp.sh repair-known-dev-dirty
#
# rewind-to N: deja la tabla de migraciones en versión N sin dirty (el próximo arranque del backend
# reaplicará N+1...). Si la migración N+1 quedó a medias, puede fallar hasta hacer DROP SCHEMA
# con usuario administrador (postgres).
#
# Seguridad: NO ejecutar contra bases o proyectos ajenos (p. ej. Ponti / otros tenants).
# Por defecto se aborta si la instancia Cloud SQL no pertenece al mismo PROJECT_ID que pasaste,
# salvo que exportes PYMES_SQL_FIX_ALLOW_FOREIGN_INSTANCE=yes (solo si sos responsable de ese riesgo).
#
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-}"
MODE="${1:-}"
ARG="${2:-}"

if [[ -z "$PROJECT_ID" ]]; then
  echo "Definí PROJECT_ID (ej. pymes-dev-352318)." >&2
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
  echo "Instalá cloud-sql-proxy v2 (https://cloud.google.com/sql/docs/mysql/sql-proxy) o definí CLOUD_SQL_PROXY_BIN." >&2
  exit 1
fi

DBRAW="$(gcloud secrets versions access latest --secret=DATABASE_URL --project="$PROJECT_ID")"
export DBRAW

eval "$(python3 <<'PY'
import os, re
from urllib.parse import unquote

raw = os.environ["DBRAW"].strip().replace("\n", "")
m = re.match(r"postgres://([^:]+):([^@]*)@/([^?]+)\?(?:.*host=/cloudsql/([^&]+))", raw)
if not m:
    raise SystemExit("DATABASE_URL inesperado (se espera ...@/DB?...host=/cloudsql/PROJECT:REGION:INSTANCE)")
user, pw, db, conn = m.group(1), unquote(m.group(2)), m.group(3), m.group(4)
# Emit shell-safe assignments (password may contain quotes)
import shlex
print(f"export PGINSTANCE_CONN={shlex.quote(conn)}")
print(f"export PGUSER={shlex.quote(user)}")
print(f"export PGPASSWORD={shlex.quote(pw)}")
print(f"export PGDATABASE={shlex.quote(db)}")
PY
)"

LOCAL_PORT="${LOCAL_PORT:-15432}"

echo "Instancia: $PGINSTANCE_CONN"
echo "Base de datos: $PGDATABASE (usuario $PGUSER)"

conn_project="${PGINSTANCE_CONN%%:*}"
if [[ "$conn_project" != "$PROJECT_ID" ]]; then
  echo "ERROR: DATABASE_URL apunta a Cloud SQL del proyecto '$conn_project', distinto de PROJECT_ID='$PROJECT_ID'." >&2
  echo "No se ejecuta nada para evitar tocar infraestructura ajena (p. ej. Ponti)." >&2
  echo "Corregí el secreto DATABASE_URL en este proyecto o exportá PYMES_SQL_FIX_ALLOW_FOREIGN_INSTANCE=yes si asumís el riesgo." >&2
  if [[ "${PYMES_SQL_FIX_ALLOW_FOREIGN_INSTANCE:-}" != "yes" ]]; then
    exit 2
  fi
  echo "Advertencia: PYMES_SQL_FIX_ALLOW_FOREIGN_INSTANCE=yes — continuando contra instancia foránea." >&2
fi

"$PROXY_BIN" --address 127.0.0.1 --port "$LOCAL_PORT" "$PGINSTANCE_CONN" &
proxy_pid=$!
cleanup() { kill "$proxy_pid" 2>/dev/null || true; wait "$proxy_pid" 2>/dev/null || true; }
trap cleanup EXIT
sleep 2

export PGHOST=127.0.0.1
export PGPORT="$LOCAL_PORT"

psql_cmd=(psql -v ON_ERROR_STOP=1)

show_status() {
  "${psql_cmd[@]}" -c "SELECT 'pymes_core', version, dirty FROM pymes_core_schema_migrations ORDER BY version;"
  "${psql_cmd[@]}" -c "SELECT 'post_scheduling', version, dirty FROM pymes_core_post_scheduling_schema_migrations ORDER BY version;" 2>/dev/null || true
}

check_clean() {
  local dirty_rows
  dirty_rows="$("${psql_cmd[@]}" -Atq -c "SELECT 'pymes_core:' || version FROM pymes_core_schema_migrations WHERE dirty IS TRUE;" || true)"
  dirty_rows="${dirty_rows}"$'\n'"$("${psql_cmd[@]}" -Atq -c "SELECT 'post_scheduling:' || version FROM pymes_core_post_scheduling_schema_migrations WHERE dirty IS TRUE;" 2>/dev/null || true)"
  dirty_rows="$(printf '%s' "$dirty_rows" | sed '/^[[:space:]]*$/d')"
  if [[ -n "$dirty_rows" ]]; then
    echo "ERROR: migraciones dirty detectadas:" >&2
    printf '%s\n' "$dirty_rows" >&2
    show_status >&2
    exit 3
  fi
  echo "OK: no hay migraciones dirty."
}

repair_known_dev_dirty() {
  local version dirty has_tenants has_orgs
  version="$("${psql_cmd[@]}" -Atq -c "SELECT version FROM pymes_core_schema_migrations LIMIT 1;")"
  dirty="$("${psql_cmd[@]}" -Atq -c "SELECT dirty FROM pymes_core_schema_migrations LIMIT 1;")"
  has_tenants="$("${psql_cmd[@]}" -Atq -c "SELECT to_regclass('public.tenants') IS NOT NULL;")"
  has_orgs="$("${psql_cmd[@]}" -Atq -c "SELECT to_regclass('public.orgs') IS NOT NULL;")"

  if [[ "$version" == "75" && "$dirty" == "t" && ( "$has_tenants" == "t" || "$has_orgs" == "t" ) ]]; then
    echo "Reparando dirty conocido: migración 75 falló durante el rename/hardening tenant. Rewind a 74 para reejecutarla con SQL idempotente."
    "${psql_cmd[@]}" -c "DELETE FROM pymes_core_schema_migrations;
INSERT INTO pymes_core_schema_migrations (version, dirty) VALUES (74, false);"
    show_status
    return
  fi

  if [[ "$dirty" == "t" ]]; then
    echo "ERROR: dirty state no reconocido para reparación automática: version=$version dirty=$dirty tenants=$has_tenants orgs=$has_orgs" >&2
    show_status >&2
    exit 4
  fi

  echo "OK: no hay dirty state conocido para reparar."
}

case "$MODE" in
  status)
    show_status
    ;;
  check-clean)
    check_clean
    ;;
  clear-dirty)
    "${psql_cmd[@]}" -c "UPDATE pymes_core_schema_migrations SET dirty = false WHERE dirty = true;"
    show_status
    ;;
  rewind-to)
    if [[ -z "$ARG" || ! "$ARG" =~ ^[0-9]+$ ]]; then
      echo "Uso: $0 rewind-to <version_entera>" >&2
      exit 1
    fi
    "${psql_cmd[@]}" -c "DELETE FROM pymes_core_schema_migrations;
INSERT INTO pymes_core_schema_migrations (version, dirty) VALUES ($ARG, false);"
    show_status
    ;;
  repair-known-dev-dirty)
    repair_known_dev_dirty
    ;;
  *)
    echo "Modos: status | check-clean | clear-dirty | rewind-to <n> | repair-known-dev-dirty" >&2
    exit 1
    ;;
esac
