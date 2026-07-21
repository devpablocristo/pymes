-- Renombra constraints heredadas para que reflejen el namespace privado de Pymes.
ALTER TABLE IF EXISTS pymes_notification_preferences
    RENAME CONSTRAINT notification_preferences_pkey TO pymes_notification_preferences_pkey;

ALTER TABLE IF EXISTS pymes_notification_preferences
    RENAME CONSTRAINT notification_preferences_user_id_fkey TO pymes_notification_preferences_user_id_fkey;

ALTER TABLE IF EXISTS pymes_notification_preferences
    RENAME CONSTRAINT notification_preferences_user_id_notification_type_channel_key
    TO pymes_notification_preferences_user_id_notification_type_channel_key;

ALTER TABLE IF EXISTS pymes_notification_log
    RENAME CONSTRAINT notification_log_pkey TO pymes_notification_log_pkey;

ALTER TABLE IF EXISTS pymes_notification_log
    RENAME CONSTRAINT notification_log_org_id_fkey TO pymes_notification_log_org_id_fkey;

ALTER TABLE IF EXISTS pymes_notification_log
    RENAME CONSTRAINT notification_log_user_id_fkey TO pymes_notification_log_user_id_fkey;

ALTER TABLE IF EXISTS pymes_notification_log
    RENAME CONSTRAINT notification_log_dedup_key_key TO pymes_notification_log_dedup_key_key;
