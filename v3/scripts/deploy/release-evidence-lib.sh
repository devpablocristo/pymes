#!/usr/bin/env bash

# Shared, read-only validation for the immutable Pymes v3 release-evidence
# buckets. This file is sourced by the bootstrap and publication commands.

PYMES_RELEASE_EVIDENCE_PROJECT=pymes-dev-352318
PYMES_RELEASE_EVIDENCE_PROJECT_NUMBER=884236221349
PYMES_RELEASE_EVIDENCE_REGION=us-central1
PYMES_RELEASE_EVIDENCE_RETENTION_SECONDS=31557600
PYMES_RELEASE_EVIDENCE_BUILDER="pymes-v3-gh-build@${PYMES_RELEASE_EVIDENCE_PROJECT}.iam.gserviceaccount.com"

pymes_release_evidence_fail() {
  echo "release evidence: $*" >&2
  return 1
}

pymes_release_evidence_validate_environment() {
  case "$1" in
    stg|prd) ;;
    *)
      pymes_release_evidence_fail \
        "environment must be exactly stg or prd"
      return 2
      ;;
  esac
}

pymes_release_evidence_bucket_name() {
  local environment="$1"
  pymes_release_evidence_validate_environment "$environment" || return
  printf 'pymes-v3-release-evidence-%s-%s\n' \
    "$environment" "$PYMES_RELEASE_EVIDENCE_PROJECT_NUMBER"
}

pymes_release_evidence_validate_bucket() {
  local bucket_json="$1" environment="$2" require_locked="$3"
  local bucket
  bucket=$(pymes_release_evidence_bucket_name "$environment") || return
  case "$require_locked" in true|false) ;; *)
    pymes_release_evidence_fail "internal lock expectation is invalid"
    return 2
  esac

  jq -e \
    --arg bucket "$bucket" \
    --arg project_number "$PYMES_RELEASE_EVIDENCE_PROJECT_NUMBER" \
    --arg region "${PYMES_RELEASE_EVIDENCE_REGION^^}" \
    --arg environment "$environment" \
    --argjson retention "$PYMES_RELEASE_EVIDENCE_RETENTION_SECONDS" \
    --argjson require_locked "$require_locked" '
      .name == $bucket and
      ((.projectNumber | tostring) == $project_number) and
      (.location | ascii_upcase) == $region and
      .storageClass == "STANDARD" and
      .iamConfiguration.uniformBucketLevelAccess.enabled == true and
      .iamConfiguration.publicAccessPrevention == "enforced" and
      (.versioning.enabled // false) == false and
      ((.lifecycle.rule // []) | length) == 0 and
      (.retentionPolicy.retentionPeriod | tonumber) >= $retention and
      (
        if $require_locked
        then .retentionPolicy.isLocked == true
        else (.retentionPolicy.isLocked // false) | type == "boolean"
        end
      ) and
      .labels.app == "pymes-v3" and
      .labels.component == "release-evidence" and
      .labels.environment == $environment and
      .labels.managed_by == "pymes-v3"
    ' <<<"$bucket_json" >/dev/null || {
      pymes_release_evidence_fail \
        "bucket identity, protection, retention, lifecycle, or labels differ"
      return 1
    }
}

pymes_release_evidence_validate_bucket_iam() {
  local policy_json="$1"
  local builder_member="serviceAccount:${PYMES_RELEASE_EVIDENCE_BUILDER}"
  local expected actual
  expected=$(
    printf '%s\n' \
      roles/storage.legacyBucketReader \
      roles/storage.objectCreator \
      roles/storage.objectViewer |
      jq -Rsc 'split("\n") | map(select(length > 0)) | sort'
  )
  actual=$(jq -c --arg member "$builder_member" '
    [
      .bindings[]?
      | select((.members // []) | index($member) != null)
      | .role
    ] | sort
  ' <<<"$policy_json") || return

  jq -e \
    --argjson expected "$expected" \
    --argjson actual "$actual" \
    --arg member "$builder_member" '
      $actual == $expected and
      all(
        .bindings[]?;
        (
          ((.members // []) | index("allUsers") == null) and
          ((.members // []) | index("allAuthenticatedUsers") == null)
        )
      ) and
      all(
        .bindings[]?
        | select((.members // []) | index($member) != null);
        (.condition // null) == null
      )
    ' <<<"$policy_json" >/dev/null || {
      pymes_release_evidence_fail \
        "bucket IAM is public, conditional, or differs for the release builder"
      return 1
    }
}

pymes_release_evidence_validate_direct_auth() {
  local active_account impersonated
  for variable in \
    CLOUDSDK_AUTH_ACCESS_TOKEN \
    CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE \
    CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT; do
    if [[ -n "${!variable:-}" ]]; then
      pymes_release_evidence_fail \
        "refusing delegated or overridden gcloud credentials: $variable"
      return 1
    fi
  done
  active_account=$(gcloud config get-value account 2>/dev/null)
  impersonated=$(gcloud config get-value auth/impersonate_service_account \
    2>/dev/null || true)
  [[ "$active_account" == "softponti@gmail.com" ]] || {
    pymes_release_evidence_fail \
      "bootstrap requires the reviewed operator softponti@gmail.com"
    return 1
  }
  [[ -z "$impersonated" || "$impersonated" == "(unset)" ]] || {
    pymes_release_evidence_fail \
      "bootstrap refuses gcloud service-account impersonation"
    return 1
  }
}

pymes_release_evidence_validate_project() {
  local project_json="$1"
  jq -e \
    --arg project "$PYMES_RELEASE_EVIDENCE_PROJECT" \
    --arg project_number "$PYMES_RELEASE_EVIDENCE_PROJECT_NUMBER" '
      .projectId == $project and
      ((.projectNumber | tostring) == $project_number) and
      .lifecycleState == "ACTIVE"
    ' <<<"$project_json" >/dev/null || {
      pymes_release_evidence_fail "GCP project identity differs"
      return 1
    }
}
