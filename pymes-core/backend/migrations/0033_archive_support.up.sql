-- Agrega archived_at a appointments y quotes para soportar soft delete canónico.
-- products y suppliers usan la tabla parties que ya tiene deleted_at.

ALTER TABLE appointments ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_appointments_archived ON appointments (org_id, archived_at) WHERE archived_at IS NOT NULL;

ALTER TABLE quotes ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_quotes_archived ON quotes (org_id, archived_at) WHERE archived_at IS NOT NULL;
