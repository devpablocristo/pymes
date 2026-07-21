-- Reverso de 0033_archive_support.up.sql
-- WARNING: borra archived_at de appointments y quotes (incluye los valores).
-- Si la migration 0041 ya droppeó appointments, los DROP de appointments
-- son no-op (IF EXISTS lo cubre).

DROP INDEX IF EXISTS idx_quotes_archived;
ALTER TABLE quotes DROP COLUMN IF EXISTS archived_at;

DROP INDEX IF EXISTS idx_appointments_archived;
ALTER TABLE IF EXISTS appointments DROP COLUMN IF EXISTS archived_at;
