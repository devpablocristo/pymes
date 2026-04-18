#!/usr/bin/env bash
# Despliegue dev de pymes a GCP (Cloud Run + Cloud SQL + Artifact Registry + Firebase Hosting).
# Idempotente: creaciones usan `|| true` cuando el recurso puede ya existir.
#
# Requisitos:
#   - gcloud instalado y autenticado (`gcloud auth login`)
#   - Billing account ID disponible
#   - Node.js 20+ para build frontend y ejecutar firebase-tools vía npx
#
# Variables override por env:
#   PROJECT_ID       (default: pymes-dev-XXX autogenerado si no existe)
#   REGION           (default: us-central1)
#   BILLING_ACCOUNT  (obligatorio en el primer run si se crea el project)
#
# Uso:
#   ./scripts/deploy-gcp-dev.sh
#   PROJECT_ID=pymes-dev-352318 ./scripts/deploy-gcp-dev.sh   # re-deploy

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-us-central1}"
BILLING_ACCOUNT="${BILLING_ACCOUNT:-}"
CLERK_SECRET_KEY_VALUE="${CLERK_SECRET_KEY_VALUE:-${CLERK_SECRET_KEY:-}}"
CLERK_JWT_ISSUER_VALUE="${CLERK_JWT_ISSUER_VALUE:-${JWT_ISSUER:-}}"
CLERK_JWKS_URL_VALUE="${CLERK_JWKS_URL_VALUE:-${JWKS_URL:-}}"
FRONTEND_CLERK_PUBLISHABLE_KEY="${FRONTEND_CLERK_PUBLISHABLE_KEY:-${VITE_CLERK_PUBLISHABLE_KEY:-}}"
REVIEW_URL_VALUE="${REVIEW_URL_VALUE:-${REVIEW_URL:-}}"
REVIEW_API_KEY_VALUE="${REVIEW_API_KEY_VALUE:-${REVIEW_API_KEY:-}}"
AI_LLM_PROVIDER_VALUE="${AI_LLM_PROVIDER_VALUE:-${LLM_PROVIDER:-gemini}}"
AI_GEMINI_MODEL_VALUE="${AI_GEMINI_MODEL_VALUE:-${GEMINI_MODEL:-gemini-2.0-flash-lite}}"
AI_GEMINI_VERTEX_PROJECT_VALUE="${AI_GEMINI_VERTEX_PROJECT_VALUE:-${GEMINI_VERTEX_PROJECT:-}}"
AI_GEMINI_VERTEX_LOCATION_VALUE="${AI_GEMINI_VERTEX_LOCATION_VALUE:-${GEMINI_VERTEX_LOCATION:-${REGION}}}"
FIREBASE_TOOLS_VERSION="${FIREBASE_TOOLS_VERSION:-13}"

log() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }

upsert_secret() {
  local name="$1"
  local value="$2"
  if [[ -z "$value" ]]; then
    return 0
  fi
  if gcloud secrets describe "$name" --project="$PROJECT_ID" >/dev/null 2>&1; then
    printf '%s' "$value" | gcloud secrets versions add "$name" --data-file=- --project="$PROJECT_ID" >/dev/null
  else
    printf '%s' "$value" | gcloud secrets create "$name" --data-file=- --project="$PROJECT_ID" >/dev/null
  fi
}

# ── 1. Project ──
if [[ -z "$PROJECT_ID" ]]; then
  PROJECT_ID="pymes-dev-$(date +%s | tail -c 7)"
  log "creando project $PROJECT_ID"
  gcloud projects create "$PROJECT_ID" --name="Pymes Dev"
  if [[ -n "$BILLING_ACCOUNT" ]]; then
    gcloud billing projects link "$PROJECT_ID" --billing-account="$BILLING_ACCOUNT"
  else
    echo "BILLING_ACCOUNT no seteado; linkeá billing manualmente y reintentá." >&2
    exit 1
  fi
fi
gcloud config set project "$PROJECT_ID"

log "habilitando APIs"
gcloud services enable \
  run.googleapis.com sqladmin.googleapis.com artifactregistry.googleapis.com \
  secretmanager.googleapis.com cloudbuild.googleapis.com storage.googleapis.com \
  compute.googleapis.com servicenetworking.googleapis.com aiplatform.googleapis.com \
  firebase.googleapis.com firebasehosting.googleapis.com \
  --project="$PROJECT_ID"

# ── 2. Artifact Registry ──
log "Artifact Registry"
gcloud artifacts repositories create pymes \
  --repository-format=docker --location="$REGION" \
  --description="Pymes dev images" --project="$PROJECT_ID" 2>/dev/null || true

