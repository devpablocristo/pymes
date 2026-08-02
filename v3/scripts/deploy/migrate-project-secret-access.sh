#!/usr/bin/env bash
set -euo pipefail

# Replaces the four historical project-wide Secret Manager readers with
# resource-level grants. Cloud Run references are discovered from services,
# jobs and every retained revision before any IAM mutation. GitHub identities
# have additional lifecycle gates because their programmatic use cannot be
# inferred from Cloud Run.

readonly expected_project=pymes-dev-352318
readonly expected_project_number=884236221349
readonly expected_region=us-central1
readonly expected_operator=softponti@gmail.com
readonly legacy_pymes_principal="principalSet://iam.googleapis.com/projects/${expected_project_number}/locations/global/workloadIdentityPools/github-actions-pool/attribute.repository/devpablocristo/pymes"

project=${PYMES_GCP_PROJECT:-$expected_project}
region=${PYMES_GCP_REGION:-$expected_region}
mode=${PYMES_PROJECT_SECRET_ACCESS_MODE:-plan}
scope=${PYMES_PROJECT_SECRET_ACCESS_SCOPE:-all}
operator_email=${PYMES_PROJECT_SECRET_ACCESS_OPERATOR_EMAIL:-}
confirmation=${PYMES_PROJECT_SECRET_ACCESS_CONFIRM:-}

core_email="pymes-core-runtime-stg@${project}.iam.gserviceaccount.com"
vertical_email="pymes-vertical-runtime-stg@${project}.iam.gserviceaccount.com"
shared_github_email="github-actions@${project}.iam.gserviceaccount.com"
dedicated_github_email="pymes-github-actions-stg@${project}.iam.gserviceaccount.com"

[[ "$project" == "$expected_project" ]] || {
  echo "project secret-access migration is restricted to $expected_project" >&2
  exit 2
}
[[ "$region" == "$expected_region" ]] || {
  echo "project secret-access migration is restricted to $expected_region" >&2
  exit 2
}
case "$mode" in
  plan|audit|apply) ;;
  *)
    echo "PYMES_PROJECT_SECRET_ACCESS_MODE must be plan, audit or apply" >&2
    exit 2
    ;;
esac
case "$scope" in
  runtime|github|all) ;;
  *)
    echo "PYMES_PROJECT_SECRET_ACCESS_SCOPE must be runtime, github or all" >&2
    exit 2
    ;;
esac

scope_emails() {
  case "$scope" in
    runtime)
      printf '%s\n' "$core_email" "$vertical_email"
      ;;
    github)
      printf '%s\n' "$shared_github_email" "$dedicated_github_email"
      ;;
    all)
      printf '%s\n' \
        "$core_email" \
        "$vertical_email" \
        "$shared_github_email" \
        "$dedicated_github_email"
      ;;
  esac
}

expected_workload_secrets() {
  case "$1" in
    "$core_email")
      printf '%s\n' \
        pymes-clerk-secret-key-stg \
        pymes-companion-api-key-stg \
        pymes-companion-internal-jwt-secret-stg \
        pymes-database-url-stg \
        pymes-governance-api-key-stg \
        pymes-internal-service-token-stg
      ;;
    "$vertical_email")
      printf '%s\n' \
        pymes-database-url-stg \
        pymes-internal-service-token-stg
      ;;
  esac
}

expected_direct_secrets() {
  case "$1" in
    "$core_email"|"$vertical_email")
      expected_workload_secrets "$1"
      ;;
    "$shared_github_email")
      # This grant already exists and is preserved conservatively. Current
      # source and 30-day audit-log inspection did not prove a live caller.
      printf '%s\n' GOVERNANCE_API_KEY
      ;;
    "$dedicated_github_email")
      ;;
  esac
}

