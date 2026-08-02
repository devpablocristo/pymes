#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script="$script_dir/migrate-project-secret-access.sh"
fake_bin="$script_dir/testdata/project-secret-access"
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

write_state() {
  jq -n '
    def env($names):
      $names | map({valueFrom: {secretKeyRef: {name: .}}});
    {
      configured_project: "pymes-dev-352318",
      active_account: "softponti@gmail.com",
      config_properties: {
        "auth/impersonate_service_account": "(unset)",
        "auth/credential_file_override": "(unset)",
        "auth/access_token_file": "(unset)",
        "auth/login_config_file": "(unset)"
      },
      project: {
        projectId: "pymes-dev-352318",
        projectNumber: "884236221349",
        lifecycleState: "ACTIVE"
      },
      roles: {
        "roles/cloudsql.client": ["cloudsql.instances.connect"],
        "roles/run.admin": ["run.services.update"],
        "roles/secretmanager.secretAccessor": [
          "secretmanager.versions.access"
        ],
        "roles/secretmanager.admin": [
          "secretmanager.versions.access",
          "secretmanager.secrets.setIamPolicy"
        ],
        "roles/owner": ["secretmanager.versions.access"]
      },
      project_policy: {
        version: 1,
        bindings: [
          {
            role: "roles/owner",
            members: ["user:softponti@gmail.com"]
          },
          {
            role: "roles/cloudsql.client",
            members: [
              "serviceAccount:pymes-core-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com",
              "serviceAccount:pymes-vertical-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com"
            ]
          },
          {
            role: "roles/run.admin",
            members: [
              "serviceAccount:github-actions@pymes-dev-352318.iam.gserviceaccount.com",
              "serviceAccount:pymes-github-actions-stg@pymes-dev-352318.iam.gserviceaccount.com"
            ]
          },
          {
            role: "roles/secretmanager.secretAccessor",
            members: [
              "serviceAccount:github-actions@pymes-dev-352318.iam.gserviceaccount.com",
              "serviceAccount:pymes-core-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com",
              "serviceAccount:pymes-github-actions-stg@pymes-dev-352318.iam.gserviceaccount.com",
              "serviceAccount:pymes-vertical-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com"
            ]
          },
          {
            role: "roles/secretmanager.admin",
            members: [
              "serviceAccount:pymes-github-actions-stg@pymes-dev-352318.iam.gserviceaccount.com"
            ]
          }
        ]
      },
      run: {
        regions: [{locationId: "us-central1"}, {locationId: "us-east1"}],
        "us-central1": {
          services: [
            {
              metadata: {
                name: "pymes-core",
                labels: {"cloud.googleapis.com/location": "us-central1"}
              },
              spec: {
                template: {
                  spec: {
                    serviceAccountName: "pymes-core-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com",
                    containers: [{
                      env: env([
                        "pymes-clerk-secret-key-stg",
                        "pymes-companion-api-key-stg",
                        "pymes-companion-internal-jwt-secret-stg",
                        "pymes-database-url-stg",
                        "pymes-governance-api-key-stg",
                        "pymes-internal-service-token-stg"
                      ])
                    }]
                  }
                }
              }
            },
            {
              metadata: {
                name: "pymes-beauty",
                labels: {"cloud.googleapis.com/location": "us-central1"}
              },
              spec: {
                template: {
                  spec: {
                    serviceAccountName: "pymes-vertical-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com",
                    containers: [{
                      env: env([
                        "pymes-database-url-stg",
                        "pymes-internal-service-token-stg"
                      ])
                    }]
                  }
                }
              }
            }
          ],
          jobs: [],
          revisions: [
            {
              metadata: {
                name: "pymes-core-00001",
                labels: {"cloud.googleapis.com/location": "us-central1"}
              },
              spec: {
                serviceAccountName: "pymes-core-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com",
                containers: [{
                  env: env([
                    "pymes-clerk-secret-key-stg",
                    "pymes-companion-api-key-stg",
                    "pymes-companion-internal-jwt-secret-stg",
                    "pymes-database-url-stg",
                    "pymes-governance-api-key-stg",
                    "pymes-internal-service-token-stg"
                  ])
                }]
              }
            }
          ]
        },
        "us-east1": {services: [], jobs: [], revisions: []}
      },
      secrets: {
        "pymes-clerk-secret-key-stg": {policy: {version: 1, bindings: []}},
        "pymes-companion-api-key-stg": {policy: {version: 1, bindings: []}},
        "pymes-companion-internal-jwt-secret-stg": {policy: {version: 1, bindings: []}},
        "pymes-database-url-stg": {policy: {version: 1, bindings: []}},
        "pymes-governance-api-key-stg": {policy: {version: 1, bindings: []}},
        "pymes-internal-service-token-stg": {policy: {version: 1, bindings: []}},
        "GOVERNANCE_API_KEY": {
          policy: {
            version: 1,
            bindings: [{
              role: "roles/secretmanager.secretAccessor",
              members: [
                "serviceAccount:github-actions@pymes-dev-352318.iam.gserviceaccount.com"
              ]
            }]
          }
        }
      },
      accounts: {
        "github-actions@pymes-dev-352318.iam.gserviceaccount.com": {
          disabled: false,
          policy: {
            bindings: [{
              role: "roles/iam.workloadIdentityUser",
              members: [
                "principalSet://iam.googleapis.com/projects/884236221349/locations/global/workloadIdentityPools/github-actions-pool/attribute.repository/devpablocristo/unrelated-repository"
              ]
            }]
          }
        },
        "pymes-github-actions-stg@pymes-dev-352318.iam.gserviceaccount.com": {
          disabled: false,
          policy: {
            bindings: [{
              role: "roles/iam.workloadIdentityUser",
              members: [
                "principalSet://iam.googleapis.com/projects/884236221349/locations/global/workloadIdentityPools/github-actions-pool/attribute.repository/devpablocristo/pymes"
              ]
            }]
          }
        }
      },
      response_loss: {add: {}, remove: {}},
      calls: []
    }
  ' >"$state_file"
}

