-- 0015: Drop schema huerfano workshops.bicycles.
-- Despues de unificar work_orders en 0014, ningun modulo Go referencia la tabla `bicycles`.
-- (Las OT polimorficas guardan target_id como referencia opaca; no hay FK ni queries reales sobre esta tabla.)

DROP TABLE IF EXISTS workshops.bicycles CASCADE;
