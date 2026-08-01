#!/usr/bin/env sh
set -eu

# Docker-only role bootstrap. The accounting PostgreSQL container uses trust
# authentication on its isolated development network, so no reusable password
# is stored in the repository. Production always uses Secret Manager URLs.
: "${ACCOUNTING_ADMIN_DATABASE_URL:?ACCOUNTING_ADMIN_DATABASE_URL is required}"

psql "$ACCOUNTING_ADMIN_DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'accounting_owner') THEN
    CREATE ROLE accounting_owner NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'accounting_runtime') THEN
    CREATE ROLE accounting_runtime LOGIN NOINHERIT;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'accounting_control') THEN
    CREATE ROLE accounting_control LOGIN NOINHERIT;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'accounting_migrate') THEN
    CREATE ROLE accounting_migrate LOGIN NOINHERIT;
  END IF;
END
$$;

ALTER ROLE accounting_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
ALTER ROLE accounting_runtime LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
ALTER ROLE accounting_control LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
ALTER ROLE accounting_migrate LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
GRANT accounting_owner TO accounting_control, accounting_migrate;
ALTER DATABASE pymes_accounting OWNER TO accounting_owner;
REVOKE ALL ON DATABASE pymes_accounting FROM PUBLIC;
GRANT CONNECT ON DATABASE pymes_accounting TO accounting_runtime, accounting_control, accounting_migrate;
REVOKE CREATE ON DATABASE pymes_accounting FROM accounting_runtime;

-- Integration tests may have provisioned tenant objects through the local
-- bootstrap superuser before the least-privilege roles existed. Reconcile
-- only registered headless tenant schemas in this isolated Docker database;
-- Cloud environments perform ownership setup with their migration identity.
DO $$
DECLARE
  tenant_schema record;
  tenant_relation record;
BEGIN
  IF to_regclass('public.pymes_accounting_organizations') IS NULL THEN
    RETURN;
  END IF;

  FOR tenant_schema IN
    SELECT mapping.schema_name
    FROM public.pymes_accounting_organizations AS mapping
    JOIN pg_namespace AS namespace ON namespace.nspname = mapping.schema_name
  LOOP
    FOR tenant_relation IN
      SELECT relation.relkind, relation.relname
      FROM pg_class AS relation
      JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
      WHERE namespace.nspname = tenant_schema.schema_name
        AND relation.relkind IN ('r', 'p', 'S')
        AND relation.relowner <> (SELECT oid FROM pg_roles WHERE rolname = 'accounting_owner')
    LOOP
      IF tenant_relation.relkind = 'S' THEN
        EXECUTE format(
          'ALTER SEQUENCE %I.%I OWNER TO accounting_owner',
          tenant_schema.schema_name,
          tenant_relation.relname
        );
      ELSE
        EXECUTE format(
          'ALTER TABLE %I.%I OWNER TO accounting_owner',
          tenant_schema.schema_name,
          tenant_relation.relname
        );
      END IF;
    END LOOP;

    EXECUTE format(
      'ALTER SCHEMA %I OWNER TO accounting_owner',
      tenant_schema.schema_name
    );
  END LOOP;
END
$$;
SQL
