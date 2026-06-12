#!/usr/bin/env bash
# Prepara Pymes para GitHub Flow STG/PRD/Preview sin crear instancias Cloud SQL.
#
# Requisitos:
#   - gcloud autenticado con permisos de Resource Manager, IAM, Service Usage,
#     Artifact Registry, Secret Manager, Cloud Run, Firebase y Cloud SQL Admin.
#   - Billing account y folder IDs provistos por env cuando haya que crear proyectos.
#
# Variables principales:
#   BILLING_ACCOUNT_ID        Requerido si el proyecto STG/PRD no existe.
#   STG_FOLDER_ID             Folder destino para pymes-dev-352318.
#   PRD_FOLDER_ID             Folder destino para pymes-prd-352318.
#   NONPRD_DBS_FOLDER_ID      Folder de DBs non-prod si es distinto de STG.
#   WIF_PRINCIPAL_SET         Principal set del repo para Workload Identity Federation.
#   WIF_PROVIDER_PROJECT_ID   Opcional; si se define, sincroniza la condicion OIDC.
#
# Este script NO crea Cloud SQL instances. Usa:
#   pymes-dev-352318:us-central1:pymes-dev-db
set -euo pipefail

REGION="${REGION:-us-central1}"
STG_PROJECT_ID="${STG_PROJECT_ID:-pymes-dev-352318}"
PRD_PROJECT_ID="${PRD_PROJECT_ID:-pymes-prd-352318}"
NONPRD_DB_PROJECT_ID="${NONPRD_DB_PROJECT_ID:-pymes-dev-352318}"
NONPRD_SQL_INSTANCE="${NONPRD_SQL_INSTANCE:-pymes-dev-db}"
NONPRD_SQL_CONNECTION="${NONPRD_DB_PROJECT_ID}:${REGION}:${NONPRD_SQL_INSTANCE}"
ARTIFACT_REPOSITORY="${ARTIFACT_REPOSITORY:-pymes}"

STG_GHA_SA="${STG_GHA_SA:-pymes-github-actions-stg}"
STG_CORE_SA="${STG_CORE_SA:-pymes-core-runtime-stg}"
STG_VERTICAL_SA="${STG_VERTICAL_SA:-pymes-vertical-runtime-stg}"

STG_DB_NAME="${STG_DB_NAME:-pymes_stg}"
STG_DB_USER="${STG_DB_USER:-pymes_stg_app}"
PREVIEW_DB_USER="${PREVIEW_DB_USER:-pymes_preview_app}"
SQL_ADMIN_USER="${SQL_ADMIN_USER:-nonprd_sql_admin}"
WIF_PROVIDER_PROJECT_ID="${WIF_PROVIDER_PROJECT_ID:-}"
WIF_POOL_ID="${WIF_POOL_ID:-github-actions-pool}"
WIF_PROVIDER_ID="${WIF_PROVIDER_ID:-github-actions-provider}"
WIF_PROVIDER_CONDITION="${WIF_PROVIDER_CONDITION:-((assertion.repository=='devpablocristo/pymes' || assertion.repository=='devpablocristo/companion' || assertion.repository=='devpablocristo/nexus' || assertion.repository=='devpablocristo/axis') && assertion.ref=='refs/heads/develop') || (assertion.repository=='devpablocristo/pymes' && assertion.event_name=='pull_request' && assertion.base_ref=='main')}"

required_apis=(
  run.googleapis.com
  artifactregistry.googleapis.com
  secretmanager.googleapis.com
  iam.googleapis.com
  iamcredentials.googleapis.com
  sts.googleapis.com
  sqladmin.googleapis.com
  firebase.googleapis.com
  firebasehosting.googleapis.com
  cloudbuild.googleapis.com
  logging.googleapis.com
  monitoring.googleapis.com
)

log() {
  printf '\n==> %s\n' "$*"
}

ensure_project() {
  local project_id="$1"
  local folder_id="$2"
  if gcloud projects describe "$project_id" >/dev/null 2>&1; then
    log "Proyecto existente: $project_id"
  else
    if [[ -z "${BILLING_ACCOUNT_ID:-}" || -z "$folder_id" ]]; then
      echo "Falta BILLING_ACCOUNT_ID o folder ID para crear $project_id." >&2
      exit 1
    fi
    log "Creando proyecto $project_id"
    gcloud projects create "$project_id" --folder="$folder_id" --name="$project_id"
    gcloud billing projects link "$project_id" --billing-account="$BILLING_ACCOUNT_ID"
  fi
}

