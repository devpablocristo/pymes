#!/usr/bin/env bash
set -euo pipefail
set +x
umask 077

# Deploys Pymes v3 into the shared GCP project. It deliberately refuses to
# start incomplete or mutable releases: images must be digest-pinned, every
# referenced secret must have an enabled numeric version, private callers use
# Direct VPC egress, and production fiscal storage always uses Cloud KMS.

: "${PYMES_DEPLOY_ENV:?set PYMES_DEPLOY_ENV to stg or prd}"
case "$PYMES_DEPLOY_ENV" in stg|prd) ;; *) echo "PYMES_DEPLOY_ENV must be stg or prd" >&2; exit 2 ;; esac

deploy_stage=${PYMES_DEPLOY_STAGE:-operational}
case "$deploy_stage" in
  bootstrap|operational) ;;
  *) echo "PYMES_DEPLOY_STAGE must be bootstrap or operational" >&2; exit 2 ;;
esac
if [[ "$deploy_stage" == "bootstrap" && "$PYMES_DEPLOY_ENV" != "stg" ]]; then
  echo "PYMES_DEPLOY_STAGE=bootstrap is allowed only with PYMES_DEPLOY_ENV=stg" >&2
  exit 2
fi
export PYMES_DEPLOY_STAGE="$deploy_stage"

: "${PYMES_API_IMAGE:?set PYMES_API_IMAGE}"
: "${PYMES_WEB_IMAGE:?set PYMES_WEB_IMAGE}"
: "${PYMES_WORKER_IMAGE:?set PYMES_WORKER_IMAGE}"
: "${PYMES_FISCAL_IMAGE:?set PYMES_FISCAL_IMAGE}"
: "${PYMES_ACCOUNTING_IMAGE:?set PYMES_ACCOUNTING_IMAGE}"
: "${PYMES_ACCOUNTING_ADMIN_IMAGE:?set PYMES_ACCOUNTING_ADMIN_IMAGE}"
: "${PYMES_PROVISION_IMAGE:?set PYMES_PROVISION_IMAGE}"
: "${PYMES_MIGRATE_IMAGE:?set PYMES_MIGRATE_IMAGE}"
: "${PYMES_FISCAL_MIGRATE_IMAGE:?set PYMES_FISCAL_MIGRATE_IMAGE}"
: "${PYMES_ACCOUNTING_MIGRATE_IMAGE:?set PYMES_ACCOUNTING_MIGRATE_IMAGE}"
: "${PYMES_CLOUDSQL_INSTANCE:?set PYMES_CLOUDSQL_INSTANCE (project:region:instance)}"
: "${PYMES_CLERK_ISSUER:?set PYMES_CLERK_ISSUER}"
: "${PYMES_INTERNAL_KMS_KEY_VERSION:?set PYMES_INTERNAL_KMS_KEY_VERSION to an explicit EC_SIGN_ED25519 CryptoKeyVersion resource}"
: "${PYMES_RELEASE_SHA:?set PYMES_RELEASE_SHA to the exact 40-character Pymes source commit}"
if [[ ! "$PYMES_RELEASE_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "PYMES_RELEASE_SHA must be exactly 40 lowercase hexadecimal characters" >&2
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
digest_pattern='^[^[:space:]@]+@sha256:[0-9a-f]{64}$'
for image_variable in "${image_variables[@]}"; do
  image=${!image_variable}
  if [[ ! "$image" =~ $digest_pattern ]]; then
    echo "$image_variable must be an immutable image reference pinned by @sha256:<64 lowercase hex>" >&2
    exit 2
  fi
done
web_release_digest=${PYMES_WEB_IMAGE##*@}
web_release_marker="${PYMES_DEPLOY_ENV}:${PYMES_RELEASE_SHA}:${web_release_digest}"

if [[ "$deploy_stage" == "bootstrap" ]]; then
  bootstrap_public_origin="https://pymes-v3-stg-bootstrap.invalid"
  PYMES_PUBLIC_BASE_URL=${PYMES_PUBLIC_BASE_URL:-$bootstrap_public_origin}
  PYMES_CLERK_AUTHORIZED_PARTIES=${PYMES_CLERK_AUTHORIZED_PARTIES:-$bootstrap_public_origin}
  if [[ "$PYMES_PUBLIC_BASE_URL" != "$bootstrap_public_origin" ||
        "$PYMES_CLERK_AUTHORIZED_PARTIES" != "$bootstrap_public_origin" ]]; then
    echo "bootstrap must use only the reserved fail-closed origin $bootstrap_public_origin" >&2
    exit 2
  fi
else
  : "${PYMES_PUBLIC_BASE_URL:?set PYMES_PUBLIC_BASE_URL to the public Web origin}"
  : "${PYMES_CLERK_AUTHORIZED_PARTIES:?set PYMES_CLERK_AUTHORIZED_PARTIES}"
fi
export PYMES_PUBLIC_BASE_URL PYMES_CLERK_AUTHORIZED_PARTIES

public_origin_pattern='^https://[^/@|?#]+$'
if [[ ! "$PYMES_PUBLIC_BASE_URL" =~ $public_origin_pattern ]]; then
  echo "PYMES_PUBLIC_BASE_URL must be an HTTPS origin without credentials, path, query, fragment or trailing slash" >&2
  exit 2
fi
authorized_party_found=false
IFS=',' read -r -a authorized_parties <<<"$PYMES_CLERK_AUTHORIZED_PARTIES"
for authorized_party in "${authorized_parties[@]}"; do
  authorized_party=${authorized_party#"${authorized_party%%[![:space:]]*}"}
  authorized_party=${authorized_party%"${authorized_party##*[![:space:]]}"}
  if [[ ! "$authorized_party" =~ $public_origin_pattern ]]; then
    echo "every PYMES_CLERK_AUTHORIZED_PARTIES entry must be an HTTPS origin" >&2
    exit 2
  fi
  if [[ "$authorized_party" == "$PYMES_PUBLIC_BASE_URL" ]]; then
    authorized_party_found=true
  fi
done
if [[ "$authorized_party_found" != "true" ]]; then
  echo "PYMES_CLERK_AUTHORIZED_PARTIES must include PYMES_PUBLIC_BASE_URL exactly" >&2
  exit 2
fi

dry_run=${PYMES_CLOUD_RUN_DRY_RUN:-false}
case "$dry_run" in
  true|false) ;;
  *) echo "PYMES_CLOUD_RUN_DRY_RUN must be true or false" >&2; exit 2 ;;
esac
if [[ "$dry_run" != "true" &&
      -n "${PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN:-}" ]]; then
  echo "PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN is permitted only with PYMES_CLOUD_RUN_DRY_RUN=true" >&2
  exit 2
fi
if [[ "$dry_run" != "true" &&
      -n "${PYMES_CLOUD_RUN_ACTIVE_SERVICES_DRY_RUN:-}" ]]; then
  echo "PYMES_CLOUD_RUN_ACTIVE_SERVICES_DRY_RUN is permitted only with PYMES_CLOUD_RUN_DRY_RUN=true" >&2
  exit 2
fi

pergo_enabled=${PYMES_PERGO_ENABLED:-false}
case "$pergo_enabled" in
  true|false) ;;
  *) echo "PYMES_PERGO_ENABLED must be true or false" >&2; exit 2 ;;
esac
pergo_url=${PYMES_PERGO_URL:-}
pergo_workspace_id=${PYMES_PERGO_WORKSPACE_ID:-}
pergo_channel=${PYMES_PERGO_CHANNEL:-whatsapp}
if [[ "$pergo_enabled" == "true" ]]; then
  : "${pergo_url:?set PYMES_PERGO_URL when PerGo is enabled}"
  : "${pergo_workspace_id:?set PYMES_PERGO_WORKSPACE_ID when PerGo is enabled}"
  if [[ ! "$pergo_workspace_id" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$ ]]; then
    echo "PYMES_PERGO_WORKSPACE_ID must be one canonical lowercase UUIDv4 without delimiters or control characters" >&2
    exit 2
  fi
  case "$pergo_channel" in
    whatsapp|whatsapp_cloud) ;;
    *) echo "PYMES_PERGO_CHANNEL must be whatsapp or whatsapp_cloud" >&2; exit 2 ;;
  esac
  if [[ "$pergo_url" != https://* ||
        "$pergo_url" == *$'\r'* ||
        "$pergo_url" == *$'\n'* ||
        "$pergo_url" == *[[:space:]]* ||
        "$pergo_url" == *"|"* ||
        "$pergo_url" == *","* ||
        "$pergo_url" == *"@"* ||
        "$pergo_url" == *"?"* ||
        "$pergo_url" == *"#"* ]]; then
    echo "PYMES_PERGO_URL must be an explicit HTTPS URL without credentials, query, fragment or Cloud Run delimiters" >&2
    exit 2
  fi
fi

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
prefix="pymes-v3-${PYMES_DEPLOY_ENV}"
if [[ "$PYMES_CLOUDSQL_INSTANCE" != "${project}:${region}:"* ]]; then
  echo "PYMES_CLOUDSQL_INSTANCE must belong to project $project and region $region" >&2
  exit 2
fi
if [[ ! "$PYMES_CLERK_ISSUER" =~ ^https://[^/@\|\?\#]+$ ]]; then
  echo "PYMES_CLERK_ISSUER must be an HTTPS issuer origin without credentials or delimiters" >&2
  exit 2
fi
network=${PYMES_VPC_NETWORK:-default}
subnet=${PYMES_VPC_SUBNET:-pymes-v3-serverless}
subnet_cidr=${PYMES_VPC_SUBNET_CIDR:-10.120.0.0/24}
nat_router=${PYMES_VPC_NAT_ROUTER:-pymes-v3-serverless}
nat_name=${PYMES_VPC_NAT_NAME:-pymes-v3-serverless}
if [[ ! "$network" =~ ^[a-z]([-a-z0-9]*[a-z0-9])?$ ||
      ! "$subnet" =~ ^[a-z]([-a-z0-9]*[a-z0-9])?$ ||
      ! "$nat_router" =~ ^[a-z]([-a-z0-9]*[a-z0-9])?$ ||
      ! "$nat_name" =~ ^[a-z]([-a-z0-9]*[a-z0-9])?$ ]]; then
  echo "VPC network, subnet, router and NAT names must be explicit valid GCP resource names" >&2
  exit 2
fi
if [[ ! "$subnet_cidr" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]{1,2})$ ]]; then
  echo "PYMES_VPC_SUBNET_CIDR must be an IPv4 CIDR" >&2
  exit 2
fi
subnet_prefix=${BASH_REMATCH[2]}
if (( subnet_prefix < 20 || subnet_prefix > 26 )); then
  echo "PYMES_VPC_SUBNET_CIDR prefix must be between /20 and /26 for Direct VPC egress" >&2
  exit 2
