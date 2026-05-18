-- 0050: Eliminar sistema anterior de appointments.
-- La migración 0041 ya copió los datos a scheduling_bookings.

-- 1. Renombrar columnas de tenant_settings.
ALTER TABLE tenant_settings RENAME COLUMN appointment_label TO scheduling_label;
ALTER TABLE tenant_settings RENAME COLUMN appointment_reminder_hours TO scheduling_reminder_hours;

-- 2. Eliminar columna anterior appointments_enabled (scheduling_enabled ya existe).
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS appointments_enabled;

-- 3. Eliminar tablas anteriores.
DROP TABLE IF EXISTS appointment_slots;
DROP TABLE IF EXISTS appointments;

-- 4. Eliminar permisos RBAC anteriores de 'appointments' (ya existen como 'scheduling').
DELETE FROM role_permissions WHERE resource = 'appointments';
