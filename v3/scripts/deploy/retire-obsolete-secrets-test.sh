#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script="$script_dir/retire-obsolete-secrets.sh"
fake_bin="$script_dir/testdata/obsolete-secrets"
tmp_dir=$(mktemp -d)
trap 'rm -rf -- "$tmp_dir"' EXIT
state_file="$tmp_dir/state.json"
stdout_file="$tmp_dir/stdout"
stderr_file="$tmp_dir/stderr"

tests=0

pass() {
  tests=$((tests + 1))
  printf 'ok %d - %s\n' "$tests" "$1"
}

expect_success() {
  local description="$1"
  shift
  if "$@" >"$stdout_file" 2>"$stderr_file"; then
    pass "$description"
    return
  fi
  echo "not ok - expected success: $description" >&2
  sed -n '1,160p' "$stdout_file" >&2
  sed -n '1,160p' "$stderr_file" >&2
  exit 1
}

expect_failure() {
  local description="$1"
  shift
  if "$@" >"$stdout_file" 2>"$stderr_file"; then
    echo "not ok - expected failure: $description" >&2
    sed -n '1,160p' "$stdout_file" >&2
    exit 1
  fi
  pass "$description"
}

write_initial_state() {
  jq -n '
    {
      project: {
        projectId: "pymes-dev-352318",
        projectNumber: "884236221349",
        lifecycleState: "ACTIVE"
      },
      configured_project: "pymes-dev-352318",
      active_account: "softponti@gmail.com",
      project_policy: {
        version: 1,
        bindings: [{
          role: "roles/owner",
          members: ["user:softponti@gmail.com"]
        }]
      },
      iam_roles: {
        "roles/owner": ["secretmanager.versions.access"],
        "roles/secretmanager.secretAccessor": [
          "secretmanager.versions.access"
        ]
      },
      config_properties: {
        "auth/impersonate_service_account": "(unset)",
        "auth/credential_file_override": "(unset)",
        "auth/access_token_file": "(unset)",
        "auth/login_config_file": "(unset)"
      },
      run: {
        services: [{
          metadata: {
            name: "unrelated-service",
            labels: {
              "cloud.googleapis.com/location": "us-central1"
            }
          }
        }],
        jobs: [],
        revisions: []
      },
      secrets: {
        "pymes-v3-stg-fiscal-credential": {
          name: "projects/884236221349/secrets/pymes-v3-stg-fiscal-credential",
          policy: {
            version: 1,
            bindings: [{
              role: "roles/secretmanager.secretAccessor",
              members: [
                "serviceAccount:pymes-v3-fiscal-stg@pymes-dev-352318.iam.gserviceaccount.com"
              ]
            }]
          },
          versions: [{
            name: "projects/884236221349/secrets/pymes-v3-stg-fiscal-credential/versions/1",
            state: "DISABLED"
          }]
        },
        "pymes-v3-stg-internal-jwt-seed": {
          name: "projects/884236221349/secrets/pymes-v3-stg-internal-jwt-seed",
          policy: {
            version: 1,
            bindings: [{
              role: "roles/secretmanager.secretAccessor",
              members: [
                "serviceAccount:pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com",
                "serviceAccount:pymes-v3-provision-stg@pymes-dev-352318.iam.gserviceaccount.com",
                "serviceAccount:pymes-v3-worker-stg@pymes-dev-352318.iam.gserviceaccount.com"
              ]
            }]
          },
          versions: [
            {
              name: "projects/884236221349/secrets/pymes-v3-stg-internal-jwt-seed/versions/1",
              state: "ENABLED"
            },
            {
              name: "projects/884236221349/secrets/pymes-v3-stg-internal-jwt-seed/versions/2",
              state: "DISABLED"
            }
          ]
        }
      },
      response_loss: {
        remove: {},
        disable: {}
      },
      calls: []
    }
  ' >"$state_file"
}

