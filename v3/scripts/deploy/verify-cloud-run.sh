#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077

# Read-only post-deploy verifier. It proves that the exact release digests,
# identities, private topology and release marker reached Cloud Run.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=release-candidate-tag.sh
source "$script_dir/release-candidate-tag.sh"

: "${PYMES_DEPLOY_ENV:?set PYMES_DEPLOY_ENV to stg or prd}"
case "$PYMES_DEPLOY_ENV" in
  stg|prd) ;;
  *) echo "PYMES_DEPLOY_ENV must be stg or prd" >&2; exit 2 ;;
esac
deploy_stage=${PYMES_DEPLOY_STAGE:-operational}
case "$deploy_stage" in
  bootstrap|operational) ;;
  *) echo "PYMES_DEPLOY_STAGE must be bootstrap or operational" >&2; exit 2 ;;
esac
if [[ "$deploy_stage" == "bootstrap" && "$PYMES_DEPLOY_ENV" != "stg" ]]; then
  echo "PYMES_DEPLOY_STAGE=bootstrap is allowed only with PYMES_DEPLOY_ENV=stg" >&2
  exit 2
fi
if [[ -n "${PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN:-}" ]]; then
  echo "PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN is not accepted by the live verifier" >&2
  exit 2
fi

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
: "${PYMES_VPC_NETWORK:?set PYMES_VPC_NETWORK explicitly}"
: "${PYMES_VPC_SUBNET:?set PYMES_VPC_SUBNET explicitly}"
network=$PYMES_VPC_NETWORK
subnet=$PYMES_VPC_SUBNET
prefix="pymes-v3-${PYMES_DEPLOY_ENV}"

if [[ "$deploy_stage" == "bootstrap" ]]; then
  bootstrap_public_origin="https://pymes-v3-stg-bootstrap.invalid"
  PYMES_PUBLIC_BASE_URL=${PYMES_PUBLIC_BASE_URL:-$bootstrap_public_origin}
  PYMES_CLERK_AUTHORIZED_PARTIES=${PYMES_CLERK_AUTHORIZED_PARTIES:-$bootstrap_public_origin}
  if [[ "$PYMES_PUBLIC_BASE_URL" != "$bootstrap_public_origin" ||
        "$PYMES_CLERK_AUTHORIZED_PARTIES" != "$bootstrap_public_origin" ]]; then
    echo "bootstrap must use only the reserved fail-closed origin $bootstrap_public_origin" >&2
    exit 2
  fi
fi
: "${PYMES_RELEASE_SHA:?set PYMES_RELEASE_SHA}"
: "${PYMES_PUBLIC_BASE_URL:?set PYMES_PUBLIC_BASE_URL}"
: "${PYMES_CLOUDSQL_INSTANCE:?set PYMES_CLOUDSQL_INSTANCE}"
: "${PYMES_CLERK_ISSUER:?set PYMES_CLERK_ISSUER}"
: "${PYMES_CLERK_AUTHORIZED_PARTIES:?set PYMES_CLERK_AUTHORIZED_PARTIES}"
: "${PYMES_INTERNAL_KMS_KEY_VERSION:?set PYMES_INTERNAL_KMS_KEY_VERSION}"
: "${PYMES_INTERNAL_JWKS_JSON:?set PYMES_INTERNAL_JWKS_JSON}"
: "${PYMES_PREFLIGHT_TOKEN:?set PYMES_PREFLIGHT_TOKEN}"

if [[ ! "$PYMES_RELEASE_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "PYMES_RELEASE_SHA must be one full lowercase commit SHA" >&2
  exit 2
fi
if [[ ! "$PYMES_PREFLIGHT_TOKEN" =~ ^[0-9a-f]{64}$ ]]; then
  echo "PYMES_PREFLIGHT_TOKEN must be exactly 32 random bytes encoded as lowercase hexadecimal" >&2
  exit 2
fi
if [[ "$deploy_stage" == "bootstrap" ]]; then
  verify_phase=${PYMES_CLOUD_RUN_VERIFY_PHASE:-pretraffic}
else
  verify_phase=${PYMES_CLOUD_RUN_VERIFY_PHASE:-active}
fi
case "$verify_phase" in
  pretraffic|active) ;;
  *) echo "PYMES_CLOUD_RUN_VERIFY_PHASE must be pretraffic or active" >&2; exit 2 ;;
esac
if [[ "$deploy_stage" == "bootstrap" && "$verify_phase" != "pretraffic" ]]; then
  echo "bootstrap verification is restricted to PYMES_CLOUD_RUN_VERIFY_PHASE=pretraffic" >&2
  exit 2
fi
candidate_tag=$(pymes_release_candidate_tag "$PYMES_RELEASE_SHA")
pymes_validate_release_candidate_tag "$PYMES_RELEASE_SHA" "$candidate_tag"
if [[ -n "${PYMES_CLOUD_RUN_CANDIDATE_TAG:-}" &&
      "$PYMES_CLOUD_RUN_CANDIDATE_TAG" != "$candidate_tag" ]]; then
  echo "PYMES_CLOUD_RUN_CANDIDATE_TAG must equal $candidate_tag" >&2
  exit 2