# ── 3. Cloud SQL db-f1-micro ──
log "Cloud SQL pymes-dev-db (si no existe)"
if ! gcloud sql instances describe pymes-dev-db --project="$PROJECT_ID" >/dev/null 2>&1; then
  ROOT_PASS=$(openssl rand -base64 24 | tr -d '+/=' | head -c 24)
  gcloud sql instances create pymes-dev-db \
    --database-version=POSTGRES_16 --tier=db-f1-micro --edition=ENTERPRISE \
    --region="$REGION" --storage-size=10 --storage-type=HDD \
    --root-password="$ROOT_PASS" --project="$PROJECT_ID"
  gcloud sql databases create pymes --instance=pymes-dev-db --project="$PROJECT_ID"

  APP_PASS=$(openssl rand -base64 24 | tr -d '+/=' | head -c 24)
  gcloud sql users create pymes_app --instance=pymes-dev-db --password="$APP_PASS" --project="$PROJECT_ID"

  CONN=$(gcloud sql instances describe pymes-dev-db --project="$PROJECT_ID" --format="value(connectionName)")
  DB_URL="postgres://pymes_app:${APP_PASS}@/pymes?host=/cloudsql/${CONN}&sslmode=disable"
  printf '%s' "$DB_URL" | gcloud secrets create DATABASE_URL --data-file=- --project="$PROJECT_ID"

  printf '%s' "${CLERK_SECRET_KEY_VALUE:-placeholder}" | gcloud secrets create CLERK_SECRET_KEY --data-file=- --project="$PROJECT_ID" 2>/dev/null || true
  printf '%s' "${CLERK_JWT_ISSUER_VALUE:-placeholder}" | gcloud secrets create CLERK_JWT_ISSUER --data-file=- --project="$PROJECT_ID" 2>/dev/null || true
  printf 'placeholder' | gcloud secrets create JWT_SECRET --data-file=- --project="$PROJECT_ID" 2>/dev/null || true
fi

upsert_secret CLERK_SECRET_KEY "$CLERK_SECRET_KEY_VALUE"
upsert_secret CLERK_JWT_ISSUER "$CLERK_JWT_ISSUER_VALUE"
upsert_secret REVIEW_API_KEY "$REVIEW_API_KEY_VALUE"

# ── 4. IAM para Cloud Run SA ──
log "IAM service account"
PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format="value(projectNumber)")
SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"
for s in DATABASE_URL CLERK_SECRET_KEY CLERK_JWT_ISSUER JWT_SECRET REVIEW_API_KEY; do
  if gcloud secrets describe "$s" --project="$PROJECT_ID" >/dev/null 2>&1; then
    gcloud secrets add-iam-policy-binding "$s" --member="serviceAccount:$SA" \
      --role="roles/secretmanager.secretAccessor" --project="$PROJECT_ID" >/dev/null
  fi
done
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$SA" --role="roles/cloudsql.client" >/dev/null
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:$SA" --role="roles/aiplatform.user" >/dev/null

# ── 5. Build + push imágenes ──
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/pymes/pymes-core:dev"
AI_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/pymes/pymes-ai:dev"

# Backend: necesita parent dir con core/ hermano
log "staging backend context"
BACKEND_CTX="$(mktemp -d)"
trap 'rm -rf "$BACKEND_CTX"' EXIT
rsync -a --exclude '.git/' --exclude 'node_modules/' --exclude 'frontend/' \
      --exclude 'ai/' --exclude 'dist/' --exclude '.terraform/' --exclude 'tmp/' \
      "$REPO_ROOT/" "$BACKEND_CTX/pymes/"
rsync -a --exclude '.git/' "$(cd "$REPO_ROOT/.." && pwd)/core/scheduling/" "$BACKEND_CTX/core/scheduling/"

log "building backend image"
( cd "$BACKEND_CTX" && gcloud builds submit . \
    --config pymes/pymes-core/backend/cloudbuild.yaml \
    --substitutions="_IMAGE=${BACKEND_IMAGE}" \
    --project="$PROJECT_ID" )

# Primer deploy: backend URL placeholder, luego lo actualizamos
TMP_BACKEND_URL="https://pymes-core-${PROJECT_NUMBER}.${REGION}.run.app"

# ── 6. Deploy Cloud Run backend ──
log "deploy backend (Cloud Run)"
CONN=$(gcloud sql instances describe pymes-dev-db --project="$PROJECT_ID" --format="value(connectionName)")
BACKEND_ENV_VARS="ENVIRONMENT=development,PUBLIC_BASE_URL=${TMP_BACKEND_URL}"
if [[ -n "$CLERK_JWKS_URL_VALUE" ]]; then
  BACKEND_ENV_VARS="${BACKEND_ENV_VARS},JWKS_URL=${CLERK_JWKS_URL_VALUE}"
