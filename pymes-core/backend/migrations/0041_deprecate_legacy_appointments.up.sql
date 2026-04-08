-- Migración: deprecar sistema legacy de appointments en favor del módulo scheduling.
-- 1. Copiar datos de appointments → scheduling_bookings (si no existen ya)
-- 2. Asegurar columna tenant_settings.scheduling_enabled y reflejar appointments_enabled
-- 3. Mantener tabla appointments como referencia histórica (no se elimina aún)

-- Debe existir antes del UPDATE (bases nuevas no tenían esta columna hasta esta migración).
ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS scheduling_enabled boolean NOT NULL DEFAULT false;

-- Heredar el flag legacy sin pisar un true ya persistido en scheduling_enabled.
UPDATE tenant_settings
SET scheduling_enabled = scheduling_enabled OR COALESCE(appointments_enabled, false);

-- Migrar appointments legacy solo si el esquema scheduling ya existe (tablas del módulo scheduling).
DO $body$
BEGIN
  IF to_regclass('public.scheduling_bookings') IS NULL THEN
    RETURN;
  END IF;

  INSERT INTO scheduling_bookings (
      id, org_id, branch_id, service_id, resource_id, party_id,
      reference, customer_name, customer_phone, customer_email,
      status, source, idempotency_key,
      start_at, end_at, occupies_from, occupies_until,
      notes, metadata, created_by,
      confirmed_at, cancelled_at,
      created_at, updated_at
  )
  SELECT
      a.id,
      a.org_id,
      COALESCE(
          (SELECT id FROM scheduling_branches WHERE org_id = a.org_id AND active = true ORDER BY created_at LIMIT 1),
          a.org_id
      ),
      COALESCE(
          (SELECT id FROM scheduling_services WHERE org_id = a.org_id AND active = true ORDER BY created_at LIMIT 1),
          a.org_id
      ),
      COALESCE(
          (SELECT r.id FROM scheduling_resources r
           JOIN scheduling_branches b ON b.id = r.branch_id AND b.org_id = a.org_id
           WHERE r.active = true ORDER BY r.created_at LIMIT 1),
          a.org_id
      ),
      a.party_id,
      COALESCE(a.title, 'Turno migrado'),
      COALESCE(a.party_name, ''),
      COALESCE(a.party_phone, ''),
      '',
      CASE a.status
          WHEN 'scheduled' THEN 'pending_confirmation'
          WHEN 'confirmed' THEN 'confirmed'
          WHEN 'in_progress' THEN 'in_service'
          WHEN 'completed' THEN 'completed'
          WHEN 'cancelled' THEN 'cancelled'
          WHEN 'no_show' THEN 'no_show'
          ELSE 'pending_confirmation'
      END,
      'admin',
      'legacy-appointment-' || a.id::text,
      a.start_at,
      COALESCE(a.end_at, a.start_at + make_interval(mins => COALESCE(a.duration, 30))),
      a.start_at,
      COALESCE(a.end_at, a.start_at + make_interval(mins => COALESCE(a.duration, 30))),
      COALESCE(a.notes, ''),
      COALESCE(a.metadata, '{}'::jsonb),
      COALESCE(a.created_by, 'migration'),
      CASE WHEN a.status = 'confirmed' THEN a.updated_at END,
      CASE WHEN a.status = 'cancelled' THEN a.updated_at END,
      a.created_at,
      a.updated_at
  FROM appointments a
  WHERE a.archived_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM scheduling_bookings sb
      WHERE sb.idempotency_key = 'legacy-appointment-' || a.id::text
      AND sb.org_id = a.org_id
  );

  UPDATE appointments
  SET metadata = jsonb_set(
      COALESCE(metadata, '{}'::jsonb),
      '{migrated_to_scheduling}',
      'true'::jsonb
  )
  WHERE archived_at IS NULL
  AND metadata IS DISTINCT FROM jsonb_set(COALESCE(metadata, '{}'::jsonb), '{migrated_to_scheduling}', 'true'::jsonb);
END;
$body$;
