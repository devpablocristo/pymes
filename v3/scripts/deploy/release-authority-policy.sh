#!/usr/bin/env bash

pymes_release_expected_folder=673291958610
pymes_release_expected_organization=663017421195
pymes_release_expected_project_number=884236221349
pymes_release_pool_path=projects/884236221349/locations/global/workloadIdentityPools/pymes-v3-release-pool
pymes_release_build_principal="principal://iam.googleapis.com/${pymes_release_pool_path}/subject/repo:devpablocristo/pymes:ref:refs/heads/main"
pymes_release_stg_principal="principal://iam.googleapis.com/${pymes_release_pool_path}/subject/repo:devpablocristo/pymes:environment:stg"
pymes_release_prd_principal="principal://iam.googleapis.com/${pymes_release_pool_path}/subject/repo:devpablocristo/pymes:environment:prd"

pymes_release_project_iam_read_permissions=(
  artifactregistry.repositories.getIamPolicy
  cloudasset.assets.analyzeIamPolicy
  cloudasset.assets.searchAllIamPolicies
  cloudasset.assets.searchAllResources
  cloudsql.instances.get
  compute.routers.get
  compute.subnetworks.get
  iam.roles.get
  iam.serviceAccountKeys.list
  iam.serviceAccounts.get
  iam.serviceAccounts.getIamPolicy
  iam.workloadIdentityPoolProviders.get
  iam.workloadIdentityPools.get
  logging.logEntries.list
  orgpolicy.policy.get
  resourcemanager.projects.get
  resourcemanager.projects.getIamPolicy
  run.jobs.list
  run.revisions.list
  run.services.list
)

pymes_release_kms_policy_read_permissions=(
  cloudkms.cryptoKeys.get
  cloudkms.cryptoKeys.getIamPolicy
  cloudkms.keyRings.get
  cloudkms.keyRings.getIamPolicy
  cloudkms.locations.get
)

pymes_release_organization_iam_read_permissions=(
  iam.roles.get
  resourcemanager.organizations.getIamPolicy
)

pymes_release_folder_iam_read_permissions=(
  resourcemanager.folders.getIamPolicy
)

pymes_release_inverse_project_permissions=(
  artifactregistry.repositories.create
  cloudkms.cryptoKeys.create
  cloudkms.keyRings.create
  iam.serviceAccounts.create
  resourcemanager.projects.setIamPolicy
  run.jobs.create
  run.services.create
  secretmanager.secrets.create
  serviceusage.services.disable
  serviceusage.services.enable
)

pymes_release_inverse_service_account_permissions=(
  iam.serviceAccountKeys.create
  iam.serviceAccounts.actAs
  iam.serviceAccounts.delete
  iam.serviceAccounts.disable
  iam.serviceAccounts.enable
  iam.serviceAccounts.getAccessToken
  iam.serviceAccounts.getOpenIdToken
  iam.serviceAccounts.implicitDelegation
  iam.serviceAccounts.setIamPolicy
  iam.serviceAccounts.signBlob
  iam.serviceAccounts.signJwt
)

pymes_release_inverse_run_service_permissions=(
  run.routes.invoke
  run.services.delete
  run.services.setIamPolicy
  run.services.update
)

pymes_release_inverse_run_job_permissions=(
  run.jobs.delete
  run.jobs.run
  run.jobs.runWithOverrides
  run.jobs.setIamPolicy
  run.jobs.update
)

pymes_release_inverse_secret_permissions=(
  secretmanager.secrets.delete
  secretmanager.secrets.setIamPolicy
  secretmanager.secrets.update
  secretmanager.versions.access
  secretmanager.versions.add
  secretmanager.versions.destroy
  secretmanager.versions.disable
  secretmanager.versions.enable
)

pymes_release_inverse_artifact_permissions=(
  artifactregistry.repositories.delete
  artifactregistry.repositories.deleteArtifacts
  artifactregistry.repositories.downloadArtifacts
  artifactregistry.repositories.setIamPolicy
  artifactregistry.repositories.update
  artifactregistry.repositories.uploadArtifacts
  artifactregistry.tags.create
  artifactregistry.tags.delete
  artifactregistry.tags.update
  artifactregistry.versions.delete
)

pymes_release_inverse_kms_keyring_permissions=(
  cloudkms.cryptoKeys.create
  cloudkms.keyRings.setIamPolicy
  cloudkms.keyRings.update
)

pymes_release_inverse_kms_key_permissions=(
  cloudkms.cryptoKeys.setIamPolicy
  cloudkms.cryptoKeys.update
  cloudkms.cryptoKeyVersions.destroy
  cloudkms.cryptoKeyVersions.useToDecrypt
  cloudkms.cryptoKeyVersions.useToEncrypt
  cloudkms.cryptoKeyVersions.useToSign
  cloudkms.cryptoKeyVersions.useToVerify
  cloudkms.cryptoKeyVersions.viewPublicKey
)

