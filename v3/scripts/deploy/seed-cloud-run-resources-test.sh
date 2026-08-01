#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=seed-cloud-run-resources.sh
source "$script_dir/seed-cloud-run-resources.sh"
# shellcheck source=initial-seed-audit-bounds.sh
source "$script_dir/initial-seed-audit-bounds.sh"

scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
sha=0123456789abcdef0123456789abcdef01234567
accounting_sha=89abcdef0123456789abcdef0123456789abcdef
digest=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
manifest="$scratch/manifest.env"
api_image="us-central1-docker.pkg.dev/pymes-dev-352318/pymes/api-image@sha256:${digest}"
api_account="pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com"
migrate_image="us-central1-docker.pkg.dev/pymes-dev-352318/pymes/migrate-image@sha256:${digest}"
migrate_account="pymes-v3-migrate-stg@pymes-dev-352318.iam.gserviceaccount.com"

write_manifest() {
  local extra=${1:-}
  {
    echo "PYMES_RELEASE_ENV=stg"
    echo "PYMES_SOURCE_SHA=$sha"
    echo "PYMES_OPEN_ACCOUNTING_SOURCE_SHA=$accounting_sha"
    for key in \
      PYMES_API_IMAGE \
      PYMES_WEB_IMAGE \
      PYMES_WORKER_IMAGE \
      PYMES_FISCAL_IMAGE \
      PYMES_ACCOUNTING_IMAGE \
      PYMES_ACCOUNTING_ADMIN_IMAGE \
      PYMES_PROVISION_IMAGE \
      PYMES_MIGRATE_IMAGE \
      PYMES_FISCAL_MIGRATE_IMAGE \
      PYMES_ACCOUNTING_MIGRATE_IMAGE; do
      image=$(tr '[:upper:]_' '[:lower:]-' <<<"${key#PYMES_}")
      echo "$key=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/${image}@sha256:${digest}"
    done
    [[ -z "$extra" ]] || echo "$extra"
  } >"$manifest"
}

expect_failure() {
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "FAIL: $description" >&2
    exit 1
  fi
}

fake_gcloud_property=
gcloud() {
  [[ "$1" == config && "$2" == get-value ]] || return 1
  if [[ "$3" == "$fake_gcloud_property" ]]; then
    echo "pymes-seed-delegated@pymes-dev-352318.iam.gserviceaccount.com"
  else
    echo "(unset)"
  fi
}

validate_seed_audit_fixture() {
  local logs="$1" allowed_resources="$2" allowed_sas="$3"
  jq -e \
    --argjson allowed_resources "$allowed_resources" \
    --argjson allowed_sas "$allowed_sas" \
    --arg project pymes-dev-352318 \
    --arg project_number 884236221349 \
    --arg region us-central1 \
    --arg operator softponti@gmail.com \
    --argjson limit 1001 \
    -f "$script_dir/initial-seed-audit.jq" \
    <<<"$logs" >/dev/null
}

assert_direct_gcloud_auth
export CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT=delegated@example.invalid
expect_failure "seed accepted an impersonated gcloud environment" \
  assert_direct_gcloud_auth
unset CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT
fake_gcloud_property=auth/impersonate_service_account
expect_failure "seed accepted an impersonated gcloud property" \
  assert_direct_gcloud_auth
fake_gcloud_property=
[[ "$(initial_seed_audit_end_at 2026-08-01T14:54:30Z 120)" == \
   "2026-08-01T14:56:30Z" ]]
expect_failure "audit upper bound accepted a zero grace period" \
  initial_seed_audit_end_at 2026-08-01T14:54:30Z 0
expect_failure "audit upper bound accepted a non-RFC3339 timestamp" \
  initial_seed_audit_end_at 2026-08-01T14:54:30.500Z 120

write_manifest
validate_manifest "$manifest" stg
[[ "${manifest_values[PYMES_SOURCE_SHA]}" == "$sha" ]]

write_manifest "UNREVIEWED=value"
expect_failure "manifest accepted an unknown key" \
  validate_manifest "$manifest" stg

write_manifest "PYMES_API_IMAGE=us-central1-docker.pkg.dev/pymes-dev-352318/pymes/duplicate@sha256:${digest}"
expect_failure "manifest accepted a duplicate key" \
  validate_manifest "$manifest" stg

write_manifest
expect_failure "manifest accepted the wrong environment" \
  validate_manifest "$manifest" prd