invoke() {
  local mode="$1" environment="$2" confirmation="$3"
  shift 3
  env \
    -u CLOUDSDK_AUTH_ACCESS_TOKEN \
    -u CLOUDSDK_AUTH_ACCESS_TOKEN_FILE \
    -u CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE \
    -u CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT \
    -u CLOUDSDK_AUTH_LOGIN_CONFIG_FILE \
    -u GOOGLE_APPLICATION_CREDENTIALS \
    -u GOOGLE_GHA_CREDS_PATH \
    PATH="$fake_bin:$PATH" \
    FAKE_GCLOUD_STATE="$state_file" \
    PYMES_GCP_PROJECT=pymes-dev-352318 \
    PYMES_GCP_REGION=us-central1 \
    PYMES_OBSOLETE_SECRETS_MODE="$mode" \
    PYMES_OBSOLETE_SECRETS_ENV="$environment" \
    PYMES_OBSOLETE_SECRETS_OPERATOR_EMAIL=softponti@gmail.com \
    PYMES_OBSOLETE_SECRETS_CONFIRM="$confirmation" \
    "$@" \
    "$script"
}

invoke_default_plan() {
  env \
    -u PYMES_OBSOLETE_SECRETS_MODE \
    -u PYMES_OBSOLETE_SECRETS_ENV \
    -u PYMES_OBSOLETE_SECRETS_CONFIRM \
    -u PYMES_OBSOLETE_SECRETS_OPERATOR_EMAIL \
    PATH="$fake_bin:$PATH" \
    FAKE_GCLOUD_STATE="$state_file" \
    "$script"
}

assert_no_mutations() {
  jq -e '
    all(
      .calls[]?;
      (
        (.[0:3] != ["secrets", "remove-iam-policy-binding", .[2]]) and
        (.[0:3] != ["secrets", "versions", "disable"])
      )
    )
  ' "$state_file" >/dev/null
}

assert_no_destructive_calls() {
  jq -e '
    all(
      .calls[]?;
      (
        (join(" ") | test("(^| )(delete|destroy|set-iam-policy)( |$)"))
        | not
      )
    )
  ' "$state_file" >/dev/null
}

mutation_count() {
  jq '
    [
      .calls[]
      | select(
          .[0:2] == ["secrets", "remove-iam-policy-binding"] or
          .[0:3] == ["secrets", "versions", "disable"]
        )
    ]
    | length
  ' "$state_file"
}

write_initial_state
expect_success "default mode is a read-free plan" invoke_default_plan
grep -Fq "No GCP resources changed." "$stdout_file"
jq -e '.calls | length == 0' "$state_file" >/dev/null
pass "plan does not invoke gcloud"

write_initial_state
expect_failure \
  "project override outside the exact Pymes project is rejected" \
  env PYMES_GCP_PROJECT=other-project \
    PYMES_OBSOLETE_SECRETS_MODE=plan "$script"

write_initial_state
expect_failure \
  "environment outside the allowlist is rejected" \
  env PYMES_OBSOLETE_SECRETS_ENV=dev \
    PYMES_OBSOLETE_SECRETS_MODE=plan "$script"

write_initial_state
expect_failure \
  "delegated auth environment is rejected before cloud inspection" \
  invoke audit stg "" \
    CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT=deployer@example.com
assert_no_mutations

write_initial_state
jq '.config_properties["auth/impersonate_service_account"] = "deployer@example.com"' \
  "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "delegated auth gcloud property is rejected" \
  invoke audit stg ""
assert_no_mutations

write_initial_state
jq '.active_account = "automation@example.com"' \
  "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "unexpected active gcloud account is rejected" \
  invoke audit stg ""
assert_no_mutations

write_initial_state
jq '
  .project_policy.bindings += [{
    role: "roles/secretmanager.secretAccessor",
    members: [
      "serviceAccount:legacy-reader@pymes-dev-352318.iam.gserviceaccount.com"
    ]
  }]
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "inherited non-human secret access blocks retirement" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
assert_no_mutations

for resource_kind in services jobs revisions; do
  write_initial_state
  jq --arg kind "$resource_kind" '
    .run[$kind] = [{
      metadata: {name: ("legacy-" + $kind)},
      spec: {
        template: {
          containers: [{
            env: [{
              valueSource: {
                secretKeyRef: {
                  secret: "pymes-v3-stg-internal-jwt-seed"
                }
              }
            }]
          }]
        }
      }
    }]
  ' "$state_file" >"$tmp_dir/next.json"
  mv "$tmp_dir/next.json" "$state_file"
  expect_failure \
    "Cloud Run ${resource_kind} reference blocks retirement" \
    invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
  assert_no_mutations
