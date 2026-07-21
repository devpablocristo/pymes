-- 0008_employees_and_rbac.down.sql

DROP TRIGGER IF EXISTS trg_roles_updated_at ON roles;
DROP TRIGGER IF EXISTS trg_employees_updated_at ON employees;

DROP TABLE IF EXISTS scheduler_runs;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS employees;
