#!/bin/sh
set -eu

: "${FISCAL_DATABASE_URL:?FISCAL_DATABASE_URL is required}"
: "${FISCAL_DEPLOY_ENV:?FISCAL_DEPLOY_ENV is required}"

case "$FISCAL_DEPLOY_ENV" in
  stg|prd) ;;
  *)
    echo "FISCAL_DEPLOY_ENV must be stg or prd" >&2
    exit 2
    ;;
esac

database="pymes_v3_fiscal_${FISCAL_DEPLOY_ENV}"
session_role="pymes_v3_fiscal_migrate_${FISCAL_DEPLOY_ENV}"
effective_role="pymes_v3_fiscal_owner_${FISCAL_DEPLOY_ENV}"
socket="/cloudsql/pymes-dev-352318:us-central1:pymes-dev-db"
url_prefix="postgres://${session_role}:"
url_suffix="@/${database}?host=${socket}&sslmode=disable&options=-c%20role%3D${effective_role}"

case "$FISCAL_DATABASE_URL" in
  "$url_prefix"*"$url_suffix") ;;
  *)
    echo "FISCAL_DATABASE_URL differs from the canonical Pymes fiscal migration target" >&2
    exit 2
    ;;
esac
password=${FISCAL_DATABASE_URL#"$url_prefix"}
password=${password%"$url_suffix"}
case "$password" in
  ""|*[!0-9a-f]*)
    echo "FISCAL_DATABASE_URL contains a non-canonical migration credential" >&2
    exit 2
    ;;
esac
if [ "${#password}" -ne 64 ] ||
  [ "$FISCAL_DATABASE_URL" != "${url_prefix}${password}${url_suffix}" ]; then
  echo "FISCAL_DATABASE_URL contains a non-canonical migration credential" >&2
  exit 2
fi

actual_identity=$(
  psql "$FISCAL_DATABASE_URL" -XAt -F '|' -v ON_ERROR_STOP=1 \
    -c 'SELECT current_database(), session_user, current_user'
)
expected_identity="${database}|${session_role}|${effective_role}"
if [ "$actual_identity" != "$expected_identity" ]; then
  echo "fiscal database identity differs from the canonical Pymes migration target" >&2
  exit 1
fi

migrations_dir=${FISCAL_MIGRATIONS_DIR:-/migrations}
for file in "$migrations_dir"/*.sql; do
  [ -f "$file" ] || {
    echo "fiscal migration file unavailable" >&2
    exit 1
  }
  psql "$FISCAL_DATABASE_URL" -X -v ON_ERROR_STOP=1 -f "$file"
done
