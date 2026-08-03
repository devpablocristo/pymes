#!/usr/bin/env bash

# Canonical deployment boundary for Pymes v3. Mutating scripts source this
# file and fail before their first GCP write when an operator or workflow tries
# to redirect them to another project, region or shared resource.

readonly PYMES_GCP_TARGET_PROJECT=pymes-dev-352318
readonly PYMES_GCP_TARGET_PROJECT_NUMBER=884236221349
readonly PYMES_GCP_TARGET_REGION=us-central1
readonly PYMES_GCP_TARGET_ARTIFACT_REPOSITORY=pymes
readonly PYMES_GCP_TARGET_NETWORK=default
readonly PYMES_GCP_TARGET_SUBNET=pymes-v3-serverless
readonly PYMES_GCP_TARGET_SUBNET_CIDR=10.120.0.0/24
readonly PYMES_GCP_TARGET_NAT_ROUTER=pymes-v3-serverless
readonly PYMES_GCP_TARGET_NAT_NAME=pymes-v3-serverless
readonly PYMES_GCP_TARGET_CLOUDSQL_INSTANCE=pymes-dev-db

pymes_require_canonical_project() {
  local project=${1:-}
  [[ "$project" == "$PYMES_GCP_TARGET_PROJECT" ]] || {
    echo "Pymes v3 GCP operations are restricted to project $PYMES_GCP_TARGET_PROJECT" >&2
    return 2
  }
}

pymes_require_canonical_project_region() {
  local project=${1:-} region=${2:-}
  pymes_require_canonical_project "$project" || return
  [[ "$region" == "$PYMES_GCP_TARGET_REGION" ]] || {
    echo "Pymes v3 GCP operations are restricted to region $PYMES_GCP_TARGET_REGION" >&2
    return 2
  }
}

pymes_require_canonical_project_number() {
  local project_number=${1:-}
  [[ "$project_number" == "$PYMES_GCP_TARGET_PROJECT_NUMBER" ]] || {
    echo "Pymes v3 GCP operations require project number $PYMES_GCP_TARGET_PROJECT_NUMBER" >&2
    return 2
  }
}

pymes_require_canonical_artifact_repository() {
  local repository=${1:-}
  [[ "$repository" == "$PYMES_GCP_TARGET_ARTIFACT_REPOSITORY" ]] || {
    echo "Pymes v3 image operations are restricted to Artifact Registry repository $PYMES_GCP_TARGET_ARTIFACT_REPOSITORY" >&2
    return 2
  }
}

pymes_require_canonical_network_target() {
  local network=${1:-} subnet=${2:-} subnet_cidr=${3:-}
  local router=${4:-} nat=${5:-}
  [[ "$network" == "$PYMES_GCP_TARGET_NETWORK" &&
     "$subnet" == "$PYMES_GCP_TARGET_SUBNET" &&
     "$subnet_cidr" == "$PYMES_GCP_TARGET_SUBNET_CIDR" &&
     "$router" == "$PYMES_GCP_TARGET_NAT_ROUTER" &&
     "$nat" == "$PYMES_GCP_TARGET_NAT_NAME" ]] || {
    echo "Pymes v3 network operations are restricted to the reviewed pymes-v3-serverless resources" >&2
    return 2
  }
}

pymes_require_canonical_cloudsql_connection() {
  local connection_name=${1:-}
  local expected="${PYMES_GCP_TARGET_PROJECT}:${PYMES_GCP_TARGET_REGION}:${PYMES_GCP_TARGET_CLOUDSQL_INSTANCE}"
  [[ "$connection_name" == "$expected" ]] || {
    echo "Pymes v3 database operations are restricted to Cloud SQL connection $expected" >&2
    return 2
  }
}

pymes_require_canonical_internal_kms_versions() {
  local environment=${1:-} primary=${2:-} overlap=${3:-}
  local version version_pattern
  case "$environment" in
    stg|prd) ;;
    *)
      echo "Pymes v3 internal KMS validation requires environment stg or prd" >&2
      return 2
      ;;
  esac
  version_pattern="^projects/${PYMES_GCP_TARGET_PROJECT}/locations/${PYMES_GCP_TARGET_REGION}/keyRings/pymes-v3-${environment}/cryptoKeys/internal-jwt-signing/cryptoKeyVersions/[1-9][0-9]*$"
  [[ "$primary" =~ $version_pattern ]] || {
    echo "PYMES_INTERNAL_KMS_KEY_VERSION must pin the canonical Pymes v3 ${environment} signing key" >&2
    return 2
  }
  IFS=',' read -r -a overlap_versions <<<"$overlap"
  for version in "${overlap_versions[@]}"; do
    [[ -z "$version" ]] && continue
    [[ "$version" =~ $version_pattern ]] || {
      echo "every PYMES_INTERNAL_KMS_OVERLAP_KEY_VERSIONS entry must pin the canonical Pymes v3 ${environment} signing key" >&2
      return 2
    }
  done
}