if [[ "$mode" == "plan" ]]; then
  printf 'PLAN project=%s region=%s scope=%s mode=migrate-project-secret-access\n' \
    "$project" "$region" "$scope"
  printf '%s\n' \
    "PLAN inventory Cloud Run services, jobs and all retained revisions before mutation" \
    "PLAN grant every reviewed secret first, re-read each direct policy, then remove project access one member at a time" \
    "PLAN core-runtime exact secrets=pymes-clerk-secret-key-stg,pymes-companion-api-key-stg,pymes-companion-internal-jwt-secret-stg,pymes-database-url-stg,pymes-governance-api-key-stg,pymes-internal-service-token-stg" \
    "PLAN vertical-runtime exact secrets=pymes-database-url-stg,pymes-internal-service-token-stg" \
    "PLAN shared-github exact preserved direct secret=GOVERNANCE_API_KEY and require zero Cloud Run consumers" \
    "PLAN dedicated-github exact secrets=none and require completed legacy-WIF retirement plus disabled account" \
    "PLAN remove roles/secretmanager.admin from the disabled dedicated GitHub account because it independently grants secretmanager.versions.access" \
    "PLAN never access secret payloads and never delete secrets, versions, workloads or service accounts" \
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
      echo "project secret-access migration forbids delegated credentials: $variable" >&2
      return 1
    }
  done
  for property in \
    auth/impersonate_service_account \
    auth/credential_file_override \
    auth/access_token_file \
    auth/login_config_file; do
    value=$(gcloud config get-value "$property" 2>/dev/null) || return
    [[ -z "$value" || "$value" == "(unset)" ]] || {
      echo "project secret-access migration forbids delegated credentials: $property" >&2
      return 1
    }
  done
  active_account=$(gcloud auth list \
    --filter=status:ACTIVE --format='value(account)')
  [[ "$operator_email" == "$expected_operator" &&
     "$active_account" == "$expected_operator" ]] || {
    echo "active account and PYMES_PROJECT_SECRET_ACCESS_OPERATOR_EMAIL must both equal $expected_operator" >&2
    return 1
  }
  configured_project=$(gcloud config get-value project 2>/dev/null)
  [[ "$configured_project" == "$project" ]] || {
    echo "active gcloud project must be $project" >&2
    return 1
  }
}

assert_project_identity() {
  local project_json
  project_json=$(gcloud projects describe "$project" --format=json)
  jq -e \
    --arg project "$expected_project" \
    --arg number "$expected_project_number" '
      type == "object" and
      .projectId == $project and
      ((.projectNumber | tostring) == $number) and
      .lifecycleState == "ACTIVE"
    ' <<<"$project_json" >/dev/null || {
    echo "GCP project identity differs from the reviewed shared project" >&2
    return 1
  }
}

validate_resource_array() {
  local description="$1" payload="$2"
  jq -e 'type == "array" and all(.[]; type == "object")' \
    <<<"$payload" >/dev/null || {
    echo "$description returned malformed JSON" >&2
    return 1
  }
}

services_json=
jobs_json=
revisions_json='[]'

load_cloud_run_inventory() {
  local cloud_region region_revisions
  # An inherited run/region silently narrows list calls. Clearing it asks the
  # Cloud Run API for the global services/jobs inventory without probing
  # regions forbidden by the effective resource-location policy. Revisions
  # require an explicit region, so their authoritative region set is derived
  # from that global service inventory.
  services_json=$(CLOUDSDK_RUN_REGION= gcloud run services list \
    --project="$project" --platform=managed --format=json)
  jobs_json=$(CLOUDSDK_RUN_REGION= gcloud run jobs list \
    --project="$project" --format=json)
  validate_resource_array "global Cloud Run services" "$services_json"
  validate_resource_array "global Cloud Run jobs" "$jobs_json"
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

  revisions_json='[]'
  while IFS= read -r cloud_region; do
    [[ -n "$cloud_region" ]] || continue
    region_revisions=$(gcloud run revisions list \
      --project="$project" --region="$cloud_region" \
      --platform=managed --format=json)
    validate_resource_array \
      "Cloud Run revisions in $cloud_region" "$region_revisions"
    revisions_json=$(printf '%s\n%s\n' \
      "$revisions_json" "$region_revisions" | jq -sc 'add')
  done < <(
    jq -r '
      [.[].metadata.labels["cloud.googleapis.com/location"]]
      | unique
      | sort
      | .[]
    ' <<<"$services_json"
  )
}

