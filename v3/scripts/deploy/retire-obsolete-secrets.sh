#!/usr/bin/env bash
set -euo pipefail

# Retires only the two obsolete Secret Manager inputs left behind by the
# pre-KMS/pre-tenant Pymes v3 design. The operation is deliberately
# recoverable: IAM members are removed one at a time and enabled versions are
# disabled, while versions and secret containers are never destroyed.

readonly expected_project=pymes-dev-352318
readonly expected_project_number=884236221349
readonly expected_region=us-central1
readonly expected_operator=softponti@gmail.com
readonly expected_human_secret_admin="user:${expected_operator}"

project=${PYMES_GCP_PROJECT:-$expected_project}
region=${PYMES_GCP_REGION:-$expected_region}
mode=${PYMES_OBSOLETE_SECRETS_MODE:-plan}
target_environment=${PYMES_OBSOLETE_SECRETS_ENV:-all}
operator_email=${PYMES_OBSOLETE_SECRETS_OPERATOR_EMAIL:-}
confirmation=${PYMES_OBSOLETE_SECRETS_CONFIRM:-}

[[ "$project" == "$expected_project" ]] || {
  echo "obsolete-secret retirement is restricted to $expected_project" >&2
  exit 2
}
[[ "$region" == "$expected_region" ]] || {
  echo "obsolete-secret retirement is restricted to $expected_region" >&2
  exit 2
}
case "$mode" in
  plan|audit|apply) ;;
  *)
    echo "PYMES_OBSOLETE_SECRETS_MODE must be plan, audit or apply" >&2
    exit 2
    ;;
esac
case "$target_environment" in
  stg|prd|all) ;;
  *)
    echo "PYMES_OBSOLETE_SECRETS_ENV must be stg, prd or all" >&2
    exit 2
    ;;
esac

if [[ "$target_environment" == "all" ]]; then
  environments=(stg prd)
else
  environments=("$target_environment")
fi

internal_obsolete_accessors() {
  local environment="$1"
  printf '%s\n' \
    "serviceAccount:pymes-v3-api-${environment}@${project}.iam.gserviceaccount.com" \
    "serviceAccount:pymes-v3-provision-${environment}@${project}.iam.gserviceaccount.com" \
    "serviceAccount:pymes-v3-worker-${environment}@${project}.iam.gserviceaccount.com"
}

fiscal_obsolete_accessor() {
  local environment="$1"
  printf 'serviceAccount:pymes-v3-fiscal-%s@%s.iam.gserviceaccount.com\n' \
    "$environment" "$project"
}

if [[ "$mode" == "plan" ]]; then
  printf 'PLAN project=%s region=%s environments=%s mode=retire-obsolete-secrets\n' \
    "$project" "$region" "$target_environment"
  printf '%s\n' \
    "PLAN prove direct human gcloud authentication and exact project identity" \
    "PLAN require zero non-human project-level roles carrying secretmanager.versions.access" \
    "PLAN inspect every Cloud Run service, job and revision in the project for either obsolete secret"
  for environment in "${environments[@]}"; do
    fiscal_secret="pymes-v3-${environment}-fiscal-credential"
    internal_secret="pymes-v3-${environment}-internal-jwt-seed"
    printf 'PLAN secret=%s require=enabled_versions:0 remove_only_accessor=%s\n' \
      "$fiscal_secret" "$(fiscal_obsolete_accessor "$environment")"
    printf 'PLAN secret=%s remove_only_accessors=%s disable_enabled_versions=true\n' \
      "$internal_secret" "$(internal_obsolete_accessors "$environment" | paste -sd, -)"
  done
  printf '%s\n' \
    "PLAN preserve every secret container and every secret version; destroy/delete/set-policy operations are forbidden" \
    "No GCP resources changed."
  exit 0
fi

for command in gcloud jq; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "$command is required" >&2
    exit 1
  }
done

