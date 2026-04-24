DROP INDEX IF EXISTS idx_audit_log_hash_version;
ALTER TABLE audit_log
    DROP COLUMN IF EXISTS payload_hash,
    DROP COLUMN IF EXISTS hash_version;

DROP INDEX IF EXISTS idx_ai_agent_events_review;
DROP INDEX IF EXISTS idx_ai_agent_events_request;
DROP INDEX IF EXISTS idx_ai_agent_events_capability;
ALTER TABLE IF EXISTS ai_agent_events
    DROP COLUMN IF EXISTS payload_hash,
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS review_request_id,
    DROP COLUMN IF EXISTS confirmation_id,
    DROP COLUMN IF EXISTS capability_id,
    DROP COLUMN IF EXISTS request_id;

DROP TABLE IF EXISTS agent_idempotency_records;
DROP TABLE IF EXISTS agent_confirmations;

