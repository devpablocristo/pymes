#!/usr/bin/env sh
set -eu
umask 077

# Usage: SERVICE=pymes PYMES_DATABASE_URL=... ./scripts/backup-postgres.sh /secure/backups/pymes.sqlc
# The target path is explicit to prevent accidental overwrite of an unknown
# backup. Credentials stay in the runtime URL/secret manager, never in files.
script_dir=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
libpq_exec="$script_dir/libpq-exec.py"
test -r "$libpq_exec" || {
  echo "libpq command wrapper is required: $libpq_exec" >&2
  exit 1
}

for required_command in basename chmod cut dirname ln mktemp pg_dump psql python3 rm sha256sum; do
  command -v "$required_command" >/dev/null 2>&1 || {
    echo "$required_command is required" >&2
    exit 1
  }
done

service=${SERVICE:-pymes}
case "$service" in
  pymes)
    : "${PYMES_DATABASE_URL:?PYMES_DATABASE_URL is required}"
    connection_variable=PYMES_DATABASE_URL
    ;;
  fiscal)
    : "${FISCAL_DATABASE_URL:?FISCAL_DATABASE_URL is required}"
    connection_variable=FISCAL_DATABASE_URL
    ;;
  accounting)
    : "${ACCOUNTING_DATABASE_URL:?ACCOUNTING_DATABASE_URL is required}"
    connection_variable=ACCOUNTING_DATABASE_URL
    ;;
  *) echo "SERVICE must be pymes, fiscal or accounting" >&2; exit 2 ;;
esac
target=${1:?explicit backup path is required}
case "$target" in
  /*) ;;
  *) echo "backup path must be absolute" >&2; exit 2 ;;
esac
case "$target" in *.dump|*.sqlc) ;; *) echo "backup must end in .dump or .sqlc" >&2; exit 2;; esac
archive_name=$(basename "$target")
case "$archive_name" in
  ""|*[!A-Za-z0-9._-]*)
    echo "backup filename may contain only letters, digits, dot, underscore and dash" >&2
    exit 2
    ;;
esac
target_directory=$(dirname "$target")
test -d "$target_directory" || {
  echo "backup directory does not exist: $target_directory" >&2
  exit 2
}
manifest="$target.sha256"
if test -e "$target" || test -e "$manifest"; then
  echo "backup target or checksum already exists: $target" >&2
  exit 2
fi

source_database=$(python3 "$libpq_exec" "$connection_variable" \
  psql -X -v ON_ERROR_STOP=1 -Atc \
  'SELECT current_database()')
case "$source_database" in
  ""|*[!A-Za-z0-9_.-]*)
    echo "source database has an unsafe identifier" >&2
    exit 2
    ;;
esac
test "${#source_database}" -le 63 || {
  echo "source database has an unsafe identifier" >&2
  exit 2
}

archive_temporary=$(mktemp \
  "$target_directory/.${archive_name}.archive.XXXXXX")
manifest_temporary=$(mktemp \
  "$target_directory/.${archive_name}.manifest.XXXXXX")
published_archive=false
cleanup() {
  rm -f "$archive_temporary" "$manifest_temporary"
  if test "$published_archive" = true && test ! -e "$manifest"; then
    rm -f "$target"
  fi
}
trap cleanup EXIT INT TERM

python3 "$libpq_exec" "$connection_variable" \
  pg_dump --format=custom --no-owner --file "$archive_temporary"
test -s "$archive_temporary" || {
  echo "pg_dump produced an empty archive" >&2
  exit 1
}
digest=$(sha256sum "$archive_temporary" | cut -d ' ' -f 1)
case "$digest" in
  *[!0-9a-f]*|"") echo "could not calculate backup SHA256" >&2; exit 1 ;;
esac
test "${#digest}" -eq 64 || {
  echo "could not calculate backup SHA256" >&2
  exit 1
}
{
  printf '%s\n' 'format=pymes-postgres-backup-v1'
  printf 'service=%s\n' "$service"
  printf 'source_database=%s\n' "$source_database"
  printf 'archive=%s\n' "$archive_name"
  printf 'sha256=%s\n' "$digest"
} >"$manifest_temporary"
chmod 600 "$archive_temporary" "$manifest_temporary"

# Hard links publish complete temporary files atomically and, unlike a plain
# rename, fail instead of overwriting if another backup chose the same target.
# If the manifest publication fails, the trap removes the new archive.
ln "$archive_temporary" "$target" || {
  echo "backup target appeared during backup: $target" >&2
  exit 2
}
published_archive=true
rm -f "$archive_temporary"
ln "$manifest_temporary" "$manifest" || {
  echo "backup checksum appeared during backup: $manifest" >&2
  exit 2
}
rm -f "$manifest_temporary"
published_archive=false
trap - EXIT INT TERM