fi
if [[ -n "$REVIEW_URL_VALUE" ]]; then
  BACKEND_ENV_VARS="${BACKEND_ENV_VARS},REVIEW_URL=${REVIEW_URL_VALUE}"
fi
BACKEND_SECRET_VARS="DATABASE_URL=DATABASE_URL:latest,CLERK_SECRET_KEY=CLERK_SECRET_KEY:latest,JWT_ISSUER=CLERK_JWT_ISSUER:latest"
if [[ -n "$REVIEW_API_KEY_VALUE" ]]; then
  BACKEND_SECRET_VARS="${BACKEND_SECRET_VARS},REVIEW_API_KEY=REVIEW_API_KEY:latest"
fi
gcloud run deploy pymes-core --image="$BACKEND_IMAGE" --region="$REGION" \
  --platform=managed --allow-unauthenticated \
  --add-cloudsql-instances="$CONN" \
  --set-env-vars="$BACKEND_ENV_VARS" \
  --set-secrets="$BACKEND_SECRET_VARS" \
  --min-instances=0 --max-instances=3 --memory=512Mi --cpu=1 --timeout=300 \
  --project="$PROJECT_ID"

BACKEND_URL=$(gcloud run services describe pymes-core --region="$REGION" --project="$PROJECT_ID" --format="value(status.url)")
AI_VERTEX_PROJECT_EFFECTIVE="${AI_GEMINI_VERTEX_PROJECT_VALUE:-$PROJECT_ID}"
AI_VERTEX_LOCATION_EFFECTIVE="${AI_GEMINI_VERTEX_LOCATION_VALUE:-$REGION}"

validate_frontend_api_key() {
  local api_key="$1"
  [[ -n "$api_key" ]] || return 1
  local http_code
  http_code="$(curl -sS -o /dev/null -w '%{http_code}' -H "X-API-KEY: ${api_key}" "${BACKEND_URL}/v1/admin/tenant-settings" || true)"
  [[ "$http_code" == "200" ]]
}

BOOTSTRAP_ORG_NAME="${BOOTSTRAP_ORG_NAME:-Pymes Dev}"
BOOTSTRAP_ORG_SLUG_BASE="${BOOTSTRAP_ORG_SLUG:-${PROJECT_ID}}"
FRONTEND_API_KEY="${FRONTEND_API_KEY:-}"
CLERK_ENABLED=false

if [[ -n "$FRONTEND_CLERK_PUBLISHABLE_KEY" && -n "$CLERK_JWT_ISSUER_VALUE" && -n "$CLERK_JWKS_URL_VALUE" ]]; then
  CLERK_ENABLED=true
  FRONTEND_API_KEY=""
fi

if [[ "$CLERK_ENABLED" != true && -z "$FRONTEND_API_KEY" ]]; then
  FRONTEND_API_KEY="$(gcloud secrets versions access latest --secret=FRONTEND_API_KEY --project="$PROJECT_ID" 2>/dev/null || true)"
fi

if [[ "$CLERK_ENABLED" != true ]]; then
  if ! validate_frontend_api_key "$FRONTEND_API_KEY"; then
    log "bootstrap org + api key para frontend"
    BOOTSTRAP_ORG_SLUG="${BOOTSTRAP_ORG_SLUG_BASE}-$(date +%s)"
    BOOTSTRAP_RESP="$(mktemp)"
    HTTP_CODE="$(curl -sS -o "$BOOTSTRAP_RESP" -w '%{http_code}' \
      -X POST \
      -H 'Content-Type: application/json' \
      -d "{\"name\":\"${BOOTSTRAP_ORG_NAME}\",\"slug\":\"${BOOTSTRAP_ORG_SLUG}\",\"actor\":\"bootstrap\"}" \
      "${BACKEND_URL}/v1/orgs")"
    if [[ "$HTTP_CODE" != "201" ]]; then
      echo "No se pudo bootstrapear la org dev (HTTP ${HTTP_CODE})" >&2
      cat "$BOOTSTRAP_RESP" >&2
      exit 1
    fi
    FRONTEND_API_KEY="$(
      python3 - "$BOOTSTRAP_RESP" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)

raw_key = str(payload.get("raw_key", "")).strip()
if not raw_key:
    raise SystemExit("bootstrap response missing raw_key")
print(raw_key)
PY
    )"
    if gcloud secrets describe FRONTEND_API_KEY --project="$PROJECT_ID" >/dev/null 2>&1; then
      printf '%s' "$FRONTEND_API_KEY" | gcloud secrets versions add FRONTEND_API_KEY --data-file=- --project="$PROJECT_ID" >/dev/null
    else
      printf '%s' "$FRONTEND_API_KEY" | gcloud secrets create FRONTEND_API_KEY --data-file=- --project="$PROJECT_ID" >/dev/null
    fi
  fi