done

write_initial_state
jq '
  .secrets["pymes-v3-stg-fiscal-credential"].policy.bindings = [{
    role: "roles/viewer",
    members: ["user:unexpected@example.com"]
  }]
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "any fiscal-secret IAM binding fails closed" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
assert_no_mutations

write_initial_state
jq '
  .secrets["pymes-v3-stg-fiscal-credential"].policy.bindings[0].members += [
    "serviceAccount:unknown@pymes-dev-352318.iam.gserviceaccount.com"
  ]
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "unknown fiscal-credential accessor fails closed" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
assert_no_mutations

write_initial_state
jq '
  .secrets["pymes-v3-stg-fiscal-credential"].policy.bindings[0].condition = {
    title: "unexpected",
    expression: "true"
  }
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "conditional fiscal-credential binding fails closed" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
assert_no_mutations

write_initial_state
jq '
  .secrets["pymes-v3-stg-internal-jwt-seed"].policy.bindings[0].members += [
    "serviceAccount:unknown@pymes-dev-352318.iam.gserviceaccount.com"
  ]
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "unknown internal-seed accessor fails closed" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
assert_no_mutations

write_initial_state
jq '
  .secrets["pymes-v3-stg-internal-jwt-seed"].policy.bindings[0].condition = {
    title: "unexpected",
    expression: "true"
  }
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "conditional obsolete accessor binding fails closed" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
assert_no_mutations

write_initial_state
jq '.secrets["pymes-v3-stg-internal-jwt-seed"].policy.bindings = "malformed"' \
  "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "malformed IAM JSON fails closed" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
assert_no_mutations

write_initial_state
jq '.secrets["pymes-v3-stg-fiscal-credential"].versions[0].state = "ENABLED"' \
  "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure \
  "enabled fiscal credential version blocks every mutation" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
assert_no_mutations

write_initial_state
expect_failure \
  "audit rejects an internal seed that is not retired" \
  invoke audit stg ""
assert_no_mutations

write_initial_state
expect_failure \
  "apply requires the exact environment-scoped confirmation" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_PRD
assert_no_mutations

write_initial_state
jq '
  .response_loss.remove[
    "pymes-v3-stg-fiscal-credential|serviceAccount:pymes-v3-fiscal-stg@pymes-dev-352318.iam.gserviceaccount.com"
  ] = true
  |
  .response_loss.remove[
    "pymes-v3-stg-internal-jwt-seed|serviceAccount:pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com"
  ] = true
  | .response_loss.disable["pymes-v3-stg-internal-jwt-seed|1"] = true
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_success \
  "apply recovers from lost fiscal/internal IAM and version-disable responses" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
[[ "$(grep -Fc "RECOVERED IAM response loss" "$stdout_file")" == "2" ]]
grep -Fq "RECOVERED version-disable response loss" "$stdout_file"
jq -e '
  .secrets["pymes-v3-stg-internal-jwt-seed"].policy.bindings == [] and
  all(
    .secrets["pymes-v3-stg-internal-jwt-seed"].versions[];
    .state != "ENABLED"
  ) and
  .secrets["pymes-v3-stg-fiscal-credential"].policy.bindings == [] and
  all(
    .secrets["pymes-v3-stg-fiscal-credential"].versions[];
    .state != "ENABLED"
  )
' "$state_file" >/dev/null
pass "apply reaches exact recoverable postconditions"
assert_no_destructive_calls
pass "apply never deletes containers or destroys versions"

before_rerun=$(mutation_count)
expect_success \
  "a repeated apply is idempotent" \
  invoke apply stg RETIRE_OBSOLETE_PYMES_V3_STG
after_rerun=$(mutation_count)
[[ "$before_rerun" == "$after_rerun" ]]
pass "idempotent rerun performs no IAM or version mutation"

before_audit=$(mutation_count)
expect_success \
  "audit verifies the settled state without mutation" \
  invoke audit stg ""
after_audit=$(mutation_count)
[[ "$before_audit" == "$after_audit" ]]
grep -Fq "AUDIT READY environment=stg" "$stdout_file"
pass "audit is read-only and reports objective evidence"

printf '1..%d\n' "$tests"
