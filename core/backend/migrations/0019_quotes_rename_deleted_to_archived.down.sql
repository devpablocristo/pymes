BEGIN;

ALTER TABLE quotes RENAME COLUMN archived_at TO deleted_at;
ALTER INDEX idx_quotes_org_archived_at RENAME TO idx_quotes_org_deleted_at;

COMMIT;
