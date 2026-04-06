ALTER TABLE professionals.sessions DROP CONSTRAINT IF EXISTS sessions_org_id_booking_id_key;
ALTER TABLE professionals.sessions ADD CONSTRAINT sessions_org_id_appointment_id_key UNIQUE (org_id, appointment_id);
ALTER TABLE professionals.sessions RENAME COLUMN booking_id TO appointment_id;
ALTER TABLE professionals.intakes RENAME COLUMN booking_id TO appointment_id;