enable_apis() {
  local project_id="$1"
  log "Habilitando APIs en $project_id"
  gcloud services enable "${required_apis[@]}" --project="$project_id"
}

ensure_artifact_repository() {
  local project_id="$1"
  if gcloud artifacts repositories describe "$ARTIFACT_REPOSITORY" --project="$project_id" --location="$REGION" >/dev/null 2>&1; then
    log "Artifact Registry existente: $project_id/$ARTIFACT_REPOSITORY"
  else
    log "Creando Artifact Registry $ARTIFACT_REPOSITORY en $project_id"
    gcloud artifacts repositories create "$ARTIFACT_REPOSITORY" \
      --project="$project_id" \
      --location="$REGION" \
      --repository-format=docker \
      --description="Pymes container images"
  fi
}

ensure_service_account() {
  local project_id="$1"
  local account_id="$2"
  local display_name="$3"
  if gcloud iam service-accounts describe "${account_id}@${project_id}.iam.gserviceaccount.com" --project="$project_id" >/dev/null 2>&1; then
    log "Service account existente: ${account_id}@${project_id}.iam.gserviceaccount.com"
  else
    log "Creando service account $account_id en $project_id"
    gcloud iam service-accounts create "$account_id" --project="$project_id" --display-name="$display_name"
  fi
}

add_project_role() {
  local project_id="$1"
  local member="$2"
  local role="$3"
  gcloud projects add-iam-policy-binding "$project_id" \
    --member="$member" \
    --role="$role" \
    --condition=None >/dev/null
}

ensure_secret() {
  local project_id="$1"
  local secret_name="$2"
  local value="$3"
  if gcloud secrets describe "$secret_name" --project="$project_id" >/dev/null 2>&1; then
    printf '%s' "$value" | gcloud secrets versions add "$secret_name" --project="$project_id" --data-file=-
  else
    printf '%s' "$value" | gcloud secrets create "$secret_name" --project="$project_id" --replication-policy=automatic --data-file=-
  fi
}

random_password() {
  openssl rand -base64 36 | tr -d '\n'
}

urlencode() {
  python3 - "$1" <<'PY'
import sys
from urllib.parse import quote
print(quote(sys.argv[1], safe=""))
PY
}

ensure_sql_user() {
  local user="$1"
  local password="$2"
  if gcloud sql users list --instance="$NONPRD_SQL_INSTANCE" --project="$NONPRD_DB_PROJECT_ID" --format='value(name)' | grep -Fx "$user" >/dev/null; then
    log "SQL user existente: $user"
    gcloud sql users set-password "$user" --instance="$NONPRD_SQL_INSTANCE" --project="$NONPRD_DB_PROJECT_ID" --password="$password"
  else
    log "Creando SQL user $user"
    gcloud sql users create "$user" --instance="$NONPRD_SQL_INSTANCE" --project="$NONPRD_DB_PROJECT_ID" --password="$password"
  fi
}

ensure_sql_database() {
  local db_name="$1"
  if gcloud sql databases describe "$db_name" --instance="$NONPRD_SQL_INSTANCE" --project="$NONPRD_DB_PROJECT_ID" >/dev/null 2>&1; then
    log "SQL database existente: $db_name"
  else
    log "Creando SQL database $db_name"
    gcloud sql databases create "$db_name" --instance="$NONPRD_SQL_INSTANCE" --project="$NONPRD_DB_PROJECT_ID"
  fi
}

