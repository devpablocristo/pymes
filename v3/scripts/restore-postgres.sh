#!/usr/bin/env sh
set -eu
umask 077

# Recovery is intentionally opt-in: the operator must name both source and
# target database, and pg_restore refuses to continue on the first error.
script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
libpq_exec="$script_dir/libpq-exec.py"
test -r "$libpq_exec" || {
  echo "libpq command wrapper is required: $libpq_exec" >&2
  exit 1
}

for required_command in basename cut grep pg_restore psql python3 sed sha256sum; do
  command -v "$required_command" >/dev/null 2>&1 || {
    echo "$required_command is required" >&2
    exit 1
  }
done

service=${SERVICE:-pymes}
case "$service" in
  pymes)
    : "${PYMES_RESTORE_DATABASE_URL:?PYMES_RESTORE_DATABASE_URL is required}"
    connection_variable=PYMES_RESTORE_DATABASE_URL
    ;;
  fiscal)
    : "${FISCAL_RESTORE_DATABASE_URL:?FISCAL_RESTORE_DATABASE_URL is required}"
    connection_variable=FISCAL_RESTORE_DATABASE_URL
    ;;
  accounting)
    : "${ACCOUNTING_RESTORE_DATABASE_URL:?ACCOUNTING_RESTORE_DATABASE_URL is required}"
    connection_variable=ACCOUNTING_RESTORE_DATABASE_URL
    ;;
  *) echo "SERVICE must be pymes, fiscal or accounting" >&2; exit 2 ;;
esac
source=${1:?explicit backup path is required}
case "$source" in
  /*) ;;
  *) echo "restore path must be absolute" >&2; exit 2 ;;
esac
test -f "$source" && test ! -L "$source" || {
  echo "backup archive must be a regular non-symlink file" >&2
  exit 2
}
manifest="$source.sha256"
test -f "$manifest" && test ! -L "$manifest" || {
  echo "backup checksum manifest is required: $manifest" >&2
  exit 2
}

manifest_value() {
  field=$1
  matches=$(grep -c "^${field}=" "$manifest" || true)
  test "$matches" -eq 1 || {
    echo "backup manifest must contain exactly one $field" >&2
    exit 2
  }
  sed -n "s/^${field}=//p" "$manifest"
}

format=$(manifest_value format)
backup_service=$(manifest_value service)
source_database=$(manifest_value source_database)
archive=$(manifest_value archive)
expected_digest=$(manifest_value sha256)
test "$format" = pymes-postgres-backup-v1 || {
  echo "unsupported backup manifest format" >&2
  exit 2
}
test "$backup_service" = "$service" || {
  echo "backup belongs to $backup_service, not $service" >&2
  exit 2
}
test "$archive" = "$(basename "$source")" || {
  echo "backup manifest is bound to a different archive" >&2
  exit 2
}
case "$source_database" in
  ""|*[!A-Za-z0-9_.-]*)
    echo "backup manifest contains an unsafe source database" >&2
    exit 2
    ;;
esac
test "${#source_database}" -le 63 || {
  echo "backup manifest contains an unsafe source database" >&2
  exit 2
}
case "$expected_digest" in
  *[!0-9a-f]*|"") echo "backup manifest contains an invalid SHA256" >&2; exit 2 ;;
esac
test "${#expected_digest}" -eq 64 || {
  echo "backup manifest contains an invalid SHA256" >&2
  exit 2
}
actual_digest=$(sha256sum "$source" | cut -d ' ' -f 1)
test "$actual_digest" = "$expected_digest" || {
  echo "backup SHA256 does not match its manifest" >&2
  exit 2
}

# The service identity is bound twice: by the signed-off manifest and by a
# durable schema marker in the archive catalog. This prevents a correctly
# checksummed Fiscal or Accounting archive from being restored as Pymes.
catalog=$(pg_restore --list "$source")
case "$service" in
  pymes)
    printf '%s\n' "$catalog" |
      grep -Eq '[[:space:]]TABLE[[:space:]]+app[[:space:]]+organizations([[:space:]]|$)' || {
        echo "archive does not contain the Pymes schema marker" >&2
        exit 2
      }
    ;;
  fiscal)
    printf '%s\n' "$catalog" |
      grep -Eq '[[:space:]]TABLE[[:space:]]+fiscal[[:space:]]+requests([[:space:]]|$)' || {
        echo "archive does not contain the Fiscal schema marker" >&2
        exit 2
      }
    ;;
  accounting)
    printf '%s\n' "$catalog" |
      grep -Eq '[[:space:]]TABLE[[:space:]]+public[[:space:]]+pymes_accounting_organizations([[:space:]]|$)' || {
        echo "archive does not contain the Accounting schema marker" >&2
        exit 2
      }
    ;;
esac

target_state=$(python3 "$libpq_exec" "$connection_variable" \
  psql -X -v ON_ERROR_STOP=1 -AtF '|' -c \
  "SELECT
     current_database(),
     (
       SELECT count(*)
       FROM pg_catalog.pg_class AS relation
       JOIN pg_catalog.pg_namespace AS namespace
         ON namespace.oid = relation.relnamespace
       WHERE namespace.nspname NOT IN ('pg_catalog', 'information_schema')
         AND namespace.nspname !~ '^pg_toast'
         AND relation.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')
     ),
     (
       SELECT count(*)
       FROM pg_catalog.pg_stat_activity
       WHERE datname = current_database()
         AND pid <> pg_backend_pid()
     )")
target_database=${target_state%%|*}
target_counts=${target_state#*|}
target_relations=${target_counts%%|*}
target_connections=${target_counts#*|}
case "$target_database" in
  ""|*[!A-Za-z0-9_.-]*)
    echo "restore target has an unsafe database identifier" >&2
    exit 2
    ;;
esac
test "${#target_database}" -le 63 || {
  echo "restore target has an unsafe database identifier" >&2
  exit 2
}
case "$target_relations:$target_connections" in
  *[!0-9:]*|:*|*:)
    echo "could not prove that the restore target is isolated and empty" >&2
    exit 2
    ;;
esac
if test "$target_relations" -ne 0 || test "$target_connections" -ne 0; then
  echo "restore target must be a new empty database with no other connections" >&2
  exit 2
fi

expected_confirmation="RESTORE:${service}:${target_database}"
test "${RESTORE_CONFIRMATION:-}" = "$expected_confirmation" || {
  echo "refusing restore; set RESTORE_CONFIRMATION=$expected_confirmation" >&2
  exit 2
}

python3 "$libpq_exec" "$connection_variable" \
  pg_restore --no-owner --exit-on-error --single-transaction \
  --dbname "$target_database" "$source"
