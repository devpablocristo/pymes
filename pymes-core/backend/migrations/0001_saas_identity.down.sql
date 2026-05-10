-- 0001_saas_identity.down.sql — reverso completo de la migración 0001.

DROP TRIGGER IF EXISTS trg_org_usage_counters_updated_at ON org_usage_counters;
DROP TRIGGER IF EXISTS trg_tenant_settings_updated_at ON tenant_settings;
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS admin_activity_events;
DROP TABLE IF EXISTS saas_usage_event_dedup;
DROP TABLE IF EXISTS org_usage_counters;
DROP TABLE IF EXISTS tenant_settings;
DROP TABLE IF EXISTS org_api_key_scopes;
DROP TABLE IF EXISTS org_api_keys;
DROP TABLE IF EXISTS org_members;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS orgs;

-- pgcrypto se preserva (otras migraciones pueden usarlo).
