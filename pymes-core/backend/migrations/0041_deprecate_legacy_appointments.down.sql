-- Reverso de 0041_deprecate_legacy_appointments.up.sql
--
-- WARNING: este down es PARCIAL y best-effort.
--
-- 1. Borra las scheduling_bookings creadas por la migración (identificadas por
--    el idempotency_key 'legacy-appointment-...'). Estas son copias, los
--    appointments originales siguen en su tabla intactos.
-- 2. Quita el flag `metadata.migrated_to_scheduling` de los appointments.
-- 3. NO revierte el flag `tenant_settings.scheduling_enabled` porque el up
--    solo lo seteó en true si appointments_enabled era true; revertirlo
--    podría desactivar scheduling en tenants que ya lo usan legítimamente
--    aparte del migrate.
-- 4. NO dropea la columna `scheduling_enabled` porque otras migrations y
--    código vivo la consumen. Si querés droparla del todo hay que coordinar
--    con el código que la lee.

DELETE FROM scheduling_bookings
WHERE idempotency_key LIKE 'legacy-appointment-%';

DO $body$
BEGIN
  IF to_regclass('public.appointments') IS NOT NULL THEN
    UPDATE appointments
    SET metadata = metadata - 'migrated_to_scheduling'
    WHERE metadata ? 'migrated_to_scheduling';
  END IF;
END;
$body$;
