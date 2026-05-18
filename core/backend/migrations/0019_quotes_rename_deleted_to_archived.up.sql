-- El repo de quotes usa `archived_at` (convención canónica post-cutover) pero
-- la migración 0007 creó la columna como `deleted_at`. Esto rompía
-- GET /v1/quotes con `column "archived_at" does not exist`. Renombramos para
-- alinear con la convención del proyecto (CLAUDE.md sec. 5.5).

BEGIN;

ALTER TABLE quotes RENAME COLUMN deleted_at TO archived_at;
ALTER INDEX idx_quotes_org_deleted_at RENAME TO idx_quotes_org_archived_at;

COMMIT;
