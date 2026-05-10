-- Permite los 5 estados que maneja el kanban del frontend (sales/board):
-- draft, completed, paid, pending, voided. La constraint original 0007 solo
-- aceptaba completed/voided y bloqueaba el drag-and-drop entre columnas.

BEGIN;

ALTER TABLE sales DROP CONSTRAINT IF EXISTS sales_status_check;

ALTER TABLE sales
    ADD CONSTRAINT sales_status_check
    CHECK (status IN ('draft','completed','paid','pending','voided'));

COMMIT;