grant_sql_database() {
  local db_name="$1"
  local db_user="$2"
  local admin_password="$3"
  local proxy_bin="${CLOUD_SQL_PROXY_BIN:-}"
  if [[ -z "$proxy_bin" ]]; then
    if command -v cloud-sql-proxy >/dev/null 2>&1; then
      proxy_bin="$(command -v cloud-sql-proxy)"
    elif [[ -x /tmp/cloud-sql-proxy ]]; then
      proxy_bin=/tmp/cloud-sql-proxy
    fi
  fi
  if [[ -z "$proxy_bin" || ! -x "$proxy_bin" ]]; then
    echo "Falta cloud-sql-proxy. Instalalo o seteá CLOUD_SQL_PROXY_BIN para aplicar grants SQL." >&2
    exit 1
  fi
  if ! command -v psql >/dev/null 2>&1; then
    echo "Falta psql. Instalá postgresql-client para aplicar grants SQL." >&2
    exit 1
  fi

  log "Aplicando grants en $db_name para $db_user"
  "$proxy_bin" --address 127.0.0.1 --port 15435 "$NONPRD_SQL_CONNECTION" &
  local proxy_pid=$!
  cleanup_proxy() { kill "$proxy_pid" 2>/dev/null || true; wait "$proxy_pid" 2>/dev/null || true; }
  trap cleanup_proxy EXIT
  sleep 2
  PGPASSWORD="$admin_password" psql -h 127.0.0.1 -p 15435 -U "$SQL_ADMIN_USER" -d "$db_name" -v ON_ERROR_STOP=1 <<SQL
GRANT ALL PRIVILEGES ON DATABASE ${db_name} TO ${db_user};
GRANT USAGE, CREATE ON SCHEMA public TO ${db_user};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO ${db_user};
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO ${db_user};
SQL
  cleanup_proxy
  trap - EXIT
}