fi
if [[ ! "$PYMES_PUBLIC_BASE_URL" =~ ^https://[A-Za-z0-9.-]+$ ]]; then
  echo "PYMES_PUBLIC_BASE_URL must be an HTTPS origin without path or trailing slash" >&2
  exit 2
fi
if ! jq -e 'type == "object" and .keys and (.keys | type == "array")' \
  <<<"$PYMES_INTERNAL_JWKS_JSON" >/dev/null; then
  echo "PYMES_INTERNAL_JWKS_JSON is not a JWKS object" >&2
  exit 2
fi

image_variables=(
  PYMES_API_IMAGE
  PYMES_WEB_IMAGE
  PYMES_WORKER_IMAGE
  PYMES_FISCAL_IMAGE
  PYMES_ACCOUNTING_IMAGE
  PYMES_ACCOUNTING_ADMIN_IMAGE
  PYMES_PROVISION_IMAGE
  PYMES_MIGRATE_IMAGE
  PYMES_FISCAL_MIGRATE_IMAGE
  PYMES_ACCOUNTING_MIGRATE_IMAGE
)
image_pattern="^${region}-docker[.]pkg[.]dev/${project}/[a-z][a-z0-9-]*/[a-z][a-z0-9-]*@sha256:[0-9a-f]{64}$"
for variable in "${image_variables[@]}"; do
  : "${!variable:?set $variable from the validated release manifest}"
  if [[ ! "${!variable}" =~ $image_pattern ]]; then
    echo "$variable must be a digest-pinned image in the selected project and region" >&2
    exit 2
  fi
done

fail() {
  echo "Cloud Run verification failed: $*" >&2
  exit 1
}

verify_clerk_webhook_secret_lifecycle() {
  local secret="$prefix-clerk-webhook-secret" lifecycle
  lifecycle=$(gcloud secrets describe "$secret" \
    --project="$project" --format='value(labels.lifecycle)')
  if [[ "$deploy_stage" == "bootstrap" ]]; then
    [[ "$lifecycle" == "bootstrap-temporary" ]] ||
      fail "$secret must have label lifecycle=bootstrap-temporary during bootstrap"
  elif [[ "$lifecycle" == "bootstrap-temporary" ]]; then
    fail "$secret still has lifecycle=bootstrap-temporary; operational verification is forbidden"
  fi
  echo "SECURITY CLERK_WEBHOOK_SECRET secret=$secret lifecycle=${lifecycle:-unset} stage=$deploy_stage source=secret-manager"
}

basename_resource() {
  local value="$1"
  printf '%s' "${value##*/}"
}

service_json() {
  gcloud run services describe "$1" \
    --project="$project" --region="$region" --format=json
}

service_candidate_url() {
  local service="$1" candidate_url
  candidate_url=$(service_json "$service" |
    jq -er --arg tag "$candidate_tag" '
      [
        (.status.traffic // .trafficStatuses // [])[]
        | select(.tag == $tag)
        | .url
      ] | select(length == 1) | .[0]
    ')
  pymes_validate_cloud_run_tagged_url \
    "$candidate_url" "$candidate_tag" "$service" ||
    fail "$service candidate URL is not a canonical, DNS-safe Cloud Run tagged origin"
  printf '%s' "$candidate_url"
}

job_json() {
  gcloud run jobs describe "$1" \
    --project="$project" --region="$region" --format=json
}

service_image() {
  jq -er '.spec.template.spec.containers[0].image // .template.containers[0].image'
}

service_account() {
  jq -er '.spec.template.spec.serviceAccountName // .template.serviceAccount'
}

service_ingress() {
  jq -r '.metadata.annotations["run.googleapis.com/ingress"] // .ingress // empty'
}

service_release_marker() {
  jq -r '
    .spec.template.metadata.labels["pymes-v3-release"] //
    .template.labels["pymes-v3-release"] //
    .metadata.labels["pymes-v3-release"] //
    empty
  '
}

service_cloudsql() {
  jq -r '.spec.template.metadata.annotations["run.googleapis.com/cloudsql-instances"] // .template.annotations["run.googleapis.com/cloudsql-instances"] // empty'
}

service_env_value() {
  local name="$1"
  jq -er --arg name "$name" '
    [
      (.spec.template.spec.containers[0].env // .template.containers[0].env // [])[]
      | select(.name == $name)
      | .value
    ] | select(length == 1) | .[0]
  '
}

verify_service_env_value() {
  local service="$1" json="$2" name="$3" expected="$4"
  [[ "$(service_env_value "$name" <<<"$json")" == "$expected" ]] ||
    fail "$service $name differs from the exact allowlisted value"
}

job_image() {
  jq -er '
    .spec.template.spec.template.spec.containers[0].image //
    .template.template.containers[0].image
  '
}

job_account() {
  jq -er '
    .spec.template.spec.template.spec.serviceAccountName //
    .template.template.serviceAccount
  '
}

job_release_marker() {
  jq -r '
    .spec.template.spec.template.metadata.labels["pymes-v3-release"] //
    .template.template.labels["pymes-v3-release"] //
    .metadata.labels["pymes-v3-release"] //
    empty
  '
}

job_cloudsql() {
  jq -r '
    .spec.template.spec.template.metadata.annotations["run.googleapis.com/cloudsql-instances"] //
    .template.template.annotations["run.googleapis.com/cloudsql-instances"] //
    empty
  '
}

job_env_value() {
  local name="$1"
  jq -er --arg name "$name" '
    [
      (
        .spec.template.spec.template.spec.containers[0].env //
        .template.template.containers[0].env //
        []
      )[]
      | select(.name == $name)
      | .value
    ] | select(length == 1) | .[0]
  '
}

verify_job_env_value() {
  local job="$1" json="$2" name="$3" expected="$4"
  [[ "$(job_env_value "$name" <<<"$json")" == "$expected" ]] ||
    fail "$job $name differs from the exact allowlisted value"
}

secret_refs() {
  local kind="$1"
  if [[ "$kind" == "service" ]]; then
    jq -r '
      (
        .spec.template.spec.containers[0].env //
        .template.containers[0].env //
        []
      )[]
      | select(.valueFrom.secretKeyRef or .valueSource.secretKeyRef)
      | [
          .name,
          (
            .valueFrom.secretKeyRef.name //
            .valueSource.secretKeyRef.secret //
            ""
          ),
          (
            .valueFrom.secretKeyRef.key //
            .valueSource.secretKeyRef.version //
            ""
          )
        ]
      | @tsv
    '
    return
  fi
  jq -r '
    (
      .spec.template.spec.template.spec.containers[0].env //
      .template.template.containers[0].env //
      []
    )[]
    | select(.valueFrom.secretKeyRef or .valueSource.secretKeyRef)
    | [
        .name,
        (
          .valueFrom.secretKeyRef.name //
          .valueSource.secretKeyRef.secret //
          ""
        ),
        (
          .valueFrom.secretKeyRef.key //
          .valueSource.secretKeyRef.version //
          ""
        )
      ]
    | @tsv
  '
}

verify_secret_refs() {
  local kind="$1" name="$2" json="$3"
  shift 3
  local actual expected entry env_name secret_name version actual_count
  actual=$(secret_refs "$kind" <<<"$json" | LC_ALL=C sort)
  if [[ $# -eq 0 ]]; then
    [[ -z "$actual" ]] || fail "$kind $name unexpectedly references secrets"
    return
  fi
  expected=$(printf '%s\n' "$@" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort)
  actual_count=$(grep -c . <<<"$actual" || true)
  if [[ "$actual_count" -ne $# ]]; then
    fail "$kind $name has an unexpected number of secret references"
  fi
  while IFS='=' read -r env_name secret_name; do
    [[ -n "$env_name" && -n "$secret_name" ]] ||
      fail "$kind $name secret allowlist is malformed"
    entry=$(awk -F '\t' \
      -v env_name="$env_name" \
      -v secret_name="$secret_name" \
      -v project="$project" '
      $1 == env_name &&
      ($2 == secret_name || $2 == "projects/" project "/secrets/" secret_name) {
        print
        count++
      }
      END { if (count != 1) exit 1 }
    ' <<<"$actual") ||
      fail "$kind $name does not bind $env_name to $secret_name exactly once"
    version=$(cut -f3 <<<"$entry")
    [[ "$version" =~ ^[1-9][0-9]*$ ]] ||
      fail "$kind $name binds $env_name to an unpinned secret version"
  done <<<"$expected"
}

resource_containers() {
  local kind="$1"
  if [[ "$kind" == "service" ]]; then
    jq -c '.spec.template.spec.containers // .template.containers // []'
    return
  fi
  jq -c '
    .spec.template.spec.template.spec.containers //
    .template.template.containers //
    []
  '
}

verify_container_shape() {
  local kind="$1" name="$2" json="$3" containers
  containers=$(resource_containers "$kind" <<<"$json")
  jq -e '
    length == 1 and
    ((.[0].command // []) | length) == 0 and
    ((.[0].args // []) | length) == 0 and
    ((.[0].volumeMounts // []) | length) == 0 and
    ((.[0].dependsOn // []) | length) == 0
  ' <<<"$containers" >/dev/null ||
    fail "$kind $name must have exactly one container and no command, args, sidecar dependency or volume mount overrides"
  if [[ "$kind" == "service" ]]; then
    jq -e '
      (
        .spec.template.spec.volumes //
        .template.volumes //
        []
      ) | length == 0
    ' <<<"$json" >/dev/null ||
      fail "$kind $name must not define volumes"
  else
    jq -e '
      (
        .spec.template.spec.template.spec.volumes //
        .template.template.volumes //
        []
      ) | length == 0
    ' <<<"$json" >/dev/null ||
      fail "$kind $name must not define volumes"
  fi
}

resource_env_names() {
  local kind="$1"
  resource_containers "$kind" |
    jq -r '.[0].env // [] | .[].name'
}

verify_env_allowlist() {
  local kind="$1" name="$2" json="$3"
  shift 3
  local actual expected actual_count unique_count
  actual=$(resource_env_names "$kind" <<<"$json" | LC_ALL=C sort)
  expected=$(printf '%s\n' "$@" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort)
  actual_count=$(resource_env_names "$kind" <<<"$json" | wc -l)
  unique_count=$(resource_env_names "$kind" <<<"$json" | LC_ALL=C sort -u | wc -l)
  [[ "$actual_count" -eq "$unique_count" ]] ||
    fail "$kind $name contains duplicate environment variable names"
  [[ "$actual" == "$expected" ]] ||
    fail "$kind $name environment variable names differ from the exact allowlist"
}

verify_direct_vpc() {
  local kind="$1" name="$2" json="$3" interfaces egress actual_network actual_subnet
  interfaces=$(jq -r '
    .spec.template.metadata.annotations["run.googleapis.com/network-interfaces"] //
    .spec.template.spec.template.metadata.annotations["run.googleapis.com/network-interfaces"] //
    .template.annotations["run.googleapis.com/network-interfaces"] //
    .template.template.annotations["run.googleapis.com/network-interfaces"] //
    empty
  ' <<<"$json")
  egress=$(jq -r '
    .spec.template.metadata.annotations["run.googleapis.com/vpc-access-egress"] //
    .spec.template.spec.template.metadata.annotations["run.googleapis.com/vpc-access-egress"] //
    .template.annotations["run.googleapis.com/vpc-access-egress"] //
    .template.template.annotations["run.googleapis.com/vpc-access-egress"] //
    .template.vpcAccess.egress //
    empty
  ' <<<"$json")
  if [[ -n "$interfaces" ]]; then
    actual_network=$(jq -er 'fromjson | select(length == 1) | .[0].network' <<<"$interfaces")
    actual_subnet=$(jq -er 'fromjson | select(length == 1) | .[0].subnetwork' <<<"$interfaces")
  else
    actual_network=$(jq -er '
      .template.vpcAccess.networkInterfaces
      | select(length == 1) | .[0].network
    ' <<<"$json")
    actual_subnet=$(jq -er '
      .template.vpcAccess.networkInterfaces
      | select(length == 1) | .[0].subnetwork
    ' <<<"$json")
  fi
  [[ "$(basename_resource "$actual_network")" == "$network" ]] ||
    fail "$kind $name uses network $actual_network instead of $network"
  [[ "$(basename_resource "$actual_subnet")" == "$subnet" ]] ||
    fail "$kind $name uses subnet $actual_subnet instead of $subnet"
  case "${egress^^}" in
    ALL-TRAFFIC|ALL_TRAFFIC) ;;
    *) fail "$kind $name does not route all egress through the Direct VPC interface" ;;
  esac
}

verify_no_vpc() {
  local kind="$1" name="$2" json="$3"
  if jq -e '
    (
      .spec.template.metadata.annotations["run.googleapis.com/network-interfaces"] //
      .spec.template.spec.template.metadata.annotations["run.googleapis.com/network-interfaces"] //
      .template.annotations["run.googleapis.com/network-interfaces"] //
      .template.template.annotations["run.googleapis.com/network-interfaces"] //
      .template.vpcAccess.networkInterfaces //
      []
    ) | length > 0
  ' <<<"$json" >/dev/null; then
    fail "$kind $name unexpectedly has Direct VPC configuration"
  fi
}

normalize_members() {
  sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u
}

verify_internal_signing_kms() {
  local key=internal-jwt-signing key_json version version_json role
  local inherited_project inherited_keyring actual expected
  local version_pattern deploy_sa
  local -a versions=("$PYMES_INTERNAL_KMS_KEY_VERSION")
  version_pattern="^projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/${key}/cryptoKeyVersions/[1-9][0-9]*$"
  [[ "$PYMES_INTERNAL_KMS_KEY_VERSION" =~ $version_pattern ]] ||
    fail "internal KMS signing version is outside the selected environment key"
  IFS=',' read -r -a overlap_versions <<<"${PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS:-}"
  for version in "${overlap_versions[@]}"; do
    [[ -z "$version" ]] && continue
    [[ "$version" =~ $version_pattern ]] ||
      fail "overlap KMS signing version is outside the selected environment key"
    versions+=("$version")
  done
  [[ "$(printf '%s\n' "${versions[@]}" | LC_ALL=C sort -u | wc -l)" -eq "${#versions[@]}" ]] ||
    fail "internal KMS signing versions contain duplicates"
  key_json=$(gcloud kms keys describe "$key" \
    --project="$project" --location="$region" --keyring="$prefix" \
    --format=json)
  jq -e '
    .purpose == "ASYMMETRIC_SIGN" and
    .versionTemplate.algorithm == "EC_SIGN_ED25519"
  ' <<<"$key_json" >/dev/null ||
    fail "internal KMS key is not an Ed25519 asymmetric signing key"
  for version in "${versions[@]}"; do
    version_json=$(gcloud kms keys versions describe "${version##*/}" \
      --project="$project" --location="$region" --keyring="$prefix" \
      --key="$key" --format=json)
    jq -e '.state == "ENABLED" and .algorithm == "EC_SIGN_ED25519"' \
      <<<"$version_json" >/dev/null ||
      fail "selected internal KMS version is not enabled Ed25519"
  done
  deploy_sa="pymes-v3-gh-deploy-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
  for role in roles/cloudkms.signer roles/cloudkms.publicKeyViewer; do
    inherited_project=$(gcloud projects get-iam-policy "$project" \
      --flatten='bindings[].members' --filter="bindings.role=$role" \
      --format='value(bindings.members)' | normalize_members)
    inherited_keyring=$(gcloud kms keyrings get-iam-policy "$prefix" \
      --project="$project" --location="$region" \
      --flatten='bindings[].members' --filter="bindings.role=$role" \
      --format='value(bindings.members)' | normalize_members)
    [[ -z "$inherited_project" && -z "$inherited_keyring" ]] ||
      fail "$role is inherited by the internal signing key"
    actual=$(gcloud kms keys get-iam-policy "$key" \
      --project="$project" --location="$region" --keyring="$prefix" \
      --flatten='bindings[].members' --filter="bindings.role=$role" \
      --format='value(bindings.members)' | normalize_members)
    if [[ "$role" == "roles/cloudkms.signer" ]]; then
      expected=$(printf '%s\n' \
        "serviceAccount:$api_sa" \
        "serviceAccount:$worker_sa" \
        "serviceAccount:$provision_sa" | normalize_members)
    else
      expected=$(printf '%s\n' \
        "serviceAccount:$api_sa" \
        "serviceAccount:$worker_sa" \
        "serviceAccount:$provision_sa" \
        "serviceAccount:$deploy_sa" | normalize_members)
    fi
    [[ "$actual" == "$expected" ]] ||
      fail "internal signing key $role principals differ from the exact allowlist"
  done
}

verify_invokers() {
  local service="$1"
  shift
  local actual expected
  actual=$(gcloud run services get-iam-policy "$service" \
    --project="$project" --region="$region" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/run.invoker' \
    --format='value(bindings.members)' | normalize_members)
  expected=$(printf '%s\n' "$@" | normalize_members)
  [[ "$actual" == "$expected" ]] ||
    fail "$service roles/run.invoker differs; expected [${expected:-none}], got [${actual:-none}]"
}

verify_service() {
  local service="$1" expected_image="$2" expected_account="$3" expected_ingress="$4"
  local vpc="$5" expected_min="$6" expected_max="$7" expected_cpu_throttled="$8"
  local expected_scaling="${9:-auto}"
  local json service_min service_max revision_min cpu_throttled
  local scaling_mode manual_count
  json=$(service_json "$service")
  jq -e '
    (
      .metadata.annotations["run.googleapis.com/invoker-iam-disabled"] //
      .invokerIamDisabled //
      false
    ) as $disabled
    | (($disabled | tostring | ascii_downcase) != "true")
  ' <<<"$json" >/dev/null ||
    fail "$service has the Cloud Run invoker IAM check disabled"
  verify_container_shape service "$service" "$json"
  [[ "$(service_image <<<"$json")" == "$expected_image" ]] ||
    fail "$service does not run the exact release digest"
  [[ "$(service_account <<<"$json")" == "$expected_account" ]] ||
    fail "$service uses the wrong runtime service account"
  [[ "$(service_ingress <<<"$json")" == "$expected_ingress" ]] ||
    fail "$service ingress is not $expected_ingress"
  [[ "$(service_release_marker <<<"$json")" == "$PYMES_RELEASE_SHA" ]] ||
    fail "$service current revision has the wrong release marker"
  jq -e '
    (
      (.status.latestReadyRevisionName // .latestReadyRevision // "") as $ready
      | (.status.latestCreatedRevisionName // .latestCreatedRevision // "") as $created
      | ($ready | length) > 0 and
        $ready == $created
    ) and
    (
      .status.conditions //
      .conditions //
      []
      | any(
          (.type == "Ready" or .type == "READY") and
          (.status == "True" or .state == "CONDITION_SUCCEEDED")
        )
    )
  ' <<<"$json" >/dev/null ||
    fail "$service latest created revision is not ready"
  if [[ "$verify_phase" == "pretraffic" ]]; then
    jq -e --arg tag "$candidate_tag" '
      (.status.latestReadyRevisionName // .latestReadyRevision // "") as $ready
      | (.status.traffic // .trafficStatuses // []) as $traffic
      | [$traffic[] | select(.tag == $tag)] as $tagged
      | [$traffic[] | select((.percent // 0) > 0)] as $active
      | ($tagged | length) == 1 and
        ($tagged[0].revisionName // $tagged[0].revision // "") == $ready and
        ($tagged[0].percent // 0) == 0 and
        all($active[]; (.revisionName // .revision // "") != $ready) and
        (
          ($active | length) == 0 or
          (
            ([$active[].percent] | add) == 100 and
            ($active | length) == 1
          )
        )
    ' <<<"$json" >/dev/null ||
      fail "$service candidate revision is not tagged, ready and isolated at zero traffic"
    if [[ "$deploy_stage" == "bootstrap" ]]; then
      jq -e '
        [
          (.status.traffic // .trafficStatuses // [])[]
          | select((.percent // 0) > 0)
        ] | length == 0
      ' <<<"$json" >/dev/null ||
        fail "bootstrap refuses $service because it already has active traffic"
    fi
  else
    jq -e \
      --arg tag "$candidate_tag" \
      --arg service "$service" \
      --arg api_service "$prefix-api" '
      (.status.latestReadyRevisionName // .latestReadyRevision // "") as $ready
      | (.status.traffic // .trafficStatuses // []) as $traffic
      | [
          $traffic[]
          | select(((.tag // "") | length) > 0)
        ] as $tagged
      | [$traffic[] | select((.percent // 0) > 0)] as $active
      | (
          if $service == $api_service then
            ($tagged | length) == 1 and
            $tagged[0].tag == $tag and
            ($tagged[0].revisionName // $tagged[0].revision // "") == $ready
          else
            ($tagged | length) == 0
          end
        ) and
        ($active | length) == 1 and
        ($active[0].percent // 0) == 100 and
        ($active[0].revisionName // $active[0].revision // "") == $ready
    ' <<<"$json" >/dev/null ||
      fail "$service latest ready candidate must receive 100 percent with the exact settled tag policy"
  fi
  service_min=$(jq -r '
    .metadata.annotations["run.googleapis.com/minScale"] //
    .scaling.minInstanceCount //
    "0"
  ' <<<"$json")
  service_max=$(jq -r '
    .metadata.annotations["run.googleapis.com/maxScale"] //
    .scaling.maxInstanceCount //
    empty
  ' <<<"$json")
  revision_min=$(jq -r '
    .spec.template.metadata.annotations["autoscaling.knative.dev/minScale"] //
    .template.scaling.minInstanceCount //
    "0"
  ' <<<"$json")
  cpu_throttled=$(jq -r '
    .spec.template.metadata.annotations["run.googleapis.com/cpu-throttling"] //
    .template.containers[0].resources.cpuIdle //
    empty
  ' <<<"$json")
  scaling_mode=$(jq -r '
    .metadata.annotations["run.googleapis.com/scalingMode"] //
    .scaling.scalingMode //
    "automatic"
  ' <<<"$json")
  manual_count=$(jq -r '
    .metadata.annotations["run.googleapis.com/manualInstanceCount"] //
    .scaling.manualInstanceCount //
    empty
  ' <<<"$json")
  if [[ "$expected_scaling" == "manual" ]]; then
    [[ "${scaling_mode,,}" == "manual" && "$manual_count" == "$expected_min" ]] ||
      fail "$service manual scaling differs from $expected_min"
  else
    [[ "${scaling_mode,,}" != "manual" ]] ||
      fail "$service must use automatic service scaling"
    [[ "$service_min" == "$expected_min" && "$service_max" == "$expected_max" ]] ||
      fail "$service service min/max scale differs from ${expected_min}/${expected_max}"
  fi
  [[ "$revision_min" == "0" ]] ||
    fail "$service candidate revision min scale must remain zero"
  [[ "${cpu_throttled,,}" == "$expected_cpu_throttled" ]] ||
    fail "$service CPU throttling differs from $expected_cpu_throttled"
  if [[ "$service" == "$prefix-web" ]]; then
    [[ -z "$(service_cloudsql <<<"$json")" ]] ||
      fail "$service must not have Cloud SQL attached"
  else
    [[ "$(service_cloudsql <<<"$json")" == "$PYMES_CLOUDSQL_INSTANCE" ]] ||
      fail "$service Cloud SQL attachment differs"
  fi
  if [[ "$vpc" == "direct" ]]; then
    verify_direct_vpc service "$service" "$json"
  else
    verify_no_vpc service "$service" "$json"
  fi
}

api_sa="pymes-v3-api-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
web_sa="pymes-v3-web-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
worker_sa="pymes-v3-worker-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
provision_sa="pymes-v3-provision-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
fiscal_sa="pymes-v3-fiscal-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
accounting_sa="pymes-v3-accounting-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
accounting_admin_sa="pymes-v3-accounting-admin-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
migrate_sa="pymes-v3-migrate-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
fiscal_migrate_sa="pymes-v3-fiscal-migrate-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
accounting_migrate_sa="pymes-v3-acct-migrate-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"

verify_clerk_webhook_secret_lifecycle
verify_internal_signing_kms

if [[ "$deploy_stage" == "bootstrap" ]]; then
  verify_service "$prefix-api" "$PYMES_API_IMAGE" "$api_sa" internal direct 0 1 true
  verify_service "$prefix-web" "$PYMES_WEB_IMAGE" "$web_sa" internal none 0 1 true
  verify_service "$prefix-worker" "$PYMES_WORKER_IMAGE" "$worker_sa" internal direct 0 1 false manual
else
  verify_service "$prefix-api" "$PYMES_API_IMAGE" "$api_sa" all direct 0 1 true
  verify_service "$prefix-web" "$PYMES_WEB_IMAGE" "$web_sa" all none 0 1 true
  worker_instances=0
  if [[ "$verify_phase" == "active" ]]; then
    worker_instances=1
  fi
  verify_service "$prefix-worker" "$PYMES_WORKER_IMAGE" "$worker_sa" internal direct "$worker_instances" 1 false manual
fi
verify_service "$prefix-fiscal" "$PYMES_FISCAL_IMAGE" "$fiscal_sa" internal direct 0 1 true
verify_service "$prefix-accounting" "$PYMES_ACCOUNTING_IMAGE" "$accounting_sa" internal none 0 1 true
verify_service "$prefix-accounting-admin" "$PYMES_ACCOUNTING_ADMIN_IMAGE" "$accounting_admin_sa" internal none 0 1 true

if [[ "$deploy_stage" == "bootstrap" ]]; then
  verify_invokers "$prefix-api"
  verify_invokers "$prefix-web"
else
  verify_invokers "$prefix-api" allUsers
  verify_invokers "$prefix-web" allUsers
fi
verify_invokers "$prefix-worker"
verify_invokers "$prefix-fiscal" "serviceAccount:$api_sa" "serviceAccount:$worker_sa"
verify_invokers "$prefix-accounting" "serviceAccount:$worker_sa"
verify_invokers "$prefix-accounting-admin" "serviceAccount:$provision_sa"

project_invokers=$(gcloud projects get-iam-policy "$project" \
  --flatten='bindings[].members' \
  --filter='bindings.role=roles/run.invoker' \
  --format='value(bindings.members)' | normalize_members)
[[ -z "$project_invokers" ]] ||
  fail "shared project grants roles/run.invoker at project scope"

fiscal_url=$(gcloud run services describe "$prefix-fiscal" --project="$project" --region="$region" --format='value(status.url)')
accounting_url=$(gcloud run services describe "$prefix-accounting" --project="$project" --region="$region" --format='value(status.url)')
accounting_admin_url=$(gcloud run services describe "$prefix-accounting-admin" --project="$project" --region="$region" --format='value(status.url)')
web_url=$(gcloud run services describe "$prefix-web" --project="$project" --region="$region" --format='value(status.url)')

worker_json=$(service_json "$prefix-worker")
api_json=$(service_json "$prefix-api")
for tuple in \
  "worker:FISCAL_ADAPTER_URL:${fiscal_url}" \
  "worker:ACCOUNTING_URL:${accounting_url}" \
  "api:FISCAL_ADAPTER_URL:${fiscal_url}" \
  "api:PYMES_INTERNAL_KMS_KEY_VERSION:${PYMES_INTERNAL_KMS_KEY_VERSION}" \
  "worker:PYMES_INTERNAL_KMS_KEY_VERSION:${PYMES_INTERNAL_KMS_KEY_VERSION}" \
  "api:PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS:${PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS:-}" \
  "worker:PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS:${PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS:-}"; do
  IFS=: read -r owner key expected <<<"$tuple"
  if [[ "$owner" == "worker" ]]; then
    actual=$(service_env_value "$key" <<<"$worker_json")
  else
    actual=$(service_env_value "$key" <<<"$api_json")
  fi
  [[ "$actual" == "$expected" ]] || fail "$owner $key differs from the expected value"
done
web_json=$(service_json "$prefix-web")
api_candidate_url=$(service_candidate_url "$prefix-api")
[[ "$(service_env_value PYMES_API_UPSTREAM <<<"$web_json")" == "$api_candidate_url" ]] ||
  fail "$prefix-web does not proxy to the exact tagged API candidate URL"
for owner in api web; do
  if [[ "$owner" == "api" ]]; then
    json=$api_json
  else
    json=$web_json
  fi
  [[ "$(service_env_value PYMES_PREFLIGHT_TAG <<<"$json")" == "$candidate_tag" ]] ||
    fail "$owner preflight tag differs from the exact release tag"
  [[ "$(service_env_value PYMES_PREFLIGHT_TOKEN <<<"$json")" == "$PYMES_PREFLIGHT_TOKEN" ]] ||
    fail "$owner preflight capability differs from the masked release capability"
done
web_release_marker="${PYMES_DEPLOY_ENV}:${PYMES_RELEASE_SHA}:${PYMES_WEB_IMAGE##*@}"
[[ "$(service_env_value PYMES_RELEASE_MARKER <<<"$web_json")" == "$web_release_marker" ]] ||
  fail "$prefix-web runtime release marker differs from its environment, source SHA and image digest"

actual_authorized_parties=$(service_env_value PYMES_CLERK_AUTHORIZED_PARTIES <<<"$api_json")
[[ "$actual_authorized_parties" == "$PYMES_CLERK_AUTHORIZED_PARTIES" ]] ||
  fail "$prefix-api Clerk authorized parties differ"
authorized_public_origin=false
IFS=',' read -r -a parties <<<"$actual_authorized_parties"
for party in "${parties[@]}"; do
  party=${party#"${party%%[![:space:]]*}"}
  party=${party%"${party##*[![:space:]]}"}
  [[ "$party" == "$PYMES_PUBLIC_BASE_URL" ]] && authorized_public_origin=true
done
[[ "$authorized_public_origin" == "true" ]] ||
  fail "$prefix-api does not authorize the exact public origin"

fiscal_mode=${PYMES_FISCAL_MODE:-mock}
case "$fiscal_mode" in
  mock|arca) ;;
  *) fail "PYMES_FISCAL_MODE must be mock or arca" ;;
esac
fiscal_json=$(service_json "$prefix-fiscal")
[[ "$(service_env_value FISCAL_ADAPTER_MODE <<<"$fiscal_json")" == "$fiscal_mode" ]] ||
  fail "$prefix-fiscal mode differs"
expected_fiscal_kms="projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/fiscal-vault"
[[ "$(service_env_value FISCAL_KMS_KEY_NAME <<<"$fiscal_json")" == "$expected_fiscal_kms" ]] ||
  fail "$prefix-fiscal vault KMS key differs"
if [[ "$fiscal_mode" == "arca" ]]; then
  : "${PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN:?set homologation issuer pattern}"
  : "${PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN:?set production issuer pattern}"
  [[ "$(service_env_value FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN <<<"$fiscal_json")" == "$PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN" ]] ||
    fail "$prefix-fiscal homologation issuer policy differs"
  [[ "$(service_env_value FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN <<<"$fiscal_json")" == "$PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN" ]] ||
    fail "$prefix-fiscal production issuer policy differs"
fi

pergo_enabled=${PYMES_PERGO_ENABLED:-false}
google_enabled=${PYMES_GOOGLE_CALENDAR_ENABLED:-false}
for boolean in "$pergo_enabled" "$google_enabled"; do
  case "$boolean" in true|false) ;; *) fail "feature flags must be true or false" ;; esac
done
if [[ "$deploy_stage" == "bootstrap" ]]; then
  [[ "$fiscal_mode" == "mock" ]] ||
    fail "bootstrap requires PYMES_FISCAL_MODE=mock"
  [[ "$pergo_enabled" == "false" ]] ||
    fail "bootstrap requires PYMES_PERGO_ENABLED=false"
  [[ "$google_enabled" == "false" ]] ||
    fail "bootstrap requires PYMES_GOOGLE_CALENDAR_ENABLED=false"
fi
for json in "$api_json" "$worker_json"; do
  [[ "$(service_env_value PYMES_PERGO_ENABLED <<<"$json")" == "$pergo_enabled" ]] ||
    fail "PerGo feature flag differs between desired and deployed config"
  [[ "$(service_env_value PYMES_GOOGLE_CALENDAR_ENABLED <<<"$json")" == "$google_enabled" ]] ||
    fail "Google Calendar feature flag differs between desired and deployed config"
done
if [[ "$pergo_enabled" == "true" ]]; then
  : "${PYMES_PERGO_URL:?set PYMES_PERGO_URL}"
  : "${PYMES_PERGO_AUDIENCE:?set PYMES_PERGO_AUDIENCE}"
  : "${PYMES_PERGO_WORKSPACE_ID:?set PYMES_PERGO_WORKSPACE_ID}"
  if [[ ! "$PYMES_PERGO_WORKSPACE_ID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$ ]]; then
    fail "PYMES_PERGO_WORKSPACE_ID must be one canonical lowercase UUIDv4"
  fi
  if [[ "$PYMES_PERGO_URL" != https://* ||
        "$PYMES_PERGO_URL" == *$'\r'* ||
        "$PYMES_PERGO_URL" == *$'\n'* ||
        "$PYMES_PERGO_URL" == *[[:space:]]* ||
        "$PYMES_PERGO_URL" == *"|"* ||
        "$PYMES_PERGO_URL" == *","* ||
        "$PYMES_PERGO_URL" == *"@"* ||
        "$PYMES_PERGO_URL" == *"?"* ||
        "$PYMES_PERGO_URL" == *"#"* ]]; then
    fail "PYMES_PERGO_URL is not an explicit safe HTTPS URL"
  fi
  pergo_audience_pattern='^https://[^/@|?#]+$'
  if [[ ! "$PYMES_PERGO_AUDIENCE" =~ $pergo_audience_pattern ||
        "$PYMES_PERGO_AUDIENCE" == *$'\r'* ||
        "$PYMES_PERGO_AUDIENCE" == *$'\n'* ||
        "$PYMES_PERGO_AUDIENCE" == *[[:space:]]* ||
        "$PYMES_PERGO_AUDIENCE" == *"|"* ||
        "$PYMES_PERGO_AUDIENCE" == *","* ||
        "$PYMES_PERGO_AUDIENCE" == *"@"* ||
        "$PYMES_PERGO_AUDIENCE" == *"?"* ||
        "$PYMES_PERGO_AUDIENCE" == *"#"* ]]; then
    fail "PYMES_PERGO_AUDIENCE is not an exact safe HTTPS origin without path"
  fi
  [[ "$(service_env_value PERGO_URL <<<"$worker_json")" == "$PYMES_PERGO_URL" ]] ||
    fail "$prefix-worker PerGo URL differs"
  [[ "$(service_env_value PYMES_PERGO_AUDIENCE <<<"$worker_json")" == "$PYMES_PERGO_AUDIENCE" ]] ||
    fail "$prefix-worker PerGo audience differs"
  [[ "$(service_env_value PERGO_WORKSPACE_ID <<<"$worker_json")" == "$PYMES_PERGO_WORKSPACE_ID" ]] ||
    fail "$prefix-worker PerGo workspace differs"
  [[ "$(service_env_value PERGO_CHANNEL <<<"$worker_json")" == "${PYMES_PERGO_CHANNEL:-whatsapp}" ]] ||
    fail "$prefix-worker PerGo channel differs"
  [[ "$(service_env_value PERGO_ALLOW_GLOBAL_ROUTE_FALLBACK <<<"$worker_json")" == "false" ]] ||
    fail "$prefix-worker must disable the global PerGo routing fallback"
fi
if [[ "$google_enabled" == "true" ]]; then
  : "${PYMES_GOOGLE_CLIENT_ID:?set PYMES_GOOGLE_CLIENT_ID}"
  : "${PYMES_GOOGLE_REDIRECT_URL:?set PYMES_GOOGLE_REDIRECT_URL}"
  : "${PYMES_CALENDAR_KMS_KEY:?set PYMES_CALENDAR_KMS_KEY}"
  [[ "$PYMES_GOOGLE_REDIRECT_URL" == "${PYMES_PUBLIC_BASE_URL}/api/v1/calendars/google/oauth/callback" ]] ||
    fail "Google callback is not on the exact public origin"
  for json in "$api_json" "$worker_json"; do
    [[ "$(service_env_value PYMES_GOOGLE_CLIENT_ID <<<"$json")" == "$PYMES_GOOGLE_CLIENT_ID" ]] ||
      fail "Google client ID differs between desired and deployed config"
    [[ "$(service_env_value PYMES_GOOGLE_REDIRECT_URL <<<"$json")" == "$PYMES_GOOGLE_REDIRECT_URL" ]] ||
      fail "Google redirect differs between desired and deployed config"
    [[ "$(service_env_value PYMES_CALENDAR_KMS_KEY <<<"$json")" == "$PYMES_CALENDAR_KMS_KEY" ]] ||
      fail "Calendar KMS key differs between desired and deployed config"
  done
fi

[[ "$(service_env_value PYMES_CLERK_ISSUER <<<"$api_json")" == "$PYMES_CLERK_ISSUER" ]] ||
  fail "$prefix-api Clerk issuer differs"
[[ "$(service_env_value PYMES_CLERK_AUDIENCE <<<"$api_json")" == "pymes-v3" ]] ||
  fail "$prefix-api Clerk audience differs"
for service in "$prefix-api" "$prefix-worker" "$prefix-fiscal" "$prefix-accounting" "$prefix-accounting-admin"; do
  json=$(service_json "$service")
  [[ "$(service_env_value PYMES_INTERNAL_ISSUER <<<"$json")" == "pymes-v3" ]] ||
    fail "$service internal credential issuer differs"
done

accounting_json=$(service_json "$prefix-accounting")
accounting_admin_json=$(service_json "$prefix-accounting-admin")

verify_service_env_value "$prefix-api" "$api_json" PYMES_ENVIRONMENT production
verify_service_env_value "$prefix-api" "$api_json" PYMES_HTTP_ADDR :8080
verify_service_env_value "$prefix-worker" "$worker_json" PYMES_ENVIRONMENT production
verify_service_env_value "$prefix-worker" "$worker_json" PYMES_RELEASE_SHA "$PYMES_RELEASE_SHA"
verify_service_env_value "$prefix-worker" "$worker_json" PYMES_WORKER_HTTP_ADDR :8080
verify_service_env_value "$prefix-worker" "$worker_json" PYMES_WORKER_INTERVAL_MS 250
verify_service_env_value "$prefix-worker" "$worker_json" PYMES_WORKER_METRICS_INTERVAL 60s
verify_service_env_value "$prefix-fiscal" "$fiscal_json" PYMES_ENVIRONMENT production
verify_service_env_value "$prefix-fiscal" "$fiscal_json" PORT 8080
verify_service_env_value "$prefix-accounting" "$accounting_json" PYMES_ENVIRONMENT production
verify_service_env_value "$prefix-accounting" "$accounting_json" PORT 8080
verify_service_env_value "$prefix-accounting-admin" "$accounting_admin_json" PYMES_ENVIRONMENT production
verify_service_env_value "$prefix-accounting-admin" "$accounting_admin_json" PORT 8080
verify_service_env_value "$prefix-accounting-admin" "$accounting_admin_json" ACCOUNTING_ADMIN_OPERATION serve
verify_service_env_value "$prefix-accounting-admin" "$accounting_admin_json" \
  ACCOUNTING_RUNTIME_ROLE "pymes_v3_accounting_${PYMES_DEPLOY_ENV}"
verify_service_env_value "$prefix-accounting-admin" "$accounting_admin_json" \
  ACCOUNTING_OWNER_ROLE "pymes_v3_accounting_owner_${PYMES_DEPLOY_ENV}"

api_env_allowlist=(
  FISCAL_ADAPTER_URL
  PYMES_CLERK_AUDIENCE
  PYMES_CLERK_AUTHORIZED_PARTIES
  PYMES_CLERK_ISSUER
  PYMES_CLERK_SECRET_KEY
  PYMES_CLERK_WEBHOOK_SECRET
  PYMES_DATABASE_URL
  PYMES_ENVIRONMENT
  PYMES_GOOGLE_CALENDAR_ENABLED
  PYMES_HTTP_ADDR
  PYMES_INTERNAL_ISSUER
  PYMES_INTERNAL_KMS_KEY_VERSION
  PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS
  PYMES_PERGO_ENABLED
  PYMES_PREFLIGHT_TAG
  PYMES_PREFLIGHT_TOKEN
  PYMES_SCHEDULING_ACTION_TOKEN_SECRET
)
worker_env_allowlist=(
  ACCOUNTING_URL
  FISCAL_ADAPTER_URL
  PYMES_DATABASE_URL
  PYMES_ENVIRONMENT
  PYMES_GOOGLE_CALENDAR_ENABLED
  PYMES_INTERNAL_ISSUER
  PYMES_INTERNAL_KMS_KEY_VERSION
  PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS
  PYMES_PERGO_ENABLED
  PYMES_RELEASE_SHA
  PYMES_SCHEDULING_ACTION_TOKEN_SECRET
  PYMES_WORKER_HTTP_ADDR
  PYMES_WORKER_INTERVAL_MS
  PYMES_WORKER_METRICS_INTERVAL
)
fiscal_env_allowlist=(
  FISCAL_ADAPTER_MODE
  FISCAL_DATABASE_URL
  FISCAL_KMS_KEY_NAME
  PORT
  PYMES_ENVIRONMENT
  PYMES_INTERNAL_ISSUER
  PYMES_INTERNAL_JWKS_JSON
)
if [[ "$fiscal_mode" == "mock" ]]; then
  fiscal_env_allowlist+=(FISCAL_MOCK_SCENARIO)
  verify_service_env_value "$prefix-fiscal" "$fiscal_json" FISCAL_MOCK_SCENARIO authorized
else
  fiscal_env_allowlist+=(
    FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN
    FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN
  )
fi
accounting_env_allowlist=(
  ACCOUNTING_DATABASE_URL
  PORT
  PYMES_ENVIRONMENT
  PYMES_INTERNAL_ISSUER
  PYMES_INTERNAL_JWKS_JSON
)
accounting_admin_env_allowlist=(
  ACCOUNTING_ADMIN_DATABASE_URL
  ACCOUNTING_ADMIN_OPERATION
  ACCOUNTING_OWNER_ROLE
  ACCOUNTING_RUNTIME_ROLE
  PORT
  PYMES_ENVIRONMENT
  PYMES_INTERNAL_ISSUER
  PYMES_INTERNAL_JWKS_JSON
)
web_env_allowlist=(
  PYMES_API_UPSTREAM
  PYMES_PREFLIGHT_TAG
  PYMES_PREFLIGHT_TOKEN
  PYMES_RELEASE_MARKER
)

if [[ "$google_enabled" == "true" ]]; then
  api_env_allowlist+=(
    PYMES_CALENDAR_KMS_KEY
    PYMES_GOOGLE_CLIENT_ID
    PYMES_GOOGLE_CLIENT_SECRET
    PYMES_GOOGLE_REDIRECT_URL
  )
  worker_env_allowlist+=(
    PYMES_CALENDAR_KMS_KEY
    PYMES_GOOGLE_CLIENT_ID
    PYMES_GOOGLE_CLIENT_SECRET
    PYMES_GOOGLE_REDIRECT_URL
  )
fi
if [[ "$pergo_enabled" == "true" ]]; then
  api_env_allowlist+=(PERGO_WEBHOOK_SECRETS PERGO_WORKSPACE_ID)
  worker_env_allowlist+=(
    PERGO_ALLOW_GLOBAL_ROUTE_FALLBACK
    PERGO_API_KEY
    PERGO_CHANNEL
    PERGO_TIMEOUT
    PERGO_URL
    PERGO_WORKSPACE_ID
    PYMES_PERGO_AUDIENCE
  )
  verify_service_env_value "$prefix-api" "$api_json" PERGO_WORKSPACE_ID "$PYMES_PERGO_WORKSPACE_ID"
  verify_service_env_value "$prefix-worker" "$worker_json" PERGO_TIMEOUT 5s
fi

api_tracing_names=$(resource_env_names service <<<"$api_json" |
  grep -E '^(OTEL_EXPORTER_OTLP_ENDPOINT|PYMES_TRACE_SAMPLE_RATIO|PYMES_TRACING_EXPORTER)$' || true)
worker_tracing_names=$(resource_env_names service <<<"$worker_json" |
  grep -E '^(OTEL_EXPORTER_OTLP_ENDPOINT|PYMES_TRACE_SAMPLE_RATIO|PYMES_TRACING_EXPORTER)$' || true)
if [[ -n "$api_tracing_names" || -n "$worker_tracing_names" ]]; then
  expected_tracing_names=$'OTEL_EXPORTER_OTLP_ENDPOINT\nPYMES_TRACE_SAMPLE_RATIO\nPYMES_TRACING_EXPORTER'
  [[ "$(LC_ALL=C sort <<<"$api_tracing_names")" == "$expected_tracing_names" &&
     "$(LC_ALL=C sort <<<"$worker_tracing_names")" == "$expected_tracing_names" ]] ||
    fail "API and worker must configure the complete tracing environment together"
  tracing_endpoint=$(service_env_value OTEL_EXPORTER_OTLP_ENDPOINT <<<"$api_json")
  tracing_ratio=$(service_env_value PYMES_TRACE_SAMPLE_RATIO <<<"$api_json")
  [[ "$tracing_endpoint" == "$(service_env_value OTEL_EXPORTER_OTLP_ENDPOINT <<<"$worker_json")" &&
     "$tracing_ratio" == "$(service_env_value PYMES_TRACE_SAMPLE_RATIO <<<"$worker_json")" ]] ||
    fail "API and worker tracing values differ"
  [[ "$(service_env_value PYMES_TRACING_EXPORTER <<<"$api_json")" == "otlp" &&
     "$(service_env_value PYMES_TRACING_EXPORTER <<<"$worker_json")" == "otlp" ]] ||
    fail "API and worker tracing exporter must be otlp"
  if [[ "$tracing_endpoint" == *$'\r'* ||
        "$tracing_endpoint" == *$'\n'* ||
        "$tracing_endpoint" == *"|"* ||
        "$tracing_endpoint" == *"@"* ||
        "$tracing_endpoint" == *"?"* ||
        "$tracing_endpoint" == *"#"* ]]; then
    fail "deployed tracing endpoint contains forbidden delimiters"
  fi
  if ! awk -v ratio="$tracing_ratio" 'BEGIN { exit !(ratio ~ /^[0-9]+([.][0-9]+)?$/ && ratio > 0 && ratio <= 1) }'; then
    fail "deployed tracing ratio is outside (0,1]"
  fi
  api_env_allowlist+=(
    OTEL_EXPORTER_OTLP_ENDPOINT
    PYMES_TRACE_SAMPLE_RATIO
    PYMES_TRACING_EXPORTER
  )
  worker_env_allowlist+=(
    OTEL_EXPORTER_OTLP_ENDPOINT
    PYMES_TRACE_SAMPLE_RATIO
    PYMES_TRACING_EXPORTER
  )
fi

verify_env_allowlist service "$prefix-api" "$api_json" "${api_env_allowlist[@]}"
verify_env_allowlist service "$prefix-web" "$web_json" "${web_env_allowlist[@]}"
verify_env_allowlist service "$prefix-worker" "$worker_json" "${worker_env_allowlist[@]}"
verify_env_allowlist service "$prefix-fiscal" "$fiscal_json" "${fiscal_env_allowlist[@]}"
verify_env_allowlist service "$prefix-accounting" "$accounting_json" "${accounting_env_allowlist[@]}"
verify_env_allowlist service "$prefix-accounting-admin" "$accounting_admin_json" "${accounting_admin_env_allowlist[@]}"

api_secret_allowlist=(
  "PYMES_CLERK_SECRET_KEY=${prefix}-clerk-secret-key"
  "PYMES_CLERK_WEBHOOK_SECRET=${prefix}-clerk-webhook-secret"
  "PYMES_DATABASE_URL=${prefix}-database-url"
  "PYMES_SCHEDULING_ACTION_TOKEN_SECRET=${prefix}-scheduling-action-token-secret"
)
worker_secret_allowlist=(
  "PYMES_DATABASE_URL=${prefix}-worker-database-url"
  "PYMES_SCHEDULING_ACTION_TOKEN_SECRET=${prefix}-scheduling-action-token-secret"
)
if [[ "$google_enabled" == "true" ]]; then
  api_secret_allowlist+=("PYMES_GOOGLE_CLIENT_SECRET=${prefix}-google-client-secret")
  worker_secret_allowlist+=("PYMES_GOOGLE_CLIENT_SECRET=${prefix}-google-client-secret")
fi
if [[ "$pergo_enabled" == "true" ]]; then
  api_secret_allowlist+=("PERGO_WEBHOOK_SECRETS=${prefix}-pergo-webhook-secrets")
  worker_secret_allowlist+=("PERGO_API_KEY=${prefix}-pergo-api-key")
fi
verify_secret_refs service "$prefix-api" "$api_json" "${api_secret_allowlist[@]}"
verify_secret_refs service "$prefix-web" "$web_json"
verify_secret_refs service "$prefix-worker" "$worker_json" "${worker_secret_allowlist[@]}"
verify_secret_refs service "$prefix-fiscal" "$fiscal_json" \
  "FISCAL_DATABASE_URL=${prefix}-fiscal-database-url"
verify_secret_refs service "$prefix-accounting" "$accounting_json" \
  "ACCOUNTING_DATABASE_URL=${prefix}-accounting-database-url"
verify_secret_refs service "$prefix-accounting-admin" "$accounting_admin_json" \
  "ACCOUNTING_ADMIN_DATABASE_URL=${prefix}-accounting-admin-database-url"

expected_jwks=$(jq -cS . <<<"$PYMES_INTERNAL_JWKS_JSON")
for service in "$prefix-fiscal" "$prefix-accounting" "$prefix-accounting-admin"; do
  json=$(service_json "$service")
  actual_jwks=$(service_env_value PYMES_INTERNAL_JWKS_JSON <<<"$json")
  [[ "$(jq -cS . <<<"$actual_jwks")" == "$expected_jwks" ]] ||
    fail "$service JWKS does not match the selected KMS versions"
done

verify_job() {
  local job="$1" expected_image="$2" expected_account="$3" vpc="$4" require_execution="$5"
  local json task_count max_retries
  json=$(job_json "$job")
  verify_container_shape job "$job" "$json"
  [[ "$(job_image <<<"$json")" == "$expected_image" ]] ||
    fail "$job does not use the exact release digest"
  [[ "$(job_account <<<"$json")" == "$expected_account" ]] ||
    fail "$job uses the wrong runtime service account"
  [[ "$(job_release_marker <<<"$json")" == "$PYMES_RELEASE_SHA" ]] ||
    fail "$job has the wrong release marker"
  [[ "$(job_cloudsql <<<"$json")" == "$PYMES_CLOUDSQL_INSTANCE" ]] ||
    fail "$job Cloud SQL attachment differs"
  task_count=$(jq -r '
    .spec.template.spec.taskCount //
    .template.taskCount //
    "1"
  ' <<<"$json")
  max_retries=$(jq -r '
    .spec.template.spec.template.spec.maxRetries //
    .template.template.maxRetries //
    empty
  ' <<<"$json")
  [[ "$task_count" == "1" && "$max_retries" == "0" ]] ||
    fail "$job task count/max retries differs from 1/0"
  if [[ "$vpc" == "direct" ]]; then
    verify_direct_vpc job "$job" "$json"
  else
    verify_no_vpc job "$job" "$json"
  fi
  if [[ "$require_execution" == "true" ]]; then
    executions=$(gcloud run jobs executions list --job="$job" \
      --project="$project" --region="$region" \
      --sort-by='~metadata.creationTimestamp' --limit=1 --format=json)
    jq -e '
      length == 1 and
      (
        .[0].status.conditions //
        .[0].conditions //
        []
      | any(
          (.type == "Completed" or .type == "COMPLETED") and
          (.status == "True" or .state == "CONDITION_SUCCEEDED")
        )
      )
    ' <<<"$executions" >/dev/null ||
      fail "$job latest execution did not complete successfully"
  fi
}

verify_job "$prefix-migrate" "$PYMES_MIGRATE_IMAGE" "$migrate_sa" none true
verify_job "$prefix-fiscal-migrate" "$PYMES_FISCAL_MIGRATE_IMAGE" "$fiscal_migrate_sa" none true
verify_job "$prefix-accounting-migrate" "$PYMES_ACCOUNTING_MIGRATE_IMAGE" "$accounting_migrate_sa" none true
verify_job "$prefix-accounting-grants" "$PYMES_ACCOUNTING_ADMIN_IMAGE" "$accounting_admin_sa" none true
verify_job "$prefix-provision-org" "$PYMES_PROVISION_IMAGE" "$provision_sa" direct false
verify_secret_refs job "$prefix-migrate" "$(job_json "$prefix-migrate")" \
  "PYMES_DATABASE_URL=${prefix}-migrate-database-url"
verify_secret_refs job "$prefix-fiscal-migrate" "$(job_json "$prefix-fiscal-migrate")" \
  "FISCAL_DATABASE_URL=${prefix}-fiscal-migrate-database-url"
verify_secret_refs job "$prefix-accounting-migrate" "$(job_json "$prefix-accounting-migrate")" \
  "DATABASE_URL=${prefix}-accounting-migrate-database-url"
verify_secret_refs job "$prefix-accounting-grants" "$(job_json "$prefix-accounting-grants")" \
  "ACCOUNTING_ADMIN_DATABASE_URL=${prefix}-accounting-admin-database-url"
verify_secret_refs job "$prefix-provision-org" "$(job_json "$prefix-provision-org")" \
  "PYMES_DATABASE_URL=${prefix}-database-url"
provision_json=$(job_json "$prefix-provision-org")
grants_json=$(job_json "$prefix-accounting-grants")
verify_env_allowlist job "$prefix-migrate" "$(job_json "$prefix-migrate")" \
  PYMES_DATABASE_URL
verify_env_allowlist job "$prefix-fiscal-migrate" "$(job_json "$prefix-fiscal-migrate")" \
  FISCAL_DATABASE_URL
verify_env_allowlist job "$prefix-accounting-migrate" "$(job_json "$prefix-accounting-migrate")" \
  DATABASE_URL
verify_env_allowlist job "$prefix-accounting-grants" "$grants_json" \
  ACCOUNTING_ADMIN_DATABASE_URL \
  ACCOUNTING_ADMIN_OPERATION \
  ACCOUNTING_OWNER_ROLE \
  ACCOUNTING_RUNTIME_ROLE
verify_env_allowlist job "$prefix-provision-org" "$provision_json" \
  ACCOUNTING_PROVISIONING_URL \
  PYMES_DATABASE_URL \
  PYMES_ENVIRONMENT \
  PYMES_INTERNAL_ISSUER \
  PYMES_INTERNAL_KMS_KEY_VERSION \
  PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS
verify_job_env_value "$prefix-accounting-grants" "$grants_json" \
  ACCOUNTING_ADMIN_OPERATION sync-runtime-grants
verify_job_env_value "$prefix-accounting-grants" "$grants_json" \
  ACCOUNTING_RUNTIME_ROLE "pymes_v3_accounting_${PYMES_DEPLOY_ENV}"
verify_job_env_value "$prefix-accounting-grants" "$grants_json" \
  ACCOUNTING_OWNER_ROLE "pymes_v3_accounting_owner_${PYMES_DEPLOY_ENV}"
verify_job_env_value "$prefix-provision-org" "$provision_json" \
  ACCOUNTING_PROVISIONING_URL "$accounting_admin_url"
verify_job_env_value "$prefix-provision-org" "$provision_json" \
  PYMES_ENVIRONMENT production
verify_job_env_value "$prefix-provision-org" "$provision_json" \
  PYMES_INTERNAL_ISSUER pymes-v3
verify_job_env_value "$prefix-provision-org" "$provision_json" \
  PYMES_INTERNAL_KMS_KEY_VERSION "$PYMES_INTERNAL_KMS_KEY_VERSION"
verify_job_env_value "$prefix-provision-org" "$provision_json" \
  PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS "${PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS:-}"

[[ "$web_url" == https://* ]] ||
  fail "$prefix-web has no HTTPS Cloud Run URL"

if [[ "$deploy_stage" == "bootstrap" ]]; then
  echo "verified immutable Pymes v3 stg bootstrap release ${PYMES_RELEASE_SHA} phase=pretraffic traffic=0 ingress=internal unauthenticated=denied worker_min=0 tag_public_access=denied"
  exit 0
fi

headers=$(mktemp)
session_headers=$(mktemp)
session_body=$(mktemp)
api_headers=$(mktemp)
preflight_curl_config=$(mktemp)
cleanup() {
  rm -f -- \
    "$headers" \
    "$session_headers" \
    "$session_body" \
    "$api_headers" \
    "$preflight_curl_config"
}
trap cleanup EXIT
chmod 600 "$preflight_curl_config"
printf 'header = "X-Pymes-Preflight-Token: %s"\n' \
  "$PYMES_PREFLIGHT_TOKEN" >"$preflight_curl_config"

http_exact() {
  local label="$1" url="$2" expected_status="$3" header_file="$4" body_file="$5"
  local capability="${6:-none}"
  local result status effective redirect
  local -a arguments=(
    --disable
    --proto '=https' --tlsv1.2 --silent --show-error
    --max-redirs 0 --connect-timeout 10 --max-time 30
    --header='Accept: application/json'
    --dump-header "$header_file" --output "$body_file"
    --write-out=$'%{http_code}\n%{url_effective}\n%{redirect_url}'
  )
  if [[ "$capability" == "preflight" ]]; then
    arguments+=(--config "$preflight_curl_config")
  fi
  result=$(curl "${arguments[@]}" "$url") ||
    fail "$label request failed"
  mapfile -t curl_result <<<"$result"
  status=${curl_result[0]:-}
  effective=${curl_result[1]:-}
  redirect=${curl_result[2]:-}
  if [[ "$effective" != "$url" || -n "$redirect" ]]; then
    if [[ "$label" == "public readiness" ]]; then
      fail "public readiness redirected outside HTTPS or changed the exact origin"
    fi
    fail "$label redirected or changed the exact origin"
  fi
  if grep -Eiq '^location[[:space:]]*:' "$header_file"; then
    fail "$label returned a redirect Location header"
  fi
  if [[ "$status" != "$expected_status" ]]; then
    if [[ "$label" == "public same-origin API proxy" ]]; then
      fail "public same-origin API proxy returned HTTP $status instead of $expected_status"
    fi
    fail "$label returned HTTP $status instead of $expected_status"
  fi
}

if [[ "$verify_phase" == "pretraffic" ]]; then
  public_check_origin=$(service_candidate_url "$prefix-web")
  api_check_origin=$api_candidate_url
else
  public_check_origin=$PYMES_PUBLIC_BASE_URL
  api_check_origin=
fi
[[ "$public_check_origin" =~ ^https://[^/?#]+$ ]] ||
  fail "public verification origin is not one exact HTTPS origin"
[[ "$api_candidate_url" =~ ^https://[^/?#]+$ ]] ||
  fail "candidate API verification origin is not one exact HTTPS origin"

if [[ "$verify_phase" == "pretraffic" ]]; then
  http_exact "unauthenticated candidate Web gate" \
    "${public_check_origin}/readyz" 404 "$headers" /dev/null
  http_exact "unauthenticated candidate API gate" \
    "${api_check_origin}/readyz" 404 "$api_headers" /dev/null
  http_exact "public readiness" \
    "${public_check_origin}/readyz" 200 "$headers" /dev/null preflight
  http_exact "candidate API readiness" \
    "${api_check_origin}/readyz" 200 "$api_headers" /dev/null preflight
else
  http_exact "public readiness" \
    "${public_check_origin}/readyz" 200 "$headers" /dev/null
  http_exact "active tagged API gate" \
    "${api_candidate_url}/readyz" 404 "$api_headers" /dev/null
fi
public_release_marker=$(awk '
  BEGIN { IGNORECASE=1 }
  /^x-pymes-release:/ {
    sub(/^[^:]*:[[:space:]]*/, "", $0)
    gsub("\r", "", $0)
    value=$0
  }
  END { print value }
' "$headers")
[[ "$public_release_marker" == "$web_release_marker" ]] ||
  fail "public readiness does not expose the exact deployed Web release marker"

http_exact "public same-origin API proxy" \
  "${public_check_origin}/api/v1/session" 403 \
  "$session_headers" "$session_body" \
  "$([[ "$verify_phase" == "pretraffic" ]] && echo preflight || echo none)"
jq -e '
  type == "object" and
  .code == "FORBIDDEN" and
  (.message | type == "string" and length > 0)
' "$session_body" >/dev/null ||
  fail "public same-origin API proxy did not return the canonical FORBIDDEN JSON"
session_content_type=$(awk '
  BEGIN { IGNORECASE=1 }
  /^content-type:/ {
    sub(/^[^:]*:[[:space:]]*/, "", $0)
    gsub("\r", "", $0)
    value=$0
  }
  END { print tolower(value) }
' "$session_headers")
[[ "$session_content_type" == application/json* ]] ||
  fail "public same-origin API proxy did not preserve application/json"
session_release_marker=$(awk '
  BEGIN { IGNORECASE=1 }
  /^x-pymes-release:/ {
    sub(/^[^:]*:[[:space:]]*/, "", $0)
    gsub("\r", "", $0)
    value=$0
  }
  END { print value }
' "$session_headers")
[[ "$session_release_marker" == "$web_release_marker" ]] ||
  fail "public same-origin API proxy does not expose the exact Web release marker"

echo "verified immutable Pymes v3 ${PYMES_DEPLOY_ENV} release ${PYMES_RELEASE_SHA} stage=${deploy_stage} phase=${verify_phase}"
