#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
scratch_dir=$(mktemp -d)
trap 'rm -rf -- "$scratch_dir"' EXIT
mkdir -p "$scratch_dir/bin" "$scratch_dir/migrations"
touch "$scratch_dir/migrations/001.sql"
call_log="$scratch_dir/psql.log"

cat >"$scratch_dir/bin/psql" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$FISCAL_TEST_CALL_LOG"
if [[ " $* " == *" -c SELECT current_database(), session_user, current_user "* ]]; then
  printf '%s|%s|%s\n' \
    "${FISCAL_TEST_DATABASE}" \
    "${FISCAL_TEST_SESSION_ROLE}" \
    "${FISCAL_TEST_EFFECTIVE_ROLE}"
  exit 0
fi
FAKE
chmod +x "$scratch_dir/bin/psql"

password=$(printf 'a%.0s' {1..64})

canonical_database_url() {
  local environment="$1"
  printf 'postgres://pymes_v3_fiscal_migrate_%s:%s@/pymes_v3_fiscal_%s?host=/cloudsql/pymes-dev-352318:us-central1:pymes-dev-db&sslmode=disable&options=-c%%20role%%3Dpymes_v3_fiscal_owner_%s' \
    "$environment" "$password" "$environment" "$environment"
}

assert_success() {
  local environment="$1"
  local database="pymes_v3_fiscal_${environment}"
  local session_role="pymes_v3_fiscal_migrate_${environment}"
  local effective_role="pymes_v3_fiscal_owner_${environment}"
  local database_url
  database_url=$(canonical_database_url "$environment")
  : >"$call_log"
  PATH="$scratch_dir/bin:$PATH" \
  FISCAL_DATABASE_URL="$database_url" \
  FISCAL_DEPLOY_ENV="$environment" \
  FISCAL_MIGRATIONS_DIR="$scratch_dir/migrations" \
  FISCAL_TEST_DATABASE="$database" \
  FISCAL_TEST_SESSION_ROLE="$session_role" \
  FISCAL_TEST_EFFECTIVE_ROLE="$effective_role" \
  FISCAL_TEST_CALL_LOG="$call_log" \
    "$script_dir/migrate.sh"
  grep -Fq -- "-c SELECT current_database(), session_user, current_user" "$call_log"
  grep -Fq -- "-f $scratch_dir/migrations/001.sql" "$call_log"
}

assert_identity_rejected() {
  local case_name="$1" database="$2" session_role="$3" effective_role="$4"
  : >"$call_log"
  if PATH="$scratch_dir/bin:$PATH" \
    FISCAL_DATABASE_URL="$database_url" \
    FISCAL_DEPLOY_ENV="$environment" \
    FISCAL_MIGRATIONS_DIR="$scratch_dir/migrations" \
    FISCAL_TEST_DATABASE="$database" \
    FISCAL_TEST_SESSION_ROLE="$session_role" \
    FISCAL_TEST_EFFECTIVE_ROLE="$effective_role" \
    FISCAL_TEST_CALL_LOG="$call_log" \
    "$script_dir/migrate.sh" >/dev/null 2>&1; then
    echo "$case_name fiscal database identity was accepted" >&2
    exit 1
  fi
  grep -Fq -- "-c SELECT current_database(), session_user, current_user" "$call_log" || {
    echo "fiscal identity preflight did not run for $case_name mismatch" >&2
    exit 1
  }
  if grep -Fq -- " -f " "$call_log"; then
    echo "fiscal migration DDL ran after $case_name identity mismatch" >&2
    exit 1
  fi
}

assert_url_rejected() {
  local case_name="$1" unsafe_url="$2"
  : >"$call_log"
  if PATH="$scratch_dir/bin:$PATH" \
    FISCAL_DATABASE_URL="$unsafe_url" \
    FISCAL_DEPLOY_ENV="$environment" \
    FISCAL_MIGRATIONS_DIR="$scratch_dir/migrations" \
    FISCAL_TEST_CALL_LOG="$call_log" \
    "$script_dir/migrate.sh" >/dev/null 2>&1; then
    echo "$case_name fiscal database URL was accepted" >&2
    exit 1
  fi
  [[ ! -s "$call_log" ]] || {
    echo "$case_name fiscal database URL reached psql" >&2
    exit 1
  }
}

assert_success stg
assert_success prd

environment=stg
database=pymes_v3_fiscal_stg
session_role=pymes_v3_fiscal_migrate_stg
effective_role=pymes_v3_fiscal_owner_stg
database_url=$(canonical_database_url "$environment")

assert_url_rejected foreign-target \
  "${database_url/pymes-dev-352318/other-project}"
short_password=$(printf 'a%.0s' {1..63})
assert_url_rejected short-credential \
  "${database_url/$password/$short_password}"
nonhex_password="$(printf 'a%.0s' {1..63})g"
assert_url_rejected nonhex-credential \
  "${database_url/$password/$nonhex_password}"

: >"$call_log"
if PATH="$scratch_dir/bin:$PATH" \
  FISCAL_DATABASE_URL="$database_url" \
  FISCAL_DEPLOY_ENV=prd \
  FISCAL_MIGRATIONS_DIR="$scratch_dir/migrations" \
  FISCAL_TEST_CALL_LOG="$call_log" \
  "$script_dir/migrate.sh" >/dev/null 2>&1; then
  echo "fiscal database URL from another environment was accepted" >&2
  exit 1
fi
[[ ! -s "$call_log" ]] || {
  echo "cross-environment fiscal database URL reached psql" >&2
  exit 1
}

: >"$call_log"
if env -u FISCAL_DEPLOY_ENV \
  "PATH=$scratch_dir/bin:$PATH" \
  "FISCAL_DATABASE_URL=$database_url" \
  "FISCAL_MIGRATIONS_DIR=$scratch_dir/migrations" \
  "FISCAL_TEST_CALL_LOG=$call_log" \
  "$script_dir/migrate.sh" >/dev/null 2>&1; then
  echo "missing fiscal deployment environment was accepted" >&2
  exit 1
fi
[[ ! -s "$call_log" ]] || {
  echo "missing fiscal deployment environment reached psql" >&2
  exit 1
}

: >"$call_log"
if PATH="$scratch_dir/bin:$PATH" \
  FISCAL_DATABASE_URL="$database_url" \
  FISCAL_DEPLOY_ENV=production \
  FISCAL_MIGRATIONS_DIR="$scratch_dir/migrations" \
  FISCAL_TEST_CALL_LOG="$call_log" \
  "$script_dir/migrate.sh" >/dev/null 2>&1; then
  echo "unknown fiscal deployment environment was accepted" >&2
  exit 1
fi
[[ ! -s "$call_log" ]] || {
  echo "unknown fiscal deployment environment reached psql" >&2
  exit 1
}

assert_identity_rejected database other "$session_role" "$effective_role"
assert_identity_rejected session-role "$database" other "$effective_role"
assert_identity_rejected effective-role "$database" "$session_role" other

echo "Fiscal migration target policy verified"
