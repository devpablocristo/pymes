#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=cloud-restore-drill.sh
source "$script_dir/cloud-restore-drill.sh"

scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT

sha=0123456789abcdef0123456789abcdef01234567
accounting_sha=89abcdef0123456789abcdef0123456789abcdef
digest=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
drill_id=restorea1
state="$scratch/restore-state.json"
mutations="$scratch/mutations.log"
declare -A fake_databases=()

fail_test() {
  echo "cloud restore drill test failed: $*" >&2
  exit 1
}

expect_failure() {
  local description="$1"
  shift
  if "$@" >"$scratch/expected-failure.log" 2>&1; then
    fail_test "$description"
  fi
}

write_backup() {
  local service="$1" database="$2" archive="$3"
  printf 'fake-%s-archive\n' "$service" >"$archive"
  {
    printf 'format=pymes-postgres-backup-v1\n'
    printf 'service=%s\n' "$service"
    printf 'source_database=%s\n' "$database"
    printf 'archive=%s\n' "$(basename "$archive")"
    printf 'sha256=%s\n' "$(sha256sum "$archive" | awk '{print $1}')"
  } >"${archive}.sha256"
}

write_release_manifest() {
  local manifest="$1"
  {
    printf 'PYMES_RELEASE_ENV=stg\n'
    printf 'PYMES_SOURCE_SHA=%s\n' "$sha"
    printf 'PYMES_OPEN_ACCOUNTING_SOURCE_SHA=%s\n' "$accounting_sha"
    printf 'PYMES_API_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-api@sha256:%s\n' "$digest"
    printf 'PYMES_WEB_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/pymes-v3-web-stg@sha256:%s\n' "$digest"
    for image in \
      worker:pymes-v3-worker \
      fiscal:pymes-v3-fiscal \
      accounting:pymes-v3-accounting \
      accounting_admin:pymes-v3-accounting-admin \
      provision:pymes-v3-provision \
      migrate:pymes-v3-migrate \
      fiscal_migrate:pymes-v3-fiscal-migrate \
      accounting_migrate:pymes-v3-accounting-migrate; do
      key=${image%%:*}
      name=${image#*:}
      printf 'PYMES_%s_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/%s@sha256:%s\n' \
        "${key^^}" "$name" "$digest"
    done
  } >"$manifest"
}

pg_restore() {
  [[ "$1" == --list ]]
  case "$(basename "$2")" in
    pymes.dump) echo '1; 0 0 TABLE app organizations owner' ;;
    fiscal.dump) echo '1; 0 0 TABLE fiscal requests owner' ;;
    accounting.dump)
      echo '1; 0 0 TABLE public pymes_accounting_organizations owner'
      ;;
    *) return 1 ;;
  esac
}

gcloud() {
  case "$1 ${2:-} ${3:-}" in
    "config get-value account") echo softponti@gmail.com ;;
    "config get-value auth/impersonate_service_account") echo '(unset)' ;;
    "sql instances describe")
      jq -n '{
        name: "pymes-dev-db",
        region: "us-central1",
        state: "RUNNABLE",
        databaseVersion: "POSTGRES_15",
        selfLink:
          "https://sqladmin.googleapis.com/sql/v1beta4/projects/pymes-dev-352318/instances/pymes-dev-db"
      }'
      ;;
    *) return 1 ;;
  esac
}

psql() {
  echo 'postgres|restore_admin|t'
}

cloud_restore_now() {
  echo 2026-08-01T12:00:00Z
}

cloud_restore_describe_database() {
  local database="$1"
  if [[ -n "${fake_databases[$database]+present}" ]]; then
    printf 'restore_admin|%s\n' "${fake_databases[$database]}"
  fi
}

cloud_restore_create_database() {
  local database="$1" marker="$2"
  [[ -z "${fake_databases[$database]+present}" ]] || return 1
  fake_databases["$database"]="$marker"
  printf 'CREATE %s\n' "$database" >>"$mutations"
}

cloud_restore_drop_database() {
  local database="$1"
  [[ -n "${fake_databases[$database]+present}" ]] || return 1
  unset 'fake_databases[$database]'
  printf 'DROP %s\n' "$database" >>"$mutations"
}

