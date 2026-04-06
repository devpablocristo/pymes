-- Archivo lógico para bicicletas y órdenes de bicicletería (paridad CRUD consola).

-- Bicicletas
DROP INDEX IF EXISTS workshops.workshops_bicycles_org_frame_idx;

ALTER TABLE workshops.bicycles
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NULL;

CREATE UNIQUE INDEX IF NOT EXISTS workshops_bicycles_org_frame_active_idx
    ON workshops.bicycles (org_id, frame_number)
    WHERE archived_at IS NULL;

-- Órdenes de trabajo bike shop
DROP INDEX IF EXISTS workshops.workshops_bike_work_orders_org_number_idx;

ALTER TABLE workshops.bike_work_orders
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ NULL;

CREATE UNIQUE INDEX IF NOT EXISTS workshops_bike_work_orders_org_number_active_idx
    ON workshops.bike_work_orders (org_id, number)
    WHERE archived_at IS NULL;