fi
subnet_address=${subnet_cidr%/*}
IFS='.' read -r subnet_octet1 subnet_octet2 subnet_octet3 subnet_octet4 \
  <<<"$subnet_address"
for subnet_octet in \
  "$subnet_octet1" "$subnet_octet2" "$subnet_octet3" "$subnet_octet4"; do
  if [[ ! "$subnet_octet" =~ ^[0-9]+$ ]] ||
    (( 10#$subnet_octet > 255 )); then
    echo "PYMES_VPC_SUBNET_CIDR contains an invalid IPv4 address" >&2
    exit 2
  fi
done
if ! (( 10#$subnet_octet1 == 10 ||
        (10#$subnet_octet1 == 172 &&
         10#$subnet_octet2 >= 16 && 10#$subnet_octet2 <= 31) ||
        (10#$subnet_octet1 == 192 && 10#$subnet_octet2 == 168) )); then
  echo "PYMES_VPC_SUBNET_CIDR must use an RFC1918 private IPv4 range" >&2
  exit 2
fi
subnet_address_value=$(( (10#$subnet_octet1 << 24) |
                          (10#$subnet_octet2 << 16) |
                          (10#$subnet_octet3 << 8) |
                          10#$subnet_octet4 ))
subnet_host_bits=$((32 - subnet_prefix))
subnet_host_mask=$(((1 << subnet_host_bits) - 1))
if (( (subnet_address_value & subnet_host_mask) != 0 )); then
  echo "PYMES_VPC_SUBNET_CIDR must be aligned to its prefix" >&2
  exit 2
fi
export CLOUDSDK_CORE_PROJECT="$project"
export PYMES_GCP_PROJECT="$project"
export PYMES_GCP_REGION="$region"
export PYMES_VPC_NETWORK="$network"
export PYMES_VPC_SUBNET="$subnet"
export PYMES_INTERNAL_KMS_KEY_VERSION
export PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS="${PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS:-}"
google_calendar_enabled=${PYMES_GOOGLE_CALENDAR_ENABLED:-false}
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
fiscal_mode=${PYMES_FISCAL_MODE:-mock}
case "$fiscal_mode" in
  mock|arca) ;;
  *) echo "PYMES_FISCAL_MODE must be mock or arca" >&2; exit 2 ;;
esac
fiscal_kms_key="projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/fiscal-vault"
calendar_kms_key="projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/calendar-tokens"
secrets_kms_key="projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/secrets"
fiscal_environment="FISCAL_ADAPTER_MODE=$fiscal_mode|FISCAL_KMS_KEY_NAME=$fiscal_kms_key"
if [[ "$fiscal_mode" == "mock" ]]; then
  for variable in PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN; do
    if [[ -n "${!variable:-}" ]]; then
      echo "$variable requires PYMES_FISCAL_MODE=arca" >&2
      exit 2
    fi
  done
  fiscal_environment+="|FISCAL_MOCK_SCENARIO=authorized"
else
  : "${PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN:?set the reviewed ARCA homologation certificate issuer pattern}"
  : "${PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN:?set the reviewed ARCA production certificate issuer pattern}"
  for value in "$PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN" "$PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN"; do
    if [[ "$value" == *"|"* || "$value" == *$'\n'* || ${#value} -gt 256 ]]; then
      echo "ARCA issuer patterns must not contain Cloud Run delimiters/newlines and must be at most 256 bytes" >&2
      exit 2
    fi
  done
  fiscal_environment+="|FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN=$PYMES_FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN|FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN=$PYMES_FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN"
fi
echo "FISCAL mode=$fiscal_mode kms=environment-scoped"

case "$google_calendar_enabled" in
  true|false) ;;
  *) echo "PYMES_GOOGLE_CALENDAR_ENABLED must be true or false" >&2; exit 2 ;;
esac
calendar_environment="|PYMES_GOOGLE_CALENDAR_ENABLED=false"
google_client_secret_name=
if [[ "$google_calendar_enabled" == "true" ]]; then
  : "${PYMES_GOOGLE_CLIENT_ID:?set the environment-specific Google OAuth client ID}"
  : "${PYMES_GOOGLE_REDIRECT_URL:?set the single global BFF Google OAuth callback}"
  : "${PYMES_CALENDAR_KMS_KEY:?set the environment-specific Calendar token CryptoKey}"
  expected_google_redirect="${PYMES_PUBLIC_BASE_URL}/api/v1/calendars/google/oauth/callback"
  if [[ "$PYMES_GOOGLE_REDIRECT_URL" != "$expected_google_redirect" ]]; then
    echo "PYMES_GOOGLE_REDIRECT_URL must equal the public same-origin callback $expected_google_redirect" >&2
    exit 2
  fi
  for value in "$PYMES_GOOGLE_CLIENT_ID" "$PYMES_GOOGLE_REDIRECT_URL" "$PYMES_CALENDAR_KMS_KEY"; do
    if [[ "$value" == *"|"* || "$value" == *$'\n'* ]]; then
      echo "Google Calendar environment values must not contain Cloud Run delimiters or newlines" >&2
      exit 2
    fi
  done
  expected_calendar_kms_key=$calendar_kms_key
  if [[ "$PYMES_CALENDAR_KMS_KEY" != "$expected_calendar_kms_key" ]]; then
    echo "PYMES_CALENDAR_KMS_KEY must be $expected_calendar_kms_key" >&2
    exit 2
  fi
  google_client_secret_name="$prefix-google-client-secret"
  calendar_environment="|PYMES_GOOGLE_CALENDAR_ENABLED=true|PYMES_GOOGLE_CLIENT_ID=$PYMES_GOOGLE_CLIENT_ID|PYMES_GOOGLE_REDIRECT_URL=$PYMES_GOOGLE_REDIRECT_URL|PYMES_CALENDAR_KMS_KEY=$PYMES_CALENDAR_KMS_KEY"
  echo "GOOGLE_CALENDAR status=enabled callback=same-origin kms=environment-scoped"
else
  for variable in PYMES_GOOGLE_CLIENT_ID PYMES_GOOGLE_REDIRECT_URL PYMES_CALENDAR_KMS_KEY; do
    if [[ -n "${!variable:-}" ]]; then
      echo "$variable requires PYMES_GOOGLE_CALENDAR_ENABLED=true" >&2
      exit 2
    fi
  done
  echo "GOOGLE_CALENDAR status=disabled callback=unset"
fi

if [[ "$deploy_stage" == "bootstrap" ]]; then
  if [[ "$fiscal_mode" != "mock" ]]; then
    echo "bootstrap requires PYMES_FISCAL_MODE=mock" >&2
    exit 2
  fi
  if [[ "$pergo_enabled" != "false" ]]; then
    echo "bootstrap requires PYMES_PERGO_ENABLED=false" >&2
    exit 2
  fi
  if [[ "$google_calendar_enabled" != "false" ]]; then
    echo "bootstrap requires PYMES_GOOGLE_CALENDAR_ENABLED=false" >&2
    exit 2
  fi
  echo "DEPLOY STAGE stage=bootstrap environment=stg fiscal=mock pergo=false google=false promotion=disabled"
else
  echo "DEPLOY STAGE stage=operational environment=$PYMES_DEPLOY_ENV promotion=enabled"
fi

version_pattern="^projects/${project}/locations/${region}/keyRings/${prefix}/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/[1-9][0-9]*$"
if [[ ! "$PYMES_INTERNAL_KMS_KEY_VERSION" =~ $version_pattern ]]; then
  echo "PYMES_INTERNAL_KMS_KEY_VERSION must pin a numeric version in the ${prefix} key ring" >&2
  exit 2
fi
IFS=',' read -r -a overlap_versions <<<"$PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS"
for version in "${overlap_versions[@]}"; do
  [[ -z "$version" ]] && continue
  if [[ ! "$version" =~ $version_pattern ]]; then
    echo "every PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS entry must pin a numeric version in the ${prefix} key ring" >&2
    exit 2
  fi
done

tracing_environment=
tracing_endpoint=${OTEL_EXPORTER_OTLP_ENDPOINT:-}
if [[ -n "$tracing_endpoint" ]]; then
  tracing_exporter=${PYMES_TRACING_EXPORTER:-otlp}
  tracing_sample_ratio=${PYMES_TRACE_SAMPLE_RATIO:-0.1}
  if [[ "$tracing_exporter" != "otlp" ]]; then
    echo "PYMES_TRACING_EXPORTER must be otlp when OTEL_EXPORTER_OTLP_ENDPOINT is set" >&2
    exit 2
  fi
  if [[ "$tracing_endpoint" == *"|"* || "$tracing_endpoint" == *$'\n'* || "$tracing_endpoint" == *"@"* || "$tracing_endpoint" == *"?"* || "$tracing_endpoint" == *"#"* ]]; then
    echo "OTEL_EXPORTER_OTLP_ENDPOINT must be an explicit endpoint without credentials, query, fragment or Cloud Run env delimiters" >&2
    exit 2
  fi
  if ! awk -v ratio="$tracing_sample_ratio" 'BEGIN { exit !(ratio ~ /^[0-9]+([.][0-9]+)?$/ && ratio > 0 && ratio <= 1) }'; then
    echo "PYMES_TRACE_SAMPLE_RATIO must be greater than zero and at most one" >&2
    exit 2
  fi
  tracing_environment="|PYMES_TRACING_EXPORTER=otlp|OTEL_EXPORTER_OTLP_ENDPOINT=$tracing_endpoint|PYMES_TRACE_SAMPLE_RATIO=$tracing_sample_ratio"
  echo "TRACING status=configured exporter=otlp endpoint=explicit sample_ratio=$tracing_sample_ratio"
else
  echo "TRACING status=pending exporter=none endpoint=unset"
fi

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=release-authority-policy.sh
source "$script_dir/release-authority-policy.sh"
# shellcheck source=release-candidate-tag.sh
source "$script_dir/release-candidate-tag.sh"
if [[ "$dry_run" == "true" ]]; then
  : "${PYMES_INTERNAL_JWKS_JSON:?set PYMES_INTERNAL_JWKS_JSON for dry-run validation}"
  resolved_internal_jwks=$PYMES_INTERNAL_JWKS_JSON
else
  resolved_internal_jwks=$(cd "$script_dir/../../backend" && go run ./cmd/internal-jwks)
fi
if [[ -n "${PYMES_INTERNAL_JWKS_JSON:-}" && "$PYMES_INTERNAL_JWKS_JSON" != "$resolved_internal_jwks" ]]; then
  echo "PYMES_INTERNAL_JWKS_JSON does not match the selected KMS key versions" >&2
  exit 2
fi
PYMES_INTERNAL_JWKS_JSON=$resolved_internal_jwks
case "$PYMES_INTERNAL_JWKS_JSON" in
  *'|'*) echo "PYMES_INTERNAL_JWKS_JSON must not contain the Cloud Run env delimiter |" >&2; exit 2 ;;
esac
export PYMES_INTERNAL_JWKS_JSON
candidate_tag=$(pymes_release_candidate_tag "$PYMES_RELEASE_SHA")
pymes_validate_release_candidate_tag "$PYMES_RELEASE_SHA" "$candidate_tag"
: "${PYMES_PREFLIGHT_TOKEN:?set PYMES_PREFLIGHT_TOKEN to the ephemeral release capability}"
if [[ ! "$PYMES_PREFLIGHT_TOKEN" =~ ^[0-9a-f]{64}$ ]]; then
  echo "PYMES_PREFLIGHT_TOKEN must be exactly 32 random bytes encoded as lowercase hexadecimal" >&2
  exit 2
fi

gcloud_command() {
  if [[ "$dry_run" == "true" ]]; then
    printf 'DRY-RUN'
    printf ' %q' gcloud "$@"
    printf '\n'
    return
  fi
  gcloud "$@"
}

verify_network_egress() {
  local subnet_json router_json nat_json
  if [[ "$dry_run" == "true" ]]; then
    echo "SECURITY NETWORK network=$network subnet=$subnet cidr=$subnet_cidr private_google_access=true public_nat=$nat_router/$nat_name vpc_egress=all-traffic"
    return
  fi
  subnet_json=$(gcloud compute networks subnets describe "$subnet" \
    --project="$project" --region="$region" --format=json)
  jq -e \
    --arg network "$network" \
    --arg region "$region" \
    --arg cidr "$subnet_cidr" \
    '
      (.network | endswith("/global/networks/" + $network)) and
      (.region | endswith("/regions/" + $region)) and
      .ipCidrRange == $cidr and
      .privateIpGoogleAccess == true
    ' <<<"$subnet_json" >/dev/null || {
    echo "subnet $subnet must belong to $network in $region, use $subnet_cidr, and enable Private Google Access" >&2
    exit 1
  }
  router_json=$(gcloud compute routers describe "$nat_router" \
    --project="$project" --region="$region" --format=json)
  jq -e --arg network "$network" \
    '.network | endswith("/global/networks/" + $network)' \
    <<<"$router_json" >/dev/null || {
    echo "Cloud Router $nat_router does not belong to network $network" >&2
    exit 1
  }
  nat_json=$(gcloud compute routers nats describe "$nat_name" \
    --router="$nat_router" --project="$project" --region="$region" --format=json)
  jq -e \
    --arg subnet "$subnet" \
    --arg region "$region" \
    '
      .sourceSubnetworkIpRangesToNat == "LIST_OF_SUBNETWORKS" and
      any(
        .subnetworks[]?;
        (.name | endswith("/regions/" + $region + "/subnetworks/" + $subnet)) and
        (.sourceIpRangesToNat | index("ALL_IP_RANGES") != null)
      ) and
      (
        .natIpAllocateOption == "AUTO_ONLY" or
        (
          .natIpAllocateOption == "MANUAL_ONLY" and
          ((.natIps // []) | length) > 0
        )
      ) and
      (
        (.endpointTypes // ["ENDPOINT_TYPE_VM"]) |
        index("ENDPOINT_TYPE_VM") != null
      )
    ' <<<"$nat_json" >/dev/null || {
    echo "Cloud NAT $nat_router/$nat_name must provide public NAT for all ranges of subnet $subnet" >&2
    exit 1
  }
  echo "SECURITY NETWORK network=$network subnet=$subnet cidr=$subnet_cidr private_google_access=true public_nat=$nat_router/$nat_name vpc_egress=all-traffic"
}

verify_data_key() {
  local key_resource="$1" expected_csv="$2" key key_json direct_members
  local inherited_project inherited_keyring expected_members
  key=${key_resource##*/}
  expected_members=$(tr ',' '\n' <<<"$expected_csv" |
    sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ "$dry_run" == "true" ]]; then
    echo "SECURITY KMS key=$key primary=enabled rotation=90d direct_crypto_principals=$expected_csv inherited_crypto_principals=none"
    return
  fi
  key_json=$(gcloud kms keys describe "$key" \
    --project="$project" --location="$region" --keyring="$prefix" \
    --format=json)
  jq -e '
    .purpose == "ENCRYPT_DECRYPT" and
    .versionTemplate.algorithm == "GOOGLE_SYMMETRIC_ENCRYPTION" and
    .primary.state == "ENABLED" and
    .primary.algorithm == "GOOGLE_SYMMETRIC_ENCRYPTION" and
    .rotationPeriod == "7776000s" and
    ((.nextRotationTime // "") | length) > 0
  ' <<<"$key_json" >/dev/null || {
    echo "KMS key $key must have an enabled symmetric primary and a 90-day rotation schedule" >&2
    exit 1
  }
  inherited_project=$(gcloud projects get-iam-policy "$project" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/cloudkms.cryptoKeyEncrypterDecrypter' \
    --format='value(bindings.members)' |
    sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  inherited_keyring=$(gcloud kms keyrings get-iam-policy "$prefix" \
    --project="$project" --location="$region" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/cloudkms.cryptoKeyEncrypterDecrypter' \
    --format='value(bindings.members)' |
    sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ -n "$inherited_project" || -n "$inherited_keyring" ]]; then
    echo "KMS key $key inherits cryptoKeyEncrypterDecrypter grants from project or key-ring scope" >&2
    exit 1
  fi
  direct_members=$(gcloud kms keys get-iam-policy "$key" \
    --project="$project" --location="$region" --keyring="$prefix" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/cloudkms.cryptoKeyEncrypterDecrypter' \
    --format='value(bindings.members)' |
    sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ "$direct_members" != "$expected_members" ]]; then
    echo "KMS key $key has unexpected direct cryptoKeyEncrypterDecrypter principals" >&2
    exit 1
  fi
  echo "SECURITY KMS key=$key primary=enabled rotation=90d direct_crypto_principals=$expected_csv inherited_crypto_principals=none"
}

