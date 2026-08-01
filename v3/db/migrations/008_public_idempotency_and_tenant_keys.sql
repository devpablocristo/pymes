BEGIN;

-- Bind the public HTTP idempotency key to the canonical command identity.
-- Existing rows predate the public header and receive a deterministic value so
-- the migration remains safe on databases that were used by early spikes.
ALTER TABLE app.idempotency_records
  ADD COLUMN IF NOT EXISTS idempotency_key text;

UPDATE app.idempotency_records
SET idempotency_key = operation || ':' || source_id || ':' || source_version::text
WHERE idempotency_key IS NULL;

ALTER TABLE app.idempotency_records
  ALTER COLUMN idempotency_key SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idempotency_records_public_key_unique
  ON app.idempotency_records (org_id, operation, idempotency_key);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'idempotency_records_source_version_positive'
      AND conrelid = 'app.idempotency_records'::regclass
  ) THEN
    ALTER TABLE app.idempotency_records
      ADD CONSTRAINT idempotency_records_source_version_positive
      CHECK (source_version > 0);
  END IF;
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'idempotency_records_public_key_not_blank'
      AND conrelid = 'app.idempotency_records'::regclass
  ) THEN
    ALTER TABLE app.idempotency_records
      ADD CONSTRAINT idempotency_records_public_key_not_blank
      CHECK (btrim(idempotency_key) <> '');
  END IF;
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'idempotency_records_public_key_shape'
      AND conrelid = 'app.idempotency_records'::regclass
  ) THEN
    ALTER TABLE app.idempotency_records
      ADD CONSTRAINT idempotency_records_public_key_shape
      CHECK (
        idempotency_key = btrim(idempotency_key)
        AND char_length(idempotency_key) <= 255
      );
  END IF;
END
$$;

-- Tenant-owned identifiers are only unique inside an organization.  RLS hid
-- cross-tenant rows, but the old global primary keys still prevented two
-- organizations from choosing the same source identifier.
ALTER TABLE app.open_item_applications
  DROP CONSTRAINT IF EXISTS open_item_applications_payment_id_fkey;

DO $$
DECLARE
  table_name text;
  constraint_name text;
BEGIN
  FOREACH table_name IN ARRAY ARRAY[
    'parties',
    'sales',
    'purchases',
    'payments',
    'open_item_applications',
    'accounting_application_commands',
    'accounting_reversals'
  ]
  LOOP
    SELECT conname
    INTO constraint_name
    FROM pg_constraint
    WHERE conrelid = format('app.%I', table_name)::regclass
      AND contype = 'p';

    IF constraint_name IS NOT NULL
       AND pg_get_constraintdef(
         (SELECT oid FROM pg_constraint
          WHERE conrelid = format('app.%I', table_name)::regclass
            AND conname = constraint_name)
       ) = 'PRIMARY KEY (id)' THEN
      EXECUTE format('ALTER TABLE app.%I DROP CONSTRAINT %I', table_name, constraint_name);
      EXECUTE format(
        'ALTER TABLE app.%I ADD CONSTRAINT %I PRIMARY KEY (org_id, id)',
        table_name,
        table_name || '_pkey'
      );
    END IF;
  END LOOP;
END
$$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'open_item_applications_payment_tenant_fkey'
      AND conrelid = 'app.open_item_applications'::regclass
  ) THEN
    ALTER TABLE app.open_item_applications
      ADD CONSTRAINT open_item_applications_payment_tenant_fkey
      FOREIGN KEY (org_id, payment_id)
      REFERENCES app.payments (org_id, id);
  END IF;
END
$$;

COMMIT;
