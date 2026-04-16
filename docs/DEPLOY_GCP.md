# Deploy a GCP (dev)

Este stack se despliega completo en GCP con una cuenta personal (sin Workspace / sin org):

- **Cloud Run** sirve backend (Go) + frontend (nginx static).
- **Cloud SQL Postgres `db-f1-micro`** (tier más chico, ~US$9/mes siempre prendido).
- **Artifact Registry** guarda las imágenes Docker.
- **Secret Manager** tiene `DATABASE_URL` y placeholders Clerk/JWT.
- **Cloud Build** compila las imágenes (evita drama de arch local).

## Deploy primera vez

```bash
# Login (abre navegador). Hacelo una sola vez por máquina.
gcloud auth login

# Billing account ID: gcloud billing accounts list
BILLING_ACCOUNT=XXXXXX-XXXXXX-XXXXXX ./scripts/deploy-gcp-dev.sh
```

El script crea project `pymes-dev-XXXXX`, Cloud SQL, builds imágenes, deploya y devuelve URLs. Es idempotente: volvé a correrlo para rebuild/redeploy.

## Re-deploy sobre el mismo project

```bash
PROJECT_ID=pymes-dev-352318 ./scripts/deploy-gcp-dev.sh
```

## Secretos reales (Clerk/JWT)

El primer deploy deja los secretos con valor `placeholder`. Para cargarlos:

```bash
printf '%s' 'sk_live_real_key' | \
  gcloud secrets versions add CLERK_SECRET_KEY --data-file=- --project=$PROJECT_ID

printf '%s' 'https://clerk.tu-dominio.com' | \
  gcloud secrets versions add CLERK_JWT_ISSUER --data-file=- --project=$PROJECT_ID

# Cloud Run toma la `:latest` al siguiente deploy; o forzá ya:
gcloud run services update pymes-core --region=us-central1 --project=$PROJECT_ID
```

Para **Clerk publishable key** (front), rebuild con `--build-arg VITE_CLERK_PUBLISHABLE_KEY=pk_live_...` — no se puede inyectar post-build porque Vite lo embebe.

## Seeds en Cloud SQL

Las migraciones corren en el startup del backend Go (wire/bootstrap.go). Los seeds son distintos — hay que conectar vía Cloud SQL Auth Proxy:

```bash
cloud-sql-proxy pymes-dev-352318:us-central1:pymes-dev-db &
# ahora Postgres está en 127.0.0.1:5432

# Exportá DATABASE_URL apuntando al proxy y corré:
DATABASE_URL='postgres://pymes_app:<app_pass>@127.0.0.1:5432/pymes?sslmode=disable' \
  bash scripts/seeds/load.sh
```

Password de `pymes_app` queda en Secret Manager si querés recuperarlo:
```bash
gcloud secrets versions access latest --secret=DATABASE_URL --project=$PROJECT_ID
```

## Costos estimados (dev, idle)

| Recurso | ~US$/mes |
|---|---|
| Cloud SQL `db-f1-micro` (HDD 10GB) | 9 |
| Cloud Run backend (min 0, idle) | 0 |
| Cloud Run frontend (min 0, idle) | 0 |
| Artifact Registry (<500MB) | <0.10 |
| Cloud Build (primeros 120 min/día free) | 0 |
| **Total estimado idle** | **~US$9** |

Con tráfico real de pocas requests/día suma centavos.

## Tear down

```bash
gcloud projects delete $PROJECT_ID
```

Eso borra todo (Cloud Run, Cloud SQL, buckets, secrets, imágenes). Irrevertible.

## Arquitectura resultante

```
Internet
   │
   ├─→ pymes-frontend.run.app   (nginx static, Vite build)
   │        ↓ llama a VITE_API_URL
   └─→ pymes-core.run.app       (Go, Gin)
              ↓ unix socket /cloudsql/...
        Cloud SQL Postgres 16 (db-f1-micro)
```

## Estado actual del proyecto ya deployado

Si corriste el script, la info queda en:

- **Project**: `pymes-dev-352318`
- **Backend**: https://pymes-core-884236221349.us-central1.run.app
- **Frontend**: https://pymes-frontend-884236221349.us-central1.run.app
- **Cloud SQL**: `pymes-dev-352318:us-central1:pymes-dev-db`
