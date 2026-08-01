BEGIN;

CREATE TABLE IF NOT EXISTS app.organization_feature_flags (
  org_id text PRIMARY KEY REFERENCES app.organizations(id) ON DELETE CASCADE,
  scheduling_enabled boolean NOT NULL DEFAULT false,
  whatsapp_enabled boolean NOT NULL DEFAULT false,
  google_calendar_enabled boolean NOT NULL DEFAULT false,
  fiscal_real_enabled boolean NOT NULL DEFAULT false,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by text NOT NULL DEFAULT 'system'
    CHECK (btrim(updated_by) <> '' AND char_length(updated_by) <= 255)
);

INSERT INTO app.organization_feature_flags (org_id)
SELECT id FROM app.organizations
ON CONFLICT (org_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS app.organization_feature_flag_audit (
  -- Deliberately no FK to organizations: this immutable rollout proof must
  -- survive lifecycle cleanup and cannot participate in cascading deletes.
  org_id text NOT NULL,
  version bigint NOT NULL CHECK (version > 0),
  scheduling_enabled boolean NOT NULL,
  whatsapp_enabled boolean NOT NULL,
  google_calendar_enabled boolean NOT NULL,
  fiscal_real_enabled boolean NOT NULL,
  changed_by text NOT NULL
    CHECK (btrim(changed_by) <> '' AND char_length(changed_by) <= 255),
  changed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, version)
);

-- The old context-local setting is migrated once and then retired. Pymes owns
-- one rollout switch per capability, never two competing sources of truth.
UPDATE app.organization_feature_flags AS flags
SET whatsapp_enabled = true,
    updated_at = now(),
    updated_by = 'system:migrate-notifications'
FROM app.notification_settings AS settings
WHERE settings.org_id = flags.org_id
  AND settings.whatsapp_enabled
  AND NOT flags.whatsapp_enabled;

UPDATE app.notification_settings
SET whatsapp_enabled = false,
    updated_at = now()
WHERE whatsapp_enabled;

INSERT INTO app.organization_feature_flag_audit (
  org_id, version, scheduling_enabled, whatsapp_enabled,
  google_calendar_enabled, fiscal_real_enabled, changed_by, changed_at
)
SELECT
  org_id, version, scheduling_enabled, whatsapp_enabled,
  google_calendar_enabled, fiscal_real_enabled, updated_by, updated_at
FROM app.organization_feature_flags
ON CONFLICT (org_id, version) DO NOTHING;

ALTER TABLE app.organization_feature_flags ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.organization_feature_flags FORCE ROW LEVEL SECURITY;
ALTER TABLE app.organization_feature_flag_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.organization_feature_flag_audit FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS organization_feature_flags_org_isolation
  ON app.organization_feature_flags;
CREATE POLICY organization_feature_flags_org_isolation
  ON app.organization_feature_flags
  USING (org_id = NULLIF(current_setting('app.org_id', true), ''))
  WITH CHECK (org_id = NULLIF(current_setting('app.org_id', true), ''));

DROP POLICY IF EXISTS organization_feature_flag_audit_org_isolation
  ON app.organization_feature_flag_audit;
CREATE POLICY organization_feature_flag_audit_org_isolation
  ON app.organization_feature_flag_audit
  USING (org_id = NULLIF(current_setting('app.org_id', true), ''))
  WITH CHECK (org_id = NULLIF(current_setting('app.org_id', true), ''));

CREATE OR REPLACE FUNCTION app.reject_organization_feature_flag_audit_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'organization feature flag audit is immutable'
    USING ERRCODE = '55000';
END
$$;

DROP TRIGGER IF EXISTS organization_feature_flag_audit_no_update_delete
  ON app.organization_feature_flag_audit;
CREATE TRIGGER organization_feature_flag_audit_no_update_delete
  BEFORE UPDATE OR DELETE ON app.organization_feature_flag_audit
  FOR EACH ROW
  EXECUTE FUNCTION app.reject_organization_feature_flag_audit_mutation();

DROP TRIGGER IF EXISTS organization_feature_flag_audit_no_truncate
  ON app.organization_feature_flag_audit;
CREATE TRIGGER organization_feature_flag_audit_no_truncate
  BEFORE TRUNCATE ON app.organization_feature_flag_audit
  FOR EACH STATEMENT
  EXECUTE FUNCTION app.reject_organization_feature_flag_audit_mutation();

COMMIT;