cloud_restore_database_query() {
  local variable="$1"
  case "$variable" in
    PYMES_RESTORE_DATABASE_URL)
      echo "pymes_v3_restore_stg_${drill_id}|true"
      ;;
    FISCAL_RESTORE_DATABASE_URL)
      echo "pymes_v3_fiscal_restore_stg_${drill_id}|true"
      ;;
    ACCOUNTING_RESTORE_DATABASE_URL)
      echo "pymes_v3_accounting_restore_stg_${drill_id}|true"
      ;;
    *) return 1 ;;
  esac
}

cloud_restore_run_service_restore() {
  local service="$1" archive="$2" target="$3"
  [[ -f "$archive" && -n "${fake_databases[$target]+present}" ]] || return 1
  printf 'RESTORE %s %s\n' "$service" "$target" >>"$mutations"
}

pymes_backup="$scratch/pymes.dump"
fiscal_backup="$scratch/fiscal.dump"
accounting_backup="$scratch/accounting.dump"
release_manifest="$scratch/release.env"
write_backup pymes pymes_v3_stg "$pymes_backup"
write_backup fiscal pymes_v3_fiscal_stg "$fiscal_backup"
write_backup accounting pymes_v3_accounting_stg "$accounting_backup"
write_release_manifest "$release_manifest"
release_checksum=$(sha256sum "$release_manifest" | awk '{print $1}')

validator="$scratch/validator.sh"
cp -- "$script_dir/testdata/cloud-restore-drill/validator.sh" "$validator"
chmod 700 "$validator"
validator_sha=$(sha256sum "$validator" | awk '{print $1}')

run_drill() {
  local selected_mode="$1"
  local restore_confirmation=${2:-}
  local cleanup_confirmation=${3:-}
  PYMES_RESTORE_DRILL_MODE="$selected_mode" \
  PYMES_RESTORE_DRILL_ENV=stg \
  PYMES_RESTORE_DRILL_ID="$drill_id" \
  PYMES_RESTORE_DRILL_SOURCE_SHA="$sha" \
  PYMES_RESTORE_DRILL_ACCOUNTING_SHA="$accounting_sha" \
  PYMES_RESTORE_DRILL_RELEASE_MANIFEST="$release_manifest" \
  PYMES_RESTORE_DRILL_RELEASE_MANIFEST_SHA256="$release_checksum" \
  PYMES_RESTORE_DRILL_STATE="$state" \
  PYMES_RESTORE_DRILL_PYMES_BACKUP="$pymes_backup" \
  PYMES_RESTORE_DRILL_FISCAL_BACKUP="$fiscal_backup" \
  PYMES_RESTORE_DRILL_ACCOUNTING_BACKUP="$accounting_backup" \
  PYMES_RESTORE_DRILL_CONFIRMATION="$restore_confirmation" \
  PYMES_RESTORE_DRILL_CLEANUP_CONFIRMATION="$cleanup_confirmation" \
  PYMES_RESTORE_DRILL_VALIDATOR="${PYMES_RESTORE_DRILL_VALIDATOR:-}" \
  PYMES_RESTORE_DRILL_VALIDATOR_SHA256="${PYMES_RESTORE_DRILL_VALIDATOR_SHA256:-}" \
  PYMES_RESTORE_DATABASE_URL="${PYMES_RESTORE_DATABASE_URL:-}" \
  FISCAL_RESTORE_DATABASE_URL="${FISCAL_RESTORE_DATABASE_URL:-}" \
  ACCOUNTING_RESTORE_DATABASE_URL="${ACCOUNTING_RESTORE_DATABASE_URL:-}" \
  PYMES_CLOUDSQL_INSTANCE=pymes-dev-db \
  PGHOST=/cloudsql/pymes-dev-352318:us-central1:pymes-dev-db \
  PGPORT=5432 \
  PGDATABASE=postgres \
  PGUSER=restore_admin \
  PGPASSWORD=not-a-real-secret \
    cloud_restore_drill_main
}