invoke() {
  local mode="$1" scope="$2" confirmation="$3"
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
    PYMES_PROJECT_SECRET_ACCESS_MODE="$mode" \
    PYMES_PROJECT_SECRET_ACCESS_SCOPE="$scope" \
    PYMES_PROJECT_SECRET_ACCESS_OPERATOR_EMAIL=softponti@gmail.com \
    PYMES_PROJECT_SECRET_ACCESS_CONFIRM="$confirmation" \
    "$@" "$script"
}

mutation_count() {
  jq '[
    .calls[]
    | select(
        .[0:2] == ["secrets", "add-iam-policy-binding"] or
        .[0:2] == ["projects", "remove-iam-policy-binding"]
      )
  ] | length' "$state_file"
}

write_state
expect_success "default plan is cloud-read-free" \
  env -u PYMES_PROJECT_SECRET_ACCESS_MODE \
    -u PYMES_PROJECT_SECRET_ACCESS_SCOPE "$script"
jq -e '.calls | length == 0' "$state_file" >/dev/null
pass "plan performs no gcloud calls"

write_state
expect_failure "audit rejects pending project-wide runtime access" \
  invoke audit runtime ""
[[ "$(mutation_count)" == 0 ]]
pass "audit never mutates IAM"

write_state
jq '
  .run["us-east1"].jobs = [{
    metadata: {
      name: "unexpected-job",
      labels: {"cloud.googleapis.com/location": "us-east1"}
    },
    spec: {
      template: {
        spec: {
          template: {
            spec: {
              serviceAccountName: "pymes-core-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com",
              containers: [{
                env: [{
                  valueSource: {
                    secretKeyRef: {secret: "unexpected-secret"}
                  }
                }]
              }]
            }
          }
        }
      }
    }
  }]
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_failure "a secret reference in a second region fails closed" \
  invoke apply runtime MIGRATE_PROJECT_SECRET_ACCESS_RUNTIME
[[ "$(mutation_count)" == 0 ]]
pass "cross-region drift is detected before mutation"

write_state
jq '
  .response_loss.add[
    "pymes-clerk-secret-key-stg|serviceAccount:pymes-core-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com"
  ] = true
  |
  .response_loss.remove[
    "roles/secretmanager.secretAccessor|serviceAccount:pymes-core-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com"
  ] = true
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_success "runtime apply recovers lost grant/removal responses" \
  invoke apply runtime MIGRATE_PROJECT_SECRET_ACCESS_RUNTIME
grep -Fq "RECOVERED direct-grant response loss" "$stdout_file"
grep -Fq "RECOVERED project-IAM response loss" "$stdout_file"
jq -e '
  all(
    .project_policy.bindings[]?;
    .role != "roles/secretmanager.secretAccessor" or
    (
      (.members // [])
      | index("serviceAccount:pymes-core-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com") == null
    )
  ) and
  all(
    .project_policy.bindings[]?;
    .role != "roles/secretmanager.secretAccessor" or
    (
      (.members // [])
      | index("serviceAccount:pymes-vertical-runtime-stg@pymes-dev-352318.iam.gserviceaccount.com") == null
    )
  )
' "$state_file" >/dev/null
pass "runtime project-wide grants are removed"

before=$(mutation_count)
expect_success "settled runtime audit is read-only" invoke audit runtime ""
after=$(mutation_count)
[[ "$before" == "$after" ]]
pass "settled audit performs no mutation"

write_state
expect_failure "GitHub scope blocks until legacy WIF retirement disables account" \
  invoke apply github MIGRATE_PROJECT_SECRET_ACCESS_GITHUB
[[ "$(mutation_count)" == 0 ]]
pass "GitHub lifecycle blocker precedes mutation"

write_state
jq '
  .accounts["pymes-github-actions-stg@pymes-dev-352318.iam.gserviceaccount.com"].disabled = true
  |
  .accounts["pymes-github-actions-stg@pymes-dev-352318.iam.gserviceaccount.com"].policy.bindings = []
' "$state_file" >"$tmp_dir/next.json"
mv "$tmp_dir/next.json" "$state_file"
expect_success "GitHub scope removes accessor and redundant admin after retirement" \
  invoke apply github MIGRATE_PROJECT_SECRET_ACCESS_GITHUB
jq -e '
  all(
    .project_policy.bindings[]?;
    (.role != "roles/secretmanager.secretAccessor" and
     .role != "roles/secretmanager.admin") or
    all(
      .members[]?;
      . != "serviceAccount:github-actions@pymes-dev-352318.iam.gserviceaccount.com" and
      . != "serviceAccount:pymes-github-actions-stg@pymes-dev-352318.iam.gserviceaccount.com"
    )
  )
' "$state_file" >/dev/null
pass "GitHub project-wide secret roles are absent"

printf '1..%d\n' "$tests"
