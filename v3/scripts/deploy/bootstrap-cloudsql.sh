#!/usr/bin/env bash
set -euo pipefail

# Provisions least-privilege PostgreSQL roles for Pymes v3. Administrative
# credentials are accepted through libpq environment variables so no password
# appears in the psql process arguments.
: "${PGHOST:?set PGHOST for the Cloud SQL Auth Proxy or private endpoint}"
: "${PGPORT:?set PGPORT}"
: "${PGUSER:?set the administrative PostgreSQL user}"
: "${PGPASSWORD:?set the administrative PostgreSQL password}"
PGDATABASE=${PGDATABASE:-postgres}
export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE

bootstrap_env=${PYMES_BOOTSTRAP_ENV:-all}
case "$bootstrap_env" in stg|prd|all) ;; *) echo "PYMES_BOOTSTRAP_ENV must be stg, prd, or all" >&2; exit 2 ;; esac
bootstrap_components=${PYMES_BOOTSTRAP_COMPONENTS:?set PYMES_BOOTSTRAP_COMPONENTS to a comma-separated list: api,worker,fiscal,accounting,accounting-admin,all}
rotate_credentials=${PYMES_ROTATE_DATABASE_CREDENTIALS:-false}
case "$rotate_credentials" in true|false) ;; *) echo "PYMES_ROTATE_DATABASE_CREDENTIALS must be true or false" >&2; exit 2 ;; esac
case ",$bootstrap_components," in
  *,all,*)
    bootstrap_components=all
    ;;
esac
for component in ${bootstrap_components//,/ }; do
  case "$component" in
    api|worker|fiscal|accounting|accounting-admin|all) ;;
    *) echo "unknown PYMES_BOOTSTRAP_COMPONENTS entry: $component" >&2; exit 2 ;;
  esac
done
if [[ "$bootstrap_components" == "all" || ",$bootstrap_components," == *,api,* ||
      ",$bootstrap_components," == *,worker,* || ",$bootstrap_components," == *,fiscal,* ||
      ",$bootstrap_components," == *,accounting,* ]]; then
  if [[ "$rotate_credentials" != "true" ]]; then
    echo "runtime/migration credential writes require PYMES_ROTATE_DATABASE_CREDENTIALS=true" >&2
    exit 2
  fi
fi

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
instance=${PYMES_CLOUDSQL_INSTANCE:-"$project:$region:pymes-dev-db"}
export CLOUDSDK_CORE_PROJECT="$project"

cleanup_memberships() {
  local owner_role="$1" runtime_role="$2"
  export PYMES_DATABASE_OWNER_ROLE="$owner_role" PYMES_DATABASE_RUNTIME_ROLE="$runtime_role"
  psql --no-password --dbname="$PGDATABASE" -v ON_ERROR_STOP=1 >/dev/null <<'SQL' || true
\getenv owner_role PYMES_DATABASE_OWNER_ROLE
\getenv runtime_role PYMES_DATABASE_RUNTIME_ROLE
SELECT format('REVOKE %I FROM CURRENT_USER', :'runtime_role')
WHERE pg_has_role(current_user, :'runtime_role', 'member')
\gexec
SELECT format('REVOKE %I FROM CURRENT_USER', :'owner_role')
WHERE pg_has_role(current_user, :'owner_role', 'member')
\gexec
SQL
  unset PYMES_DATABASE_OWNER_ROLE PYMES_DATABASE_RUNTIME_ROLE
}

publish_url() {
  local secret="$1" url="$2"
  printf '%s' "$url" | gcloud secrets versions add "$secret" --data-file=- >/dev/null
}

component_selected() {
  [[ "$bootstrap_components" == "all" || ",$bootstrap_components," == *",$1,"* ]]
}

secret_has_enabled_version() {
  local secret="$1"
  gcloud secrets versions list "$secret" --project="$project" \
    --limit=20 --format='value(state)' | grep -iq '^enabled$'
}

