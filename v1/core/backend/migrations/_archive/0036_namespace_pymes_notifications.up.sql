-- Separa las tablas de notificaciones propias de Pymes del esquema canónico de core/saas.
ALTER TABLE IF EXISTS notification_preferences
    RENAME TO pymes_notification_preferences;

ALTER TABLE IF EXISTS notification_log
    RENAME TO pymes_notification_log;

ALTER INDEX IF EXISTS idx_notification_log_org_created
    RENAME TO idx_pymes_notification_log_org_created;

ALTER INDEX IF EXISTS idx_notification_log_user_created
    RENAME TO idx_pymes_notification_log_user_created;
