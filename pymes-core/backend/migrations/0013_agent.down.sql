-- 0013_agent.down.sql

DROP TRIGGER IF EXISTS trg_agent_idempotency_updated_at ON agent_idempotency_records;

DROP TABLE IF EXISTS agent_idempotency_records;
DROP TABLE IF EXISTS agent_confirmations;
