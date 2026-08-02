#!/usr/bin/env sh
set -eu
umask 077

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
temporary_dir=$(mktemp -d)
cleanup() {
  rm -f "$temporary_dir/capture" "$temporary_dir/result"
  rmdir "$temporary_dir"
}
trap cleanup EXIT INT TERM

cat >"$temporary_dir/capture" <<'SCRIPT'
#!/usr/bin/env sh
set -eu
output=$1
shift
case "$*" in
  *postgres://*|*postgresql://*|*super-secret*)
    echo "connection URI or password reached command argv" >&2
    exit 1
    ;;
esac
{
  printf 'argv=%s\n' "$*"
  printf 'host=%s\n' "${PGHOST:-}"
  printf 'port=%s\n' "${PGPORT:-}"
  printf 'user=%s\n' "${PGUSER:-}"
  printf 'password=%s\n' "${PGPASSWORD:-}"
  printf 'database=%s\n' "${PGDATABASE:-}"
  printf 'sslmode=%s\n' "${PGSSLMODE:-}"
  printf 'options=%s\n' "${PGOPTIONS:-}"
} >"$output"
SCRIPT
chmod 700 "$temporary_dir/capture"

PYMES_TEST_DATABASE_URL='postgres://runtime:super-secret@127.0.0.1:55434/pymes_restore?sslmode=disable&options=-c%20role%3Downer' \
  python3 "$root_dir/scripts/libpq-exec.py" PYMES_TEST_DATABASE_URL \
  "$temporary_dir/capture" "$temporary_dir/result" --safe-argument

grep -qx 'argv=--safe-argument' "$temporary_dir/result"
grep -qx 'host=127.0.0.1' "$temporary_dir/result"
grep -qx 'port=55434' "$temporary_dir/result"
grep -qx 'user=runtime' "$temporary_dir/result"
grep -qx 'password=super-secret' "$temporary_dir/result"
grep -qx 'database=pymes_restore' "$temporary_dir/result"
grep -qx 'sslmode=disable' "$temporary_dir/result"
grep -qx 'options=-c role=owner' "$temporary_dir/result"

if PYMES_TEST_DATABASE_URL='postgres://runtime:super-secret@127.0.0.1:55434/pymes_restore?unknown=value' \
  python3 "$root_dir/scripts/libpq-exec.py" PYMES_TEST_DATABASE_URL \
  "$temporary_dir/capture" "$temporary_dir/result" \
  >/dev/null 2>&1; then
  echo "libpq wrapper accepted an unsupported connection parameter" >&2
  exit 1
fi

echo "libpq argv isolation verified"
