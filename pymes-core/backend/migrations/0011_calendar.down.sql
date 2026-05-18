-- 0011_calendar.down.sql

DROP TRIGGER IF EXISTS trg_calendar_sync_connections_updated_at ON calendar_sync_connections;

DROP TABLE IF EXISTS calendar_sync_errors;
DROP TABLE IF EXISTS calendar_sync_oauth_states;
DROP TABLE IF EXISTS calendar_sync_connections;
DROP TABLE IF EXISTS calendar_export_tokens;