pymes_release_project_iam_read_permissions_csv() {
  local IFS=,
  printf '%s\n' "${pymes_release_project_iam_read_permissions[*]}"
}

pymes_release_project_iam_read_permissions_json() {
  printf '%s\n' "${pymes_release_project_iam_read_permissions[@]}" |
    jq -Rsc 'split("\n") | map(select(length > 0)) | sort'
}

pymes_release_kms_policy_read_permissions_csv() {
  local IFS=,
  printf '%s\n' "${pymes_release_kms_policy_read_permissions[*]}"
}

pymes_release_kms_policy_read_permissions_json() {
  printf '%s\n' "${pymes_release_kms_policy_read_permissions[@]}" |
    jq -Rsc 'split("\n") | map(select(length > 0)) | sort'
}

pymes_release_organization_iam_read_permissions_csv() {
  local IFS=,
  printf '%s\n' "${pymes_release_organization_iam_read_permissions[*]}"
}

pymes_release_organization_iam_read_permissions_json() {
  printf '%s\n' "${pymes_release_organization_iam_read_permissions[@]}" |
    jq -Rsc 'split("\n") | map(select(length > 0)) | sort'
}

pymes_release_folder_iam_read_permissions_csv() {
  local IFS=,
  printf '%s\n' "${pymes_release_folder_iam_read_permissions[*]}"
}

pymes_release_folder_iam_read_permissions_json() {
  printf '%s\n' "${pymes_release_folder_iam_read_permissions[@]}" |
    jq -Rsc 'split("\n") | map(select(length > 0)) | sort'
}