fi

# AI
log "staging ai context"
AI_CTX="$(mktemp -d)"
trap 'rm -rf "$BACKEND_CTX" "$AI_CTX"' EXIT
rsync -a --exclude '.git/' --exclude '.venv/' --exclude '.pytest_cache/' --exclude '.ruff_cache/' \
      --exclude '__pycache__/' --exclude 'node_modules/' --exclude 'dist/' \
      "$REPO_ROOT/ai/" "$AI_CTX/ai/"

log "building ai image"
( cd "$AI_CTX" && gcloud builds submit . \
    --config ai/cloudbuild.yaml \
    --substitutions="_IMAGE=${AI_IMAGE}" \
    --project="$PROJECT_ID" )

log "deploy ai (Cloud Run)"
AI_ENV_VARS="AI_ENVIRONMENT=development,BACKEND_URL=${BACKEND_URL},LLM_PROVIDER=${AI_LLM_PROVIDER_VALUE},GEMINI_MODEL=${AI_GEMINI_MODEL_VALUE},GEMINI_VERTEX_PROJECT=${AI_VERTEX_PROJECT_EFFECTIVE},GEMINI_VERTEX_LOCATION=${AI_VERTEX_LOCATION_EFFECTIVE},AUTH_ALLOW_API_KEY=true"
if [[ -n "$CLERK_JWKS_URL_VALUE" ]]; then
  AI_ENV_VARS="${AI_ENV_VARS},JWKS_URL=${CLERK_JWKS_URL_VALUE}"
fi
if [[ -n "$CLERK_JWT_ISSUER_VALUE" ]]; then
  AI_ENV_VARS="${AI_ENV_VARS},JWT_ISSUER=${CLERK_JWT_ISSUER_VALUE}"
fi
if [[ -n "$REVIEW_URL_VALUE" ]]; then
  AI_ENV_VARS="${AI_ENV_VARS},REVIEW_URL=${REVIEW_URL_VALUE}"
fi
AI_SECRET_VARS="DATABASE_URL=DATABASE_URL:latest"
if gcloud secrets describe REVIEW_API_KEY --project="$PROJECT_ID" >/dev/null 2>&1; then
  AI_SECRET_VARS="${AI_SECRET_VARS},REVIEW_API_KEY=REVIEW_API_KEY:latest"
fi
gcloud run deploy pymes-ai --image="$AI_IMAGE" --region="$REGION" \
  --platform=managed --allow-unauthenticated \
  --add-cloudsql-instances="$CONN" \
  --set-env-vars="$AI_ENV_VARS" \
  --set-secrets="$AI_SECRET_VARS" \
  --min-instances=0 --max-instances=5 --memory=512Mi --cpu=1 --timeout=300 \
  --project="$PROJECT_ID"

AI_URL=$(gcloud run services describe pymes-ai --region="$REGION" --project="$PROJECT_ID" --format="value(status.url)")

# Frontend en Firebase Hosting
log "asegurando proyecto Firebase"
npx --yes "firebase-tools@${FIREBASE_TOOLS_VERSION}" projects:addfirebase "$PROJECT_ID" --non-interactive >/dev/null 2>&1 || true

log "building frontend static bundle"
(
  cd "$REPO_ROOT/frontend"
  export VITE_API_URL="/"
  export VITE_AI_API_URL="/"
  export VITE_CLERK_PUBLISHABLE_KEY="$FRONTEND_CLERK_PUBLISHABLE_KEY"
  export VITE_API_KEY="$FRONTEND_API_KEY"
  npm ci
  npm run build
)

log "deploy frontend (Firebase Hosting)"
npx --yes "firebase-tools@${FIREBASE_TOOLS_VERSION}" deploy \
  --project "$PROJECT_ID" \
  --only hosting \
  --non-interactive

FRONT_URL="https://${PROJECT_ID}.web.app"

# Actualizar backend con FRONTEND_URL real (CORS)
gcloud run services update pymes-core --region="$REGION" --project="$PROJECT_ID" \
  --update-env-vars="FRONTEND_URL=${FRONT_URL},AI_SERVICE_URL=${AI_URL}" >/dev/null

log "LISTO"
echo "PROJECT_ID : $PROJECT_ID"
echo "BACKEND    : $BACKEND_URL"
echo "FRONTEND   : $FRONT_URL"
echo "AI         : $AI_URL"
echo "DB         : $CONN (db-f1-micro, ~US\$9/mes)"
echo
echo "Para seeds con Cloud SQL Proxy:"
echo "  cloud-sql-proxy $CONN &"
echo "  PYMES_DB_HOST=127.0.0.1 ./scripts/seeds/load.sh"