main() {
  if [[ "$STG_PROJECT_ID" == "$NONPRD_DB_PROJECT_ID" ]]; then
    log "STG y DBs non-prod usan $STG_PROJECT_ID; manteniendo display name Pymes STG"
    gcloud projects update "$STG_PROJECT_ID" --name="Pymes STG"
  elif [[ -n "${NONPRD_DBS_FOLDER_ID:-}" ]]; then
    log "Renombrando folder $NONPRD_DBS_FOLDER_ID a NONPRD DBs"
    gcloud resource-manager folders update "$NONPRD_DBS_FOLDER_ID" --display-name="NONPRD DBs"

    log "Renombrando display name de $NONPRD_DB_PROJECT_ID a NONPRD DBs"
    gcloud projects update "$NONPRD_DB_PROJECT_ID" --name="NONPRD DBs"
  fi

  ensure_project "$STG_PROJECT_ID" "${STG_FOLDER_ID:-}"
  ensure_project "$PRD_PROJECT_ID" "${PRD_FOLDER_ID:-}"
  enable_apis "$STG_PROJECT_ID"
  enable_apis "$PRD_PROJECT_ID"
  ensure_artifact_repository "$STG_PROJECT_ID"

  ensure_service_account "$STG_PROJECT_ID" "$STG_GHA_SA" "Pymes GitHub Actions STG"
  ensure_service_account "$STG_PROJECT_ID" "$STG_CORE_SA" "Pymes Core Runtime STG"
  ensure_service_account "$STG_PROJECT_ID" "$STG_VERTICAL_SA" "Pymes Vertical Runtime STG"

  local gha_member="serviceAccount:${STG_GHA_SA}@${STG_PROJECT_ID}.iam.gserviceaccount.com"
  local core_member="serviceAccount:${STG_CORE_SA}@${STG_PROJECT_ID}.iam.gserviceaccount.com"
  local vertical_member="serviceAccount:${STG_VERTICAL_SA}@${STG_PROJECT_ID}.iam.gserviceaccount.com"

  log "Asignando roles STG"
  add_project_role "$STG_PROJECT_ID" "$gha_member" roles/run.admin
  add_project_role "$STG_PROJECT_ID" "$gha_member" roles/artifactregistry.writer
  add_project_role "$STG_PROJECT_ID" "$gha_member" roles/secretmanager.secretAccessor
  add_project_role "$STG_PROJECT_ID" "$gha_member" roles/secretmanager.admin
  add_project_role "$STG_PROJECT_ID" "$gha_member" roles/firebasehosting.admin
  add_project_role "$STG_PROJECT_ID" "$gha_member" roles/iam.serviceAccountUser
  add_project_role "$STG_PROJECT_ID" "$core_member" roles/secretmanager.secretAccessor
  add_project_role "$STG_PROJECT_ID" "$vertical_member" roles/secretmanager.secretAccessor

  log "Asignando Cloud SQL Client sobre $NONPRD_DB_PROJECT_ID"
  add_project_role "$NONPRD_DB_PROJECT_ID" "$gha_member" roles/cloudsql.client
  add_project_role "$NONPRD_DB_PROJECT_ID" "$gha_member" roles/cloudsql.admin
  add_project_role "$NONPRD_DB_PROJECT_ID" "$gha_member" roles/secretmanager.secretAccessor
  add_project_role "$NONPRD_DB_PROJECT_ID" "$core_member" roles/cloudsql.client
  add_project_role "$NONPRD_DB_PROJECT_ID" "$vertical_member" roles/cloudsql.client

  if [[ -n "${WIF_PRINCIPAL_SET:-}" ]]; then
    log "Habilitando Workload Identity Federation para $STG_GHA_SA"
    gcloud iam service-accounts add-iam-policy-binding "${STG_GHA_SA}@${STG_PROJECT_ID}.iam.gserviceaccount.com" \
      --project="$STG_PROJECT_ID" \
      --member="$WIF_PRINCIPAL_SET" \
      --role=roles/iam.workloadIdentityUser
  fi

  if [[ -n "$WIF_PROVIDER_PROJECT_ID" ]]; then
    log "Sincronizando condicion del Workload Identity Provider $WIF_PROVIDER_ID"
    gcloud iam workload-identity-pools providers update-oidc "$WIF_PROVIDER_ID" \
      --project="$WIF_PROVIDER_PROJECT_ID" \
      --location=global \
      --workload-identity-pool="$WIF_POOL_ID" \
      --attribute-condition="$WIF_PROVIDER_CONDITION"
  fi

  if [[ "$STG_PROJECT_ID" != "$NONPRD_DB_PROJECT_ID" ]]; then
    log "Validando que STG no tenga Cloud SQL instances"
    if [[ -n "$(gcloud sql instances list --project="$STG_PROJECT_ID" --format='value(name)')" ]]; then
      echo "ERROR: $STG_PROJECT_ID tiene Cloud SQL instances; el plan exige no crear instancias SQL en STG." >&2
      exit 2
    fi
  else
    log "STG usa el proyecto historico con Cloud SQL compartido; no se crean instancias nuevas."
  fi
  gcloud sql instances describe "$NONPRD_SQL_INSTANCE" --project="$NONPRD_DB_PROJECT_ID" >/dev/null

  local admin_password="${NONPRD_SQL_ADMIN_PASSWORD:-$(random_password)}"
  local stg_password="${PYMES_STG_APP_PASSWORD:-$(random_password)}"
  local preview_password="${PYMES_PREVIEW_APP_PASSWORD:-$(random_password)}"

  ensure_secret "$NONPRD_DB_PROJECT_ID" nonprd-sql-admin-password "$admin_password"
  ensure_secret "$STG_PROJECT_ID" pymes-preview-app-password "$preview_password"

  ensure_sql_user "$SQL_ADMIN_USER" "$admin_password"
  ensure_sql_user "$STG_DB_USER" "$stg_password"
  ensure_sql_user "$PREVIEW_DB_USER" "$preview_password"
  ensure_sql_database "$STG_DB_NAME"
  grant_sql_database "$STG_DB_NAME" "$STG_DB_USER" "$admin_password"

  local stg_password_encoded
  stg_password_encoded="$(urlencode "$stg_password")"
  ensure_secret "$STG_PROJECT_ID" pymes-database-url-stg "postgres://${STG_DB_USER}:${stg_password_encoded}@/${STG_DB_NAME}?host=/cloudsql/${NONPRD_SQL_CONNECTION}"

  log "Creando placeholders de secretos STG si faltan"
  ensure_secret "$STG_PROJECT_ID" pymes-clerk-secret-key-stg "${PYMES_CLERK_SECRET_KEY_STG:-placeholder}"
  ensure_secret "$STG_PROJECT_ID" pymes-governance-api-key-stg "${PYMES_GOVERNANCE_API_KEY_STG:-placeholder}"
  ensure_secret "$STG_PROJECT_ID" pymes-companion-api-key-stg "${PYMES_COMPANION_API_KEY_STG:-placeholder}"
  ensure_secret "$STG_PROJECT_ID" pymes-companion-internal-jwt-secret-stg "${PYMES_COMPANION_INTERNAL_JWT_SECRET_STG:-placeholder}"

  log "Preparación GCP completada"
  printf 'STG project: %s\nPRD project: %s\nShared SQL: %s\nSTG DB: %s\n' \
    "$STG_PROJECT_ID" "$PRD_PROJECT_ID" "$NONPRD_SQL_CONNECTION" "$STG_DB_NAME"
}

main "$@"
