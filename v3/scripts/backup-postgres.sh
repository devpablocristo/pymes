#!/usr/bin/env sh
set -eu

# Usage: SERVICE=pymes PYMES_DATABASE_URL=... ./scripts/backup-postgres.sh /secure/backups/pymes.sqlc
# The target path is explicit to prevent accidental overwrite of an unknown
# backup. Credentials stay in the runtime URL/secret manager, never in files.
service=${SERVICE:-pymes}
case "$service" in
  pymes) database_url=${PYMES_DATABASE_URL:?PYMES_DATABASE_URL is required} ;;
  fiscal) database_url=${FISCAL_DATABASE_URL:?FISCAL_DATABASE_URL is required} ;;
  accounting) database_url=${ACCOUNTING_DATABASE_URL:?ACCOUNTING_DATABASE_URL is required} ;;
  *) echo "SERVICE must be pymes, fiscal or accounting" >&2; exit 2 ;;
esac
target=${1:?explicit backup path is required}
case "$target" in *.dump|*.sqlc) ;; *) echo "backup must end in .dump or .sqlc" >&2; exit 2;; esac
if test -e "$target"; then
  echo "backup target already exists: $target" >&2
  exit 2
fi
pg_dump --format=custom --no-owner --file "$target" "$database_url"
