-- 0050: Eliminar sistema legacy de appointments.
-- La migración 0041 ya copió los datos a scheduling_bookings.

-- 1. Renombrar columnas de tenant_settings.
ALTER TABLE tenant_settings RENAME COLUMN appointment_label TO scheduling_label;
ALTER TABLE tenant_settings RENAME COLUMN appointment_reminder_hours TO scheduling_reminder_hours;

-- 2. Eliminar columna legacy appointments_enabled (scheduling_enabled ya existe).
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS appointments_enabled;

-- 3. Eliminar tablas legacy.
DROP TABLE IF EXISTS appointment_slots;
DROP TABLE IF EXISTS appointments;

-- 4. Actualizar permisos RBAC: 'appointments' → 'scheduling'.
UPDATE role_permissions SET resource = 'scheduling' WHERE resource = 'appointments';