pymes_read_inverse_permission_analysis() {
  local project="$1" permissions_csv="$2"
  local offset chunk_csv response requested_json attempt
  local -a permissions=() responses=()

  IFS=, read -r -a permissions <<<"$permissions_csv"
  ((${#permissions[@]} > 0)) || {
    echo "inverse IAM analysis requires at least one permission" >&2
    return 2
  }
  requested_json=$(printf '%s\n' "${permissions[@]}" |
    jq -Rsc '
      split("\n")
      | map(select(length > 0))
      | if length == (unique | length) then sort
        else error("duplicate inverse-IAM permission")
        end
    ') || return
  [[ "$(jq -r 'length' <<<"$requested_json")" -eq "${#permissions[@]}" ]] || {
    echo "inverse IAM analysis contains an empty permission" >&2
    return 2
  }

  for ((offset = 0; offset < ${#permissions[@]}; offset += 10)); do
    chunk_csv=$(
      IFS=,
      printf '%s' "${permissions[*]:offset:10}"
    )
    response=
    for attempt in 1 2 3; do
      if response=$(gcloud asset analyze-iam-policy \
        --project="$project" \
        --permissions="$chunk_csv" \
        --expand-groups \
        --expand-resources \
        --expand-roles \
        --analyze-service-account-impersonation \
        --output-group-edges \
        --execution-timeout=120s \
        --show-response \
        --format=json); then
        break
      fi
      [[ "$attempt" -lt 3 ]] || return
      sleep $((attempt * 5))
    done
    responses+=("$response")
  done

  printf '%s\n' "${responses[@]}" |
    jq -cs \
      --arg scope "projects/${project}" \
      --argjson requested "$requested_json" '
        {
          schemaVersion: 1,
          requestedScope: $scope,
          requestedPermissions: $requested,
          responses: .
        }
      '
}

pymes_validate_inverse_permission_analysis() {
  local analysis_json="$1" description="$2" project="$3" project_number="$4"
  shift 4
  local entry mode=protected
  local protected_json allowed_json actual_triples allowed_triples unexpected
  local -a protected=() allowed=()

  for entry in "$@"; do
    if [[ "$entry" == "--" ]]; then
      [[ "$mode" == "protected" ]] || {
        echo "$description inverse IAM allowlist has more than one separator" >&2
        return 2
      }
      mode=allowed
      continue
    fi
    if [[ "$mode" == "protected" ]]; then
      protected+=("$entry")
    else
      allowed+=("$entry")
    fi
  done
  [[ "$mode" == "allowed" && ${#protected[@]} -gt 0 ]] || {
    echo "$description inverse IAM validation requires protected pairs and an allowlist separator" >&2
    return 2
  }

  protected_json=$(printf '%s\n' "${protected[@]}" |
    jq -Rsc '
      split("\n")
      | map(select(length > 0) | split("\t"))
      | if all(.[]; length == 2 and all(.[]; length > 0))
        then unique | sort
        else error("invalid protected resource-permission pair")
        end
    ') || {
    echo "$description inverse IAM protected-pair format is invalid" >&2
    return 2
  }
  allowed_json=$(printf '%s\n' "${allowed[@]:-}" |
    jq -Rsc '
      split("\n")
      | map(select(length > 0) | split("\t"))
      | if all(.[]; length == 3 and all(.[]; length > 0))
        then unique | sort
        else error("invalid allowed resource-permission-identity triple")
        end
    ') || {
    echo "$description inverse IAM allowlist format is invalid" >&2
    return 2
  }

  jq -e \
    --arg scope "projects/${project}" \
    --arg project "$project" \
    --arg number "$project_number" \
    --slurpfile protected_input <(printf '%s\n' "$protected_json") \
    --slurpfile allowed_input <(printf '%s\n' "$allowed_json") '
      ($protected_input[0]) as $protected |
      ($allowed_input[0]) as $allowed |
      def normalize_resource:
        sub("projects/" + $number; "projects/" + $project)
        | sub("^//iam.googleapis.com/projects/-/";
            "//iam.googleapis.com/projects/" + $project + "/")
        | sub("/cryptoKeyVersions/[^/]+$"; "")
        | sub("/versions/[^/]+$"; "");
      def results:
        (
          .responses[].mainAnalysis.analysisResults[]?,
          .responses[].serviceAccountImpersonationAnalysis[]?.analysisResults[]?
        );
      def result_pairs($result):
        $result.accessControlLists[]? as $acl
        | $acl.accesses[]?
        | select(.permission? != null)
        | .permission as $permission
        | (
            ($acl.resources // [])
            | if length == 0
              then [{fullResourceName: $result.attachedResourceFullName}]
              else .
              end
          )[]
        | [(.fullResourceName | normalize_resource), $permission];
      def touches_protected($result):
        any(
          result_pairs($result);
          . as $pair | any($protected[]; . == $pair)
        );
      .schemaVersion == 1 and
      .requestedScope == $scope and
      (.requestedPermissions | length) > 0 and
      (.requestedPermissions | unique | sort) == .requestedPermissions and
      (.responses | length) > 0 and
      all(
        .responses[];
        .fullyExplored == true and
        .mainAnalysis.fullyExplored == true and
        ((.nonCriticalErrors // []) | length) == 0 and
        ((.mainAnalysis.nonCriticalErrors // []) | length) == 0 and
        .mainAnalysis.analysisQuery.scope == $scope and
        (.mainAnalysis.analysisQuery.identitySelector // null) == null and
        (.mainAnalysis.analysisQuery.resourceSelector // null) == null and
        .mainAnalysis.analysisQuery.options.expandGroups == true and
        .mainAnalysis.analysisQuery.options.expandResources == true and
        .mainAnalysis.analysisQuery.options.expandRoles == true and
        .mainAnalysis.analysisQuery.options.analyzeServiceAccountImpersonation == true and
        .mainAnalysis.analysisQuery.options.outputGroupEdges == true and
        all(.mainAnalysis.analysisResults[]?; .fullyExplored == true) and
        all(
          .serviceAccountImpersonationAnalysis[]?;
          .fullyExplored == true and
          ((.nonCriticalErrors // []) | length) == 0 and
          all(.analysisResults[]?; .fullyExplored == true)
        )
      ) and
      (
        [
          .responses[].mainAnalysis.analysisQuery.accessSelector.permissions[]?
        ] | unique | sort
      ) == .requestedPermissions and
      all(
        results;
        if touches_protected(.) then
          ((.identityList.groupEdges // []) | length) == 0 and
          ((.identityList.identities // []) | length) > 0 and
          all(.identityList.identities[]?; (.name? | type) == "string" and (.name | length) > 0)
        else
          true
        end
      ) and
      all(
        $allowed[];
        . as $triple
        | any($protected[]; . == [$triple[0], $triple[1]])
      )
    ' <<<"$analysis_json" >/dev/null || {
    echo "$description inverse IAM analysis is incomplete, malformed, group-derived, or not the requested permission-only query" >&2
    return 1
  }

  actual_triples=$(jq -r \
    --arg project "$project" \
    --arg number "$project_number" \
    --slurpfile protected_input <(printf '%s\n' "$protected_json") '
      ($protected_input[0]) as $protected |
      def normalize_resource:
        sub("projects/" + $number; "projects/" + $project)
        | sub("^//iam.googleapis.com/projects/-/";
            "//iam.googleapis.com/projects/" + $project + "/")
        | sub("/cryptoKeyVersions/[^/]+$"; "")
        | sub("/versions/[^/]+$"; "");
      [
        (
          .responses[].mainAnalysis.analysisResults[]?,
          .responses[].serviceAccountImpersonationAnalysis[]?.analysisResults[]?
        ) as $result
        | $result.accessControlLists[]? as $acl
        | $acl.accesses[]?
        | select(.permission? != null)
        | .permission as $permission
        | (
            ($acl.resources // [])
            | if length == 0
              then [{fullResourceName: $result.attachedResourceFullName}]
              else .
              end
        )[]
        | (.fullResourceName | normalize_resource) as $resource
        | select(any($protected[]; . == [$resource, $permission]))
        | $result.identityList.identities[]?.name as $identity
        | [$resource, $permission, $identity]
        | @tsv
      ]
      | unique
      | sort
      | .[]
    ' <<<"$analysis_json")
  allowed_triples=$(printf '%s\n' "${allowed[@]:-}" |
    sed '/^[[:space:]]*$/d' |
    LC_ALL=C sort -u)
  unexpected=$(comm -23 \
    <(printf '%s\n' "$actual_triples" |
      sed '/^[[:space:]]*$/d' |
      LC_ALL=C sort -u) \
    <(printf '%s\n' "$allowed_triples" |
      sed '/^[[:space:]]*$/d' |
      LC_ALL=C sort -u))
  [[ -z "$unexpected" ]] || {
    echo "$description has effective permission outside the exact resource/identity allowlist" >&2
    printf '%s\n' "$unexpected" >&2
    return 1
  }
}

pymes_verify_release_inverse_authority() {
  local project="$1" project_number="$2" region="$3"
  local environment="$4" run_resources="$5" artifact_repository="$6"
  local boundary="${7:-all}"
  local owner="user:softponti@gmail.com"
  local project_resource="//cloudresourcemanager.googleapis.com/projects/${project}"
  local repository_resource="//artifactregistry.googleapis.com/projects/${project}/locations/${region}/repositories/${artifact_repository}"
  local keyring_resource="//cloudkms.googleapis.com/projects/${project}/locations/${region}/keyRings/pymes-v3-${environment}"
  local build_email="pymes-v3-gh-build@${project}.iam.gserviceaccount.com"
  local stg_deploy_email="pymes-v3-gh-deploy-stg@${project}.iam.gserviceaccount.com"
  local environment_deploy_email="pymes-v3-gh-deploy-${environment}@${project}.iam.gserviceaccount.com"
  local build_member="serviceAccount:${build_email}"
  local deploy_member="serviceAccount:${environment_deploy_email}"
  local run_agent="serviceAccount:service-${project_number}@serverless-robot-prod.iam.gserviceaccount.com"
  local build_agent="serviceAccount:service-${project_number}@gcp-sa-cloudbuild.iam.gserviceaccount.com"
  local compute_agent="serviceAccount:service-${project_number}@compute-system.iam.gserviceaccount.com"
  local scheduler_agent="serviceAccount:service-${project_number}@gcp-sa-cloudscheduler.iam.gserviceaccount.com"
  local pubsub_agent="serviceAccount:service-${project_number}@gcp-sa-pubsub.iam.gserviceaccount.com"
  local cloudservices_agent="serviceAccount:${project_number}@cloudservices.gserviceaccount.com"
  local firebase_agent="serviceAccount:service-${project_number}@gcp-sa-firebase.iam.gserviceaccount.com"
  local artifact_agent="serviceAccount:service-${project_number}@gcp-sa-artifactregistry.iam.gserviceaccount.com"
  local secret_agent="serviceAccount:service-${project_number}@gcp-sa-secretmanager.iam.gserviceaccount.com"
  local permission resource identity component secret suffix key
  local permissions_csv analysis_json
  local -a protected=() allowed=() all_permissions=()
  local -a release_accounts=("$build_email" "$stg_deploy_email")
  local -a runtime_components=(
    api web worker provision fiscal accounting accounting-admin
    migrate fiscal-migrate acct-migrate
  )
  local -a service_components=(
    api web worker fiscal accounting accounting-admin
  )
  local -a job_components=(
    migrate fiscal-migrate accounting-migrate accounting-grants provision-org
  )
  local -a secret_suffixes=(
    clerk-secret-key clerk-webhook-secret scheduling-action-token-secret
    pergo-api-key pergo-webhook-secrets google-client-secret database-url
    worker-database-url migrate-database-url fiscal-database-url
    fiscal-migrate-database-url accounting-database-url
    accounting-admin-database-url accounting-migrate-database-url
  )
  local -a secret_accessors=()

  [[ "$project" == "pymes-dev-352318" &&
     "$project_number" == "884236221349" &&
     "$region" == "us-central1" ]] || {
    echo "inverse release boundary is restricted to the reviewed Pymes project and region" >&2
    return 2
  }
  case "$environment" in
    stg) ;;
    prd)
      release_accounts+=("pymes-v3-gh-deploy-prd@${project}.iam.gserviceaccount.com")
      ;;
    *) echo "inverse release boundary requires stg or prd" >&2; return 2 ;;
  esac
  case "$run_resources" in
    absent|present) ;;
    *) echo "inverse release boundary Run mode must be absent or present" >&2; return 2 ;;
  esac
  case "$boundary" in
    all|kms) ;;
    *) echo "inverse release boundary mode must be all or kms" >&2; return 2 ;;
  esac
  [[ "$artifact_repository" == "pymes" ]] || {
    echo "inverse release boundary is restricted to Artifact Registry repository pymes" >&2
    return 2
  }

  all_permissions=(
    "${pymes_release_inverse_kms_keyring_permissions[@]}"
    "${pymes_release_inverse_kms_key_permissions[@]}"
  )
  if [[ "$boundary" == "all" ]]; then
    all_permissions+=(
      "${pymes_release_inverse_project_permissions[@]}"
      "${pymes_release_inverse_service_account_permissions[@]}"
      "${pymes_release_inverse_secret_permissions[@]}"
      "${pymes_release_inverse_artifact_permissions[@]}"
    )
    if [[ "$run_resources" == "present" ]]; then
      all_permissions+=(
        "${pymes_release_inverse_run_service_permissions[@]}"
        "${pymes_release_inverse_run_job_permissions[@]}"
      )
    fi
  fi
  permissions_csv=$(printf '%s\n' "${all_permissions[@]}" |
    LC_ALL=C sort -u |
    paste -sd, -)

  if [[ "$boundary" == "all" ]]; then
    for permission in "${pymes_release_inverse_project_permissions[@]}"; do
    protected+=("$project_resource"$'\t'"$permission")
    allowed+=("$project_resource"$'\t'"$permission"$'\t'"$owner")
    done
    allowed+=("$project_resource"$'\t'"iam.serviceAccounts.create"$'\t'"$firebase_agent")

    for identity in "${release_accounts[@]}"; do
    resource="//iam.googleapis.com/projects/${project}/serviceAccounts/${identity}"
    for permission in "${pymes_release_inverse_service_account_permissions[@]}"; do
      protected+=("$resource"$'\t'"$permission")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$owner")
    done
    for permission in \
      iam.serviceAccounts.getAccessToken \
      iam.serviceAccounts.getOpenIdToken; do
      allowed+=("$resource"$'\t'"$permission"$'\t'"$build_agent")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$compute_agent")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$scheduler_agent")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$run_agent")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$pubsub_agent")
    done
    for permission in iam.serviceAccounts.actAs iam.serviceAccounts.signBlob; do
      allowed+=("$resource"$'\t'"$permission"$'\t'"$run_agent")
    done
    allowed+=("$resource"$'\t'"iam.serviceAccounts.actAs"$'\t'"$cloudservices_agent")
    for permission in \
      iam.serviceAccounts.implicitDelegation \
      iam.serviceAccounts.signBlob \
      iam.serviceAccounts.signJwt; do
      allowed+=("$resource"$'\t'"$permission"$'\t'"$pubsub_agent")
    done
    for permission in \
      iam.serviceAccounts.actAs \
      iam.serviceAccounts.implicitDelegation \
      iam.serviceAccounts.signJwt; do
      allowed+=("$resource"$'\t'"$permission"$'\t'"$compute_agent")
    done
    done

    resource="//iam.googleapis.com/projects/${project}/serviceAccounts/${build_email}"
  for permission in \
    iam.serviceAccounts.getAccessToken \
    iam.serviceAccounts.getOpenIdToken; do
    allowed+=("$resource"$'\t'"$permission"$'\t'"$pymes_release_build_principal")
  done
  resource="//iam.googleapis.com/projects/${project}/serviceAccounts/${stg_deploy_email}"
  for permission in \
    iam.serviceAccounts.getAccessToken \
    iam.serviceAccounts.getOpenIdToken; do
    allowed+=("$resource"$'\t'"$permission"$'\t'"$pymes_release_stg_principal")
  done
    if [[ "$environment" == "prd" ]]; then
    resource="//iam.googleapis.com/projects/${project}/serviceAccounts/${environment_deploy_email}"
    for permission in \
      iam.serviceAccounts.getAccessToken \
      iam.serviceAccounts.getOpenIdToken; do
      allowed+=("$resource"$'\t'"$permission"$'\t'"$pymes_release_prd_principal")
    done
    fi

    for component in "${runtime_components[@]}"; do
    resource="//iam.googleapis.com/projects/${project}/serviceAccounts/pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
    for permission in "${pymes_release_inverse_service_account_permissions[@]}"; do
      protected+=("$resource"$'\t'"$permission")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$owner")
    done
    allowed+=("$resource"$'\t'"iam.serviceAccounts.actAs"$'\t'"$deploy_member")
    for permission in \
      iam.serviceAccounts.getAccessToken \
      iam.serviceAccounts.getOpenIdToken; do
      allowed+=("$resource"$'\t'"$permission"$'\t'"$build_agent")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$compute_agent")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$scheduler_agent")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$run_agent")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$pubsub_agent")
    done
    for permission in iam.serviceAccounts.actAs iam.serviceAccounts.signBlob; do
      allowed+=("$resource"$'\t'"$permission"$'\t'"$run_agent")
    done
    allowed+=("$resource"$'\t'"iam.serviceAccounts.actAs"$'\t'"$cloudservices_agent")
    for permission in \
      iam.serviceAccounts.implicitDelegation \
      iam.serviceAccounts.signBlob \
      iam.serviceAccounts.signJwt; do
      allowed+=("$resource"$'\t'"$permission"$'\t'"$pubsub_agent")
    done
    for permission in \
      iam.serviceAccounts.actAs \
      iam.serviceAccounts.implicitDelegation \
      iam.serviceAccounts.signJwt; do
      allowed+=("$resource"$'\t'"$permission"$'\t'"$compute_agent")
    done
    done

    if [[ "$run_resources" == "present" ]]; then
    for component in "${service_components[@]}"; do
      resource="//run.googleapis.com/projects/${project}/locations/${region}/services/pymes-v3-${environment}-${component}"
      for permission in "${pymes_release_inverse_run_service_permissions[@]}"; do
        protected+=("$resource"$'\t'"$permission")
        allowed+=("$resource"$'\t'"$permission"$'\t'"$owner")
      done
      for permission in run.services.delete run.services.setIamPolicy run.services.update; do
        allowed+=("$resource"$'\t'"$permission"$'\t'"$deploy_member")
      done
      allowed+=("$resource"$'\t'"run.routes.invoke"$'\t'"$run_agent")
      case "$component" in
        api|web)
          allowed+=("$resource"$'\t'"run.routes.invoke"$'\t'"allUsers")
          ;;
        fiscal)
          allowed+=("$resource"$'\t'"run.routes.invoke"$'\t'"serviceAccount:pymes-v3-api-${environment}@${project}.iam.gserviceaccount.com")
          allowed+=("$resource"$'\t'"run.routes.invoke"$'\t'"serviceAccount:pymes-v3-worker-${environment}@${project}.iam.gserviceaccount.com")
          ;;
        accounting)
          allowed+=("$resource"$'\t'"run.routes.invoke"$'\t'"serviceAccount:pymes-v3-worker-${environment}@${project}.iam.gserviceaccount.com")
          ;;
        accounting-admin)
          allowed+=("$resource"$'\t'"run.routes.invoke"$'\t'"serviceAccount:pymes-v3-provision-${environment}@${project}.iam.gserviceaccount.com")
          ;;
      esac
    done
    for component in "${job_components[@]}"; do
      resource="//run.googleapis.com/projects/${project}/locations/${region}/jobs/pymes-v3-${environment}-${component}"
      for permission in "${pymes_release_inverse_run_job_permissions[@]}"; do
        protected+=("$resource"$'\t'"$permission")
        allowed+=("$resource"$'\t'"$permission"$'\t'"$owner")
        allowed+=("$resource"$'\t'"$permission"$'\t'"$deploy_member")
      done
    done
    fi

    for suffix in "${secret_suffixes[@]}"; do
    secret="pymes-v3-${environment}-${suffix}"
    resource="//secretmanager.googleapis.com/projects/${project}/secrets/${secret}"
    for permission in "${pymes_release_inverse_secret_permissions[@]}"; do
      protected+=("$resource"$'\t'"$permission")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$owner")
    done
    secret_accessors=()
    case "$suffix" in
      clerk-secret-key|clerk-webhook-secret|pergo-webhook-secrets)
        secret_accessors=(api)
        ;;
      scheduling-action-token-secret|google-client-secret)
        secret_accessors=(api worker)
        ;;
      pergo-api-key)
        secret_accessors=(worker)
        ;;
      database-url)
        secret_accessors=(api provision)
        ;;
      worker-database-url)
        secret_accessors=(worker)
        ;;
      migrate-database-url)
        secret_accessors=(migrate)
        ;;
      fiscal-database-url)
        secret_accessors=(fiscal)
        ;;
      fiscal-migrate-database-url)
        secret_accessors=(fiscal-migrate)
        ;;
      accounting-database-url)
        secret_accessors=(accounting)
        ;;
      accounting-admin-database-url)
        secret_accessors=(accounting-admin)
        ;;
      accounting-migrate-database-url)
        secret_accessors=(acct-migrate)
        ;;
    esac
    for component in "${secret_accessors[@]}"; do
      allowed+=("$resource"$'\t'"secretmanager.versions.access"$'\t'"serviceAccount:pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com")
    done
    done

    for permission in "${pymes_release_inverse_artifact_permissions[@]}"; do
    protected+=("$repository_resource"$'\t'"$permission")
    allowed+=("$repository_resource"$'\t'"$permission"$'\t'"$owner")
    done
    for permission in \
      artifactregistry.repositories.downloadArtifacts \
      artifactregistry.repositories.uploadArtifacts \
      artifactregistry.tags.create \
      artifactregistry.tags.update; do
      allowed+=("$repository_resource"$'\t'"$permission"$'\t'"$build_member")
    done
    allowed+=("$repository_resource"$'\t'"artifactregistry.repositories.downloadArtifacts"$'\t'"serviceAccount:${stg_deploy_email}")
    allowed+=("$repository_resource"$'\t'"artifactregistry.repositories.downloadArtifacts"$'\t'"serviceAccount:pymes-v3-gh-deploy-prd@${project}.iam.gserviceaccount.com")
    allowed+=("$repository_resource"$'\t'"artifactregistry.repositories.downloadArtifacts"$'\t'"$artifact_agent")
    allowed+=("$repository_resource"$'\t'"artifactregistry.versions.delete"$'\t'"$artifact_agent")
    for permission in \
      artifactregistry.repositories.deleteArtifacts \
      artifactregistry.repositories.downloadArtifacts \
      artifactregistry.repositories.uploadArtifacts \
      artifactregistry.tags.create \
      artifactregistry.tags.update; do
      allowed+=("$repository_resource"$'\t'"$permission"$'\t'"$build_agent")
    done
    for permission in \
      artifactregistry.repositories.downloadArtifacts \
      artifactregistry.repositories.uploadArtifacts; do
      allowed+=("$repository_resource"$'\t'"$permission"$'\t'"$run_agent")
    done
  fi

  for permission in "${pymes_release_inverse_kms_keyring_permissions[@]}"; do
    protected+=("$keyring_resource"$'\t'"$permission")
    allowed+=("$keyring_resource"$'\t'"$permission"$'\t'"$owner")
  done
  for key in secrets calendar-tokens fiscal-vault internal-jwt-signing; do
    resource="${keyring_resource}/cryptoKeys/${key}"
    for permission in "${pymes_release_inverse_kms_key_permissions[@]}"; do
      protected+=("$resource"$'\t'"$permission")
      allowed+=("$resource"$'\t'"$permission"$'\t'"$owner")
    done
    case "$key" in
      secrets)
        for permission in \
          cloudkms.cryptoKeyVersions.useToDecrypt \
          cloudkms.cryptoKeyVersions.useToEncrypt; do
          allowed+=("$resource"$'\t'"$permission"$'\t'"$secret_agent")
        done
        ;;
      calendar-tokens)
        for component in api worker; do
          identity="serviceAccount:pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
          for permission in \
            cloudkms.cryptoKeyVersions.useToDecrypt \
            cloudkms.cryptoKeyVersions.useToEncrypt; do
            allowed+=("$resource"$'\t'"$permission"$'\t'"$identity")
          done
        done
        ;;
      fiscal-vault)
        identity="serviceAccount:pymes-v3-fiscal-${environment}@${project}.iam.gserviceaccount.com"
        for permission in \
          cloudkms.cryptoKeyVersions.useToDecrypt \
          cloudkms.cryptoKeyVersions.useToEncrypt; do
          allowed+=("$resource"$'\t'"$permission"$'\t'"$identity")
        done
        ;;
      internal-jwt-signing)
        for component in api worker provision; do
          identity="serviceAccount:pymes-v3-${component}-${environment}@${project}.iam.gserviceaccount.com"
          allowed+=("$resource"$'\t'"cloudkms.cryptoKeyVersions.useToSign"$'\t'"$identity")
          allowed+=("$resource"$'\t'"cloudkms.cryptoKeyVersions.viewPublicKey"$'\t'"$identity")
        done
        allowed+=("$resource"$'\t'"cloudkms.cryptoKeyVersions.viewPublicKey"$'\t'"$deploy_member")
        ;;
    esac
  done

  analysis_json=$(pymes_read_inverse_permission_analysis \
    "$project" "$permissions_csv")
  pymes_validate_inverse_permission_analysis \
    "$analysis_json" "Pymes release boundary ${environment}" \
    "$project" "$project_number" \
    "${protected[@]}" -- "${allowed[@]}"
}

