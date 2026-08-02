ALTER TABLE fiscal.requests
  ADD COLUMN IF NOT EXISTS execution_state text,
  ADD COLUMN IF NOT EXISTS execution_attempt bigint,
  ADD COLUMN IF NOT EXISTS lease_token text,
  ADD COLUMN IF NOT EXISTS lease_expires_at timestamptz,
  ADD COLUMN IF NOT EXISTS dispatch_may_have_occurred boolean;

UPDATE fiscal.requests
   SET execution_state = CASE
         WHEN result->>'status' IN ('authorized','rejected') THEN 'terminal'
         ELSE 'uncertain'
       END
 WHERE execution_state IS NULL;

UPDATE fiscal.requests
   SET execution_attempt = 0
 WHERE execution_attempt IS NULL;

UPDATE fiscal.requests
   SET dispatch_may_have_occurred = result->>'status' = 'uncertain'
 WHERE dispatch_may_have_occurred IS NULL;

ALTER TABLE fiscal.requests
  ALTER COLUMN execution_state SET DEFAULT 'terminal',
  ALTER COLUMN execution_state SET NOT NULL,
  ALTER COLUMN execution_attempt SET DEFAULT 0,
  ALTER COLUMN execution_attempt SET NOT NULL,
  ALTER COLUMN dispatch_may_have_occurred SET DEFAULT false,
  ALTER COLUMN dispatch_may_have_occurred SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conrelid = 'fiscal.requests'::regclass
       AND conname = 'fiscal_requests_execution_state'
  ) THEN
    ALTER TABLE fiscal.requests
      ADD CONSTRAINT fiscal_requests_execution_state CHECK (
        execution_state IN ('claimed','in_progress','uncertain','terminal')
      );
  END IF;

  IF NOT EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conrelid = 'fiscal.requests'::regclass
       AND conname = 'fiscal_requests_execution_attempt'
  ) THEN
    ALTER TABLE fiscal.requests
      ADD CONSTRAINT fiscal_requests_execution_attempt
      CHECK (execution_attempt >= 0);
  END IF;

  IF NOT EXISTS (
    SELECT 1
      FROM pg_constraint
     WHERE conrelid = 'fiscal.requests'::regclass
       AND conname = 'fiscal_requests_lease_shape'
  ) THEN
    ALTER TABLE fiscal.requests
      ADD CONSTRAINT fiscal_requests_lease_shape CHECK (
        (
          execution_state IN ('claimed','in_progress')
          AND lease_token IS NOT NULL
          AND lease_expires_at IS NOT NULL
        )
        OR (
          execution_state IN ('uncertain','terminal')
          AND lease_token IS NULL
          AND lease_expires_at IS NULL
        )
      );
  END IF;
END
$$;

CREATE INDEX IF NOT EXISTS fiscal_requests_expired_lease_idx
  ON fiscal.requests (lease_expires_at)
  WHERE execution_state IN ('claimed','in_progress');