configure_service_database() {
  local environment="$1" component="$2" database="$3" schema="$4" runtime_role="$5" owner_role="$6" migrate_role="$7" runtime_secret="$8" migrate_secret="$9" allow_runtime_create="${10}"
  local runtime_password migrate_password runtime_url migrate_url
  runtime_password=$(openssl rand -hex 32)
  migrate_password=$(openssl rand -hex 32)
  export PYMES_DATABASE_NAME="$database" PYMES_DATABASE_SCHEMA="$schema"
  export PYMES_DATABASE_RUNTIME_ROLE="$runtime_role" PYMES_DATABASE_RUNTIME_PASSWORD="$runtime_password"
  export PYMES_DATABASE_OWNER_ROLE="$owner_role" PYMES_DATABASE_MIGRATE_ROLE="$migrate_role" PYMES_DATABASE_MIGRATE_PASSWORD="$migrate_password"
  export PYMES_DATABASE_ALLOW_RUNTIME_CREATE="$allow_runtime_create"

  if ! psql --no-password --dbname="$PGDATABASE" -v ON_ERROR_STOP=1 <<'SQL'
\getenv database_name PYMES_DATABASE_NAME
\getenv runtime_role PYMES_DATABASE_RUNTIME_ROLE
\getenv runtime_password PYMES_DATABASE_RUNTIME_PASSWORD
\getenv owner_role PYMES_DATABASE_OWNER_ROLE
\getenv migrate_role PYMES_DATABASE_MIGRATE_ROLE
\getenv migrate_password PYMES_DATABASE_MIGRATE_PASSWORD
SELECT format('CREATE ROLE %I NOLOGIN', :'owner_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'owner_role')
\gexec
SELECT format('ALTER ROLE %I NOLOGIN', :'owner_role')
\gexec
SELECT format('CREATE ROLE %I LOGIN', :'runtime_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'runtime_role')
\gexec
SELECT format('ALTER ROLE %I LOGIN PASSWORD %L', :'runtime_role', :'runtime_password')
\gexec
SELECT format('CREATE ROLE %I LOGIN', :'migrate_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'migrate_role')
\gexec
SELECT format('ALTER ROLE %I LOGIN PASSWORD %L', :'migrate_role', :'migrate_password')
\gexec
SELECT 1 / CASE WHEN EXISTS (
  SELECT 1
  FROM pg_roles
  WHERE rolname IN (:'owner_role', :'runtime_role', :'migrate_role')
    AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)
) THEN 0 ELSE 1 END AS least_privilege_roles;
SELECT format('GRANT %I TO %I', :'owner_role', :'migrate_role')
\gexec
SELECT format('GRANT %I TO CURRENT_USER', :'owner_role')
\gexec
SELECT format('GRANT %I TO CURRENT_USER', :'runtime_role')
\gexec
SELECT format('CREATE DATABASE %I OWNER %I', :'database_name', :'owner_role')
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = :'database_name')
\gexec
SELECT format('ALTER DATABASE %I OWNER TO %I', :'database_name', :'owner_role')
\gexec
SQL
  then
    cleanup_memberships "$owner_role" "$runtime_role"
    return 1
  fi

  if ! psql --no-password --dbname="$database" -v ON_ERROR_STOP=1 <<'SQL'
\getenv database_name PYMES_DATABASE_NAME
\getenv database_schema PYMES_DATABASE_SCHEMA
\getenv runtime_role PYMES_DATABASE_RUNTIME_ROLE
\getenv owner_role PYMES_DATABASE_OWNER_ROLE
\getenv migrate_role PYMES_DATABASE_MIGRATE_ROLE
\getenv allow_runtime_create PYMES_DATABASE_ALLOW_RUNTIME_CREATE
BEGIN;
SELECT format('REASSIGN OWNED BY %I TO %I', :'runtime_role', :'owner_role')
\gexec
SELECT format('REVOKE ALL ON DATABASE %I FROM PUBLIC', :'database_name')
\gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO %I', :'database_name', current_user)
\gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO %I', :'database_name', :'runtime_role')
\gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO %I', :'database_name', :'migrate_role')
\gexec
SELECT format('CREATE SCHEMA IF NOT EXISTS %I AUTHORIZATION %I', :'database_schema', :'owner_role')
\gexec
SELECT format('ALTER SCHEMA %I OWNER TO %I', :'database_schema', :'owner_role')
\gexec
SELECT format('REVOKE ALL ON SCHEMA %I FROM PUBLIC', :'database_schema')
\gexec
SELECT format('GRANT USAGE ON SCHEMA %I TO %I', :'database_schema', :'runtime_role')
\gexec
SELECT format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %I TO %I', :'database_schema', :'runtime_role')
\gexec
SELECT format('GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA %I TO %I', :'database_schema', :'runtime_role')
\gexec
SELECT format('GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA %I TO %I', :'database_schema', :'runtime_role')
\gexec
SELECT format('ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA %I GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %I', :'owner_role', :'database_schema', :'runtime_role')
\gexec
SELECT format('ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA %I GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO %I', :'owner_role', :'database_schema', :'runtime_role')
\gexec
SELECT format('ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA %I GRANT EXECUTE ON FUNCTIONS TO %I', :'owner_role', :'database_schema', :'runtime_role')
\gexec
SELECT format('GRANT CREATE ON DATABASE %I TO %I', :'database_name', :'runtime_role')
WHERE :'allow_runtime_create' = 'true'
\gexec
SELECT format('REVOKE CREATE ON DATABASE %I FROM %I', :'database_name', :'runtime_role')
WHERE :'allow_runtime_create' <> 'true'
\gexec
COMMIT;
SQL
  then
    cleanup_memberships "$owner_role" "$runtime_role"
    return 1
  fi
  cleanup_memberships "$owner_role" "$runtime_role"

  runtime_url="postgres://${runtime_role}:${runtime_password}@/${database}?host=/cloudsql/${instance}&sslmode=disable"
  migrate_url="postgres://${migrate_role}:${migrate_password}@/${database}?host=/cloudsql/${instance}&sslmode=disable&options=-c%20role%3D${owner_role}"
  publish_url "$runtime_secret" "$runtime_url"
  publish_url "$migrate_secret" "$migrate_url"
  unset PYMES_DATABASE_NAME PYMES_DATABASE_SCHEMA PYMES_DATABASE_RUNTIME_ROLE PYMES_DATABASE_RUNTIME_PASSWORD
  unset PYMES_DATABASE_OWNER_ROLE PYMES_DATABASE_MIGRATE_ROLE PYMES_DATABASE_MIGRATE_PASSWORD PYMES_DATABASE_ALLOW_RUNTIME_CREATE
  unset runtime_password migrate_password runtime_url migrate_url
  printf 'configured %s/%s runtime and migration credentials\n' "$environment" "$component"
}

configure_worker_role() {
  local environment="$1" database="$2" schema="$3" owner_role="$4" worker_role="$5" secret="$6"
  local password url
  password=$(openssl rand -hex 32)
  export PYMES_DATABASE_NAME="$database" PYMES_DATABASE_SCHEMA="$schema" PYMES_DATABASE_OWNER_ROLE="$owner_role"
  export PYMES_DATABASE_WORKER_ROLE="$worker_role" PYMES_DATABASE_WORKER_PASSWORD="$password"
  if ! psql --no-password --dbname="$PGDATABASE" -v ON_ERROR_STOP=1 <<'SQL'
\getenv owner_role PYMES_DATABASE_OWNER_ROLE
\getenv worker_role PYMES_DATABASE_WORKER_ROLE
\getenv worker_password PYMES_DATABASE_WORKER_PASSWORD
SELECT format('CREATE ROLE %I LOGIN', :'worker_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'worker_role')
\gexec
SELECT format('ALTER ROLE %I LOGIN PASSWORD %L', :'worker_role', :'worker_password')
\gexec
SELECT 1 / CASE WHEN EXISTS (
  SELECT 1
  FROM pg_roles
  WHERE rolname = :'worker_role'
    AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)
) THEN 0 ELSE 1 END AS least_privilege_worker_role;
SELECT format('GRANT %I TO CURRENT_USER', :'owner_role')
\gexec
SQL
  then
    cleanup_memberships "$owner_role" "$worker_role"
    return 1
  fi
  if ! psql --no-password --dbname="$database" -v ON_ERROR_STOP=1 <<'SQL'
\getenv database_name PYMES_DATABASE_NAME
\getenv database_schema PYMES_DATABASE_SCHEMA
\getenv owner_role PYMES_DATABASE_OWNER_ROLE
\getenv worker_role PYMES_DATABASE_WORKER_ROLE
SELECT format('GRANT CONNECT ON DATABASE %I TO %I', :'database_name', :'worker_role')
\gexec
SELECT format('GRANT USAGE ON SCHEMA %I TO %I', :'database_schema', :'worker_role')
\gexec
SELECT format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %I TO %I', :'database_schema', :'worker_role')
\gexec
SELECT format('GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA %I TO %I', :'database_schema', :'worker_role')
\gexec
SELECT format('ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA %I GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %I', :'owner_role', :'database_schema', :'worker_role')
\gexec
SELECT format('ALTER DEFAULT PRIVILEGES FOR ROLE %I IN SCHEMA %I GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO %I', :'owner_role', :'database_schema', :'worker_role')
\gexec
SQL
  then
    cleanup_memberships "$owner_role" "$worker_role"
    return 1
  fi
  cleanup_memberships "$owner_role" "$worker_role"
  url="postgres://${worker_role}:${password}@/${database}?host=/cloudsql/${instance}&sslmode=disable"
  publish_url "$secret" "$url"
  unset PYMES_DATABASE_NAME PYMES_DATABASE_SCHEMA PYMES_DATABASE_OWNER_ROLE PYMES_DATABASE_WORKER_ROLE PYMES_DATABASE_WORKER_PASSWORD password url
  printf 'configured %s/worker tenant-scoped credential\n' "$environment"
}

configure_accounting_admin_role() {
  local environment="$1" database="$2" owner_role="$3" admin_role="$4" runtime_role="$5" secret="$6"
  local password='' url='' write_password=false
  if [[ "$rotate_credentials" == "true" ]] || ! secret_has_enabled_version "$secret"; then
    password=$(openssl rand -hex 32)
    write_password=true
  fi
  export PYMES_DATABASE_NAME="$database" PYMES_DATABASE_OWNER_ROLE="$owner_role"
  export PYMES_DATABASE_ADMIN_ROLE="$admin_role" PYMES_DATABASE_ADMIN_PASSWORD="$password"
  export PYMES_DATABASE_RUNTIME_ROLE="$runtime_role"
  export PYMES_DATABASE_WRITE_PASSWORD="$write_password"
  if ! psql --no-password --dbname="$PGDATABASE" -v ON_ERROR_STOP=1 <<'SQL'
\getenv owner_role PYMES_DATABASE_OWNER_ROLE
\getenv admin_role PYMES_DATABASE_ADMIN_ROLE
\getenv admin_password PYMES_DATABASE_ADMIN_PASSWORD
\getenv write_password PYMES_DATABASE_WRITE_PASSWORD
SELECT format('CREATE ROLE %I LOGIN NOINHERIT', :'admin_role')
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = :'admin_role')
\gexec
SELECT format('ALTER ROLE %I LOGIN NOINHERIT PASSWORD %L', :'admin_role', :'admin_password')
WHERE :'write_password' = 'true'
\gexec
SELECT 1 / CASE WHEN EXISTS (
  SELECT 1
  FROM pg_roles
  WHERE rolname = :'admin_role'
    AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)
) THEN 0 ELSE 1 END AS least_privilege_accounting_admin_role;
SELECT format('GRANT %I TO %I', :'owner_role', :'admin_role')
\gexec
SELECT format('GRANT %I TO CURRENT_USER', :'owner_role')
\gexec
SQL
  then
    cleanup_memberships "$owner_role" "$admin_role"
    return 1
  fi
  if ! psql --no-password --dbname="$database" -v ON_ERROR_STOP=1 <<'SQL'