assert_direct_gcloud_auth() {
  local variable property value active_account configured_project
  for variable in \
    CLOUDSDK_AUTH_ACCESS_TOKEN \
    CLOUDSDK_AUTH_ACCESS_TOKEN_FILE \
    CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE \
    CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT \
    CLOUDSDK_AUTH_LOGIN_CONFIG_FILE \
    GOOGLE_APPLICATION_CREDENTIALS \
    GOOGLE_GHA_CREDS_PATH; do
    [[ -z "${!variable-}" ]] || {
      echo "obsolete-secret retirement forbids delegated or overridden gcloud credentials: $variable" >&2
      return 1
    }
  done
  for property in \
    auth/impersonate_service_account \
    auth/credential_file_override \
    auth/access_token_file \
    auth/login_config_file; do
    value=$(gcloud config get-value "$property" 2>/dev/null) || {
      echo "could not verify direct gcloud credential property: $property" >&2
      return 1
    }
    [[ -z "$value" || "$value" == "(unset)" ]] || {
      echo "obsolete-secret retirement forbids delegated or overridden gcloud credentials: $property" >&2
      return 1
    }
  done
  active_account=$(gcloud auth list \
    --filter=status:ACTIVE --format='value(account)') || {
    echo "could not resolve the active gcloud account" >&2
    return 1
  }
  [[ "$operator_email" == "$expected_operator" &&
     "$active_account" == "$expected_operator" ]] || {
    echo "active gcloud account and PYMES_OBSOLETE_SECRETS_OPERATOR_EMAIL must both equal $expected_operator" >&2
    return 1
  }
  configured_project=$(gcloud config get-value project 2>/dev/null) || {
    echo "could not verify the active gcloud project" >&2
    return 1
  }
  [[ "$configured_project" == "$project" ]] || {
    echo "active gcloud project must be $project" >&2
    return 1
  }
}

assert_project_identity() {
  local project_json
  project_json=$(gcloud projects describe "$project" --format=json) || {
    echo "could not read GCP project identity" >&2
    return 1
  }
  jq -e \
    --arg project "$expected_project" \
    --arg number "$expected_project_number" '
      type == "object" and
      .projectId == $project and
      ((.projectNumber | tostring) == $number) and
      .lifecycleState == "ACTIVE"
    ' <<<"$project_json" >/dev/null || {
    echo "GCP project identity differs from the reviewed Pymes project" >&2
    return 1
  }
}

