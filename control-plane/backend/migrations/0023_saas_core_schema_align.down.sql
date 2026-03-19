ALTER TABLE tenant_settings DROP COLUMN IF EXISTS past_due_since;
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS status;
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS hard_limits_json;

ALTER TABLE org_api_key_scopes RENAME COLUMN api_key_id TO key_id;
ALTER TABLE org_api_keys RENAME COLUMN api_key_hash TO key_hash;