pymes_search_release_pool_iam_assets() {
  local project="$1"
  gcloud asset search-all-iam-policies \
    --scope="projects/${project}" \
    --query="policy:\"${pymes_release_pool_path}\"" \
    --format=json
}

pymes_assert_policy_has_no_release_pool_members() {
  local policy_json="$1" description="$2"
  jq -e --arg pool "$pymes_release_pool_path" '
    all(
      .bindings[]?;
      all(.members[]?; (contains($pool) | not))
    )
  ' <<<"$policy_json" >/dev/null || {
    echo "$description contains prohibited direct release-pool authority" >&2
    return 1
  }
}

pymes_validate_release_pool_iam_assets() {
  local assets_json="$1" required_environment="$2" mode="$3"
  local required_principal
  case "$required_environment" in
    stg) required_principal=$pymes_release_stg_principal ;;
    prd) required_principal=$pymes_release_prd_principal ;;
    *) echo "release-pool asset validation requires stg or prd" >&2; return 2 ;;
  esac
  case "$mode" in
    subset|exact) ;;
    *) echo "release-pool asset validation mode must be subset or exact" >&2; return 2 ;;
  esac
  jq -e \
    --arg pool "$pymes_release_pool_path" \
    --arg build "$pymes_release_build_principal" \
    --arg stg "$pymes_release_stg_principal" \
    --arg prd "$pymes_release_prd_principal" \
    --arg required "$required_principal" \
    --arg mode "$mode" '
    def account_resources($email):
      [
        "//iam.googleapis.com/projects/pymes-dev-352318/serviceAccounts/" + $email,
        "//iam.googleapis.com/projects/884236221349/serviceAccounts/" + $email,
        "//iam.googleapis.com/projects/-/serviceAccounts/" + $email
      ];
    def expected_entry($entry; $principal; $email):
      $entry.member == $principal and
      (account_resources($email) | index($entry.resource) != null);
    [
      .[]? as $asset
      | $asset.policy.bindings[]? as $binding
      | ($binding.members // [])[]? as $member
      | select($member | contains($pool))
      | {
          resource: $asset.resource,
          role: $binding.role,
          condition: ($binding.condition // null),
          member: $member
        }
    ] as $entries
    | all(
        $entries[];
        .role == "roles/iam.workloadIdentityUser" and
        .condition == null and
        (
          expected_entry(.; $build; "pymes-v3-gh-build@pymes-dev-352318.iam.gserviceaccount.com") or
          expected_entry(.; $stg; "pymes-v3-gh-deploy-stg@pymes-dev-352318.iam.gserviceaccount.com") or
          expected_entry(.; $prd; "pymes-v3-gh-deploy-prd@pymes-dev-352318.iam.gserviceaccount.com")
        )
      ) as $entries_valid
    | ([ $entries[] | select(.member == $build) ] | length) as $build_count
    | ([ $entries[] | select(.member == $stg) ] | length) as $stg_count
    | ([ $entries[] | select(.member == $prd) ] | length) as $prd_count
    | $entries_valid and
      ($build_count <= 1 and $stg_count <= 1 and $prd_count <= 1) and
      (
        if $required == $stg then
          $prd_count == 0 and
          (
            if $mode == "exact" then
              $build_count == 1 and $stg_count == 1
            else
              true
            end
          )
        else
          $build_count == 1 and $stg_count == 1 and
          (
            if $mode == "exact" then
              $prd_count == 1
            else
              true
            end
          )
        end
      )
  ' <<<"$assets_json" >/dev/null || {
    echo "release pool has a direct principal/principalSet binding outside the exact service-account WIF trust" >&2
    return 1
  }
}

