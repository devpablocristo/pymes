#!/usr/bin/env bash
set -euo pipefail
umask 077

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(git -C "$script_dir" rev-parse --show-toplevel)
libpq_exec="$repo_root/v3/scripts/libpq-exec.py"
# shellcheck source=retain-release-manifest.sh
source "$script_dir/retain-release-manifest.sh"

cloud_restore_fail() {
  echo "cloud restore drill: $*" >&2
  return 1
}

cloud_restore_now() {
  date -u +%Y-%m-%dT%H:%M:%SZ
}

cloud_restore_validate_identifier() {
  local value="$1" label="$2"
  [[ "$value" =~ ^[a-z][a-z0-9_]{0,62}$ ]] || {
    cloud_restore_fail "$label is not a safe PostgreSQL identifier"
    return 2
  }
}

cloud_restore_manifest_value() {
  local manifest="$1" field="$2" count
  count=$(grep -c "^${field}=" "$manifest" || true)
  [[ "$count" == 1 ]] || {
    cloud_restore_fail \
      "$(basename "$manifest") must contain exactly one $field"
    return 2
  }
  sed -n "s/^${field}=//p" "$manifest"
}

cloud_restore_validate_backup() {
  local archive="$1" service="$2" expected_database="$3"
  local manifest format selected_service source_database selected_archive
  local expected_digest actual_digest catalog marker
  [[ "$archive" == /* && -f "$archive" && ! -L "$archive" ]] || {
    cloud_restore_fail "$service backup must be an absolute regular non-symlink file"
    return 2
  }
  [[ "$(basename "$archive")" =~ ^[A-Za-z0-9._-]+\.(dump|sqlc)$ ]] || {
    cloud_restore_fail "$service backup filename is unsafe"
    return 2
  }
  manifest="${archive}.sha256"
  [[ -f "$manifest" && ! -L "$manifest" ]] || {
    cloud_restore_fail "$service backup checksum manifest is missing"
    return 2
  }
  [[ "$(wc -l <"$manifest" | tr -d ' ')" == 5 ]] || {
    cloud_restore_fail "$service backup manifest has an unexpected shape"
    return 2
  }
  if grep -Evq '^(format|service|source_database|archive|sha256)=' "$manifest"; then
    cloud_restore_fail "$service backup manifest contains an unknown field"
    return 2
  fi
  format=$(cloud_restore_manifest_value "$manifest" format) || return
  selected_service=$(cloud_restore_manifest_value "$manifest" service) || return
  source_database=$(cloud_restore_manifest_value "$manifest" source_database) ||
    return
  selected_archive=$(cloud_restore_manifest_value "$manifest" archive) || return
  expected_digest=$(cloud_restore_manifest_value "$manifest" sha256) || return
  [[ "$format" == pymes-postgres-backup-v1 ]] || {
    cloud_restore_fail "$service backup format is unsupported"
    return 2
  }
  [[ "$selected_service" == "$service" ]] || {
    cloud_restore_fail "$archive belongs to $selected_service, not $service"
    return 2
  }
  [[ "$source_database" == "$expected_database" ]] || {
    cloud_restore_fail \
      "$service backup source is $source_database, expected $expected_database"
    return 2
  }
  [[ "$selected_archive" == "$(basename "$archive")" ]] || {
    cloud_restore_fail "$service backup manifest is bound to another archive"
    return 2
  }
  [[ "$expected_digest" =~ ^[0-9a-f]{64}$ ]] || {
    cloud_restore_fail "$service backup manifest SHA-256 is invalid"
    return 2
  }
  actual_digest=$(sha256sum -- "$archive" | awk '{print $1}')
  [[ "$actual_digest" == "$expected_digest" ]] || {
    cloud_restore_fail "$service backup SHA-256 differs"
    return 2
  }
  catalog=$(pg_restore --list "$archive") || return
  case "$service" in
    pymes) marker='[[:space:]]TABLE[[:space:]]+app[[:space:]]+organizations([[:space:]]|$)' ;;
    fiscal) marker='[[:space:]]TABLE[[:space:]]+fiscal[[:space:]]+requests([[:space:]]|$)' ;;
    accounting) marker='[[:space:]]TABLE[[:space:]]+public[[:space:]]+pymes_accounting_organizations([[:space:]]|$)' ;;
  esac
  grep -Eq "$marker" <<<"$catalog" || {
    cloud_restore_fail "$service archive lacks its schema marker"
    return 2
  }
  printf '%s\n' "$actual_digest"
}

cloud_restore_validate_release_manifest() {
  local manifest="$1" checksum="$2" environment="$3" source_sha="$4"
  local accounting_sha="$5" actual
  [[ "$manifest" == /* && -f "$manifest" && ! -L "$manifest" ]] || {
    cloud_restore_fail \
      "release manifest must be an absolute regular non-symlink file"
    return 2
  }
  [[ "$checksum" =~ ^[0-9a-f]{64}$ ]] || {
    cloud_restore_fail "release manifest checksum is invalid"
    return 2
  }
  actual=$(sha256sum -- "$manifest" | awk '{print $1}')
  [[ "$actual" == "$checksum" ]] || {
    cloud_restore_fail "release manifest checksum differs"
    return 2
  }
  retain_release_manifest_validate_manifest \
    "$manifest" "$environment" "$source_sha" "$accounting_sha" || return
}

cloud_restore_validate_direct_auth() {
  local active impersonated
  for variable in \
    CLOUDSDK_AUTH_ACCESS_TOKEN \
    CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE \
    CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT; do
    [[ -z "${!variable:-}" ]] || {
      cloud_restore_fail \
        "refusing delegated or overridden gcloud credentials: $variable"
      return 1
    }
  done
  active=$(gcloud config get-value account 2>/dev/null)
  impersonated=$(gcloud config get-value auth/impersonate_service_account \
    2>/dev/null || true)
  [[ "$active" == softponti@gmail.com ]] || {
    cloud_restore_fail "restore drill requires the reviewed operator"
    return 1
  }
  [[ -z "$impersonated" || "$impersonated" == "(unset)" ]] || {
    cloud_restore_fail "restore drill refuses gcloud impersonation"
    return 1
  }
}

cloud_restore_validate_instance() {
  local instance_json="$1" instance="$2"
  jq -e \
    --arg project pymes-dev-352318 \
    --arg region us-central1 \
    --arg instance "$instance" '
      .name == $instance and
      .region == $region and
      .state == "RUNNABLE" and
      (.databaseVersion | startswith("POSTGRES_")) and
      (.selfLink | contains("/projects/" + $project + "/instances/" + $instance))
    ' <<<"$instance_json" >/dev/null || {
      cloud_restore_fail \
        "Cloud SQL instance identity, region, engine, or state differs"
      return 1
    }
}

cloud_restore_admin_preflight() {
  local expected_socket="$1" current
  [[ "${PGHOST:-}" == "$expected_socket" && "${PGPORT:-}" == 5432 &&
      "${PGDATABASE:-postgres}" == postgres && -n "${PGUSER:-}" &&
      -n "${PGPASSWORD:-}" ]] || {
    cloud_restore_fail \
      "admin libpq environment must target postgres through the exact Cloud SQL proxy socket"
    return 2
  }
  current=$(psql --no-password -X -v ON_ERROR_STOP=1 -AtF '|' \
    -c "SELECT current_database(), current_user,
      (SELECT rolsuper OR rolcreatedb FROM pg_roles WHERE rolname=current_user)")
  [[ "$current" =~ ^postgres\|[A-Za-z_][A-Za-z0-9_]{0,62}\|t$ ]] || {
    cloud_restore_fail \
      "admin connection is not postgres or cannot create isolated databases"
    return 1
  }
}

cloud_restore_describe_database() {
  local database="$1"
  export PYMES_RESTORE_DRILL_DATABASE="$database"
  psql --no-password -X -v ON_ERROR_STOP=1 -AtF '|' <<'SQL'
\getenv database PYMES_RESTORE_DRILL_DATABASE
SELECT pg_get_userbyid(datdba) || '|' ||
       coalesce(shobj_description(oid, 'pg_database'), '')
FROM pg_database
WHERE datname = :'database';
SQL
  unset PYMES_RESTORE_DRILL_DATABASE
}

cloud_restore_create_database() {
  local database="$1" marker="$2"
  export PYMES_RESTORE_DRILL_DATABASE="$database"
  export PYMES_RESTORE_DRILL_MARKER="$marker"
  psql --no-password -X -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
\getenv database PYMES_RESTORE_DRILL_DATABASE
\getenv marker PYMES_RESTORE_DRILL_MARKER
SELECT format('CREATE DATABASE %I', :'database')
\gexec
SELECT format('COMMENT ON DATABASE %I IS %L', :'database', :'marker')
\gexec
SQL
  unset PYMES_RESTORE_DRILL_DATABASE PYMES_RESTORE_DRILL_MARKER
}

cloud_restore_drop_database() {
  local database="$1"
  export PYMES_RESTORE_DRILL_DATABASE="$database"
  psql --no-password -X -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
\getenv database PYMES_RESTORE_DRILL_DATABASE
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = :'database' AND pid <> pg_backend_pid();
SELECT format('DROP DATABASE %I WITH (FORCE)', :'database')
\gexec
SQL
  unset PYMES_RESTORE_DRILL_DATABASE
}

cloud_restore_database_query() {
  local variable="$1" sql="$2"
  export "${variable?}"
  python3 "$libpq_exec" "$variable" \
    psql -X -v ON_ERROR_STOP=1 -Atc "$sql"
}

cloud_restore_run_service_restore() {
  local service="$1" archive="$2" target="$3" variable
  case "$service" in
    pymes) variable=PYMES_RESTORE_DATABASE_URL ;;
    fiscal) variable=FISCAL_RESTORE_DATABASE_URL ;;
    accounting) variable=ACCOUNTING_RESTORE_DATABASE_URL ;;
  esac
  export "${variable?}"
  SERVICE="$service" \
  RESTORE_CONFIRMATION="RESTORE:${service}:${target}" \
    "$repo_root/v3/scripts/restore-postgres.sh" "$archive"
}

cloud_restore_state_checksum() {
  sha256sum -- "$1" | awk '{print $1}'
}

cloud_restore_write_state() {
  local state="$1" phase="$2" environment="$3" drill_id="$4"
  local source_sha="$5" accounting_sha="$6" release_checksum="$7"
  local pymes_target="$8" fiscal_target="$9" accounting_target="${10}"
  local pymes_digest="${11}" fiscal_digest="${12}" accounting_digest="${13}"
  local marker="${14}" validator_sha="${15:-}" validation_sha="${16:-}"
  local temporary checksum_temporary
  temporary=$(mktemp "${state}.tmp.XXXXXX")
  checksum_temporary=$(mktemp "${state}.sha256.tmp.XXXXXX")
  jq -n \
    --arg schema pymes-v3-cloud-restore-drill-v1 \
    --arg phase "$phase" \
    --arg environment "$environment" \
    --arg drill_id "$drill_id" \
    --arg source_sha "$source_sha" \
    --arg accounting_sha "$accounting_sha" \
    --arg release_manifest_sha256 "$release_checksum" \
    --arg pymes_target "$pymes_target" \
    --arg fiscal_target "$fiscal_target" \
    --arg accounting_target "$accounting_target" \
    --arg pymes_backup_sha256 "$pymes_digest" \
    --arg fiscal_backup_sha256 "$fiscal_digest" \
    --arg accounting_backup_sha256 "$accounting_digest" \
    --arg ownership_marker "$marker" \
    --arg validator_sha256 "$validator_sha" \
    --arg validation_sha256 "$validation_sha" \
    --arg updated_at "$(cloud_restore_now)" '
      {
        schema: $schema,
        phase: $phase,
        environment: $environment,
        drill_id: $drill_id,
        source_sha: $source_sha,
        open_accounting_sha: $accounting_sha,
        release_manifest_sha256: $release_manifest_sha256,
        targets: {
          pymes: $pymes_target,
          fiscal: $fiscal_target,
          accounting: $accounting_target
        },
        backups: {
          pymes_sha256: $pymes_backup_sha256,
          fiscal_sha256: $fiscal_backup_sha256,
          accounting_sha256: $accounting_backup_sha256
        },
        ownership_marker: $ownership_marker,
        validator_sha256:
          (if $validator_sha256 == "" then null else $validator_sha256 end),
        validation_sha256:
          (if $validation_sha256 == "" then null else $validation_sha256 end),
        updated_at: $updated_at
      }
    ' >"$temporary"
  chmod 600 "$temporary"
  cloud_restore_state_checksum "$temporary" >"$checksum_temporary"
  chmod 600 "$checksum_temporary"
  mv -- "$temporary" "$state"
  mv -- "$checksum_temporary" "${state}.sha256"
}

cloud_restore_validate_state() {
  local state="$1" environment="$2" drill_id="$3" source_sha="$4"
  local expected_state_checksum actual_state_checksum
  [[ -f "$state" && ! -L "$state" &&
      -f "${state}.sha256" && ! -L "${state}.sha256" ]] || {
    cloud_restore_fail "state and state checksum must be regular files"
    return 2
  }
  expected_state_checksum=$(tr -d ' \n' <"${state}.sha256")
  actual_state_checksum=$(cloud_restore_state_checksum "$state")
  [[ "$expected_state_checksum" =~ ^[0-9a-f]{64}$ &&
      "$actual_state_checksum" == "$expected_state_checksum" ]] || {
    cloud_restore_fail "restore drill state checksum differs"
    return 2
  }
  jq -e \
    --arg environment "$environment" \
    --arg drill_id "$drill_id" \
    --arg source_sha "$source_sha" '
      .schema == "pymes-v3-cloud-restore-drill-v1" and
      .environment == $environment and
      .drill_id == $drill_id and
      .source_sha == $source_sha and
      (.phase | IN("prepared", "restored", "verified", "cleaned"))
    ' "$state" >/dev/null || {
      cloud_restore_fail "restore drill state identity differs"
      return 2
    }
}

cloud_restore_validate_target_connection() {
  local variable="$1" expected_database="$2" marker_relation="$3"
  local actual
  actual=$(cloud_restore_database_query "$variable" \
    "SELECT current_database() || '|' || (to_regclass('${marker_relation}') IS NOT NULL)::text") ||
    return
  [[ "$actual" == "${expected_database}|true" ]] || {
    cloud_restore_fail \
      "$variable does not target the expected restored database and marker"
    return 1
  }
}

cloud_restore_validate_validator() {
  local validator="$1" expected_sha="$2"
  local validator_mode
  [[ "$validator" == /* && -f "$validator" && ! -L "$validator" &&
      -x "$validator" ]] || {
    cloud_restore_fail \
      "validator must be an absolute executable regular non-symlink file"
    return 2
  }
  [[ "$expected_sha" =~ ^[0-9a-f]{64}$ &&
      "$(sha256sum -- "$validator" | awk '{print $1}')" == "$expected_sha" ]] || {
    cloud_restore_fail "validator checksum differs"
    return 2
  }
  [[ "$(stat -c '%u' "$validator")" == "$(id -u)" ]] || {
    cloud_restore_fail "validator must be owned by the current operator"
    return 2
  }
  validator_mode=$(stat -c '%a' "$validator")
  (( (8#$validator_mode & 8#022) == 0 )) || {
    cloud_restore_fail "validator must not be group/world writable"
    return 2
  }
}

cloud_restore_run_validator() {
  local validator="$1" output="$2" environment="$3" drill_id="$4"
  local pymes_target="$5" fiscal_target="$6" accounting_target="$7"
  PYMES_RESTORE_DRILL_ENV="$environment" \
  PYMES_RESTORE_DRILL_ID="$drill_id" \
  PYMES_RESTORE_DRILL_PYMES_TARGET="$pymes_target" \
  PYMES_RESTORE_DRILL_FISCAL_TARGET="$fiscal_target" \
  PYMES_RESTORE_DRILL_ACCOUNTING_TARGET="$accounting_target" \
  PYMES_RESTORE_DRILL_PYMES_DATABASE_URL="$PYMES_RESTORE_DATABASE_URL" \
  PYMES_RESTORE_DRILL_FISCAL_DATABASE_URL="$FISCAL_RESTORE_DATABASE_URL" \
  PYMES_RESTORE_DRILL_ACCOUNTING_DATABASE_URL="$ACCOUNTING_RESTORE_DATABASE_URL" \
    "$validator" >"$output"
}

cloud_restore_validate_witness() {
  local witness="$1" environment="$2" drill_id="$3"
  local pymes_target="$4" fiscal_target="$5" accounting_target="$6"
  jq -e \
    --arg environment "$environment" \
    --arg drill_id "$drill_id" \
    --arg pymes "$pymes_target" \
    --arg fiscal "$fiscal_target" \
    --arg accounting "$accounting_target" '
      .schema == "pymes-v3-cloud-restore-validation-v1" and
      .environment == $environment and
      .drill_id == $drill_id and
      .targets == {
        pymes: $pymes,
        fiscal: $fiscal,
        accounting: $accounting
      } and
      .migrations_applied == true and
      .tenant_isolation_verified == true and
      .probes_ready == true and
      .reconciliation_runs == 2 and
      .duplicate_fiscal_requests == 0 and
      .duplicate_accounting_commands == 0 and
      .duplicate_journal_entries == 0 and
      .unpublished_recoverable_outbox == 0
    ' "$witness" >/dev/null || {
      cloud_restore_fail \
        "validator did not prove migrations, isolation, readiness, and replay-safe reconciliation"
      return 1
    }
}

cloud_restore_drill_main() {
  local mode environment drill_id source_sha accounting_sha release_manifest
  local release_checksum state instance instance_connection expected_socket
  local pymes_backup fiscal_backup accounting_backup
  local pymes_source fiscal_source accounting_source
  local pymes_target fiscal_target accounting_target marker
  local pymes_digest fiscal_digest accounting_digest
  local instance_json confirmation phase description
  local validator validator_sha witness witness_sha temporary_witness
  local -a targets_to_drop=()

  mode=${PYMES_RESTORE_DRILL_MODE:-plan}
  environment=${PYMES_RESTORE_DRILL_ENV:-}
  drill_id=${PYMES_RESTORE_DRILL_ID:-}
  source_sha=${PYMES_RESTORE_DRILL_SOURCE_SHA:-}
  accounting_sha=${PYMES_RESTORE_DRILL_ACCOUNTING_SHA:-}
  release_manifest=${PYMES_RESTORE_DRILL_RELEASE_MANIFEST:-}
  release_checksum=${PYMES_RESTORE_DRILL_RELEASE_MANIFEST_SHA256:-}
  state=${PYMES_RESTORE_DRILL_STATE:-}
  instance=${PYMES_CLOUDSQL_INSTANCE:-pymes-dev-db}
  pymes_backup=${PYMES_RESTORE_DRILL_PYMES_BACKUP:-}
  fiscal_backup=${PYMES_RESTORE_DRILL_FISCAL_BACKUP:-}
  accounting_backup=${PYMES_RESTORE_DRILL_ACCOUNTING_BACKUP:-}

  case "$mode" in plan|restore|verify|cleanup) ;; *)
    cloud_restore_fail "mode must be plan, restore, verify, or cleanup"
    return 2
  esac
  pymes_release_evidence_validate_environment "$environment" || return
  [[ "$drill_id" =~ ^[a-z][a-z0-9]{7,15}$ ]] || {
    cloud_restore_fail \
      "PYMES_RESTORE_DRILL_ID must be 8-16 lowercase alphanumerics starting with a letter"
    return 2
  }
  [[ "$source_sha" =~ ^[0-9a-f]{40}$ &&
      "$accounting_sha" =~ ^[0-9a-f]{40}$ ]] || {
    cloud_restore_fail "source SHAs must be full lowercase Git SHAs"
    return 2
  }
  [[ "$state" == /* ]] || {
    cloud_restore_fail "state path must be absolute"
    return 2
  }
  [[ "$instance" == pymes-dev-db ]] || {
    cloud_restore_fail "restore drill is restricted to pymes-dev-db"
    return 2
  }
  for required in \
    basename date gcloud grep id jq mktemp mv pg_restore psql python3 sed \
    sha256sum stat tr wc; do
    command -v "$required" >/dev/null 2>&1 || {
      cloud_restore_fail "$required is required"
      return 1
    }
  done
  [[ -r "$libpq_exec" ]] || {
    cloud_restore_fail "libpq command wrapper is missing"
    return 1
  }

  case "$environment" in
    stg)
      pymes_source=pymes_v3_stg
      fiscal_source=pymes_v3_fiscal_stg
      accounting_source=pymes_v3_accounting_stg
      ;;
    prd)
      pymes_source=pymes_v3_prd
      fiscal_source=pymes_v3_fiscal_prd
      accounting_source=pymes_v3_accounting_prd
      ;;
  esac
  pymes_target="pymes_v3_restore_${environment}_${drill_id}"
  fiscal_target="pymes_v3_fiscal_restore_${environment}_${drill_id}"
  accounting_target="pymes_v3_accounting_restore_${environment}_${drill_id}"
  marker="pymes-v3-restore-drill:${environment}:${drill_id}:${source_sha}"
  for target in "$pymes_target" "$fiscal_target" "$accounting_target"; do
    cloud_restore_validate_identifier "$target" target || return
    [[ "$target" != "$pymes_source" && "$target" != "$fiscal_source" &&
        "$target" != "$accounting_source" ]] || {
      cloud_restore_fail "restore target collides with an active database"
      return 2
    }
  done

  cloud_restore_validate_release_manifest \
    "$release_manifest" "$release_checksum" "$environment" "$source_sha" \
    "$accounting_sha" || return
  pymes_digest=$(cloud_restore_validate_backup \
    "$pymes_backup" pymes "$pymes_source") || return
  fiscal_digest=$(cloud_restore_validate_backup \
    "$fiscal_backup" fiscal "$fiscal_source") || return
  accounting_digest=$(cloud_restore_validate_backup \
    "$accounting_backup" accounting "$accounting_source") || return

  printf 'CLOUD RESTORE PLAN environment=%s drill_id=%s release=%s instance=%s\n' \
    "$environment" "$drill_id" "$source_sha" "$instance"
  printf 'CLOUD RESTORE PLAN targets=%s,%s,%s active_databases_untouched=true\n' \
    "$pymes_target" "$fiscal_target" "$accounting_target"
  if [[ "$mode" == plan ]]; then
    [[ ! -e "$state" && ! -e "${state}.sha256" ]] || {
      cloud_restore_fail "plan refuses an existing state path"
      return 2
    }
    echo "PLAN ONLY databases_created=0 databases_restored=0 databases_deleted=0"
    return 0
  fi

  cloud_restore_validate_direct_auth || return
  instance_json=$(gcloud sql instances describe "$instance" \
    --project=pymes-dev-352318 --format=json) || return
  cloud_restore_validate_instance "$instance_json" "$instance" || return
  instance_connection="pymes-dev-352318:us-central1:${instance}"
  expected_socket="/cloudsql/${instance_connection}"
  cloud_restore_admin_preflight "$expected_socket" || return

  if [[ "$mode" == restore ]]; then
    [[ ! -e "$state" && ! -e "${state}.sha256" ]] || {
      cloud_restore_fail "restore refuses to overwrite an existing state"
      return 2
    }
    for target in "$pymes_target" "$fiscal_target" "$accounting_target"; do
      description=$(cloud_restore_describe_database "$target") || return
      [[ -z "$description" ]] || {
        cloud_restore_fail \
          "target $target already exists; no database was created"
        return 2
      }
    done
    confirmation="RESTORE_CLOUD_${environment^^}_${drill_id}_${source_sha}"
    [[ "${PYMES_RESTORE_DRILL_CONFIRMATION:-}" == "$confirmation" ]] || {
      cloud_restore_fail \
        "set PYMES_RESTORE_DRILL_CONFIRMATION=$confirmation"
      return 2
    }
    cloud_restore_write_state \
      "$state" prepared "$environment" "$drill_id" "$source_sha" \
      "$accounting_sha" "$release_checksum" "$pymes_target" "$fiscal_target" \
      "$accounting_target" "$pymes_digest" "$fiscal_digest" \
      "$accounting_digest" "$marker"
    for target in "$pymes_target" "$fiscal_target" "$accounting_target"; do
      cloud_restore_create_database "$target" "$marker" || return
    done
    cloud_restore_run_service_restore \
      pymes "$pymes_backup" "$pymes_target" || return
    cloud_restore_run_service_restore \
      fiscal "$fiscal_backup" "$fiscal_target" || return
    cloud_restore_run_service_restore \
      accounting "$accounting_backup" "$accounting_target" || return
    cloud_restore_write_state \
      "$state" restored "$environment" "$drill_id" "$source_sha" \
      "$accounting_sha" "$release_checksum" "$pymes_target" "$fiscal_target" \
      "$accounting_target" "$pymes_digest" "$fiscal_digest" \
      "$accounting_digest" "$marker"
    echo "CLOUD RESTORE RESTORED state=$state traffic_changed=false secrets_changed=false"
    return 0
  fi

  cloud_restore_validate_state "$state" "$environment" "$drill_id" \
    "$source_sha" || return
  phase=$(jq -r .phase "$state")
  for target in "$pymes_target" "$fiscal_target" "$accounting_target"; do
    description=$(cloud_restore_describe_database "$target") || return
    if [[ -z "$description" && "$mode" == cleanup && "$phase" == prepared ]]; then
      continue
    fi
    [[ -n "$description" && "${description#*|}" == "$marker" ]] || {
      cloud_restore_fail \
        "target $target is absent or lacks the exact ownership marker"
      return 1
    }
    targets_to_drop+=("$target")
  done

  if [[ "$mode" == verify ]]; then
    [[ "$phase" == restored || "$phase" == verified ]] || {
      cloud_restore_fail "only a restored drill can be verified"
      return 2
    }
    for variable in \
      PYMES_RESTORE_DATABASE_URL \
      FISCAL_RESTORE_DATABASE_URL \
      ACCOUNTING_RESTORE_DATABASE_URL; do
      [[ -n "${!variable:-}" ]] || {
        cloud_restore_fail "set $variable"
        return 2
      }
    done
    cloud_restore_validate_target_connection \
      PYMES_RESTORE_DATABASE_URL "$pymes_target" app.organizations || return
    cloud_restore_validate_target_connection \
      FISCAL_RESTORE_DATABASE_URL "$fiscal_target" fiscal.requests || return
    cloud_restore_validate_target_connection \
      ACCOUNTING_RESTORE_DATABASE_URL "$accounting_target" \
      public.pymes_accounting_organizations || return

    validator=${PYMES_RESTORE_DRILL_VALIDATOR:-}
    validator_sha=${PYMES_RESTORE_DRILL_VALIDATOR_SHA256:-}
    cloud_restore_validate_validator "$validator" "$validator_sha" || return
    witness="${state}.validation.json"
    [[ ! -e "$witness" ]] || {
      cloud_restore_fail "refusing to overwrite validation evidence"
      return 2
    }
    temporary_witness=$(mktemp "${witness}.tmp.XXXXXX")
    if ! cloud_restore_run_validator \
      "$validator" "$temporary_witness" "$environment" "$drill_id" \
      "$pymes_target" "$fiscal_target" "$accounting_target"; then
      rm -f -- "$temporary_witness"
      cloud_restore_fail "restore validator failed"
      return 1
    fi
    cloud_restore_validate_witness \
      "$temporary_witness" "$environment" "$drill_id" "$pymes_target" \
      "$fiscal_target" "$accounting_target" || {
        rm -f -- "$temporary_witness"
        return 1
      }
    chmod 600 "$temporary_witness"
    mv -- "$temporary_witness" "$witness"
    witness_sha=$(cloud_restore_state_checksum "$witness")
    printf '%s\n' "$witness_sha" >"${witness}.sha256"
    chmod 600 "${witness}.sha256"
    cloud_restore_write_state \
      "$state" verified "$environment" "$drill_id" "$source_sha" \
      "$accounting_sha" "$release_checksum" "$pymes_target" "$fiscal_target" \
      "$accounting_target" "$pymes_digest" "$fiscal_digest" \
      "$accounting_digest" "$marker" "$validator_sha" "$witness_sha"
    echo "CLOUD RESTORE VERIFIED state=$state witness=$witness traffic_changed=false"
    return 0
  fi

  [[ "$phase" == prepared || "$phase" == restored || "$phase" == verified ]] || {
    cloud_restore_fail "only an owned active drill can be cleaned"
    return 2
  }
  confirmation="DELETE_RESTORE_DRILL_${environment^^}_${drill_id}_${source_sha}"
  [[ "${PYMES_RESTORE_DRILL_CLEANUP_CONFIRMATION:-}" == "$confirmation" ]] || {
    cloud_restore_fail \
      "set PYMES_RESTORE_DRILL_CLEANUP_CONFIRMATION=$confirmation"
    return 2
  }
  for target in "${targets_to_drop[@]}"; do
    cloud_restore_drop_database "$target" || return
  done
  cloud_restore_write_state \
    "$state" cleaned "$environment" "$drill_id" "$source_sha" \
    "$accounting_sha" "$release_checksum" "$pymes_target" "$fiscal_target" \
    "$accounting_target" "$pymes_digest" "$fiscal_digest" \
    "$accounting_digest" "$marker" \
    "$(jq -r '.validator_sha256 // ""' "$state")" \
    "$(jq -r '.validation_sha256 // ""' "$state")"
  echo "CLOUD RESTORE CLEANED state=$state active_databases_untouched=true"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  cloud_restore_drill_main "$@"
fi
