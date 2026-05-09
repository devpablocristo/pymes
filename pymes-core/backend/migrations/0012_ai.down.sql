-- 0012_ai.down.sql

DROP TRIGGER IF EXISTS trg_ai_conversations_updated_at ON ai_conversations;
DROP TRIGGER IF EXISTS trg_ai_dossiers_updated_at ON ai_dossiers;

DROP TABLE IF EXISTS ai_agent_events;
DROP TABLE IF EXISTS ai_usage_daily;
DROP TABLE IF EXISTS ai_conversations;
DROP TABLE IF EXISTS ai_dossiers;