assert_no_nonhuman_project_secret_access() {
  local policy_json role role_json
  policy_json=$(gcloud projects get-iam-policy "$project" --format=json) || {
    echo "could not read project IAM policy" >&2
    return 1
  }
  jq -e '
    type == "object" and
    ((.bindings // []) | type == "array") and
    all(
      .bindings[]?;
      type == "object" and
      (.role | type == "string") and
      ((.members // []) | type == "array") and
      all(.members[]?; type == "string") and
      ((.condition // null) == null or ((.condition // null) | type == "object"))
    )
  ' <<<"$policy_json" >/dev/null || {
    echo "project IAM policy is malformed" >&2
    return 1
  }

  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    role_json=$(gcloud iam roles describe "$role" \
      --project="$project" --format=json) || {
      echo "could not inspect project IAM role: $role" >&2
      return 1
    }
    jq -e '
      type == "object" and
      ((.includedPermissions // []) | type == "array") and
      all(.includedPermissions[]?; type == "string")
    ' <<<"$role_json" >/dev/null || {
      echo "project IAM role metadata is malformed: $role" >&2
      return 1
    }
    if ! jq -e '
      (.includedPermissions // [])
      | index("secretmanager.versions.access") != null
    ' <<<"$role_json" >/dev/null; then
      continue
    fi
    jq -e \
      --arg role "$role" \
      --arg owner "$expected_human_secret_admin" '
        all(
          .bindings[]? | select(.role == $role);
          $role == "roles/owner" and
          ((.condition // null) == null) and
          (.members == [$owner])
        )
      ' <<<"$policy_json" >/dev/null || {
      echo "non-human or unexpected project-level secret access remains through $role" >&2
      return 1
    }
  done < <(
    jq -r '.bindings[]?.role' <<<"$policy_json" | LC_ALL=C sort -u
  )
}

target_secret_names() {
  local environment
  for environment in "${environments[@]}"; do
    printf '%s\n' \
      "pymes-v3-${environment}-fiscal-credential" \
      "pymes-v3-${environment}-internal-jwt-seed"
  done
}

assert_no_cloud_run_secret_references() {
  local resource_kind resources_json services_json cloud_region

  inspect_resources() {
    local inspected_kind="$1" inspected_json="$2" secret references
    jq -e 'type == "array" and all(.[]; type == "object")' \
      <<<"$inspected_json" >/dev/null || {
      echo "Cloud Run $inspected_kind returned malformed JSON" >&2
      return 1
    }
    while IFS= read -r secret; do
      references=$(jq -r --arg secret "$secret" '
        .[]
        | select(any(.. | strings; contains($secret)))
        | (.metadata.name // .name // "<unknown>")
        | tostring
      ' <<<"$inspected_json" | LC_ALL=C sort -u)
      if [[ -n "$references" ]]; then
        echo "Cloud Run $inspected_kind still reference obsolete secret $secret:" >&2
        printf '%s\n' "$references" >&2
        return 1
      fi
    done < <(target_secret_names)
  }

  services_json=$(CLOUDSDK_RUN_REGION= gcloud run services list \
    --project="$project" --platform=managed --format=json) || {
    echo "could not list the global Cloud Run services inventory" >&2
    return 1
  }
  inspect_resources "services" "$services_json"
  jq -e '
    all(
      .[];
      (.metadata.labels["cloud.googleapis.com/location"] | type == "string") and
      (.metadata.labels["cloud.googleapis.com/location"] |
        test("^[a-z][a-z0-9-]*[a-z0-9]$"))
    )
  ' <<<"$services_json" >/dev/null || {
    echo "global Cloud Run services omit a valid region" >&2
    return 1
  }

  resources_json=$(CLOUDSDK_RUN_REGION= gcloud run jobs list \
    --project="$project" --format=json) || {
    echo "could not list the global Cloud Run jobs inventory" >&2
    return 1
  }
  inspect_resources "jobs" "$resources_json"

  while IFS= read -r cloud_region; do
    [[ -n "$cloud_region" ]] || continue
    resources_json=$(gcloud run revisions list \
      --project="$project" --region="$cloud_region" \
      --platform=managed --format=json) || {
      echo "could not list Cloud Run revisions in $cloud_region" >&2
      return 1
    }
    inspect_resources "revisions in $cloud_region" "$resources_json"
  done < <(
    jq -r '
      [.[].metadata.labels["cloud.googleapis.com/location"]]
      | unique
      | sort
      | .[]
    ' <<<"$services_json"
  )
}

assert_secret_container() {
  local secret="$1" secret_json
  secret_json=$(gcloud secrets describe "$secret" \
    --project="$project" --format=json) || {
    echo "could not read obsolete secret container: $secret" >&2
    return 1
  }
  jq -e \
    --arg project "$project" \
    --arg number "$expected_project_number" \
    --arg secret "$secret" '
      type == "object" and
      (
        .name == ("projects/" + $project + "/secrets/" + $secret) or
        .name == ("projects/" + $number + "/secrets/" + $secret)
      )
    ' <<<"$secret_json" >/dev/null || {
    echo "obsolete secret metadata has an unexpected resource identity: $secret" >&2
    return 1
  }
}

read_secret_policy() {
  local secret="$1" policy_json
  policy_json=$(gcloud secrets get-iam-policy "$secret" \
    --project="$project" --format=json) || {
    echo "could not read IAM policy for obsolete secret: $secret" >&2
    return 1
  }
  jq -e '
    type == "object" and
    ((.bindings // []) | type == "array") and
    all(
      .bindings[]?;
      type == "object" and
      (.role | type == "string") and
      ((.members // []) | type == "array") and
      all(.members[]?; type == "string") and
      ((.condition // null) == null or ((.condition // null) | type == "object"))
    )
  ' <<<"$policy_json" >/dev/null || {
    echo "IAM policy for obsolete secret is malformed: $secret" >&2
    return 1
  }
  printf '%s\n' "$policy_json"
}

assert_fiscal_policy_is_known_or_empty() {
  local environment="$1" secret="$2" policy_json="$3" fiscal_member
  fiscal_member=$(fiscal_obsolete_accessor "$environment")
  jq -e --arg fiscal "$fiscal_member" '
    (.bindings // []) as $bindings
    | (
        ($bindings | length) == 0 or
        (
          ($bindings | length) == 1 and
          $bindings[0].role == "roles/secretmanager.secretAccessor" and
          (($bindings[0].condition // null) == null) and
          $bindings[0].members == [$fiscal]
        )
      )
  ' \
    <<<"$policy_json" >/dev/null || {
    echo "fiscal credential has an unexpected IAM accessor or binding: $secret" >&2
    return 1
  }
}

assert_internal_policy_is_known_or_empty() {
  local environment="$1" secret="$2" policy_json="$3"
  local api_member provision_member worker_member
  api_member="serviceAccount:pymes-v3-api-${environment}@${project}.iam.gserviceaccount.com"
  provision_member="serviceAccount:pymes-v3-provision-${environment}@${project}.iam.gserviceaccount.com"
  worker_member="serviceAccount:pymes-v3-worker-${environment}@${project}.iam.gserviceaccount.com"
  jq -e \
    --arg api "$api_member" \
    --arg provision "$provision_member" \
    --arg worker "$worker_member" '
      (.bindings // []) as $bindings
      | (
          ($bindings | length) == 0 or
          (
            ($bindings | length) == 1 and
            $bindings[0].role == "roles/secretmanager.secretAccessor" and
            (($bindings[0].condition // null) == null) and
            (($bindings[0].members // []) | length) > 0 and
            (($bindings[0].members | unique | length) == ($bindings[0].members | length)) and
            all(
              $bindings[0].members[];
              . == $api or . == $provision or . == $worker
            )
          )
        )
    ' <<<"$policy_json" >/dev/null || {
    echo "internal JWT seed has an unexpected accessor or IAM binding: $secret" >&2
    return 1
  }
}

read_secret_versions() {
  local secret="$1" versions_json
  versions_json=$(gcloud secrets versions list "$secret" \
    --project="$project" --format=json) || {
    echo "could not list versions for obsolete secret: $secret" >&2
    return 1
  }
  jq -e \
    --arg project "$project" \
    --arg number "$expected_project_number" \
    --arg secret "$secret" '
      type == "array" and
      ((map(.name) | unique | length) == length) and
      all(
        .[];
        type == "object" and
        (.state == "ENABLED" or .state == "DISABLED" or .state == "DESTROYED") and
        (
          .name
          | test(
              "^projects/(" + $project + "|" + $number + ")/secrets/" +
              $secret + "/versions/[1-9][0-9]*$"
            )
        )
      )
    ' <<<"$versions_json" >/dev/null || {
    echo "version metadata for obsolete secret is malformed: $secret" >&2
    return 1
  }
  printf '%s\n' "$versions_json"
}

assert_no_enabled_versions() {
  local secret="$1" versions_json="$2"
  jq -e 'all(.[]; .state != "ENABLED")' \
    <<<"$versions_json" >/dev/null || {
    echo "obsolete secret still has an enabled version: $secret" >&2
    return 1
  }
}

preflight_environment() {
  local environment="$1" fiscal_secret internal_secret policy_json versions_json
  fiscal_secret="pymes-v3-${environment}-fiscal-credential"
  internal_secret="pymes-v3-${environment}-internal-jwt-seed"

  assert_secret_container "$fiscal_secret"
  policy_json=$(read_secret_policy "$fiscal_secret")
  assert_fiscal_policy_is_known_or_empty \
    "$environment" "$fiscal_secret" "$policy_json"
  versions_json=$(read_secret_versions "$fiscal_secret")
  assert_no_enabled_versions "$fiscal_secret" "$versions_json"

  assert_secret_container "$internal_secret"
  policy_json=$(read_secret_policy "$internal_secret")
  assert_internal_policy_is_known_or_empty \
    "$environment" "$internal_secret" "$policy_json"
  read_secret_versions "$internal_secret" >/dev/null
}

policy_has_member() {
  local policy_json="$1" member="$2"
  jq -e --arg member "$member" '
    any(
      .bindings[]?;
      .role == "roles/secretmanager.secretAccessor" and
      ((.members // []) | index($member) != null)
    )
  ' <<<"$policy_json" >/dev/null
}

remove_fiscal_accessor() {
  local environment="$1" secret="$2" member="$3"
  local policy_json mutation_status=0
  policy_json=$(read_secret_policy "$secret")
  assert_fiscal_policy_is_known_or_empty \
    "$environment" "$secret" "$policy_json"
  if ! policy_has_member "$policy_json" "$member"; then
    return
  fi

  gcloud secrets remove-iam-policy-binding "$secret" \
    --project="$project" \
    --member="$member" \
    --role=roles/secretmanager.secretAccessor \
    --condition=None \
    --quiet >/dev/null || mutation_status=$?

  policy_json=$(read_secret_policy "$secret")
  assert_fiscal_policy_is_known_or_empty \
    "$environment" "$secret" "$policy_json"
  if policy_has_member "$policy_json" "$member"; then
    echo "obsolete fiscal accessor remains after removal attempt: $secret $member" >&2
    return 1
  fi
  if ((mutation_status != 0)); then
    echo "RECOVERED IAM response loss after verified removal: $secret $member"
  fi
}

remove_internal_accessor() {
  local environment="$1" secret="$2" member="$3"
  local policy_json mutation_status=0
  policy_json=$(read_secret_policy "$secret")
  assert_internal_policy_is_known_or_empty \
    "$environment" "$secret" "$policy_json"
  if ! policy_has_member "$policy_json" "$member"; then
    return
  fi

  gcloud secrets remove-iam-policy-binding "$secret" \
    --project="$project" \
    --member="$member" \
    --role=roles/secretmanager.secretAccessor \
    --condition=None \
    --quiet >/dev/null || mutation_status=$?

  policy_json=$(read_secret_policy "$secret")
  assert_internal_policy_is_known_or_empty \
    "$environment" "$secret" "$policy_json"
  if policy_has_member "$policy_json" "$member"; then
    echo "obsolete accessor remains after removal attempt: $secret $member" >&2
    return 1
  fi
  if ((mutation_status != 0)); then
    echo "RECOVERED IAM response loss after verified removal: $secret $member"
  fi
}

describe_secret_version() {
  local secret="$1" version="$2" version_json
  version_json=$(gcloud secrets versions describe "$version" \
    --secret="$secret" --project="$project" --format=json) || {
    echo "could not verify obsolete secret version: $secret/$version" >&2
    return 1
  }
  jq -e \
    --arg project "$project" \
    --arg number "$expected_project_number" \
    --arg secret "$secret" \
    --arg version "$version" '
      type == "object" and
      (
        .name == ("projects/" + $project + "/secrets/" + $secret + "/versions/" + $version) or
        .name == ("projects/" + $number + "/secrets/" + $secret + "/versions/" + $version)
      ) and
      (.state == "ENABLED" or .state == "DISABLED" or .state == "DESTROYED")
    ' <<<"$version_json" >/dev/null || {
    echo "obsolete secret version returned malformed metadata: $secret/$version" >&2
    return 1
  }
  printf '%s\n' "$version_json"
}

disable_internal_version() {
  local secret="$1" version="$2" version_json mutation_status=0
  [[ "$version" =~ ^[1-9][0-9]*$ ]] || {
    echo "refusing malformed secret version identifier: $secret/$version" >&2
    return 1
  }
  gcloud secrets versions disable "$version" \
    --secret="$secret" --project="$project" --quiet >/dev/null ||
    mutation_status=$?

  version_json=$(describe_secret_version "$secret" "$version")
  jq -e '.state == "DISABLED"' <<<"$version_json" >/dev/null || {
    echo "obsolete secret version is not disabled after mutation attempt: $secret/$version" >&2
    return 1
  }
  if ((mutation_status != 0)); then
    echo "RECOVERED version-disable response loss after verified postcondition: $secret/$version"
  fi
}

retire_fiscal_secret() {
  local environment="$1" secret member policy_json versions_json
  secret="pymes-v3-${environment}-fiscal-credential"
  member=$(fiscal_obsolete_accessor "$environment")

  assert_no_cloud_run_secret_references
  assert_no_nonhuman_project_secret_access
  remove_fiscal_accessor "$environment" "$secret" "$member"

  policy_json=$(read_secret_policy "$secret")
  assert_fiscal_policy_is_known_or_empty \
    "$environment" "$secret" "$policy_json"
  jq -e '((.bindings // []) | length) == 0' \
    <<<"$policy_json" >/dev/null || {
    echo "obsolete fiscal credential retains IAM bindings: $secret" >&2
    return 1
  }
  versions_json=$(read_secret_versions "$secret")
  assert_no_enabled_versions "$secret" "$versions_json"
}

retire_internal_secret() {
  local environment="$1" secret policy_json versions_json member version
  secret="pymes-v3-${environment}-internal-jwt-seed"

  # Recheck immediately before changing each secret. A stale revision is a
  # hard stop even if the active service template no longer mounts the value.
  assert_no_cloud_run_secret_references
  assert_no_nonhuman_project_secret_access

  while IFS= read -r member; do
    remove_internal_accessor "$environment" "$secret" "$member"
  done < <(internal_obsolete_accessors "$environment")

  versions_json=$(read_secret_versions "$secret")
  while IFS= read -r version; do
    [[ -n "$version" ]] || continue
    disable_internal_version "$secret" "$version"
  done < <(
    jq -r '
      .[]
      | select(.state == "ENABLED")
      | (.name | split("/") | last)
    ' <<<"$versions_json"
  )

  policy_json=$(read_secret_policy "$secret")
  assert_internal_policy_is_known_or_empty \
    "$environment" "$secret" "$policy_json"
  jq -e '((.bindings // []) | length) == 0' \
    <<<"$policy_json" >/dev/null || {
    echo "obsolete internal JWT seed retains IAM bindings: $secret" >&2
    return 1
  }
  versions_json=$(read_secret_versions "$secret")
  assert_no_enabled_versions "$secret" "$versions_json"
}

audit_environment() {
  local environment="$1" fiscal_secret internal_secret policy_json versions_json
  fiscal_secret="pymes-v3-${environment}-fiscal-credential"
  internal_secret="pymes-v3-${environment}-internal-jwt-seed"

  assert_secret_container "$fiscal_secret"
  policy_json=$(read_secret_policy "$fiscal_secret")
  assert_fiscal_policy_is_known_or_empty \
    "$environment" "$fiscal_secret" "$policy_json"
  jq -e '((.bindings // []) | length) == 0' \
    <<<"$policy_json" >/dev/null || {
    echo "obsolete fiscal credential retains IAM bindings: $fiscal_secret" >&2
    return 1
  }
  versions_json=$(read_secret_versions "$fiscal_secret")
  assert_no_enabled_versions "$fiscal_secret" "$versions_json"

  assert_secret_container "$internal_secret"
  policy_json=$(read_secret_policy "$internal_secret")
  assert_internal_policy_is_known_or_empty \
    "$environment" "$internal_secret" "$policy_json"
  jq -e '((.bindings // []) | length) == 0' \
    <<<"$policy_json" >/dev/null || {
    echo "obsolete internal JWT seed retains IAM bindings: $internal_secret" >&2
    return 1
  }
  versions_json=$(read_secret_versions "$internal_secret")
  assert_no_enabled_versions "$internal_secret" "$versions_json"

  printf 'AUDIT READY environment=%s fiscal=%s internal=%s containers=preserved versions=non-enabled direct_iam=empty inherited_nonhuman=none cloud_run_refs=none\n' \
    "$environment" "$fiscal_secret" "$internal_secret"
}

assert_direct_gcloud_auth
assert_project_identity
assert_no_nonhuman_project_secret_access
assert_no_cloud_run_secret_references

# Preflight every selected environment before the first mutation so an
# unexpected PRD policy cannot leave an otherwise-valid STG half-applied.
for environment in "${environments[@]}"; do
  preflight_environment "$environment"
done

if [[ "$mode" == "audit" ]]; then
  for environment in "${environments[@]}"; do
    audit_environment "$environment"
  done
  echo "obsolete Pymes v3 secrets are retired and recoverable"
  exit 0
fi

expected_confirmation="RETIRE_OBSOLETE_PYMES_V3_${target_environment^^}"
[[ "$confirmation" == "$expected_confirmation" ]] || {
  echo "apply requires PYMES_OBSOLETE_SECRETS_CONFIRM=$expected_confirmation" >&2
  exit 2
}

for environment in "${environments[@]}"; do
  retire_fiscal_secret "$environment"
  retire_internal_secret "$environment"
done

assert_no_cloud_run_secret_references
assert_no_nonhuman_project_secret_access
for environment in "${environments[@]}"; do
  audit_environment "$environment"
done

echo "obsolete Pymes v3 secrets retired without deleting containers or destroying versions"
