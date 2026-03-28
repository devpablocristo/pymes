-- Revierte el namespace privado de Pymes para la bandeja in-app.
ALTER TABLE IF EXISTS pymes_in_app_notifications
    RENAME TO in_app_notifications;

ALTER INDEX IF EXISTS idx_pymes_in_app_notifications_user_created
    RENAME TO idx_in_app_notifications_user_created;

ALTER INDEX IF EXISTS idx_pymes_in_app_notifications_org_created
    RENAME TO idx_in_app_notifications_org_created;
