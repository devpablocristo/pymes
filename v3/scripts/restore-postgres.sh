#!/usr/bin/env sh
set -eu

# Recovery is intentionally opt-in: the operator must name both source and
# target database, and pg_restore refuses to continue on the first error.
service=${SERVICE:-pymes}
case "$service" in
  pymes) database_url=${PYMES_RESTORE_DATABASE_URL:?PYMES_RESTORE_DATABASE_URL is required} ;;
  fiscal) database_url=${FISCAL_RESTORE_DATABASE_URL:?FISCAL_RESTORE_DATABASE_URL is required} ;;
  accounting) database_url=${ACCOUNTING_RESTORE_DATABASE_URL:?ACCOUNTING_RESTORE_DATABASE_URL is required} ;;
  *) echo "SERVICE must be pymes, fiscal or accounting" >&2; exit 2 ;;
esac
source=${1:?explicit backup path is required}
test -f "$source"
pg_restore --clean --if-exists --no-owner --exit-on-error --dbname "$database_url" "$source"
