-- La migración 0034 reemplazó cualquier esquema previo y no es reversible con precisión.
-- Para rollback operativo, removemos la tabla creada por 0034 luego de revertir 0035.
DROP INDEX IF EXISTS idx_in_app_notifications_user_created;
DROP INDEX IF EXISTS idx_in_app_notifications_org_created;
DROP TABLE IF EXISTS in_app_notifications;
