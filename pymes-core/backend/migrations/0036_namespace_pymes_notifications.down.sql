-- Revierte el namespace privado solo si los nombres canónicos siguen libres.
DO $$
BEGIN
    IF to_regclass('public.notification_preferences') IS NULL
       AND to_regclass('public.pymes_notification_preferences') IS NOT NULL THEN
        EXECUTE 'ALTER TABLE pymes_notification_preferences RENAME TO notification_preferences';
    END IF;

    IF to_regclass('public.notification_log') IS NULL
       AND to_regclass('public.pymes_notification_log') IS NOT NULL THEN
        EXECUTE 'ALTER TABLE pymes_notification_log RENAME TO notification_log';
    END IF;

    IF to_regclass('public.idx_notification_log_org_created') IS NULL
       AND to_regclass('public.idx_pymes_notification_log_org_created') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_pymes_notification_log_org_created RENAME TO idx_notification_log_org_created';
    END IF;

    IF to_regclass('public.idx_notification_log_user_created') IS NULL
       AND to_regclass('public.idx_pymes_notification_log_user_created') IS NOT NULL THEN
        EXECUTE 'ALTER INDEX idx_pymes_notification_log_user_created RENAME TO idx_notification_log_user_created';
    END IF;
END $$;