\getenv database_name PYMES_DATABASE_NAME
\getenv admin_role PYMES_DATABASE_ADMIN_ROLE
\getenv runtime_role PYMES_DATABASE_RUNTIME_ROLE
SELECT format('GRANT CONNECT ON DATABASE %I TO %I', :'database_name', :'admin_role')
\gexec
SELECT format('REVOKE CREATE ON DATABASE %I FROM %I', :'database_name', :'runtime_role')
\gexec
SELECT format('REVOKE CREATE ON SCHEMA public FROM %I', :'runtime_role')
\gexec
SELECT 1 / CASE WHEN has_database_privilege(:'runtime_role', :'database_name', 'CREATE')
  OR has_schema_privilege(:'runtime_role', 'public', 'CREATE')
  THEN 0 ELSE 1 END AS runtime_has_no_ddl;
SQL
  then
    cleanup_memberships "$owner_role" "$admin_role"
    return 1
  fi
  cleanup_memberships "$owner_role" "$admin_role"
  if [[ "$write_password" == "true" ]]; then
    url="postgres://${admin_role}:${password}@/${database}?host=/cloudsql/${instance}&sslmode=disable&options=-c%20role%3D${owner_role}"
    publish_url "$secret" "$url"
  fi
  unset PYMES_DATABASE_NAME PYMES_DATABASE_OWNER_ROLE PYMES_DATABASE_ADMIN_ROLE PYMES_DATABASE_ADMIN_PASSWORD
  unset PYMES_DATABASE_RUNTIME_ROLE
  unset PYMES_DATABASE_WRITE_PASSWORD password url write_password
  printf 'configured %s/accounting administrative credential\n' "$environment"
}

