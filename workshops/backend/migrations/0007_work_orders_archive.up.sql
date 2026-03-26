-- Archivo lógico de órdenes de trabajo (paridad CRUD consola con clientes / vehículos / servicios).
DROP INDEX IF EXISTS workshops.workshops_work_orders_org_number_idx;

ALTER TABLE workshops.work_orders
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NULL;

CREATE UNIQUE INDEX IF NOT EXISTS workshops_work_orders_org_number_active_idx
    ON workshops.work_orders (org_id, number)
    WHERE archived_at IS NULL;