resources_for_email() {
  local email="$1"
  printf '%s\n%s\n%s\n' \
    "$services_json" "$jobs_json" "$revisions_json" |
    jq -sc --arg email "$email" '
      .[0] as $services
      | .[1] as $jobs
      | .[2] as $revisions
      |
      def owns($resource):
        any(
          $resource | .. | objects;
          (.serviceAccountName? == $email) or
          (.serviceAccount? == $email)
        );
      def secret_names($resource):
        [
          $resource
          | ..
          | objects
          | if ((.secretKeyRef? // null) | type) == "object" then
              (.secretKeyRef.name // .secretKeyRef.secret // empty)
            elif ((.secret? // null) | type) == "object" then
              (.secret.secretName // .secret.secret // empty)
            else empty end
        ]
        | map(select(type == "string" and length > 0))
        | unique;
      [
        ($services[]? | ["service", .]),
        ($jobs[]? | ["job", .]),
        ($revisions[]? | ["revision", .])
      ]
      | map(
          select(owns(.[1]))
          | {
              kind: .[0],
              name: (.[1].metadata.name // .[1].name // "<unknown>"),
              region: (
                .[1].metadata.labels["cloud.googleapis.com/location"] //
                .[1].metadata.labels["run.googleapis.com/location"] //
                .[1].region //
                "<unknown>"
              ),
              secrets: secret_names(.[1])
            }
        )
      | sort_by(.kind, .region, .name)
    '
}

assert_workload_inventory() {
  local email="$1" resources observed expected resource_count
  resources=$(resources_for_email "$email")
  resource_count=$(jq 'length' <<<"$resources")
  observed=$(jq -r '[.[].secrets[]?] | unique | sort | .[]' <<<"$resources")
  expected=$(expected_workload_secrets "$email" | LC_ALL=C sort -u)

  case "$email" in
    "$core_email"|"$vertical_email")
      ((resource_count > 0)) || {
        echo "expected runtime account has no Cloud Run services/jobs/revisions: $email" >&2
        return 1
      }
      ;;
    "$shared_github_email"|"$dedicated_github_email")
      ((resource_count == 0)) || {
        echo "GitHub identity is attached to Cloud Run resources: $email" >&2
        jq -c '.[]' <<<"$resources" >&2
        return 1
      }
      ;;
  esac
  [[ "$observed" == "$expected" ]] || {
    echo "Cloud Run secret set differs from the reviewed set for $email" >&2
    printf 'expected:\n%s\nobserved:\n%s\n' "$expected" "$observed" >&2
    return 1
  }
  printf 'INVENTORY member=%s resources=%s secrets=%s\n' \
    "$email" \
    "$resource_count" \
    "$(printf '%s\n' "$observed" | paste -sd, -)"
}

project_policy=

read_project_policy() {
  project_policy=$(gcloud projects get-iam-policy "$project" --format=json)
  jq -e '
    type == "object" and
    ((.bindings // []) | type == "array") and
    all(
      .bindings[]?;
      (.role | type == "string") and
      ((.members // []) | type == "array") and
      all(.members[]?; type == "string")
    )
  ' <<<"$project_policy" >/dev/null || {
    echo "project IAM policy is malformed" >&2
    return 1
  }
}

member_roles() {
  local email="$1"
  jq -r --arg member "serviceAccount:${email}" '
    .bindings[]?
    | select((.members // []) | index($member) != null)
    | .role
  ' <<<"$project_policy" | LC_ALL=C sort -u
}

secret_read_roles() {
  local email="$1" role role_json
  while IFS= read -r role; do
    [[ -n "$role" ]] || continue
    if [[ "$role" != roles/* ]]; then
      echo "cannot prove custom role lacks secret payload access for $email: $role" >&2
      return 1
    fi
    role_json=$(gcloud iam roles describe "$role" --format=json) || {
      echo "could not resolve IAM role permissions: $role" >&2
      return 1
    }
    jq -e '
      type == "object" and
      ((.includedPermissions // []) | type == "array")
    ' <<<"$role_json" >/dev/null || {
      echo "IAM role metadata is malformed: $role" >&2
      return 1
    }
    if jq -e '
      (.includedPermissions // [])
      | index("secretmanager.versions.access") != null
    ' <<<"$role_json" >/dev/null; then
      printf '%s\n' "$role"
    fi
  done < <(member_roles "$email")
}

assert_known_project_secret_roles_before_apply() {
  local email="$1" roles expected
  roles=$(secret_read_roles "$email")
  case "$email" in
    "$dedicated_github_email")
      expected=$'roles/secretmanager.admin\nroles/secretmanager.secretAccessor'
      ;;
    *)
      expected=roles/secretmanager.secretAccessor
      ;;
  esac
  if [[ -z "$roles" ]]; then
    return
  fi
  [[ "$roles" == "$expected" ]] || {
    echo "unexpected project-level secret payload roles for $email" >&2
    printf 'expected pending roles:\n%s\nobserved:\n%s\n' "$expected" "$roles" >&2
    return 1
  }
}

assert_no_project_secret_roles() {
  local email="$1" roles
  roles=$(secret_read_roles "$email")
  [[ -z "$roles" ]] || {
    echo "project-level secret payload access remains for $email:" >&2
    printf '%s\n' "$roles" >&2
    return 1
  }
}

direct_secret_assets() {
  local email="$1" assets_json
  assets_json=$(gcloud asset search-all-iam-policies \
    --scope="projects/${project}" \
    --asset-types=secretmanager.googleapis.com/Secret \
    --query="policy:${email}" \
    --format=json)
  validate_resource_array "Cloud Asset Secret Manager policies" "$assets_json"
  jq -r --arg member "serviceAccount:${email}" '
    .[]?
    | select(
        any(
          .policy.bindings[]?;
          .role == "roles/secretmanager.secretAccessor" and
          ((.members // []) | index($member) != null)
        )
      )
    | (.resource | split("/") | last)
  ' <<<"$assets_json" | LC_ALL=C sort -u
}

assert_no_unexpected_direct_secrets() {
  local email="$1" observed expected
  observed=$(direct_secret_assets "$email")
  expected=$(expected_direct_secrets "$email" | LC_ALL=C sort -u)
  # Before runtime migration the expected direct grants may be absent, but no
  # unreviewed direct secret is allowed.
  comm -23 \
    <(printf '%s\n' "$observed" | sed '/^$/d' | LC_ALL=C sort -u) \
    <(printf '%s\n' "$expected" | sed '/^$/d' | LC_ALL=C sort -u) |
    if IFS= read -r unexpected; then
      echo "unexpected direct secret grant for $email: $unexpected" >&2
      return 1
    fi
}

read_secret_policy() {
  local secret="$1" policy_json
  gcloud secrets describe "$secret" --project="$project" --format=json |
    jq -e \
      --arg project "$project" \
      --arg number "$expected_project_number" \
      --arg secret "$secret" '
        .name == ("projects/" + $project + "/secrets/" + $secret) or
        .name == ("projects/" + $number + "/secrets/" + $secret)
      ' >/dev/null || {
    echo "secret identity is missing or malformed: $secret" >&2
    return 1
  }
  policy_json=$(gcloud secrets get-iam-policy "$secret" \
    --project="$project" --format=json)
  jq -e '
    type == "object" and
    ((.bindings // []) | type == "array") and
    all(
      .bindings[]?;
      (.role | type == "string") and
      ((.members // []) | type == "array") and
      ((.condition // null) == null or ((.condition // null) | type == "object"))
    )
  ' <<<"$policy_json" >/dev/null || {
    echo "secret IAM policy is malformed: $secret" >&2
    return 1
  }
  printf '%s\n' "$policy_json"
}

policy_has_direct_accessor() {
  local policy_json="$1" email="$2"
  jq -e --arg member "serviceAccount:${email}" '
    any(
      .bindings[]?;
      .role == "roles/secretmanager.secretAccessor" and
      ((.condition // null) == null) and
      ((.members // []) | index($member) != null)
    )
  ' <<<"$policy_json" >/dev/null
}

ensure_direct_accessor() {
  local email="$1" secret="$2" policy_json mutation_status=0
  policy_json=$(read_secret_policy "$secret")
  if policy_has_direct_accessor "$policy_json" "$email"; then
    return
  fi
  gcloud secrets add-iam-policy-binding "$secret" \
    --project="$project" \
    --member="serviceAccount:${email}" \
    --role=roles/secretmanager.secretAccessor \
    --condition=None --quiet >/dev/null || mutation_status=$?
  policy_json=$(read_secret_policy "$secret")
  policy_has_direct_accessor "$policy_json" "$email" || {
    echo "direct secret grant is absent after mutation attempt: $secret $email" >&2
    return 1
  }
  if ((mutation_status != 0)); then
    echo "RECOVERED direct-grant response loss: $secret $email"
  fi
}

assert_all_direct_accessors() {
  local email="$1" secret policy_json
  while IFS= read -r secret; do
    [[ -n "$secret" ]] || continue
    policy_json=$(read_secret_policy "$secret")
    policy_has_direct_accessor "$policy_json" "$email" || {
      echo "required direct secret grant is absent: $secret $email" >&2
      return 1
    }
  done < <(expected_direct_secrets "$email")
}

remove_project_role() {
  local email="$1" role="$2" roles mutation_status=0
  read_project_policy
  roles=$(member_roles "$email")
  if ! grep -Fxq "$role" <<<"$roles"; then
    return
  fi
  gcloud projects remove-iam-policy-binding "$project" \
    --member="serviceAccount:${email}" \
    --role="$role" \
    --condition=None --quiet >/dev/null || mutation_status=$?
  read_project_policy
  roles=$(member_roles "$email")
  if grep -Fxq "$role" <<<"$roles"; then
    echo "project role remains after removal attempt: $email $role" >&2
    return 1
  fi
  if ((mutation_status != 0)); then
    echo "RECOVERED project-IAM response loss: $email $role"
  fi
}

assert_github_retirement_preconditions() {
  local state policy_json
  state=$(gcloud iam service-accounts describe "$dedicated_github_email" \
    --project="$project" --format='value(disabled)')
  [[ "$state" == "True" ]] || {
    echo "BLOCKED: retire legacy Pymes WIF and disable $dedicated_github_email before GitHub secret-access migration" >&2
    return 1
  }
  policy_json=$(gcloud iam service-accounts get-iam-policy \
    "$dedicated_github_email" --project="$project" --format=json)
  jq -e --arg principal "$legacy_pymes_principal" '
    all(.bindings[]?.members[]?; . != $principal)
  ' <<<"$policy_json" >/dev/null || {
    echo "BLOCKED: dedicated GitHub account still trusts the legacy Pymes principal" >&2
    return 1
  }
  policy_json=$(gcloud iam service-accounts get-iam-policy \
    "$shared_github_email" --project="$project" --format=json)
  jq -e --arg principal "$legacy_pymes_principal" '
    all(.bindings[]?.members[]?; . != $principal)
  ' <<<"$policy_json" >/dev/null || {
    echo "BLOCKED: shared GitHub account still trusts the legacy Pymes principal" >&2
    return 1
  }
}

preflight_email() {
  local email="$1"
  assert_workload_inventory "$email"
  assert_no_unexpected_direct_secrets "$email"
  assert_known_project_secret_roles_before_apply "$email"
}

audit_email() {
  local email="$1"
  assert_workload_inventory "$email" >/dev/null
  assert_no_unexpected_direct_secrets "$email"
  assert_all_direct_accessors "$email"
  assert_no_project_secret_roles "$email"
  printf 'AUDIT READY member=%s direct_secrets=%s project_secret_access=none\n' \
    "$email" \
    "$(expected_direct_secrets "$email" | paste -sd, -)"
}

assert_direct_gcloud_auth
assert_project_identity
load_cloud_run_inventory
read_project_policy

if [[ "$scope" == "github" || "$scope" == "all" ]]; then
  assert_github_retirement_preconditions
fi

while IFS= read -r email; do
  preflight_email "$email"
done < <(scope_emails)

if [[ "$mode" == "audit" ]]; then
  while IFS= read -r email; do
    audit_email "$email"
  done < <(scope_emails)
  echo "project-wide Secret Manager readers are retired for scope=$scope"
  exit 0
fi

expected_confirmation="MIGRATE_PROJECT_SECRET_ACCESS_${scope^^}"
[[ "$confirmation" == "$expected_confirmation" ]] || {
  echo "apply requires PYMES_PROJECT_SECRET_ACCESS_CONFIRM=$expected_confirmation" >&2
  exit 2
}

while IFS= read -r email; do
  while IFS= read -r secret; do
    [[ -n "$secret" ]] || continue
    ensure_direct_accessor "$email" "$secret"
  done < <(expected_direct_secrets "$email")
done < <(scope_emails)

# Re-discover immediately before removing the broad fallback. A concurrent
# revision that introduces another secret aborts without removing project IAM.
load_cloud_run_inventory
while IFS= read -r email; do
  assert_workload_inventory "$email" >/dev/null
  assert_all_direct_accessors "$email"
done < <(scope_emails)

while IFS= read -r email; do
  remove_project_role "$email" roles/secretmanager.secretAccessor
  if [[ "$email" == "$dedicated_github_email" ]]; then
    remove_project_role "$email" roles/secretmanager.admin
  fi
done < <(scope_emails)

while IFS= read -r email; do
  audit_email "$email"
done < <(scope_emails)

echo "project-wide Secret Manager access replaced by exact direct grants for scope=$scope"
