# GitHub Flow Environments

Pymes deploya desde GitHub Flow:

- PR contra `main`: CI y preview efímero.
- Push a `main`: deploy automático a STG.
- PRD: workflow manual con `environment: prd`.

## Proyectos

- STG: `pymes-dev-352318`
- PRD preparado: `pymes-prd-352318`
- DBs non-prod compartidas: `pymes-dev-352318`

`pymes-dev-352318` conserva el project ID historico, pero operativamente es
el proyecto STG. El proyecto temporal `pymes-stg-352318` quedo descartado por
cuota de billing y no debe usarse.

La instancia Cloud SQL compartida es:

```text
pymes-dev-352318:us-central1:pymes-dev-db
```

No se crean instancias Cloud SQL en STG ni en PRD desde estos workflows.

## Bases Non-Prod

- Legacy: `pymes`
- STG: `pymes_stg`
- Preview: `pymes_preview_pr_<PR>`

STG usa el secret `pymes-database-url-stg` en `pymes-dev-352318`.
Preview crea el secret `pymes-database-url-pr-<PR>` en `pymes-dev-352318`
y lo elimina en `preview-cleanup.yml`.

## Workflows

- `ci.yml`: corre en PRs a `main` y pushes a `main`.
- `deploy-stg.yml`: despliega `all`, `core`, `ui`, `verticals` o una vertical puntual.
- `deploy-prd.yml`: valida que PRD no use proyectos ni DB non-prod; no trae default productivo.
- `preview-pr.yml`: dispara preview en PRs no draft.
- `preview-deploy.yml`: reusable/manual para previews.
- `preview-cleanup.yml`: borra Cloud Run preview, Firebase channel, DB preview y secret preview.
- `preview-audit.yml`: lista previews sin PR abierto.

## Preparacion GCP

El script idempotente está en:

```bash
scripts/infra/setup_gcp_github_flow.sh
```

Variables habituales:

```bash
export BILLING_ACCOUNT_ID=...
export STG_PROJECT_ID=pymes-dev-352318
export STG_FOLDER_ID=175650469464
export PRD_FOLDER_ID=...
export WIF_PRINCIPAL_SET='principalSet://iam.googleapis.com/projects/.../locations/global/workloadIdentityPools/github-actions-pool/attribute.repository/devpablocristo/pymes'
export WIF_PROVIDER_PROJECT_ID=pymes-dev-352318
scripts/infra/setup_gcp_github_flow.sh
```

Cuando `STG_PROJECT_ID` y `NONPRD_DB_PROJECT_ID` son el mismo proyecto, el
script no renombra el proyecto a `NONPRD DBs`; lo mantiene como `Pymes STG`.
Prepara SAs, roles, secrets y bases logicas, y valida que se use la instancia
existente `pymes-dev-db`.

El Workload Identity Provider debe aceptar el deploy STG/PRD desde `main` y
los previews de PR contra `main`:

```text
assertion.repository=='devpablocristo/pymes' && (assertion.ref=='refs/heads/main' || (assertion.event_name=='pull_request' && assertion.base_ref=='main'))
```
