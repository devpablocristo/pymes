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
GOVERNANCE_URL_EFFECTIVE="${GOVERNANCE_URL:-}"
GOVERNANCE_API_KEY_EFFECTIVE="${GOVERNANCE_API_KEY:-}"
# AI_*_VALUE vars eliminadas: pymes-ai decomisionado (Fase 4
# modular-swinging-hummingbird). Companion gestiona su propio deploy de LLM.
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
upsert_secret GOVERNANCE_API_KEY "$GOVERNANCE_API_KEY_EFFECTIVE"

# ── 4. IAM para Cloud Run SA ──
log "IAM service account"
PROJECT_NUMBER=$(gcloud projects describe "$PROJECT_ID" --format="value(projectNumber)")
SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"
for s in DATABASE_URL CLERK_SECRET_KEY CLERK_JWT_ISSUER JWT_SECRET GOVERNANCE_API_KEY; do
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
# Nota: pymes-ai fue decomisionado (modular-swinging-hummingbird Fase 4). El
# chat lo sirve Companion (repo hermano); este script ya no construye ni
# deploya el servicio pymes-ai.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/pymes/core:dev"

# Backend: necesita parent dir con core/ hermano
log "staging backend context"
BACKEND_CTX="$(mktemp -d)"
trap 'rm -rf "$BACKEND_CTX"' EXIT
rsync -a --exclude '.git/' --exclude 'node_modules/' --exclude 'ui/' \
      --exclude 'ai/' --exclude 'dist/' --exclude '.terraform/' --exclude 'tmp/' \
      "$REPO_ROOT/" "$BACKEND_CTX/pymes/"
rsync -a --exclude '.git/' "$(cd "$REPO_ROOT/.." && pwd)/core/scheduling/" "$BACKEND_CTX/core/scheduling/"

log "building backend image"
( cd "$BACKEND_CTX" && gcloud builds submit . \
    --config pymes/core/backend/cloudbuild.yaml \
    --substitutions="_IMAGE=${BACKEND_IMAGE}" \
    --project="$PROJECT_ID" )

# Primer deploy: backend URL placeholder, luego lo actualizamos
TMP_BACKEND_URL="https://core-${PROJECT_NUMBER}.${REGION}.run.app"

# ── 6. Deploy Cloud Run backend ──
log "deploy backend (Cloud Run)"
CONN=$(gcloud sql instances describe pymes-dev-db --project="$PROJECT_ID" --format="value(connectionName)")
BACKEND_ENV_VARS="ENVIRONMENT=development,PUBLIC_BASE_URL=${TMP_BACKEND_URL}"
if [[ -n "$CLERK_JWKS_URL_VALUE" ]]; then
  BACKEND_ENV_VARS="${BACKEND_ENV_VARS},JWKS_URL=${CLERK_JWKS_URL_VALUE}"
fi
if [[ -n "$GOVERNANCE_URL_EFFECTIVE" ]]; then
  BACKEND_ENV_VARS="${BACKEND_ENV_VARS},GOVERNANCE_URL=${GOVERNANCE_URL_EFFECTIVE}"
fi
BACKEND_SECRET_VARS="DATABASE_URL=DATABASE_URL:latest,CLERK_SECRET_KEY=CLERK_SECRET_KEY:latest,JWT_ISSUER=CLERK_JWT_ISSUER:latest"
if [[ -n "$GOVERNANCE_API_KEY_EFFECTIVE" ]]; then
  BACKEND_SECRET_VARS="${BACKEND_SECRET_VARS},GOVERNANCE_API_KEY=GOVERNANCE_API_KEY:latest"
fi
gcloud run deploy core --image="$BACKEND_IMAGE" --region="$REGION" \
  --platform=managed --allow-unauthenticated \
  --add-cloudsql-instances="$CONN" \
  --set-env-vars="$BACKEND_ENV_VARS" \
  --set-secrets="$BACKEND_SECRET_VARS" \
  --min-instances=0 --max-instances=3 --memory=512Mi --cpu=1 --timeout=300 \
  --project="$PROJECT_ID"

BACKEND_URL=$(gcloud run services describe core --region="$REGION" --project="$PROJECT_ID" --format="value(status.url)")

CLERK_ENABLED=false

if [[ -n "$FRONTEND_CLERK_PUBLISHABLE_KEY" && -n "$CLERK_SECRET_KEY_VALUE" && -n "$CLERK_JWT_ISSUER_VALUE" && -n "$CLERK_JWKS_URL_VALUE" ]]; then
  CLERK_ENABLED=true
fi

if [[ "$CLERK_ENABLED" != true ]]; then
  echo "El deploy DEV requiere Clerk: seteá FRONTEND_CLERK_PUBLISHABLE_KEY, CLERK_SECRET_KEY_VALUE, CLERK_JWT_ISSUER_VALUE y CLERK_JWKS_URL_VALUE." >&2
  exit 1
fi

# AI: pymes-ai decomisionado. El chat ahora lo sirve Companion (sibling
# repo); su deploy se gestiona desde companion/.github/workflows.

# Frontend en Firebase Hosting
log "asegurando proyecto Firebase"
npx --yes "firebase-tools@${FIREBASE_TOOLS_VERSION}" projects:addfirebase "$PROJECT_ID" --non-interactive >/dev/null 2>&1 || true

log "building ui static bundle"
(
  cd "$REPO_ROOT/ui"
  export VITE_API_URL="/"
  export VITE_COMPANION_BASE_URL="/"
  export VITE_CLERK_PUBLISHABLE_KEY="$FRONTEND_CLERK_PUBLISHABLE_KEY"
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
gcloud run services update core --region="$REGION" --project="$PROJECT_ID" \
  --update-env-vars="FRONTEND_URL=${FRONT_URL}" >/dev/null

log "LISTO"
echo "PROJECT_ID : $PROJECT_ID"
echo "BACKEND    : $BACKEND_URL"
echo "FRONTEND   : $FRONT_URL"
echo "DB         : $CONN (db-f1-micro, ~US\$9/mes)"
echo
echo "Para seeds con Cloud SQL Proxy:"
echo "  cloud-sql-proxy $CONN &"
echo "  PYMES_DB_HOST=127.0.0.1 ./scripts/seeds/load.sh"