if [[ "$bootstrap_env" == "all" ]]; then
  environments=(stg prd)
else
  environments=("$bootstrap_env")
fi

for environment in "${environments[@]}"; do
  prefix="pymes-v3-${environment}"
  if component_selected api; then
    configure_service_database "$environment" api "pymes_v3_${environment}" app \
      "pymes_v3_app_${environment}" "pymes_v3_owner_${environment}" "pymes_v3_migrate_${environment}" \
      "$prefix-database-url" "$prefix-migrate-database-url" false
  fi
  if component_selected worker; then
    configure_worker_role "$environment" "pymes_v3_${environment}" app "pymes_v3_owner_${environment}" \
      "pymes_v3_worker_${environment}" "$prefix-worker-database-url"
  fi
  if component_selected fiscal; then
    configure_service_database "$environment" fiscal "pymes_v3_fiscal_${environment}" fiscal \
      "pymes_v3_fiscal_${environment}" "pymes_v3_fiscal_owner_${environment}" "pymes_v3_fiscal_migrate_${environment}" \
      "$prefix-fiscal-database-url" "$prefix-fiscal-migrate-database-url" false
  fi
  if component_selected accounting; then
    configure_service_database "$environment" accounting "pymes_v3_accounting_${environment}" public \
      "pymes_v3_accounting_${environment}" "pymes_v3_accounting_owner_${environment}" "pymes_v3_accounting_migrate_${environment}" \
      "$prefix-accounting-database-url" "$prefix-accounting-migrate-database-url" false
  fi
  if component_selected accounting-admin; then
    configure_accounting_admin_role "$environment" "pymes_v3_accounting_${environment}" \
      "pymes_v3_accounting_owner_${environment}" "pymes_v3_accounting_admin_${environment}" \
      "pymes_v3_accounting_${environment}" \
      "$prefix-accounting-admin-database-url"
  fi
done
