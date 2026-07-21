DO $$
BEGIN
    IF to_regclass('public.org_settings') IS NULL
       AND to_regclass('public.tenant_settings') IS NOT NULL THEN
        ALTER TABLE tenant_settings RENAME TO org_settings;
    END IF;

    IF to_regclass('public.org_invitations') IS NULL
       AND to_regclass('public.tenant_invitations') IS NOT NULL THEN
        ALTER TABLE tenant_invitations RENAME TO org_invitations;
    END IF;
END $$;

ALTER INDEX IF EXISTS idx_tenant_settings_stripe_customer RENAME TO idx_org_settings_stripe_customer;
ALTER INDEX IF EXISTS idx_tenant_settings_past_due_since RENAME TO idx_org_settings_past_due_since;
ALTER INDEX IF EXISTS idx_tenant_invitations_pending_email RENAME TO idx_org_invitations_pending_email;
ALTER INDEX IF EXISTS idx_tenant_invitations_org_status RENAME TO idx_org_invitations_org_status;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgname = 'trg_tenant_settings_updated_at'
          AND tgrelid = 'public.org_settings'::regclass
    ) THEN
        ALTER TRIGGER trg_tenant_settings_updated_at ON org_settings RENAME TO trg_org_settings_updated_at;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgname = 'trg_tenant_invitations_updated_at'
          AND tgrelid = 'public.org_invitations'::regclass
    ) THEN
        ALTER TRIGGER trg_tenant_invitations_updated_at ON org_invitations RENAME TO trg_org_invitations_updated_at;
    END IF;
END $$;
