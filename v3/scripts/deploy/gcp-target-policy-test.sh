#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=gcp-target-policy.sh
source "$script_dir/gcp-target-policy.sh"

expect_rejected() {
  local description="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "expected target policy rejection: $description" >&2
    exit 1
  fi
}

pymes_require_canonical_project "$PYMES_GCP_TARGET_PROJECT"
pymes_require_canonical_project_region \
  "$PYMES_GCP_TARGET_PROJECT" "$PYMES_GCP_TARGET_REGION"
pymes_require_canonical_project_number "$PYMES_GCP_TARGET_PROJECT_NUMBER"
pymes_require_canonical_artifact_repository \
  "$PYMES_GCP_TARGET_ARTIFACT_REPOSITORY"
pymes_require_canonical_network_target \
  "$PYMES_GCP_TARGET_NETWORK" \
  "$PYMES_GCP_TARGET_SUBNET" \
  "$PYMES_GCP_TARGET_SUBNET_CIDR" \
  "$PYMES_GCP_TARGET_NAT_ROUTER" \
  "$PYMES_GCP_TARGET_NAT_NAME"
pymes_require_canonical_cloudsql_connection \
  "${PYMES_GCP_TARGET_PROJECT}:${PYMES_GCP_TARGET_REGION}:${PYMES_GCP_TARGET_CLOUDSQL_INSTANCE}"
canonical_kms_version="projects/${PYMES_GCP_TARGET_PROJECT}/locations/${PYMES_GCP_TARGET_REGION}/keyRings/pymes-v3-stg/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/1"
pymes_require_canonical_internal_kms_versions \
  stg "$canonical_kms_version" ""
pymes_require_canonical_internal_kms_versions \
  stg "$canonical_kms_version" \
  "${canonical_kms_version},projects/${PYMES_GCP_TARGET_PROJECT}/locations/${PYMES_GCP_TARGET_REGION}/keyRings/pymes-v3-stg/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/2"

expect_rejected "foreign project" \
  pymes_require_canonical_project unrelated-project
expect_rejected "foreign region" \
  pymes_require_canonical_project_region \
  "$PYMES_GCP_TARGET_PROJECT" europe-west1
expect_rejected "foreign project number" \
  pymes_require_canonical_project_number 111111111111
expect_rejected "foreign Artifact Registry repository" \
  pymes_require_canonical_artifact_repository unrelated
expect_rejected "shared network resource outside Pymes" \
  pymes_require_canonical_network_target \
  "$PYMES_GCP_TARGET_NETWORK" unrelated-subnet \
  "$PYMES_GCP_TARGET_SUBNET_CIDR" \
  "$PYMES_GCP_TARGET_NAT_ROUTER" \
  "$PYMES_GCP_TARGET_NAT_NAME"
expect_rejected "foreign Cloud SQL connection" \
  pymes_require_canonical_cloudsql_connection \
  "unrelated-project:us-central1:pymes-dev-db"
expect_rejected "foreign internal KMS project" \
  pymes_require_canonical_internal_kms_versions \
  stg \
  "projects/unrelated-project/locations/us-central1/keyRings/pymes-v3-stg/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/1" \
  ""
expect_rejected "cross-environment internal KMS overlap" \
  pymes_require_canonical_internal_kms_versions \
  stg "$canonical_kms_version" \
  "projects/${PYMES_GCP_TARGET_PROJECT}/locations/${PYMES_GCP_TARGET_REGION}/keyRings/pymes-v3-prd/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/1"

echo "Pymes v3 canonical GCP target policy verified"
