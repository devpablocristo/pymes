-- Renombrar appointment_id → booking_id en todas las tablas.
ALTER TABLE professionals.intakes RENAME COLUMN appointment_id TO booking_id;
ALTER TABLE professionals.sessions RENAME COLUMN appointment_id TO booking_id;

-- Actualizar constraint unique.
ALTER TABLE professionals.sessions DROP CONSTRAINT IF EXISTS sessions_org_id_appointment_id_key;
ALTER TABLE professionals.sessions ADD CONSTRAINT sessions_org_id_booking_id_key UNIQUE (org_id, booking_id);