pymes_validate_enforced_boolean_org_policy() {
  local policy_json="$1" constraint="$2"
  jq -e \
    --arg name "projects/${pymes_release_expected_project_number}/policies/${constraint}" '
      .name == $name and
      (.spec.rules | length) == 1 and
      .spec.rules[0].enforce == true
    ' <<<"$policy_json" >/dev/null || {
    echo "effective organization policy is not fail-closed and enforced: $constraint" >&2
    return 1
  }
}

pymes_validate_release_account_workload_inventory() {
  local assets_json="$1" services_json="$2" jobs_json="$3"
  local account="$4" description="$5"
  local revisions_json="${6:-[]}"
  jq -e --arg account "$account" '
    [
      .[]?
      | select(
          .assetType == "iam.googleapis.com/ServiceAccount" and
          ((.name // "") | endswith("/serviceAccounts/" + $account))
        )
    ] | length == 1
  ' <<<"$assets_json" >/dev/null &&
    jq -e --arg account "$account" '
      all(
        .[]?;
        (.spec.template.spec.serviceAccountName // "") != $account
      )
    ' <<<"$services_json" >/dev/null &&
    jq -e --arg account "$account" '
      all(
        .[]?;
        (.spec.template.spec.template.spec.serviceAccountName // "") != $account
      )
    ' <<<"$jobs_json" >/dev/null &&
    jq -e --arg account "$account" '
      all(
        .[]?;
        (
          .spec.serviceAccountName //
          .spec.template.spec.serviceAccountName //
          ""
        ) != $account
      )
    ' <<<"$revisions_json" >/dev/null || {
    echo "$description is attached to a workload or its complete account inventory cannot be proven" >&2
    return 1
  }
  jq -e --arg account "$account" '
    all(
      .[]?;
      .assetType == "iam.googleapis.com/ServiceAccount" and
      ((.name // "") | endswith("/serviceAccounts/" + $account))
    )
  ' <<<"$assets_json" >/dev/null || {
    echo "$description appears in a non-service-account Cloud Asset workload" >&2
    return 1
  }
}

pymes_verify_release_account_not_attached() {
  local project="$1" account="$2" description="$3"
  local assets_json services_json jobs_json revisions_json
  assets_json=$(gcloud asset search-all-resources \
    --scope="projects/${project}" \
    --query="$account" \
    --read-mask='name,assetType,location,additionalAttributes' \
    --format=json)
  services_json=$(CLOUDSDK_RUN_REGION= gcloud run services list \
    --project="$project" --format=json)
  jobs_json=$(CLOUDSDK_RUN_REGION= gcloud run jobs list \
    --project="$project" --format=json)
  revisions_json=$(CLOUDSDK_RUN_REGION= gcloud run revisions list \
    --project="$project" --format=json)
  pymes_validate_release_account_workload_inventory \
    "$assets_json" "$services_json" "$jobs_json" "$account" "$description" \
    "$revisions_json"
}