: >"$mutations"
run_drill plan >/dev/null
[[ ! -s "$mutations" && ! -e "$state" ]] ||
  fail_test "plan mode changed a database or state"

expect_failure "restore accepted no typed confirmation" run_drill restore
[[ ! -s "$mutations" && ! -e "$state" ]] ||
  fail_test "unconfirmed restore changed state"

pymes_target="pymes_v3_restore_stg_${drill_id}"
fake_databases["$pymes_target"]=unexpected
confirmation="RESTORE_CLOUD_STG_${drill_id}_${sha}"
expect_failure "restore adopted a preexisting target" \
  run_drill restore "$confirmation"
[[ ! -s "$mutations" && ! -e "$state" ]] ||
  fail_test "preexisting target check was not pre-mutation"
unset 'fake_databases[$pymes_target]'

run_drill restore "$confirmation" >/dev/null
[[ "$(jq -r .phase "$state")" == restored ]] ||
  fail_test "successful restore did not record restored phase"
[[ "$(grep -c '^CREATE ' "$mutations")" == 3 &&
    "$(grep -c '^RESTORE ' "$mutations")" == 3 ]] ||
  fail_test "restore did not create and restore exactly three targets"

expect_failure "verification accepted no reviewed validator" run_drill verify
[[ "$(jq -r .phase "$state")" == restored ]] ||
  fail_test "failed validation changed state"

fiscal_target="pymes_v3_fiscal_restore_stg_${drill_id}"
saved_fiscal_marker=${fake_databases[$fiscal_target]}
fake_databases["$fiscal_target"]=foreign-marker
PYMES_RESTORE_DRILL_VALIDATOR="$validator" \
PYMES_RESTORE_DRILL_VALIDATOR_SHA256="$validator_sha" \
PYMES_RESTORE_DATABASE_URL=postgres://pymes-restore \
FISCAL_RESTORE_DATABASE_URL=postgres://fiscal-restore \
ACCOUNTING_RESTORE_DATABASE_URL=postgres://accounting-restore \
  expect_failure "verification accepted a target without ownership" \
    run_drill verify
fake_databases["$fiscal_target"]="$saved_fiscal_marker"

PYMES_RESTORE_DRILL_VALIDATOR="$validator" \
PYMES_RESTORE_DRILL_VALIDATOR_SHA256="$validator_sha" \
PYMES_RESTORE_DATABASE_URL=postgres://pymes-restore \
FISCAL_RESTORE_DATABASE_URL=postgres://fiscal-restore \
ACCOUNTING_RESTORE_DATABASE_URL=postgres://accounting-restore \
  run_drill verify >/dev/null
[[ "$(jq -r .phase "$state")" == verified ]] ||
  fail_test "validated restore did not record verified phase"
[[ -f "${state}.validation.json" &&
    "$(jq -r .reconciliation_runs "${state}.validation.json")" == 2 ]] ||
  fail_test "validation evidence is missing"

expect_failure "cleanup accepted no destructive confirmation" run_drill cleanup
[[ "$(grep -c '^DROP ' "$mutations" || true)" == 0 ]] ||
  fail_test "unconfirmed cleanup dropped a database"

cleanup_confirmation="DELETE_RESTORE_DRILL_STG_${drill_id}_${sha}"
run_drill cleanup '' "$cleanup_confirmation" >/dev/null
[[ "$(jq -r .phase "$state")" == cleaned ]] ||
  fail_test "cleanup did not preserve a cleaned evidence state"
[[ "$(grep -c '^DROP ' "$mutations")" == 3 ]] ||
  fail_test "cleanup did not drop exactly three owned targets"
for target in \
  "pymes_v3_restore_stg_${drill_id}" \
  "pymes_v3_fiscal_restore_stg_${drill_id}" \
  "pymes_v3_accounting_restore_stg_${drill_id}"; do
  [[ -z "${fake_databases[$target]+present}" ]] ||
    fail_test "cleanup left target $target"
done

echo "cloud restore drill tests passed: local evidence preflight, three isolated targets, ownership guards, reviewed validation and explicit cleanup"
