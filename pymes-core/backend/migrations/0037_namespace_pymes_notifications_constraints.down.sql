-- Revierte el rename de constraints privadas de Pymes si los nombres legacy siguen libres.
DO $$
BEGIN
    IF to_regclass('public.pymes_notification_preferences') IS NOT NULL THEN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'notification_preferences_pkey'
        ) THEN
            EXECUTE 'ALTER TABLE pymes_notification_preferences RENAME CONSTRAINT pymes_notification_preferences_pkey TO notification_preferences_pkey';
        END IF;

        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'notification_preferences_user_id_fkey'
        ) THEN
            EXECUTE 'ALTER TABLE pymes_notification_preferences RENAME CONSTRAINT pymes_notification_preferences_user_id_fkey TO notification_preferences_user_id_fkey';
        END IF;

        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'notification_preferences_user_id_notification_type_channel_key'
        ) THEN
            EXECUTE 'ALTER TABLE pymes_notification_preferences RENAME CONSTRAINT pymes_notification_preferences_user_id_notification_type_channel_key TO notification_preferences_user_id_notification_type_channel_key';
        END IF;
    END IF;

    IF to_regclass('public.pymes_notification_log') IS NOT NULL THEN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'notification_log_pkey'
        ) THEN
            EXECUTE 'ALTER TABLE pymes_notification_log RENAME CONSTRAINT pymes_notification_log_pkey TO notification_log_pkey';
        END IF;

        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'notification_log_org_id_fkey'
        ) THEN
            EXECUTE 'ALTER TABLE pymes_notification_log RENAME CONSTRAINT pymes_notification_log_org_id_fkey TO notification_log_org_id_fkey';
        END IF;

        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'notification_log_user_id_fkey'
        ) THEN
            EXECUTE 'ALTER TABLE pymes_notification_log RENAME CONSTRAINT pymes_notification_log_user_id_fkey TO notification_log_user_id_fkey';
        END IF;

        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint
            WHERE conname = 'notification_log_dedup_key_key'
        ) THEN
            EXECUTE 'ALTER TABLE pymes_notification_log RENAME CONSTRAINT pymes_notification_log_dedup_key_key TO notification_log_dedup_key_key';
        END IF;
    END IF;
END $$;
