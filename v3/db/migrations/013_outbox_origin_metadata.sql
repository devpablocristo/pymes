BEGIN;

ALTER TABLE app.sales
  ADD COLUMN IF NOT EXISTS request_id text,
  ADD COLUMN IF NOT EXISTS actor_ref text,
  ADD COLUMN IF NOT EXISTS source_version integer;

ALTER TABLE app.purchases
  ADD COLUMN IF NOT EXISTS request_id text,
  ADD COLUMN IF NOT EXISTS actor_ref text,
  ADD COLUMN IF NOT EXISTS source_version integer;

ALTER TABLE app.payments
  ADD COLUMN IF NOT EXISTS snapshot_digest char(64),
  ADD COLUMN IF NOT EXISTS request_id text,
  ADD COLUMN IF NOT EXISTS actor_ref text,
  ADD COLUMN IF NOT EXISTS source_version integer;

ALTER TABLE app.accounting_reversals
  ADD COLUMN IF NOT EXISTS request_id text,
  ADD COLUMN IF NOT EXISTS actor_ref text,
  ADD COLUMN IF NOT EXISTS source_version integer;

ALTER TABLE app.accounting_application_commands
  ADD COLUMN IF NOT EXISTS request_id text,
  ADD COLUMN IF NOT EXISTS actor_ref text,
  ADD COLUMN IF NOT EXISTS source_version integer;

UPDATE app.sales
SET request_id = COALESCE(request_id, 'migration:sale:' || id),
    actor_ref = COALESCE(actor_ref, 'system:migration'),
    source_version = COALESCE(source_version, 1)
WHERE request_id IS NULL OR actor_ref IS NULL OR source_version IS NULL;

UPDATE app.purchases
SET request_id = COALESCE(request_id, 'migration:purchase:' || id),
    actor_ref = COALESCE(actor_ref, 'system:migration'),
    source_version = COALESCE(source_version, 1)
WHERE request_id IS NULL OR actor_ref IS NULL OR source_version IS NULL;

UPDATE app.payments
SET snapshot_digest = COALESCE(
      snapshot_digest,
      md5(id || ':' || amount::text || ':' || currency || ':' || direction)
      ||
      md5('pymes-v3:' || id || ':' || amount::text || ':' || currency || ':' || direction)
    ),
    request_id = COALESCE(request_id, 'migration:payment:' || id),
    actor_ref = COALESCE(actor_ref, 'system:migration'),
    source_version = COALESCE(source_version, 1)
WHERE snapshot_digest IS NULL
   OR request_id IS NULL
   OR actor_ref IS NULL
   OR source_version IS NULL;

UPDATE app.accounting_reversals
SET request_id = COALESCE(request_id, 'migration:reversal:' || id),
    actor_ref = COALESCE(actor_ref, 'system:migration'),
    source_version = COALESCE(source_version, 1)
WHERE request_id IS NULL OR actor_ref IS NULL OR source_version IS NULL;

UPDATE app.accounting_application_commands
SET request_id = COALESCE(request_id, 'migration:application:' || id),
    actor_ref = COALESCE(actor_ref, 'system:migration'),
    source_version = COALESCE(source_version, 1)
WHERE request_id IS NULL OR actor_ref IS NULL OR source_version IS NULL;

ALTER TABLE app.sales
  ALTER COLUMN request_id SET NOT NULL,
  ALTER COLUMN actor_ref SET NOT NULL,
  ALTER COLUMN source_version SET NOT NULL;
ALTER TABLE app.purchases
  ALTER COLUMN request_id SET NOT NULL,
  ALTER COLUMN actor_ref SET NOT NULL,
  ALTER COLUMN source_version SET NOT NULL;
ALTER TABLE app.payments
  ALTER COLUMN snapshot_digest SET NOT NULL,
  ALTER COLUMN request_id SET NOT NULL,
  ALTER COLUMN actor_ref SET NOT NULL,
  ALTER COLUMN source_version SET NOT NULL;
ALTER TABLE app.accounting_reversals
  ALTER COLUMN request_id SET NOT NULL,
  ALTER COLUMN actor_ref SET NOT NULL,
  ALTER COLUMN source_version SET NOT NULL;
ALTER TABLE app.accounting_application_commands
  ALTER COLUMN request_id SET NOT NULL,
  ALTER COLUMN actor_ref SET NOT NULL,
  ALTER COLUMN source_version SET NOT NULL;