repository_json='{
  "full_name":"devpablocristo/pymes",
  "id":1173650578,
  "default_branch":"main",
  "archived":false
}'
branch_json=$(jq -cn --arg sha "$sha" '
  {name:"main",protected:true,commit:{sha:$sha}}
')
runs_json=$(jq -cn --arg sha "$sha" '
  {
    workflow_runs: [{
      head_sha:$sha,
      head_branch:"main",
      event:"push",
      status:"completed",
      conclusion:"success"
    }]
  }
')
validate_github_release_state \
  "$repository_json" "$branch_json" "$runs_json" "$sha"
expect_failure "seed accepted an unprotected main branch" \
  validate_github_release_state \
  "$repository_json" "$(jq '.protected=false' <<<"$branch_json")" \
  "$runs_json" "$sha"
expect_failure "seed accepted a different GitHub main head" \
  validate_github_release_state \
  "$repository_json" "$(jq '.commit.sha="ffffffffffffffffffffffffffffffffffffffff"' <<<"$branch_json")" \
  "$runs_json" "$sha"
expect_failure "seed accepted a failed V3 CI run" \
  validate_github_release_state \
  "$repository_json" "$branch_json" \
  "$(jq '.workflow_runs[0].conclusion="failure"' <<<"$runs_json")" "$sha"

allowed_seed_resources='["projects/pymes-dev-352318/locations/us-central1/services/pymes-v3-stg-api"]'
allowed_seed_accounts='["pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com"]'
act_as_resource='projects/-/serviceAccounts/pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com'
seed_audit_logs=$(jq -cn --arg act_as_resource "$act_as_resource" '
  [
    {
      protoPayload: {
        authenticationInfo: {
          principalEmail: "softponti@gmail.com"
        },
        serviceName: "run.googleapis.com",
        methodName: "google.cloud.run.v2.Services.CreateService",
        resourceName: "projects/pymes-dev-352318/locations/us-central1/services/pymes-v3-stg-api",
        status: {},
        request: {
          parent: "projects/pymes-dev-352318/locations/us-central1",
          serviceId: "pymes-v3-stg-api",
          service: {
            name: "projects/pymes-dev-352318/locations/us-central1/services/pymes-v3-stg-api"
          }
        }
      },
      resource: {labels: {service_name: "pymes-v3-stg-api"}}
    },
    {
      protoPayload: {
        authenticationInfo: {
          principalEmail: "pymes-seed-delegated@pymes-dev-352318.iam.gserviceaccount.com",
          serviceAccountDelegationInfo: [{
            firstPartyPrincipal: {
              principalEmail: "softponti@gmail.com"
            }
          }]
        },
        serviceName: "iam.googleapis.com",
        methodName: "iam.serviceAccounts.actAs",
        resourceName: $act_as_resource,
        status: {},
        authorizationInfo: [{
          resource: $act_as_resource,
          permission: "iam.serviceAccounts.actAs",
          granted: true,
          permissionType: "ADMIN_WRITE"
        }],
        request: {
          name: "pymes-v3-api-stg@pymes-dev-352318.iam.gserviceaccount.com",
          project_number: "884236221349"
        },
        response: {success: true}
      }
    }
  ]
')
validate_seed_audit_fixture \
  "$seed_audit_logs" "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted actAs on an unknown service account" \
  validate_seed_audit_fixture \
  "$(jq '
    .[1].protoPayload.resourceName =
      "projects/-/serviceAccounts/unknown@pymes-dev-352318.iam.gserviceaccount.com" |
    .[1].protoPayload.authorizationInfo[0].resource =
      "projects/-/serviceAccounts/unknown@pymes-dev-352318.iam.gserviceaccount.com"
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted denied actAs" \
  validate_seed_audit_fixture \
  "$(jq '.[1].protoPayload.authorizationInfo[0].granted = false' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted another actor on an allowlisted resource" \
  validate_seed_audit_fixture \
  "$(jq '
    .[0].protoPayload.authenticationInfo.principalEmail =
      "other@example.invalid"
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted actAs with a mismatched authorization resource" \
  validate_seed_audit_fixture \
  "$(jq '
    .[1].protoPayload.authorizationInfo[0].resource =
      "projects/-/serviceAccounts/unknown@pymes-dev-352318.iam.gserviceaccount.com"
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted actAs without a successful response" \
  validate_seed_audit_fixture \
  "$(jq '.[1].protoPayload.response.success = false' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted another IAM permission" \
  validate_seed_audit_fixture \
  "$(jq '.[1].protoPayload.authorizationInfo[0].permission = "iam.serviceAccounts.get"' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted another successful IAM method" \
  validate_seed_audit_fixture \
  "$(jq '.[1].protoPayload.methodName = "google.iam.admin.v1.SetIAMPolicy"' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted mismatched actAs request name" \
  validate_seed_audit_fixture \
  "$(jq '
    .[1].protoPayload.request.name =
      "unknown@pymes-dev-352318.iam.gserviceaccount.com"
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted mismatched actAs project number" \
  validate_seed_audit_fixture \
  "$(jq '
    .[1].protoPayload.request.project_number = "123"
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted Cloud Run SetIamPolicy" \
  validate_seed_audit_fixture \
  "$(jq '
    .[0].protoPayload.methodName =
      "google.cloud.run.v2.Services.SetIamPolicy"
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted a second mutation of the same resource" \
  validate_seed_audit_fixture \
  "$(jq '
    . + [(
      .[0] |
      .protoPayload.methodName =
        "google.cloud.run.v2.Services.UpdateService"
    )]
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted a mismatched Run request target" \
  validate_seed_audit_fixture \
  "$(jq '
    .[0].protoPayload.request.service.name =
      "projects/pymes-dev-352318/locations/us-central1/services/unrelated"
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted a different Cloud Run resource" \
  validate_seed_audit_fixture \
  "$(jq '
    .[0].protoPayload.resourceName =
      "projects/pymes-dev-352318/locations/us-central1/services/unrelated" |
    .[0].resource.labels.service_name = "unrelated"
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"
expect_failure "seed audit accepted the same service name in another region" \
  validate_seed_audit_fixture \
  "$(jq '
    .[0].protoPayload.resourceName =
      "projects/pymes-dev-352318/locations/europe-west1/services/pymes-v3-stg-api" |
    .[0].resource.labels.location = "europe-west1"
  ' <<<"$seed_audit_logs")" \
  "$allowed_seed_resources" "$allowed_seed_accounts"

service_json=$(jq -cn \
  --arg sha "$sha" \
  --arg image "$api_image" \
  --arg account "$api_account" '
    {
      metadata: {
        name: "pymes-v3-stg-api",
        labels: {
          app: "pymes-v3",
          env: "stg",
          "pymes-v3-seed": "true",
          "pymes-v3-release": $sha
        },
        annotations: {
          "run.googleapis.com/ingress": "internal",
          "run.googleapis.com/scalingMode": "manual",
          "run.googleapis.com/manualInstanceCount": "0"
        }
      },
      spec: {
        template: {
          metadata: {annotations: {"autoscaling.knative.dev/minScale": "0"}},
          spec: {
            serviceAccountName: $account,
            containers: [{image: $image, env: [], envFrom: []}]
          }
        }
      },
      status: {traffic: [{percent: 0}]}
    }
  ')
verify_seed_service_json \
  "$service_json" pymes-v3-stg-api stg "$sha" "$api_image" "$api_account"
expect_failure "service verifier accepted active traffic" \
  verify_seed_service_json \
  "$(jq '.status.traffic[0].percent = 100' <<<"$service_json")" \
  pymes-v3-stg-api stg "$sha" "$api_image" "$api_account"
expect_failure "service verifier accepted public ingress" \
  verify_seed_service_json \
  "$(jq '.metadata.annotations["run.googleapis.com/ingress"] = "all"' <<<"$service_json")" \
  pymes-v3-stg-api stg "$sha" "$api_image" "$api_account"
expect_failure "service verifier accepted an environment variable" \
  verify_seed_service_json \
  "$(jq '.spec.template.spec.containers[0].env = [{name:"SECRET",value:"x"}]' <<<"$service_json")" \
  pymes-v3-stg-api stg "$sha" "$api_image" "$api_account"
expect_failure "service verifier accepted the wrong image" \
  verify_seed_service_json \
  "$service_json" pymes-v3-stg-api stg "$sha" \
  "us-central1-docker.pkg.dev/pymes-dev-352318/pymes/other@sha256:${digest}" \
  "$api_account"
expect_failure "service verifier accepted the wrong runtime identity" \
  verify_seed_service_json \
  "$service_json" pymes-v3-stg-api stg "$sha" "$api_image" \
  "pymes-v3-worker-stg@pymes-dev-352318.iam.gserviceaccount.com"
expect_failure "service verifier accepted automatic scaling" \
  verify_seed_service_json \
  "$(jq 'del(.metadata.annotations["run.googleapis.com/scalingMode"])' <<<"$service_json")" \
  pymes-v3-stg-api stg "$sha" "$api_image" "$api_account"
expect_failure "service verifier accepted a Cloud SQL attachment" \
  verify_seed_service_json \
  "$(jq '.spec.template.metadata.annotations["run.googleapis.com/cloudsql-instances"] = "project:region:db"' <<<"$service_json")" \
  pymes-v3-stg-api stg "$sha" "$api_image" "$api_account"
expect_failure "service verifier accepted a Direct VPC attachment" \
  verify_seed_service_json \
  "$(jq '.spec.template.metadata.annotations["run.googleapis.com/network-interfaces"] = "[{\"network\":\"shared\"}]"' <<<"$service_json")" \
  pymes-v3-stg-api stg "$sha" "$api_image" "$api_account"
expect_failure "service verifier accepted a Serverless VPC connector" \
  verify_seed_service_json \
  "$(jq '.spec.template.metadata.annotations["run.googleapis.com/vpc-access-connector"] = "legacy-connector"' <<<"$service_json")" \
  pymes-v3-stg-api stg "$sha" "$api_image" "$api_account"
expect_failure "service verifier accepted a secret volume" \
  verify_seed_service_json \
  "$(jq '.spec.template.spec.volumes = [{name:"secret",secret:{secretName:"x"}}]' <<<"$service_json")" \
  pymes-v3-stg-api stg "$sha" "$api_image" "$api_account"

job_json=$(jq -cn \
  --arg sha "$sha" \
  --arg image "$migrate_image" \
  --arg account "$migrate_account" '
    {
      metadata: {
        name: "pymes-v3-stg-migrate",
        labels: {
          app: "pymes-v3",
          env: "stg",
          "pymes-v3-seed": "true",
          "pymes-v3-release": $sha
        }
      },
      spec: {
        template: {
          spec: {
            taskCount: 1,
            template: {
              spec: {
                maxRetries: 0,
                serviceAccountName: $account,
                containers: [{image: $image, env: []}]
              }
            }
          }
        }
      }
    }
  ')
verify_seed_job_json \
  "$job_json" pymes-v3-stg-migrate stg "$sha" "$migrate_image" "$migrate_account"
expect_failure "job verifier accepted an environment variable" \
  verify_seed_job_json \
  "$(jq '.spec.template.spec.template.spec.containers[0].env = [{name:"DATABASE_URL",value:"x"}]' <<<"$job_json")" \
  pymes-v3-stg-migrate stg "$sha" "$migrate_image" "$migrate_account"
expect_failure "job verifier accepted the wrong image" \
  verify_seed_job_json \
  "$job_json" pymes-v3-stg-migrate stg "$sha" \
  "us-central1-docker.pkg.dev/pymes-dev-352318/pymes/other@sha256:${digest}" \
  "$migrate_account"
expect_failure "job verifier accepted a retry" \
  verify_seed_job_json \
  "$(jq '.spec.template.spec.template.spec.maxRetries = 1' <<<"$job_json")" \
  pymes-v3-stg-migrate stg "$sha" "$migrate_image" "$migrate_account"
expect_failure "job verifier accepted a Cloud SQL attachment" \
  verify_seed_job_json \
  "$(jq '.spec.template.spec.template.metadata.annotations["run.googleapis.com/cloudsql-instances"] = "project:region:db"' <<<"$job_json")" \
  pymes-v3-stg-migrate stg "$sha" "$migrate_image" "$migrate_account"
expect_failure "job verifier accepted a Serverless VPC connector" \
  verify_seed_job_json \
  "$(jq '.spec.template.spec.template.metadata.annotations["run.googleapis.com/vpc-access-connector"] = "legacy-connector"' <<<"$job_json")" \
  pymes-v3-stg-migrate stg "$sha" "$migrate_image" "$migrate_account"

if rg -q -- '--execute-now|gcloud run jobs execute|--allow-unauthenticated|--no-allow-unauthenticated' \
  "$script_dir/seed-cloud-run-resources.sh"; then
  echo "FAIL: seed script contains a job execution or IAM mutation path" >&2
  exit 1
fi
for required in \
  '--scaling=0' \
  '--no-traffic' \
  '--ingress=internal' \
  '--no-deploy-health-check' \
  '--clear-secrets' \
  '--clear-cloudsql-instances' \
  '--clear-network' \
  '--clear-volumes' \
  '--clear-volume-mounts' \
  'gcloud run jobs executions list'; do
  rg -Fq -- "$required" "$script_dir/seed-cloud-run-resources.sh" || {
    echo "FAIL: seed script omits $required" >&2
    exit 1
  }
done

echo "Cloud Run inert seed policy tests passed"
