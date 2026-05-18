-- 0005: Eliminar beauty.staff_members. Ahora el equipo vive en public.parties con role=employee.
-- En la base actual la tabla está vacía, así que no hay datos a migrar.
-- Si en el futuro hubiera filas, deberían copiarse a public.parties con
-- metadata = '{"vertical":"beauty","color":"#..."}' antes del drop.

DROP TABLE IF EXISTS beauty.staff_members CASCADE;
