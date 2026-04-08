-- 0004: Eliminar beauty.salon_services. Ahora todos los servicios viven en public.services.
-- En la base actual la tabla está vacía, así que no hay datos a migrar.
-- Si en el futuro hubiera filas, deberían copiarse a public.services con
-- metadata = '{"vertical":"beauty","segment":"salon", ...}' antes del drop.

ALTER TABLE beauty.salon_services DROP CONSTRAINT IF EXISTS beauty_salon_services_linked_service_fk;
DROP TABLE IF EXISTS beauty.salon_services CASCADE;
