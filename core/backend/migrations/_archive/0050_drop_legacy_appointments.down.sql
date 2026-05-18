-- Revertir renombrado de columnas.
ALTER TABLE tenant_settings RENAME COLUMN scheduling_label TO appointment_label;
ALTER TABLE tenant_settings RENAME COLUMN scheduling_reminder_hours TO appointment_reminder_hours;

-- Restaurar permisos RBAC.
UPDATE role_permissions SET resource = 'appointments' WHERE resource = 'scheduling';

-- No se recrean las tablas appointments / appointment_slots (datos ya migrados).
