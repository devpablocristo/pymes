DO $roles$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_migrator') THEN
        CREATE ROLE pymes_migrator
            LOGIN
            PASSWORD 'pymes_migrator'
            BYPASSRLS
            NOINHERIT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        CREATE ROLE pymes_backend
            LOGIN
            PASSWORD 'pymes_backend'
            NOBYPASSRLS
            NOINHERIT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_iam_worker') THEN
        CREATE ROLE pymes_iam_worker
            LOGIN
            PASSWORD 'pymes_iam_worker'
            NOBYPASSRLS
            NOINHERIT;
    END IF;
END
$roles$;

ALTER ROLE pymes_migrator PASSWORD 'pymes_migrator' BYPASSRLS NOINHERIT;
ALTER ROLE pymes_backend PASSWORD 'pymes_backend' NOBYPASSRLS NOINHERIT;
ALTER ROLE pymes_iam_worker PASSWORD 'pymes_iam_worker' NOBYPASSRLS NOINHERIT;

GRANT CONNECT, CREATE, TEMPORARY ON DATABASE pymes_v2 TO pymes_migrator;
GRANT CONNECT, TEMPORARY ON DATABASE pymes_v2 TO pymes_backend;
GRANT CONNECT, TEMPORARY ON DATABASE pymes_v2 TO pymes_iam_worker;
GRANT USAGE, CREATE ON SCHEMA public TO pymes_migrator;

DO $legacy$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
        ON TABLE public.schema_migrations
        TO pymes_migrator;
    END IF;
    IF to_regnamespace('app') IS NOT NULL THEN
        GRANT USAGE, CREATE ON SCHEMA app TO pymes_migrator;
    END IF;
END
$legacy$;
