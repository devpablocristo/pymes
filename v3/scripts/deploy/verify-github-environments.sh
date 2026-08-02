#!/usr/bin/env bash
set -euo pipefail

repository=${GITHUB_REPOSITORY:-devpablocristo/pymes}
requested_environment=${1:-all}
verification_scope=${2:-all-controls}
reviewer_ids_csv=${PYMES_PRD_REVIEWER_IDS:-}

[[ "$repository" == "devpablocristo/pymes" ]] || {
  echo "GitHub environment verification is restricted to devpablocristo/pymes" >&2
  exit 2
}
case "$requested_environment" in
  all) environments=(stg prd) ;;
  stg|prd) environments=("$requested_environment") ;;
  *) echo "environment must be stg, prd or all" >&2; exit 2 ;;
esac
case "$verification_scope" in
  all-controls|environment-only) ;;
  *) echo "verification scope must be all-controls or environment-only" >&2; exit 2 ;;
esac
for command in gh jq; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "$command is required" >&2
    exit 1
  }
done

expected_prd_reviewers_json=
if [[ "$verification_scope" == "all-controls" && "$requested_environment" != "stg" ]]; then
  [[ "$reviewer_ids_csv" =~ ^[1-9][0-9]*(,[1-9][0-9]*){0,5}$ ]] || {
    echo "PYMES_PRD_REVIEWER_IDS must name the exact one-to-six approved PRD reviewer IDs" >&2
    exit 2
  }
  expected_prd_reviewers_json=$(jq -cn --arg ids "$reviewer_ids_csv" '
    $ids
    | split(",")
    | map(tonumber)
    | if (unique | length) != length then error("duplicate reviewer ID") else . end
    | sort
  ')
fi

branch_json=$(gh api \
  -H 'X-GitHub-Api-Version: 2026-03-10' \
  "repos/${repository}/branches/main") || {
    echo "main branch metadata is unavailable" >&2
    exit 1
  }
jq -e '
  .protected == true and
  .protection.enabled == true and
  .protection.required_status_checks.enforcement_level == "everyone" and
  (.protection.required_status_checks.contexts | sort) == ["Pymes V3 validate"] and
  (.protection.required_status_checks.checks | length) == 1 and
  .protection.required_status_checks.checks[0].context == "Pymes V3 validate" and
  .protection.required_status_checks.checks[0].app_id == 15368
' <<<"$branch_json" >/dev/null || {
  echo "main is not protected for everyone by the exact Pymes V3 check" >&2
  exit 1
}
echo "GitHub main public protection verified: required=Pymes V3 validate enforcement=everyone"

if [[ "$verification_scope" == "all-controls" ]]; then
  branch_protection_json=$(gh api \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "repos/${repository}/branches/main/protection") || {
      echo "main branch protection does not exist" >&2
      exit 1
    }
  jq -e '
    .required_status_checks.strict == true and
    (.required_status_checks.checks | length) == 1 and
    .required_status_checks.checks[0].context == "Pymes V3 validate" and
    .required_status_checks.checks[0].app_id == 15368 and
    .enforce_admins.enabled == true and
    .required_pull_request_reviews == null and
    .restrictions == null and
    .required_linear_history.enabled == true and
    .required_conversation_resolution.enabled == true and
    .allow_force_pushes.enabled == false and
    .allow_deletions.enabled == false and
    .block_creations.enabled == false and
    .lock_branch.enabled == false and
    .allow_fork_syncing.enabled == false
  ' <<<"$branch_protection_json" >/dev/null || {
    echo "main branch protection differs from the reviewed Pymes V3 release policy" >&2
    exit 1
  }
  echo "GitHub main protection verified: required=Pymes V3 validate reviews=1 admins=enforced"
fi

for environment in "${environments[@]}"; do
  environment_json=$(gh api \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "repos/${repository}/environments/${environment}") || {
      echo "GitHub environment does not exist: $environment" >&2
      exit 1
    }
  jq -e --arg environment "$environment" '
    .name == $environment and
    .deployment_branch_policy.protected_branches == false and
    .deployment_branch_policy.custom_branch_policies == true and
    any(.protection_rules[]?; .type == "branch_policy") and
    (
      if $environment == "prd"
      then ([.protection_rules[]?.type] | sort) == ["branch_policy", "required_reviewers"]
      else ([.protection_rules[]?.type] | sort) == ["branch_policy"]
      end
    )
  ' <<<"$environment_json" >/dev/null || {
    echo "GitHub environment $environment has missing or unexpected protection rules" >&2
    exit 1
  }

  policies_json=$(gh api \
    -H 'X-GitHub-Api-Version: 2026-03-10' \
    "repos/${repository}/environments/${environment}/deployment-branch-policies")
  jq -e '
    .total_count == 1 and
    (.branch_policies | length) == 1 and
    .branch_policies[0].name == "main" and
    ((.branch_policies[0].type // "branch") == "branch")
  ' <<<"$policies_json" >/dev/null || {
    echo "GitHub environment $environment must allow only the main branch" >&2
    exit 1
  }

  if [[ "$environment" == "prd" ]]; then
    jq -e '
      .can_admins_bypass == false and
      any(
        .protection_rules[]?;
        .type == "required_reviewers" and
        .prevent_self_review == true and
        (.reviewers | length) > 0
      )
    ' <<<"$environment_json" >/dev/null || {
      echo "GitHub environment prd requires reviewers, self-review prevention and disabled admin bypass" >&2
      exit 1
    }
    if [[ "$verification_scope" == "all-controls" ]]; then
      jq -e --argjson expected "$expected_prd_reviewers_json" '
        (
          [
            .protection_rules[]?
            | select(.type == "required_reviewers")
            | .reviewers[]?
            | (.reviewer.id // .id)
          ]
          | unique
          | sort
        ) == $expected
      ' <<<"$environment_json" >/dev/null || {
        echo "GitHub environment prd reviewers differ from PYMES_PRD_REVIEWER_IDS" >&2
        exit 1
      }
    fi
  fi

  echo "GitHub environment verified: ${environment} branch=main"
done