verify_internal_signing_key() {
  local key=internal-jwt-signing key_json version version_json role
  local inherited_project inherited_keyring direct_members expected_members
  local deploy_sa="pymes-v3-gh-deploy-${PYMES_DEPLOY_ENV}@${project}.iam.gserviceaccount.com"
  local expected_signers expected_viewers
  local -a selected_versions=("$PYMES_INTERNAL_KMS_KEY_VERSION")
  expected_signers=$(printf '%s\n' \
    "serviceAccount:$api_sa" \
    "serviceAccount:$worker_sa" \
    "serviceAccount:$provision_sa" | LC_ALL=C sort -u)
  expected_viewers=$(printf '%s\n' \
    "$expected_signers" \
    "serviceAccount:$deploy_sa" | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  for version in "${overlap_versions[@]}"; do
    [[ -z "$version" ]] || selected_versions+=("$version")
  done
  if [[ "$(printf '%s\n' "${selected_versions[@]}" | LC_ALL=C sort -u | wc -l)" -ne "${#selected_versions[@]}" ]]; then
    echo "internal signing key versions must not contain duplicates" >&2
    exit 2
  fi
  if [[ "$dry_run" == "true" ]]; then
    echo "SECURITY KMS key=$key versions=enabled-ed25519 direct_signers=api,worker,provisioner direct_public_viewers=api,worker,provisioner,deployer inherited_principals=none"
    return
  fi
  key_json=$(gcloud kms keys describe "$key" \
    --project="$project" --location="$region" --keyring="$prefix" \
    --format=json)
  jq -e '
    .purpose == "ASYMMETRIC_SIGN" and
    .versionTemplate.algorithm == "EC_SIGN_ED25519"
  ' <<<"$key_json" >/dev/null || {
    echo "KMS key $key must be an asymmetric Ed25519 signing key" >&2
    exit 1
  }
  for version in "${selected_versions[@]}"; do
    version_json=$(gcloud kms keys versions describe "${version##*/}" \
      --project="$project" --location="$region" --keyring="$prefix" \
      --key="$key" --format=json)
    jq -e '
      .state == "ENABLED" and
      .algorithm == "EC_SIGN_ED25519"
    ' <<<"$version_json" >/dev/null || {
      echo "selected internal signing version is not enabled Ed25519: $version" >&2
      exit 1
    }
  done
  for role in roles/cloudkms.signer roles/cloudkms.publicKeyViewer; do
    inherited_project=$(gcloud projects get-iam-policy "$project" \
      --flatten='bindings[].members' --filter="bindings.role=$role" \
      --format='value(bindings.members)' |
      sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
    inherited_keyring=$(gcloud kms keyrings get-iam-policy "$prefix" \
      --project="$project" --location="$region" \
      --flatten='bindings[].members' --filter="bindings.role=$role" \
      --format='value(bindings.members)' |
      sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
    if [[ -n "$inherited_project" || -n "$inherited_keyring" ]]; then
      echo "$role must not be inherited by $key from project or key-ring scope" >&2
      exit 1
    fi
    direct_members=$(gcloud kms keys get-iam-policy "$key" \
      --project="$project" --location="$region" --keyring="$prefix" \
      --flatten='bindings[].members' --filter="bindings.role=$role" \
      --format='value(bindings.members)' |
      sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
    if [[ "$role" == "roles/cloudkms.signer" ]]; then
      expected_members=$expected_signers
    else
      expected_members=$expected_viewers
    fi
    if [[ "$direct_members" != "$expected_members" ]]; then
      echo "KMS key $key has unexpected direct $role principals" >&2
      exit 1
    fi
  done
  echo "SECURITY KMS key=$key versions=enabled-ed25519 direct_signers=api,worker,provisioner direct_public_viewers=api,worker,provisioner,deployer inherited_principals=none"
}

service_url() {
  local service="$1"
  if [[ "$dry_run" == "true" ]]; then
    printf 'https://%s.%s.run.internal.invalid' "$service" "$region"
    return
  fi
  gcloud run services describe "$service" --region="$region" --format='value(status.url)'
}

candidate_service_url() {
  local service="$1" service_json candidate_url
  if [[ "$dry_run" == "true" ]]; then
    candidate_url=$(printf 'https://%s---%s.%s.run.internal.invalid' \
      "$candidate_tag" "$service" "$region"
    )
  else
    service_json=$(gcloud run services describe "$service" \
      --project="$project" --region="$region" --format=json)
    candidate_url=$(jq -er --arg tag "$candidate_tag" '
      [
        (.status.traffic // .trafficStatuses // [])[]
        | select(.tag == $tag)
        | .url
      ] | select(length == 1) | .[0]
    ' <<<"$service_json")
  fi
  pymes_validate_cloud_run_tagged_url \
    "$candidate_url" "$candidate_tag" "$service"
  printf '%s' "$candidate_url"
}

ensure_service_invoker() {
  local service="$1" expected_principal="$2"
  echo "SECURITY IAM service=$service required_invoker=$expected_principal"
  if [[ "$dry_run" == "true" ]]; then
    gcloud_command run services add-iam-policy-binding "$service" \
      --region="$region" --member="$expected_principal" \
      --role=roles/run.invoker --quiet
    return
  fi
  gcloud_command run services add-iam-policy-binding "$service" \
    --region="$region" --member="$expected_principal" \
    --role=roles/run.invoker --quiet >/dev/null
}

verify_no_project_invokers() {
  local invokers
  if [[ "$dry_run" == "true" ]]; then
    echo "SECURITY PROJECT project=$project direct_roles/run.invoker=none"
    return
  fi
  invokers=$(gcloud projects get-iam-policy "$project" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/run.invoker' \
    --format='value(bindings.members)' | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ -n "$invokers" ]]; then
    echo "shared project $project must not grant roles/run.invoker at project scope; got: $invokers" >&2
    echo "move each reviewed caller to the target Cloud Run service policy, then rerun this deployment" >&2
    exit 1
  fi
  echo "SECURITY PROJECT project=$project direct_roles/run.invoker=none"
}

service_invoker_iam_check_enabled() {
  jq -e '
    (
      .metadata.annotations["run.googleapis.com/invoker-iam-disabled"] //
      .invokerIamDisabled //
      false
    ) as $disabled
    | (($disabled | tostring | ascii_downcase) != "true")
  ' >/dev/null
}

verify_private_service() {
  local service="$1" expected_csv="$2" ingress invokers expected_invokers json
  expected_invokers=$(tr ',' '\n' <<<"$expected_csv" |
    sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ "$dry_run" == "true" ]]; then
    echo "SECURITY SERVICE service=$service ingress=internal unauthenticated=denied exact_invokers=${expected_csv:-none}"
    return
  fi

  json=$(gcloud run services describe "$service" \
    --project="$project" --region="$region" --format=json)
  ingress=$(jq -r '
    .metadata.annotations["run.googleapis.com/ingress"] //
    .ingress //
    empty
  ' <<<"$json")
  if [[ "$ingress" != "internal" ]]; then
    echo "private service $service has unexpected ingress: ${ingress:-unset}" >&2
    exit 1
  fi
  if ! service_invoker_iam_check_enabled <<<"$json"; then
    echo "private service $service has the Cloud Run invoker IAM check disabled" >&2
    exit 1
  fi
  invokers=$(gcloud run services get-iam-policy "$service" --region="$region" \
    --flatten='bindings[].members' \
    --filter='bindings.role=roles/run.invoker' \
    --format='value(bindings.members)' | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u)
  if [[ "$invokers" != "$expected_invokers" ]]; then
    echo "private service $service must have exactly these roles/run.invoker members: ${expected_csv:-none}; got: ${invokers:-none}" >&2
    echo "remove unexpected invokers explicitly after reviewing their owners, then rerun this deployment" >&2
    exit 1
  fi
  echo "SECURITY SERVICE service=$service ingress=internal unauthenticated=denied exact_invokers=${expected_csv:-none}"
}

require_secret() {
  local secret="$1"
  local metadata version state
  if [[ "$dry_run" == "true" ]]; then
    secret_versions["$secret"]="DRY_RUN"
    return
  fi
  metadata=$(gcloud_command secrets versions describe latest --secret="$secret" --format='value(name.basename(),state)' 2>/dev/null || true)
  read -r version state <<<"$metadata"
  if [[ -z "$version" || "$state" != "ENABLED" ]]; then
    echo "required secret has no enabled version: $secret" >&2
    exit 1
  fi
  secret_versions["$secret"]="$version"
}

verify_clerk_webhook_secret_lifecycle() {
  local secret="$prefix-clerk-webhook-secret" lifecycle source
  if [[ "$dry_run" == "true" ]]; then
    lifecycle=${PYMES_CLERK_WEBHOOK_SECRET_LIFECYCLE_DRY_RUN:-}
    source=dry-run-simulation
  else
    lifecycle=$(gcloud secrets describe "$secret" \
      --project="$project" --format='value(labels.lifecycle)')
    source=secret-manager
  fi
  if [[ "$deploy_stage" == "bootstrap" ]]; then
    if [[ "$lifecycle" != "bootstrap-temporary" ]]; then
      echo "$secret must have label lifecycle=bootstrap-temporary during bootstrap" >&2
      exit 1
    fi
  elif [[ "$lifecycle" == "bootstrap-temporary" ]]; then
    echo "$secret still has lifecycle=bootstrap-temporary; rotate to the real Clerk endpoint secret and remove the label before operational deployment" >&2
    exit 1
  fi
  echo "SECURITY CLERK_WEBHOOK_SECRET secret=$secret lifecycle=${lifecycle:-unset} stage=$deploy_stage source=$source"
}

declare -A secret_versions=()
required_secrets=(
  "$prefix-clerk-secret-key" "$prefix-clerk-webhook-secret" \
  "$prefix-scheduling-action-token-secret" \
  "$prefix-database-url" "$prefix-worker-database-url" \
  "$prefix-migrate-database-url" "$prefix-fiscal-database-url" "$prefix-fiscal-migrate-database-url" \
  "$prefix-accounting-database-url" "$prefix-accounting-admin-database-url" \
  "$prefix-accounting-migrate-database-url"
)
if [[ "$pergo_enabled" == "true" ]]; then
  required_secrets+=(
    "$prefix-pergo-api-key"
    "$prefix-pergo-webhook-secrets"
  )
fi
for secret in "${required_secrets[@]}"; do
  require_secret "$secret"
done
if [[ -n "$google_client_secret_name" ]]; then
  require_secret "$google_client_secret_name"
fi
verify_clerk_webhook_secret_lifecycle
verify_network_egress
if [[ "$dry_run" == "true" ]]; then
  secret_manager_agent="serviceAccount:service-DRY_RUN@gcp-sa-secretmanager.iam.gserviceaccount.com"
else
  project_number=$(gcloud projects describe "$project" \
    --format='value(projectNumber)')
  : "${project_number:?could not resolve GCP project number}"
  secret_manager_agent="serviceAccount:service-${project_number}@gcp-sa-secretmanager.iam.gserviceaccount.com"
  pymes_verify_release_inverse_authority \
    "$project" "$project_number" "$region" "$PYMES_DEPLOY_ENV" present pymes kms
fi
verify_data_key "$secrets_kms_key" "$secret_manager_agent"
verify_data_key "$calendar_kms_key" \
  "serviceAccount:$api_sa,serviceAccount:$worker_sa"
verify_data_key "$fiscal_kms_key" "serviceAccount:$fiscal_sa"
verify_internal_signing_key

secret_ref() {
  printf '%s:%s' "$1" "${secret_versions[$1]}"
}

existing_sidecars() {
  local kind="$1" name="$2" json list_json count
  if [[ "$dry_run" == "true" ]]; then
    return
  fi
  if [[ "$kind" == "service" ]]; then
    list_json=$(gcloud run services list \
      --project="$project" --region="$region" \
      --filter="metadata.name=$name" --format=json)
    count=$(jq -er --arg name "$name" '
      def resource_name:
        .metadata.name //
        ((.name // "") | split("/") | last);
      [.[] | select(resource_name == $name)] | length
    ' <<<"$list_json")
    [[ "$count" == "0" ]] && return
    [[ "$count" == "1" ]] || {
      echo "service inventory returned $count exact entries for $name" >&2
      return 1
    }
    json=$(gcloud run services describe "$name" \
      --project="$project" --region="$region" --format=json)
    jq -r '
      (.spec.template.spec.containers // .template.containers // [])[1:]
      | .[].name // empty
    ' <<<"$json"
    return
  fi
  list_json=$(gcloud run jobs list \
    --project="$project" --region="$region" \
    --filter="metadata.name=$name" --format=json)
  count=$(jq -er --arg name "$name" '
    def resource_name:
      .metadata.name //
      ((.name // "") | split("/") | last);
    [.[] | select(resource_name == $name)] | length
  ' <<<"$list_json")
  [[ "$count" == "0" ]] && return
  [[ "$count" == "1" ]] || {
    echo "job inventory returned $count exact entries for $name" >&2
    return 1
  }
  json=$(gcloud run jobs describe "$name" \
    --project="$project" --region="$region" --format=json)
  jq -r '
    (
      .spec.template.spec.template.spec.containers //
      .template.template.containers //
      []
    )[1:] | .[].name // empty
  ' <<<"$json"
}

append_sidecar_removal() {
  local kind="$1" name="$2" arguments_name="$3" sidecars
  local -n arguments_ref="$arguments_name"
  sidecars=$(existing_sidecars "$kind" "$name" | paste -sd, -)
  if [[ -n "$sidecars" ]]; then
    arguments_ref+=(--remove-containers="$sidecars")
  fi
}

declare -A previous_revisions=()
declare -A previous_active_tags=()
declare -A previous_all_tags=()
declare -A previous_tag_urls=()
declare -A candidate_revisions=()
declare -A candidate_urls=()
declare -A candidate_deploy_started=()
declare -A service_existed=()
declare -a release_secret_files=()
previous_web_api_tag_mapping=
previous_web_api_url=
previous_web_api_token=

create_cloud_run_environment_file() {
  local environment="$1" output_name="$2" path
  path=$(mktemp)
  chmod 600 "$path"
  if ! jq -en --arg raw "$environment" '
      ($raw | split("|")) as $entries
      | [
          $entries[]
          | capture(
              "^(?<key>[A-Za-z_][A-Za-z0-9_]*)=(?<value>.*)$"
            )
        ] as $parsed
      | select(
          ($parsed | length) == ($entries | length) and
          ($parsed | map(.key) | unique | length) == ($parsed | length)
        )
      | $parsed
      | from_entries
    ' >"$path"; then
    rm -f -- "$path"
    echo "failed to render an unambiguous Cloud Run environment file" >&2
    return 1
  fi
  release_secret_files+=("$path")
  printf -v "$output_name" '%s' "$path"
}

cleanup_release_secret_files() {
  local path
  for path in "${release_secret_files[@]:-}"; do
    [[ -n "$path" ]] && rm -f -- "$path"
  done
  release_secret_files=()
}

capture_previous_revision() {
  local service="$1" json active active_tags all_tags tag_urls list_json count
  if [[ "$dry_run" == "true" ]]; then
    service_existed["$service"]=true
    previous_active_tags["$service"]=
    previous_all_tags["$service"]=
    previous_tag_urls["$service"]=
    if [[ ",${PYMES_CLOUD_RUN_ACTIVE_SERVICES_DRY_RUN:-}," == *",$service,"* ]]; then
      active="${service}-simulated-active"
      if [[ "$deploy_stage" == "bootstrap" ]]; then
        echo "bootstrap refuses service $service because it already has active traffic" >&2
        exit 1
      fi
      previous_revisions["$service"]=$active
      echo "RELEASE BASELINE service=$service active_revision=$active dry_run=true"
      return
    fi
    previous_revisions["$service"]=
    echo "RELEASE BASELINE service=$service active_revision=none dry_run=true"
    return
  fi
  list_json=$(gcloud run services list \
    --project="$project" --region="$region" \
    --filter="metadata.name=$service" --format=json)
  count=$(jq -er --arg service "$service" '
    def resource_name:
      .metadata.name //
      ((.name // "") | split("/") | last);
    [.[] | select(resource_name == $service)] | length
  ' <<<"$list_json")
  if [[ "$count" == "0" ]]; then
    service_existed["$service"]=false
    previous_revisions["$service"]=
    previous_active_tags["$service"]=
    previous_all_tags["$service"]=
    previous_tag_urls["$service"]=
    echo "RELEASE BASELINE service=$service active_revision=none first_deploy=true"
    return
  fi
  if [[ "$count" != "1" ]]; then
    echo "service inventory returned $count exact entries for $service" >&2
    exit 1
  fi
  json=$(gcloud run services describe "$service" \
    --project="$project" --region="$region" --format=json)
  service_existed["$service"]=true
  active=$(jq -er '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select((.percent // 0) > 0)
    ] as $active
    | if ($active | length) == 0 then
        ""
      elif (
        ($active | length) == 1 and
        ($active[0].percent // 0) == 100 and
        (($active[0].revisionName // $active[0].revision // "") | length) > 0
      ) then
        ($active[0].revisionName // $active[0].revision)
      else
        error("invalid active traffic")
      end
  ' <<<"$json") || {
    echo "service $service must have zero active traffic or exactly one explicit active revision at 100% before deployment" >&2
    exit 1
  }
  if [[ "$deploy_stage" == "bootstrap" && -n "$active" ]]; then
    echo "bootstrap refuses service $service because it already has active traffic" >&2
    exit 1
  fi
  active_tags=$(jq -er --arg active "$active" '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select(
          ($active | length) > 0 and
          ((.revisionName // .revision // "") == $active) and
          ((.tag // "") | length) > 0
        )
      | {
          tag: .tag,
          revision: (.revisionName // .revision // "")
        }
    ] as $tags
    | if all(
        $tags[];
        (.tag | test("^[a-z][a-z0-9-]{0,62}$")) and
        (.revision | test("^[a-z][a-z0-9-]{0,62}$"))
      ) then
        [$tags[] | "\(.tag)=\(.revision)"] | join(",")
      else
        error("unsafe active tag")
      end
  ' <<<"$json") || {
    echo "service $service has an unsafe tag on its active revision" >&2
    exit 1
  }
  all_tags=$(jq -er '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select(((.tag // "") | length) > 0)
      | {
          tag: .tag,
          revision: (.revisionName // .revision // ""),
          url: (.url // "")
        }
    ] as $tags
    | if all(
        $tags[];
        (.tag | test("^[a-z][a-z0-9-]{0,62}$")) and
        (.revision | test("^[a-z][a-z0-9-]{0,62}$")) and
        (.url | test("^https://[^/?#]+$"))
      ) then
        [$tags[] | "\(.tag)=\(.revision)"] | join(",")
      else
        error("unsafe tag inventory")
      end
  ' <<<"$json") || {
    echo "service $service has an unsafe tag, revision or URL" >&2
    exit 1
  }
  tag_urls=$(jq -er '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select(((.tag // "") | length) > 0)
      | .url
    ] | join("\n")
  ' <<<"$json")
  previous_revisions["$service"]=$active
  previous_active_tags["$service"]=$active_tags
  previous_all_tags["$service"]=$all_tags
  previous_tag_urls["$service"]=$tag_urls
  echo "RELEASE BASELINE service=$service active_revision=${active:-none} active_tags=${active_tags:-none} all_tags=${all_tags:-none}"
}

revision_env_value() {
  local revision_json="$1" name="$2"
  jq -er --arg name "$name" '
    (
      .spec.containers[0].env //
      .spec.template.spec.containers[0].env //
      .containers[0].env //
      []
    )
    | [.[] | select(.name == $name) | (.value // "")]
    | select(length == 1)
    | .[0]
  ' <<<"$revision_json"
}

validate_release_baseline() {
  local service web_previous api_previous web_json api_json
  local upstream web_token api_token mapping
  if [[ "$dry_run" == "true" ]]; then
    return
  fi
  for service in "${release_services[@]}"; do
    [[ "$service" == "$prefix-api" ]] && continue
    if [[ -n "${previous_all_tags[$service]:-}" ]]; then
      echo "baseline service $service has historical or active tags; only the active API tag is permitted" >&2
      exit 1
    fi
  done
  web_previous=${previous_revisions[$prefix-web]:-}
  api_previous=${previous_revisions[$prefix-api]:-}
  if [[ -z "$web_previous" && -z "$api_previous" ]]; then
    if [[ -n "${previous_all_tags[$prefix-api]:-}" ]]; then
      echo "inactive API baseline must not retain tagged URLs" >&2
      exit 1
    fi
    return
  fi
  if [[ -z "$web_previous" || -z "$api_previous" ]]; then
    echo "Web and API baselines must both be active or both be inert" >&2
    exit 1
  fi
  web_json=$(gcloud run revisions describe "$web_previous" \
    --project="$project" --region="$region" --format=json)
  api_json=$(gcloud run revisions describe "$api_previous" \
    --project="$project" --region="$region" --format=json)
  upstream=$(revision_env_value "$web_json" PYMES_API_UPSTREAM) || {
    echo "active Web baseline has no unique PYMES_API_UPSTREAM" >&2
    exit 1
  }
  web_token=$(revision_env_value "$web_json" PYMES_PREFLIGHT_TOKEN) || {
    echo "active Web baseline has no durable preflight capability metadata" >&2
    exit 1
  }
  api_token=$(revision_env_value "$api_json" PYMES_PREFLIGHT_TOKEN) || {
    echo "active API baseline has no durable preflight capability metadata" >&2
    exit 1
  }
  if [[ ! "$web_token" =~ ^[0-9a-f]{64}$ || "$web_token" != "$api_token" ]]; then
    echo "active Web and API baseline capabilities do not match" >&2
    exit 1
  fi
  mapping=$(gcloud run services describe "$prefix-api" \
    --project="$project" --region="$region" --format=json |
    jq -er \
      --arg active "$api_previous" \
      --arg upstream "$upstream" '
      [
        (.status.traffic // .trafficStatuses // [])[]
        | select(
            ((.tag // "") | test("^c-[0-9a-f]{16}$")) and
            ((.revisionName // .revision // "") == $active) and
            (.url == $upstream)
          )
        | "\(.tag)=\(.revisionName // .revision)"
      ] | select(length == 1) | .[0]
    ') || {
    echo "the active Web baseline does not point to the exact active tagged API revision" >&2
    exit 1
  }
  if [[ "${previous_all_tags[$prefix-api]:-}" != "$mapping" ]]; then
    echo "API baseline must expose exactly the tag consumed by the active Web revision" >&2
    exit 1
  fi
  if [[ "${mapping%%=*}" == "$candidate_tag" ]]; then
    echo "release $PYMES_RELEASE_SHA is already the active Web/API pair; refusing to reuse its capability tag" >&2
    exit 1
  fi
  previous_web_api_tag_mapping=$mapping
  previous_web_api_url=$upstream
  previous_web_api_token=$web_token
  echo "RELEASE BASELINE PAIR web_revision=$web_previous api_revision=$api_previous api_tag=${mapping%%=*}"
}

record_candidate_revision() {
  local service="$1" json candidate candidate_url
  if [[ "$dry_run" == "true" ]]; then
    candidate="${service}-dry-run-${PYMES_RELEASE_SHA:0:8}"
    candidate_url="https://${candidate_tag}---${service}.${region}.run.internal.invalid"
  else
    json=$(gcloud run services describe "$service" \
      --project="$project" --region="$region" --format=json)
    candidate=$(jq -er '
      .status.latestCreatedRevisionName //
      .status.latestCreatedRevision //
      empty
    ' <<<"$json")
    candidate_revisions["$service"]=$candidate
    candidate_url=$(jq -er --arg tag "$candidate_tag" '
      [
        (.status.traffic // .trafficStatuses // [])[]
        | select(.tag == $tag)
        | .url
      ] | select(length == 1) | .[0]
    ' <<<"$json") || {
      echo "service $service did not expose one candidate URL for cleanup" >&2
      exit 1
    }
    jq -e --arg tag "$candidate_tag" --arg candidate "$candidate" '
      (.status.latestReadyRevisionName // .status.latestReadyRevision // "") as $ready
      | [
          (.status.traffic // .trafficStatuses // [])[]
          | select(.tag == $tag)
          | (.revisionName // .revision // "")
        ] as $tagged
      | select(
          ($candidate | length) > 0 and
          $candidate == $ready and
          ($tagged | length) == 1 and
          $tagged[0] == $candidate
        )
    ' <<<"$json" >/dev/null || {
      echo "service $service did not produce one ready revision tagged $candidate_tag" >&2
      exit 1
    }
  fi
  pymes_validate_cloud_run_tagged_url \
    "$candidate_url" "$candidate_tag" "$service"
  candidate_revisions["$service"]=$candidate
  candidate_urls["$service"]=$candidate_url
  echo "RELEASE CANDIDATE service=$service revision=$candidate tag=$candidate_tag traffic=0 gate=required"
}

deploy() {
  local service="$1" image="$2" service_account="$3" secrets="$4" environment="$5" ingress="$6" min_instances="$7" access="$8" cpu="$9" network_mode="${10}" deploy_health_check="${11:-enabled}" scaling_mode="${12:-auto}"
  local environment_file=
  local -a arguments=(
    --region="$region" --image="$image" --service-account="$service_account"
    --labels="app=pymes-v3,env=$PYMES_DEPLOY_ENV,pymes-v3-release=$PYMES_RELEASE_SHA"
    --ingress="$ingress" --min-instances=0
    --set-cloudsql-instances="$PYMES_CLOUDSQL_INSTANCE"
    --set-secrets="$secrets" --quiet
    --command="" --args="" --clear-volumes --clear-volume-mounts
    --no-traffic --tag="$candidate_tag"
    --invoker-iam-check
    --startup-probe=httpGet.path=/readyz,httpGet.port=8080,initialDelaySeconds=0,timeoutSeconds=2,periodSeconds=5,failureThreshold=12
    --readiness-probe=httpGet.path=/readyz,httpGet.port=8080,timeoutSeconds=2,periodSeconds=5,failureThreshold=3,successThreshold=1
    --liveness-probe=httpGet.path=/healthz,httpGet.port=8080,initialDelaySeconds=5,timeoutSeconds=2,periodSeconds=30,failureThreshold=3
  )
  if [[ "$dry_run" == "true" ]]; then
    arguments+=(--set-env-vars="^|^${environment//PYMES_PREFLIGHT_TOKEN=$PYMES_PREFLIGHT_TOKEN/PYMES_PREFLIGHT_TOKEN=[REDACTED]}")
  else
    create_cloud_run_environment_file "$environment" environment_file
    arguments+=(--env-vars-file="$environment_file")
  fi
  if [[ "$deploy_health_check" == "enabled" ]]; then
    arguments+=(--deploy-health-check)
  else
    arguments+=(--no-deploy-health-check)
  fi
  if [[ "$scaling_mode" == "manual-zero" ]]; then
    arguments+=(--scaling=0)
  else
    arguments+=(--scaling=auto --min="$min_instances" --max=1)
  fi
  if [[ "$access" == "public" ]]; then
    arguments+=(--allow-unauthenticated)
  else
    arguments+=(--no-allow-unauthenticated)
  fi
  if [[ "$cpu" == "always" ]]; then
    arguments+=(--no-cpu-throttling)
  else
    arguments+=(--cpu-throttling)
  fi
  if [[ "$network_mode" == "direct" ]]; then
    arguments+=(--network="$network" --subnet="$subnet" --vpc-egress=all-traffic)
  fi
  append_sidecar_removal service "$service" arguments
  candidate_deploy_started["$service"]=true
  if ! gcloud_command run deploy "$service" "${arguments[@]}"; then
    [[ -n "$environment_file" ]] && rm -f -- "$environment_file"
    return 1
  fi
  [[ -n "$environment_file" ]] && rm -f -- "$environment_file"
  record_candidate_revision "$service"
}

deploy_web() {
  local service="$1" image="$2" service_account="$3" api_upstream="$4"
  local ingress=all access_flag=--allow-unauthenticated
  local environment environment_file=
  if [[ "$deploy_stage" == "bootstrap" ]]; then
    ingress=internal
    access_flag=--no-allow-unauthenticated
  fi
  environment="PYMES_API_UPSTREAM=$api_upstream|PYMES_RELEASE_MARKER=$web_release_marker|PYMES_PREFLIGHT_TAG=$candidate_tag|PYMES_PREFLIGHT_TOKEN=$PYMES_PREFLIGHT_TOKEN"
  local -a arguments=(
    --region="$region" --image="$image" --service-account="$service_account" \
    --labels="app=pymes-v3,env=$PYMES_DEPLOY_ENV,pymes-v3-release=$PYMES_RELEASE_SHA" \
    --ingress="$ingress" --scaling=auto --min=0 --min-instances=0 --max=1 --cpu-throttling \
    --clear-secrets --command="" --args="" --clear-volumes --clear-volume-mounts \
    --no-traffic --tag="$candidate_tag" \
    --invoker-iam-check \
    "$access_flag" --quiet \
    --deploy-health-check \
    --startup-probe=httpGet.path=/readyz,httpGet.port=8080,initialDelaySeconds=0,timeoutSeconds=2,periodSeconds=5,failureThreshold=12 \
    --readiness-probe=httpGet.path=/readyz,httpGet.port=8080,timeoutSeconds=2,periodSeconds=5,failureThreshold=3,successThreshold=1 \
    --liveness-probe=httpGet.path=/healthz,httpGet.port=8080,initialDelaySeconds=5,timeoutSeconds=2,periodSeconds=30,failureThreshold=3
  )
  if [[ "$dry_run" == "true" ]]; then
    arguments+=(--set-env-vars="^|^${environment//PYMES_PREFLIGHT_TOKEN=$PYMES_PREFLIGHT_TOKEN/PYMES_PREFLIGHT_TOKEN=[REDACTED]}")
  else
    create_cloud_run_environment_file "$environment" environment_file
    arguments+=(--env-vars-file="$environment_file")
  fi
  append_sidecar_removal service "$service" arguments
  candidate_deploy_started["$service"]=true
  if ! gcloud_command run deploy "$service" "${arguments[@]}"; then
    [[ -n "$environment_file" ]] && rm -f -- "$environment_file"
    return 1
  fi
  [[ -n "$environment_file" ]] && rm -f -- "$environment_file"
  record_candidate_revision "$service"
}

migrate() {
  local job="$1" image="$2" service_account="$3" secrets="$4"
  local -a arguments=(
    --region="$region" --image="$image" --service-account="$service_account" \
    --labels="app=pymes-v3,env=$PYMES_DEPLOY_ENV,pymes-v3-release=$PYMES_RELEASE_SHA" \
    --set-cloudsql-instances="$PYMES_CLOUDSQL_INSTANCE" --set-secrets="$secrets" \
    --clear-env-vars --command="" --args="" --clear-volumes --clear-volume-mounts \
    --tasks=1 --max-retries=0 --execute-now --wait --quiet
  )
  append_sidecar_removal job "$job" arguments
  gcloud_command run jobs deploy "$job" "${arguments[@]}"
}

run_job() {
  local job="$1" image="$2" service_account="$3" secrets="$4" environment="$5"
  local -a arguments=(
    --region="$region" --image="$image" --service-account="$service_account" \
    --labels="app=pymes-v3,env=$PYMES_DEPLOY_ENV,pymes-v3-release=$PYMES_RELEASE_SHA" \
    --set-cloudsql-instances="$PYMES_CLOUDSQL_INSTANCE" --set-secrets="$secrets" \
    --set-env-vars="^|^$environment" \
    --command="" --args="" --clear-volumes --clear-volume-mounts \
    --tasks=1 --max-retries=0 --execute-now --wait --quiet
  )
  append_sidecar_removal job "$job" arguments
  gcloud_command run jobs deploy "$job" "${arguments[@]}"
}

deploy_job_template() {
  local job="$1" image="$2" service_account="$3" secrets="$4" environment="$5"
  local -a arguments=(
    --region="$region" --image="$image" --service-account="$service_account" \
    --labels="app=pymes-v3,env=$PYMES_DEPLOY_ENV,pymes-v3-release=$PYMES_RELEASE_SHA" \
    --set-cloudsql-instances="$PYMES_CLOUDSQL_INSTANCE" --set-secrets="$secrets" \
    --set-env-vars="^|^$environment" \
    --command="" --args="" --clear-volumes --clear-volume-mounts \
    --network="$network" --subnet="$subnet" --vpc-egress=all-traffic \
    --tasks=1 --max-retries=0 --quiet
  )
  append_sidecar_removal job "$job" arguments
  gcloud_command run jobs deploy "$job" "${arguments[@]}"
}

assert_active_revision() {
  local service="$1" expected="$2" json
  [[ "$dry_run" == "true" ]] && return
  json=$(gcloud run services describe "$service" \
    --project="$project" --region="$region" --format=json)
  jq -e --arg expected "$expected" '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select((.percent // 0) > 0)
    ] as $active
    | ($active | length) == 1 and
      ($active[0].percent // 0) == 100 and
      ($active[0].revisionName // $active[0].revision // "") == $expected
  ' <<<"$json" >/dev/null || {
    echo "service $service does not route exactly 100% to revision $expected" >&2
    return 1
  }
}

assert_worker_manual_scaling() {
  local expected="$1" json
  [[ "$dry_run" == "true" ]] && return
  json=$(gcloud run services describe "$prefix-worker" \
    --project="$project" --region="$region" --format=json)
  jq -e --argjson expected "$expected" '
    (
      .metadata.annotations["run.googleapis.com/scalingMode"] //
      .scaling.scalingMode //
      ""
    ) as $mode
    | (
        .metadata.annotations["run.googleapis.com/manualInstanceCount"] //
        .scaling.manualInstanceCount //
        -1
      ) as $count
    | ($mode | ascii_downcase) == "manual" and
      ($count | tonumber) == $expected
  ' <<<"$json" >/dev/null || {
    echo "worker manual scaling does not equal $expected" >&2
    return 1
  }
}

fail_close_service() {
  local service="$1" service_json policy_json policy_file
  local update_failed=false policy_failed=false delete_failed=false
  case "$service" in
    "$prefix-api"|"$prefix-web"|"$prefix-fiscal"|"$prefix-accounting"|\
      "$prefix-accounting-admin"|"$prefix-worker") ;;
    *)
      echo "refusing to fail-close unknown service $service" >&2
      return 1
      ;;
  esac
  if service_absent "$service"; then
    echo "ROLLBACK REMOVED service=$service previous_revision=none already_absent=true"
    return
  fi
  gcloud run services update "$service" \
    --project="$project" --region="$region" \
    --ingress=internal --invoker-iam-check --quiet >/dev/null ||
    update_failed=true
  policy_json=$(gcloud run services get-iam-policy "$service" \
    --project="$project" --region="$region" --format=json)
  policy_file=$(mktemp)
  jq '
    .bindings = [
      (.bindings // [])[]
      | select(.role != "roles/run.invoker")
    ]
  ' <<<"$policy_json" >"$policy_file"
  gcloud run services set-iam-policy "$service" "$policy_file" \
    --project="$project" --region="$region" --quiet >/dev/null ||
    policy_failed=true
  rm -f -- "$policy_file"
  service_json=$(gcloud run services describe "$service" \
    --project="$project" --region="$region" --format=json)
  if ! service_invoker_iam_check_enabled <<<"$service_json"; then
    echo "ROLLBACK FAILED service=$service invoker IAM check remains disabled" >&2
    return 1
  fi
  if [[ "$(jq -r '
      .metadata.annotations["run.googleapis.com/ingress"] //
      .ingress //
      empty
    ' <<<"$service_json")" != "internal" ]]; then
    echo "ROLLBACK FAILED service=$service ingress is not fail-closed" >&2
    return 1
  fi
  if [[ -n "$(gcloud run services get-iam-policy "$service" \
      --project="$project" --region="$region" \
      --flatten='bindings[].members' \
      --filter='bindings.role=roles/run.invoker' \
      --format='value(bindings.members)' |
      sed '/^[[:space:]]*$/d')" ]]; then
    echo "ROLLBACK FAILED service=$service still has Cloud Run invokers" >&2
    return 1
  fi
  if [[ "$update_failed" == "true" || "$policy_failed" == "true" ]]; then
    echo "ROLLBACK FAIL-CLOSE command failed but readback proved closed state service=$service" >&2
  fi
  gcloud run services delete "$service" \
    --project="$project" --region="$region" \
    --quiet >/dev/null || delete_failed=true
  assert_service_absent "$service" || return 1
  if [[ "$delete_failed" == "true" ]]; then
    echo "ROLLBACK DELETE command failed but readback proved absence service=$service" >&2
  fi
  echo "ROLLBACK REMOVED service=$service previous_revision=none fail_closed_verified=true"
}

declare -a promoted_services=()
promotion_started=false
release_complete=false
worker_scaling_paused=false

service_was_promoted() {
  local expected="$1" promoted
  for promoted in "${promoted_services[@]}"; do
    [[ "$promoted" == "$expected" ]] && return 0
  done
  return 1
}

restore_previous_api_tags() {
  local service="$prefix-api"
  local tags=$previous_web_api_tag_mapping json failed=false
  if [[ -z "$tags" ]]; then
    return
  fi
  gcloud run services update-traffic "$service" \
    --project="$project" --region="$region" \
    --update-tags="$tags" --quiet >/dev/null || failed=true
  json=$(gcloud run services describe "$service" \
    --project="$project" --region="$region" --format=json)
  jq -e --arg mapping "$tags" '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select(((.tag // "") | length) > 0)
      | "\(.tag)=\(.revisionName // .revision // "")"
    ] | index($mapping) != null
  ' <<<"$json" >/dev/null || {
    echo "ROLLBACK FAILED restoring exact API tag mapping $tags" >&2
    return 1
  }
  wait_for_tagged_api_ready \
    previous-api \
    "$previous_web_api_url" \
    "$previous_web_api_token" || return 1
  if [[ "$failed" == "true" ]]; then
    echo "ROLLBACK API TAG command failed but readback proved the exact mapping" >&2
  fi
  echo "ROLLBACK API TAGS RESTORED service=$service tags=$tags" >&2
}

wait_for_tagged_api_ready() {
  local label="$1" url="$2" token="$3" config_file
  local attempt result status effective redirect
  [[ "$dry_run" == "true" ]] && return
  if [[ ! "$token" =~ ^[0-9a-f]{64}$ ]]; then
    echo "cannot verify $label because its preflight capability is invalid" >&2
    return 1
  fi
  config_file=$(mktemp)
  chmod 600 "$config_file"
  printf 'header = "X-Pymes-Preflight-Token: %s"\n' "$token" >"$config_file"
  for attempt in {1..6}; do
    result=$(curl --disable --proto '=https' --tlsv1.2 --silent --show-error \
      --max-redirs 0 --connect-timeout 5 --max-time 5 \
      --config "$config_file" \
      --output /dev/null \
      --write-out=$'%{http_code}\n%{url_effective}\n%{redirect_url}' \
      "${url}/readyz" 2>/dev/null) || result=
    mapfile -t curl_result <<<"$result"
    status=${curl_result[0]:-}
    effective=${curl_result[1]:-}
    redirect=${curl_result[2]:-}
    if [[ "$status" == "200" &&
          "$effective" == "${url}/readyz" &&
          -z "$redirect" ]]; then
      rm -f -- "$config_file"
      echo "ROLLBACK API TAG VERIFIED label=$label status=200" >&2
      return
    fi
    sleep 5
  done
  rm -f -- "$config_file"
  echo "tagged API $label did not become ready within 60 seconds" >&2
  return 1
}

service_absent() {
  local service="$1" list_json count
  list_json=$(gcloud run services list \
    --project="$project" --region="$region" \
    --filter="metadata.name=$service" --format=json) || return 1
  count=$(jq -er --arg service "$service" '
    def resource_name:
      .metadata.name //
      ((.name // "") | split("/") | last);
    [.[] | select(resource_name == $service)] | length
  ' <<<"$list_json") || return 1
  [[ "$count" == "0" ]]
}

discover_candidate_revision() {
  local service="$1" json candidate url
  service_absent "$service" && return 1
  json=$(gcloud run services describe "$service" \
    --project="$project" --region="$region" --format=json) || return 1
  candidate=$(jq -er \
    --arg tag "$candidate_tag" \
    --arg release "$PYMES_RELEASE_SHA" '
    (.status.latestCreatedRevisionName //
      .status.latestCreatedRevision //
      "") as $created
    | (
        .spec.template.metadata.labels["pymes-v3-release"] //
        .template.labels["pymes-v3-release"] //
        ""
      ) as $release_label
    |
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select(.tag == $tag)
      | (.revisionName // .revision // "")
    ] as $tagged
    | if ($tagged | length) == 1 then
        $tagged[0]
      elif ($tagged | length) == 0 and
        $release_label == $release and
        ($created | length) > 0 then
        $created
      else
        error("candidate revision is ambiguous")
      end
  ' <<<"$json") || return 1
  url=$(jq -r --arg tag "$candidate_tag" '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select(.tag == $tag)
      | .url
    ] | if length == 1 then .[0] else "" end
  ' <<<"$json")
  candidate_revisions["$service"]=$candidate
  candidate_urls["$service"]=$url
}

assert_tag_absent() {
  local service="$1" json
  json=$(gcloud run services describe "$service" \
    --project="$project" --region="$region" --format=json)
  jq -e --arg tag "$candidate_tag" '
    [
      (.status.traffic // .trafficStatuses // [])[]
      | select(.tag == $tag)
    ] | length == 0
  ' <<<"$json" >/dev/null
}

assert_public_tag_url_revoked() {
  local label="$1" url="$2" attempt status
  [[ -z "$url" || "$dry_run" == "true" ]] && return
  for attempt in {1..6}; do
    status=$(curl --disable --proto '=https' --tlsv1.2 --silent --show-error \
      --max-redirs 0 --connect-timeout 5 --max-time 5 \
      --output /dev/null --write-out='%{http_code}' "$url" 2>/dev/null) ||
      status=000
    if [[ "$status" == "404" ]]; then
      echo "RELEASE URL REVOKED label=$label status=404"
      return
    fi
    sleep 5
  done
  echo "tag URL remained reachable after bounded revocation wait: $label status=$status" >&2
  return 1
}

remove_candidate_tag() {
  local service="$1" url
  if [[ "${candidate_deploy_started[$service]:-}" != "true" ]]; then
    return
  fi
  if service_absent "$service"; then
    return
  fi
  if [[ -z "${candidate_revisions[$service]:-}" ]] &&
    ! discover_candidate_revision "$service"; then
    echo "ROLLBACK FAILED resolving candidate mutation service=$service tag=$candidate_tag" >&2
    return 1
  fi
  url=${candidate_urls[$service]:-}
  gcloud run services update-traffic "$service" \
    --project="$project" --region="$region" \
    --remove-tags="$candidate_tag" --quiet >/dev/null
  assert_tag_absent "$service" || return 1
  case "$service" in
    "$prefix-api"|"$prefix-web")
      assert_public_tag_url_revoked "$service/$candidate_tag" "$url"
      ;;
  esac
  gcloud run revisions delete "${candidate_revisions[$service]}" \
    --project="$project" --region="$region" \
    --quiet >/dev/null
  assert_revision_absent "${candidate_revisions[$service]}"
}

remove_nonworker_candidate_tags() {
  local service failed=false
  for service in "${release_services[@]}"; do
    [[ "$service" == "$prefix-worker" ]] && continue
    [[ "${candidate_deploy_started[$service]:-}" != "true" ]] && continue
    if ! remove_candidate_tag "$service"; then
      echo "ROLLBACK FAILED removing candidate tag service=$service tag=$candidate_tag" >&2
      failed=true
    fi
    if [[ -z "${previous_revisions[$service]:-}" ]] &&
      ! fail_close_service "$service"; then
      failed=true
    fi
  done
  [[ "$failed" == "false" ]]
}

settle_release_tags() {
  local service candidate json
  for service in "${release_services[@]}"; do
    candidate=${candidate_revisions[$service]:-}
    : "${candidate:?missing candidate revision for $service}"
    if [[ "$service" == "$prefix-api" ]]; then
      gcloud_command run services update-traffic "$service" \
        --project="$project" --region="$region" \
        --set-tags="$candidate_tag=$candidate" --quiet
      echo "RELEASE TAGS service=$service policy=current-api-only tag=$candidate_tag revision=$candidate"
    else
      gcloud_command run services update-traffic "$service" \
        --project="$project" --region="$region" \
        --clear-tags --quiet
      echo "RELEASE TAGS service=$service policy=none"
    fi
    if [[ "$dry_run" != "true" ]]; then
      json=$(gcloud run services describe "$service" \
        --project="$project" --region="$region" --format=json)
      jq -e \
        --arg service "$service" \
        --arg api "$prefix-api" \
        --arg tag "$candidate_tag" \
        --arg candidate "$candidate" '
        [
          (.status.traffic // .trafficStatuses // [])[]
          | select(((.tag // "") | length) > 0)
        ] as $tags
        | if $service == $api then
            ($tags | length) == 1 and
            $tags[0].tag == $tag and
            (($tags[0].revisionName // $tags[0].revision // "") == $candidate)
          else
            ($tags | length) == 0
          end
      ' <<<"$json" >/dev/null || {
        echo "service $service did not converge to the settled tag policy" >&2
        return 1
      }
    fi
  done
  if [[ -n "$previous_web_api_url" ]]; then
    assert_public_tag_url_revoked previous-api "$previous_web_api_url"
  fi
  assert_public_tag_url_revoked candidate-web "${candidate_urls[$prefix-web]:-}"
}

settle_bootstrap_tags() {
  local service json
  for service in "${release_services[@]}"; do
    gcloud_command run services update-traffic "$service" \
      --project="$project" --region="$region" \
      --clear-tags --quiet
    if [[ "$dry_run" != "true" ]]; then
      json=$(gcloud run services describe "$service" \
        --project="$project" --region="$region" --format=json)
      jq -e '
        [
          (.status.traffic // .trafficStatuses // [])[]
          | select(((.tag // "") | length) > 0)
        ] | length == 0
      ' <<<"$json" >/dev/null || {
        echo "bootstrap service $service retained a routable revision tag" >&2
        return 1
      }
    fi
    echo "BOOTSTRAP TAGS service=$service policy=none"
  done
}

assert_revision_absent() {
  local revision="$1" json count
  json=$(gcloud run revisions list \
    --project="$project" --region="$region" \
    --filter="metadata.name=$revision" --format=json) || return 1
  count=$(jq -er --arg revision "$revision" '
    def resource_name:
      .metadata.name //
      ((.name // "") | split("/") | last);
    [.[] | select(resource_name == $revision)] | length
  ' <<<"$json") || return 1
  if [[ "$count" != "0" ]]; then
    echo "candidate revision $revision still exists after quiescence" >&2
    return 1
  fi
}

assert_service_absent() {
  local service="$1"
  if ! service_absent "$service"; then
    echo "first-deploy worker service $service still exists after quiescence" >&2
    return 1
  fi
}

quiesce_worker_candidate() {
  local mode="$1" service="$prefix-worker"
  local candidate=${candidate_revisions[$service]:-}
  local previous=${previous_revisions[$service]:-}
  local failed=false
  if [[ -z "$candidate" &&
        "${candidate_deploy_started[$service]:-}" == "true" ]] &&
    ! service_absent "$service"; then
    if ! discover_candidate_revision "$service"; then
      echo "WORKER QUIESCENCE FAILED resolving attempted candidate service=$service" >&2
      return 1
    fi
    candidate=${candidate_revisions[$service]:-}
  fi
  if [[ -z "$candidate" ]]; then
    if [[ "$worker_scaling_paused" == "true" && "$previous" != "" ]]; then
      gcloud run services update "$service" \
        --project="$project" --region="$region" \
        --scaling=1 --quiet >/dev/null || failed=true
      assert_worker_manual_scaling 1 || failed=true
    fi
    [[ "$failed" == "false" ]]
    return
  fi

  echo "WORKER QUIESCENCE START service=$service revision=$candidate mode=$mode" >&2
  gcloud run services update "$service" \
    --project="$project" --region="$region" \
    --scaling=0 --quiet >/dev/null || failed=true
  assert_worker_manual_scaling 0 || failed=true

  if [[ -n "$previous" ]]; then
    if [[ "$mode" == "promoted" ]]; then
      gcloud run services update-traffic "$service" \
        --project="$project" --region="$region" \
        --to-revisions="$previous=100" --quiet >/dev/null || failed=true
      assert_active_revision "$service" "$previous" || failed=true
    fi
    gcloud run services update-traffic "$service" \
      --project="$project" --region="$region" \
      --remove-tags="$candidate_tag" --quiet >/dev/null || failed=true
    gcloud run revisions delete "$candidate" \
      --project="$project" --region="$region" \
      --quiet >/dev/null || failed=true
    assert_revision_absent "$candidate" || failed=true
    gcloud run services update "$service" \
      --project="$project" --region="$region" \
      --scaling=1 --quiet >/dev/null || failed=true
    assert_worker_manual_scaling 1 || failed=true
    assert_active_revision "$service" "$previous" || failed=true
  else
    fail_close_service "$service" || failed=true
  fi

  if [[ "$failed" == "true" ]]; then
    echo "WORKER QUIESCENCE FAILED service=$service revision=$candidate mode=$mode" >&2
    return 1
  fi
  unset 'candidate_revisions[$service]'
  echo "WORKER QUIESCENCE COMPLETE service=$service revision=$candidate mode=$mode result=$([[ -n "$previous" ]] && echo baseline-reactivated || echo first-deploy-service-removed)" >&2
}

rollback_on_exit() {
  local status=$?
  local index service previous rollback_failed=false
  trap - EXIT INT TERM
  cleanup_release_secret_files
  if [[ "$release_complete" == "true" ]]; then
    exit "$status"
  fi
  set +e
  if [[ "$promotion_started" != "true" ]]; then
    echo "ROLLBACK PRETRAFFIC release=$PYMES_RELEASE_SHA" >&2
    if ! quiesce_worker_candidate unpromoted; then
      echo "ROLLBACK INCOMPLETE: worker candidate requires immediate operator intervention" >&2
    fi
    if ! remove_nonworker_candidate_tags; then
      echo "ROLLBACK INCOMPLETE: one or more candidate tags require immediate operator intervention" >&2
    fi
    exit "$status"
  fi
  echo "ROLLBACK START release=$PYMES_RELEASE_SHA promoted=${#promoted_services[@]}" >&2
  if ! restore_previous_api_tags; then
    echo "ROLLBACK FAILED restoring the API tag required by the previous Web revision" >&2
    rollback_failed=true
  fi
  for ((index=${#promoted_services[@]} - 1; index >= 0; index--)); do
    service=${promoted_services[$index]}
    previous=${previous_revisions[$service]:-}
    if [[ "$service" == "$prefix-worker" ]]; then
      if ! quiesce_worker_candidate promoted; then
        rollback_failed=true
      fi
      continue
    fi
    if [[ -n "$previous" ]]; then
      gcloud run services update-traffic "$service" \
        --project="$project" --region="$region" \
        --to-revisions="$previous=100" --quiet >/dev/null
      if [[ $? -ne 0 ]] || ! assert_active_revision "$service" "$previous"; then
        echo "ROLLBACK FAILED service=$service revision=$previous" >&2
        rollback_failed=true
      else
        echo "ROLLBACK RESTORED service=$service revision=$previous"
      fi
    elif ! fail_close_service "$service"; then
      rollback_failed=true
    fi
  done
  if ! service_was_promoted "$prefix-worker" &&
    ! quiesce_worker_candidate unpromoted; then
    rollback_failed=true
  fi
  if ! remove_nonworker_candidate_tags; then
    rollback_failed=true
  fi
  if [[ "$rollback_failed" == "true" ]]; then
    echo "ROLLBACK INCOMPLETE: affected services require immediate operator intervention" >&2
  else
    echo "ROLLBACK COMPLETE release=$PYMES_RELEASE_SHA" >&2
  fi
  exit "$status"
}

wait_for_worker_release_ready() {
  local revision="$1" attempt result
  if [[ "$dry_run" == "true" ]]; then
    echo "WORKER RELEASE SIGNAL revision=$revision release=$PYMES_RELEASE_SHA ready=true dry_run=true"
    return
  fi
  for attempt in {1..24}; do
    result=$(gcloud logging read \
      "resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"$prefix-worker\" AND resource.labels.revision_name=\"$revision\" AND jsonPayload.event=\"worker_release_ready\" AND jsonPayload.ready=true AND jsonPayload.release_sha=\"$PYMES_RELEASE_SHA\" AND jsonPayload.revision=\"$revision\"" \
      --project="$project" --freshness=10m --limit=1 --format=json) ||
      return 1
    if jq -e 'length == 1' <<<"$result" >/dev/null; then
      echo "WORKER RELEASE SIGNAL revision=$revision release=$PYMES_RELEASE_SHA ready=true"
      return
    fi
    sleep 5
  done
  echo "worker revision $revision emitted no exact release-ready signal within 120 seconds" >&2
  return 1
}

promote_service() {
  local service="$1" candidate
  candidate=${candidate_revisions[$service]:-}
  : "${candidate:?missing candidate revision for $service}"
  if [[ "$service" == "$prefix-worker" ]]; then
    gcloud_command run services update "$service" \
      --project="$project" --region="$region" \
      --scaling=0 --quiet
    assert_worker_manual_scaling 0
  fi
  promoted_services+=("$service")
  promotion_started=true
  gcloud_command run services update-traffic "$service" \
    --project="$project" --region="$region" \
    --to-revisions="$candidate=100" --quiet
  assert_active_revision "$service" "$candidate"
  if [[ "$service" == "$prefix-worker" ]]; then
    gcloud_command run services update "$service" \
      --project="$project" --region="$region" \
      --scaling=1 --quiet
    assert_worker_manual_scaling 1
    wait_for_worker_release_ready "$candidate"
  fi
  echo "RELEASE PROMOTED service=$service revision=$candidate traffic=100"
}

fiscal_service="$prefix-fiscal"
accounting_service="$prefix-accounting"
accounting_admin_service="$prefix-accounting-admin"
release_services=(
  "$fiscal_service"
  "$accounting_service"
  "$accounting_admin_service"
  "$prefix-worker"
  "$prefix-api"
  "$prefix-web"
)
for release_service in "${release_services[@]}"; do
  capture_previous_revision "$release_service"
done
validate_release_baseline

if [[ "$dry_run" != "true" ]]; then
  trap rollback_on_exit EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
fi

migrate "$prefix-migrate" "$PYMES_MIGRATE_IMAGE" "$migrate_sa" \
  "PYMES_DATABASE_URL=$(secret_ref "$prefix-migrate-database-url")"
migrate "$prefix-fiscal-migrate" "$PYMES_FISCAL_MIGRATE_IMAGE" "$fiscal_migrate_sa" \
  "FISCAL_DATABASE_URL=$(secret_ref "$prefix-fiscal-migrate-database-url")"
migrate "$prefix-accounting-migrate" "$PYMES_ACCOUNTING_MIGRATE_IMAGE" "$accounting_migrate_sa" \
  "DATABASE_URL=$(secret_ref "$prefix-accounting-migrate-database-url")"
run_job "$prefix-accounting-grants" "$PYMES_ACCOUNTING_ADMIN_IMAGE" \
  "$accounting_admin_sa" \
  "ACCOUNTING_ADMIN_DATABASE_URL=$(secret_ref "$prefix-accounting-admin-database-url")" \
  "ACCOUNTING_ADMIN_OPERATION=sync-runtime-grants|ACCOUNTING_RUNTIME_ROLE=pymes_v3_accounting_${PYMES_DEPLOY_ENV}|ACCOUNTING_OWNER_ROLE=pymes_v3_accounting_owner_${PYMES_DEPLOY_ENV}"

deploy "$fiscal_service" "$PYMES_FISCAL_IMAGE" "$fiscal_sa" \
  "FISCAL_DATABASE_URL=$(secret_ref "$prefix-fiscal-database-url")" \
  "${fiscal_environment}|PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_JWKS_JSON=$PYMES_INTERNAL_JWKS_JSON|PORT=8080" internal 0 private throttled direct
deploy "$accounting_service" "$PYMES_ACCOUNTING_IMAGE" "$accounting_sa" \
  "ACCOUNTING_DATABASE_URL=$(secret_ref "$prefix-accounting-database-url")" \
  "PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_JWKS_JSON=$PYMES_INTERNAL_JWKS_JSON|PORT=8080" internal 0 private throttled none
deploy "$accounting_admin_service" "$PYMES_ACCOUNTING_ADMIN_IMAGE" \
  "$accounting_admin_sa" \
  "ACCOUNTING_ADMIN_DATABASE_URL=$(secret_ref "$prefix-accounting-admin-database-url")" \
  "ACCOUNTING_ADMIN_OPERATION=serve|ACCOUNTING_RUNTIME_ROLE=pymes_v3_accounting_${PYMES_DEPLOY_ENV}|ACCOUNTING_OWNER_ROLE=pymes_v3_accounting_owner_${PYMES_DEPLOY_ENV}|PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_JWKS_JSON=$PYMES_INTERNAL_JWKS_JSON|PORT=8080" \
  internal 0 private throttled none

api_principal="serviceAccount:$api_sa"
worker_principal="serviceAccount:$worker_sa"
provision_principal="serviceAccount:$provision_sa"
verify_no_project_invokers
ensure_service_invoker "$fiscal_service" "$api_principal"
ensure_service_invoker "$fiscal_service" "$worker_principal"
ensure_service_invoker "$accounting_service" "$worker_principal"
ensure_service_invoker "$accounting_admin_service" "$provision_principal"
verify_private_service "$fiscal_service" "$api_principal,$worker_principal"
verify_private_service "$accounting_service" "$worker_principal"
verify_private_service "$accounting_admin_service" "$provision_principal"

fiscal_url=$(service_url "$fiscal_service")
accounting_url=$(service_url "$accounting_service")
accounting_admin_url=$(service_url "$accounting_admin_service")
action_token_secret_ref=$(secret_ref "$prefix-scheduling-action-token-secret")
api_secrets="PYMES_CLERK_SECRET_KEY=$(secret_ref "$prefix-clerk-secret-key"),PYMES_CLERK_WEBHOOK_SECRET=$(secret_ref "$prefix-clerk-webhook-secret"),PYMES_DATABASE_URL=$(secret_ref "$prefix-database-url"),PYMES_SCHEDULING_ACTION_TOKEN_SECRET=$action_token_secret_ref"
worker_secrets="PYMES_DATABASE_URL=$(secret_ref "$prefix-worker-database-url"),PYMES_SCHEDULING_ACTION_TOKEN_SECRET=$action_token_secret_ref"
if [[ -n "$google_client_secret_name" ]]; then
  google_client_secret_ref=$(secret_ref "$google_client_secret_name")
  api_secrets="$api_secrets,PYMES_GOOGLE_CLIENT_SECRET=$google_client_secret_ref"
  worker_secrets="$worker_secrets,PYMES_GOOGLE_CLIENT_SECRET=$google_client_secret_ref"
fi
api_environment="FISCAL_ADAPTER_URL=$fiscal_url|PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_KMS_KEY_VERSION=$PYMES_INTERNAL_KMS_KEY_VERSION|PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS=$PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS|PYMES_CLERK_ISSUER=$PYMES_CLERK_ISSUER|PYMES_CLERK_AUDIENCE=pymes-v3|PYMES_CLERK_AUTHORIZED_PARTIES=$PYMES_CLERK_AUTHORIZED_PARTIES|PYMES_HTTP_ADDR=:8080|PYMES_PREFLIGHT_TAG=$candidate_tag|PYMES_PREFLIGHT_TOKEN=$PYMES_PREFLIGHT_TOKEN${calendar_environment}${tracing_environment}"
worker_environment="FISCAL_ADAPTER_URL=$fiscal_url|ACCOUNTING_URL=$accounting_url|PYMES_ENVIRONMENT=production|PYMES_RELEASE_SHA=$PYMES_RELEASE_SHA|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_KMS_KEY_VERSION=$PYMES_INTERNAL_KMS_KEY_VERSION|PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS=$PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS|PYMES_WORKER_HTTP_ADDR=:8080|PYMES_WORKER_INTERVAL_MS=250|PYMES_WORKER_METRICS_INTERVAL=60s${calendar_environment}${tracing_environment}"
if [[ "$pergo_enabled" == "true" ]]; then
  api_secrets+=",PERGO_WEBHOOK_SECRETS=$(secret_ref "$prefix-pergo-webhook-secrets")"
  api_environment+="|PYMES_PERGO_ENABLED=true|PERGO_WORKSPACE_ID=$pergo_workspace_id"
  worker_secrets+=",PERGO_API_KEY=$(secret_ref "$prefix-pergo-api-key")"
  worker_environment+="|PYMES_PERGO_ENABLED=true|PERGO_URL=$pergo_url|PERGO_WORKSPACE_ID=$pergo_workspace_id|PERGO_CHANNEL=$pergo_channel|PERGO_ALLOW_GLOBAL_ROUTE_FALLBACK=false|PERGO_TIMEOUT=5s"
else
  api_environment+="|PYMES_PERGO_ENABLED=false"
  worker_environment+="|PYMES_PERGO_ENABLED=false"
fi
api_ingress=all
api_access=public
if [[ "$deploy_stage" == "bootstrap" ]]; then
  api_ingress=internal
  api_access=private
fi
deploy "$prefix-api" "$PYMES_API_IMAGE" "$api_sa" \
  "$api_secrets" \
  "$api_environment" "$api_ingress" 0 "$api_access" throttled direct
api_url=$(candidate_service_url "$prefix-api")
deploy_web "$prefix-web" "$PYMES_WEB_IMAGE" "$web_sa" "$api_url"
if [[ "$deploy_stage" == "operational" ]]; then
  if [[ "$dry_run" == "true" || "${service_existed[$prefix-worker]}" == "true" ]]; then
    gcloud_command run services update "$prefix-worker" \
      --project="$project" --region="$region" \
      --scaling=0 --quiet
    assert_worker_manual_scaling 0
  fi
  worker_scaling_paused=true
fi
deploy "$prefix-worker" "$PYMES_WORKER_IMAGE" "$worker_sa" \
  "$worker_secrets" \
  "$worker_environment" internal 0 private always direct disabled manual-zero
verify_private_service "$prefix-worker" ""

deploy_job_template "$prefix-provision-org" "$PYMES_PROVISION_IMAGE" \
  "$provision_sa" \
  "PYMES_DATABASE_URL=$(secret_ref "$prefix-database-url")" \
  "ACCOUNTING_PROVISIONING_URL=$accounting_admin_url|PYMES_ENVIRONMENT=production|PYMES_INTERNAL_ISSUER=pymes-v3|PYMES_INTERNAL_KMS_KEY_VERSION=$PYMES_INTERNAL_KMS_KEY_VERSION|PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS=$PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS"

if [[ "$dry_run" == "true" ]]; then
  echo "PRETRAFFIC VERIFY release=$PYMES_RELEASE_SHA tag=$candidate_tag traffic=0 shape=exact same_origin=no-redirects"
else
  PYMES_DEPLOY_STAGE="$deploy_stage" \
    PYMES_CLOUD_RUN_VERIFY_PHASE=pretraffic \
    PYMES_CLOUD_RUN_CANDIDATE_TAG="$candidate_tag" \
    "$script_dir/verify-cloud-run.sh"
fi

if [[ "$deploy_stage" == "bootstrap" ]]; then
  settle_bootstrap_tags
  echo "BOOTSTRAP COMPLETE environment=stg release=$PYMES_RELEASE_SHA traffic=0 ingress=internal unauthenticated=denied worker_min=0 tag_public_access=denied promotion=skipped"
  echo "bootstrap web service URL: $(service_url "$prefix-web")"
  echo "next: configure that exact Web origin in Clerk, create the disabled webhook endpoint /api/v1/webhooks/clerk, rotate the signing secret, remove lifecycle=bootstrap-temporary, then run operational"
  cleanup_release_secret_files
  release_complete=true
  trap - EXIT INT TERM
  exit 0
fi

if [[ "$dry_run" == "true" ]]; then
  for release_service in "${release_services[@]}"; do
    echo "ROLLBACK PLAN service=$release_service previous_revision=${previous_revisions[$release_service]:-none} fail_safe=remove-invokers"
  done
  echo "WORKER QUIESCENCE PLAN revision=${candidate_revisions[$prefix-worker]} revision_min=0 deployment_health_check=disabled pretraffic_scaling=0 promotion_scaling=1 rollback_scaling=0"
fi

for release_service in \
  "$fiscal_service" \
  "$accounting_service" \
  "$accounting_admin_service" \
  "$prefix-api" \
  "$prefix-web" \
  "$prefix-worker"; do
  promote_service "$release_service"
done

settle_release_tags

if [[ "$dry_run" == "true" ]]; then
  echo "ACTIVE VERIFY release=$PYMES_RELEASE_SHA traffic=exact-100 tags=api-current-only same_origin=no-redirects"
else
  PYMES_DEPLOY_STAGE="$deploy_stage" \
    PYMES_CLOUD_RUN_VERIFY_PHASE=active \
    PYMES_CLOUD_RUN_CANDIDATE_TAG="$candidate_tag" \
    "$script_dir/verify-cloud-run.sh"
fi
cleanup_release_secret_files
release_complete=true
trap - EXIT INT TERM

echo "deployed $prefix in shared project $project; configure Clerk webhook for ${PYMES_PUBLIC_BASE_URL}/api/v1/webhooks/clerk"
if [[ "$pergo_enabled" == "true" ]]; then
  echo "configure PerGo workspace=$pergo_workspace_id callback=${PYMES_PUBLIC_BASE_URL}/api/v1/webhooks/pergo"
fi
echo "web: $(service_url "$prefix-web") public_origin=$PYMES_PUBLIC_BASE_URL same_origin_api=/api/"
echo "organization provisioning job: $prefix-provision-org (execute with explicit --id, --name, --slug and --clerk-organization-id args)"
