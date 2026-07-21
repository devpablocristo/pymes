-- Separa la bandeja propia de Pymes del esquema SaaS compartido para evitar colisiones de nombre.
ALTER TABLE IF EXISTS in_app_notifications
    RENAME TO pymes_in_app_notifications;

ALTER INDEX IF EXISTS idx_in_app_notifications_user_created
    RENAME TO idx_pymes_in_app_notifications_user_created;

ALTER INDEX IF EXISTS idx_in_app_notifications_org_created
    RENAME TO idx_pymes_in_app_notifications_org_created;