ALTER TABLE app.outbox
  ADD COLUMN IF NOT EXISTS request_id text,
  ADD COLUMN IF NOT EXISTS actor_ref text,
  ADD COLUMN IF NOT EXISTS source_version integer,
  ADD COLUMN IF NOT EXISTS snapshot_digest char(64);

ALTER TABLE app.outbox_dead_letters
  ADD COLUMN IF NOT EXISTS request_id text,
  ADD COLUMN IF NOT EXISTS actor_ref text,
  ADD COLUMN IF NOT EXISTS source_version integer,
  ADD COLUMN IF NOT EXISTS snapshot_digest char(64);

UPDATE app.outbox
SET request_id = COALESCE(request_id, 'migration:outbox:' || id::text),
    actor_ref = COALESCE(actor_ref, 'system:migration'),
    source_version = COALESCE(source_version, 1),
    snapshot_digest = COALESCE(snapshot_digest, payload_hash)
WHERE request_id IS NULL
   OR actor_ref IS NULL
   OR source_version IS NULL
   OR snapshot_digest IS NULL;

UPDATE app.outbox_dead_letters
SET request_id = COALESCE(request_id, 'migration:dead-letter:' || id::text),
    actor_ref = COALESCE(actor_ref, 'system:migration'),
    source_version = COALESCE(source_version, 1),
    snapshot_digest = COALESCE(snapshot_digest, payload_hash)
WHERE request_id IS NULL
   OR actor_ref IS NULL
   OR source_version IS NULL
   OR snapshot_digest IS NULL;

ALTER TABLE app.outbox
  ALTER COLUMN request_id SET NOT NULL,
  ALTER COLUMN actor_ref SET NOT NULL,
  ALTER COLUMN source_version SET NOT NULL,
  ALTER COLUMN snapshot_digest SET NOT NULL;

ALTER TABLE app.outbox_dead_letters
  ALTER COLUMN request_id SET NOT NULL,
  ALTER COLUMN actor_ref SET NOT NULL,
  ALTER COLUMN source_version SET NOT NULL,
  ALTER COLUMN snapshot_digest SET NOT NULL;

ALTER TABLE app.sales
  DROP CONSTRAINT IF EXISTS sales_source_version_positive;
ALTER TABLE app.sales
  ADD CONSTRAINT sales_source_version_positive CHECK (source_version > 0);
ALTER TABLE app.purchases
  DROP CONSTRAINT IF EXISTS purchases_source_version_positive;
ALTER TABLE app.purchases
  ADD CONSTRAINT purchases_source_version_positive CHECK (source_version > 0);
ALTER TABLE app.payments
  DROP CONSTRAINT IF EXISTS payments_source_metadata_valid;
ALTER TABLE app.payments
  ADD CONSTRAINT payments_source_metadata_valid CHECK (
    source_version > 0 AND snapshot_digest ~ '^[0-9a-f]{64}$'
  );
ALTER TABLE app.accounting_reversals
  DROP CONSTRAINT IF EXISTS accounting_reversals_source_version_positive;
ALTER TABLE app.accounting_reversals
  ADD CONSTRAINT accounting_reversals_source_version_positive CHECK (source_version > 0);
ALTER TABLE app.accounting_application_commands
  DROP CONSTRAINT IF EXISTS accounting_application_commands_source_version_positive;
ALTER TABLE app.accounting_application_commands
  ADD CONSTRAINT accounting_application_commands_source_version_positive CHECK (source_version > 0);
ALTER TABLE app.outbox
  DROP CONSTRAINT IF EXISTS outbox_source_metadata_valid;
ALTER TABLE app.outbox
  ADD CONSTRAINT outbox_source_metadata_valid CHECK (
    source_version > 0 AND snapshot_digest ~ '^[0-9a-f]{64}$'
  );
ALTER TABLE app.outbox_dead_letters
  DROP CONSTRAINT IF EXISTS outbox_dead_letters_source_metadata_valid;
ALTER TABLE app.outbox_dead_letters
  ADD CONSTRAINT outbox_dead_letters_source_metadata_valid CHECK (
    source_version > 0 AND snapshot_digest ~ '^[0-9a-f]{64}$'
  );

CREATE INDEX IF NOT EXISTS outbox_request_id_idx
  ON app.outbox (org_id, request_id);
CREATE INDEX IF NOT EXISTS outbox_dead_letters_request_id_idx
  ON app.outbox_dead_letters (org_id, request_id);

COMMIT;
