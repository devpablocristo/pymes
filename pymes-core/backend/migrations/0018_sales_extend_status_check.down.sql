-- Reversa: vuelve a la restricción original de 0007 (solo completed/voided).
-- ATENCIÓN: si hay filas con status fuera de ese set, el ALTER fallará. Hay
-- que normalizarlas antes manualmente.

BEGIN;

ALTER TABLE sales DROP CONSTRAINT IF EXISTS sales_status_check;

ALTER TABLE sales
    ADD CONSTRAINT sales_status_check
    CHECK (status IN ('completed','voided'));

COMMIT;
