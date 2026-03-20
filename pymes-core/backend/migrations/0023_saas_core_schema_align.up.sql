-- Align org API key columns with saas-core GORM models (api_key_hash, api_key_id).
-- Run before using github.com/devpablocristo/saas-core org/users repositories on this DB.

ALTER TABLE org_api_keys RENAME COLUMN key_hash TO api_key_hash;

ALTER TABLE org_api_key_scopes RENAME COLUMN key_id TO api_key_id;

-- saas-core admin/billing use hard_limits_json; keep hard_limits for Pymes ERP reads.
ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS hard_limits_json jsonb NOT NULL DEFAULT '{}'::jsonb;

UPDATE tenant_settings SET hard_limits_json = COALESCE(hard_limits, '{}'::jsonb)
WHERE hard_limits_json = '{}'::jsonb AND hard_limits IS NOT NULL;

-- Tenant lifecycle + billing fields expected by saas-core (may already exist from older migrations).
ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';
ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS past_due_since timestamptz;
