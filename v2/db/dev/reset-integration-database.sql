\connect postgres

DROP DATABASE IF EXISTS pymes_v2_integration WITH (FORCE);
CREATE DATABASE pymes_v2_integration;

GRANT CONNECT, CREATE, TEMPORARY
ON DATABASE pymes_v2_integration
TO pymes_migrator;
GRANT CONNECT, TEMPORARY
ON DATABASE pymes_v2_integration
TO
    pymes_backend,
    pymes_iam_worker,
    pymes_fiscal_worker,
    pymes_fiscal_accounting_worker;

\connect pymes_v2_integration

GRANT USAGE, CREATE ON SCHEMA public TO pymes_migrator;
