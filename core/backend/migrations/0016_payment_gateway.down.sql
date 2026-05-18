-- 0016_payment_gateway.down.sql
DROP TRIGGER IF EXISTS trg_payment_gateway_connections_updated_at ON payment_gateway_connections;
DROP TABLE IF EXISTS payment_gateway_events;
DROP TABLE IF EXISTS payment_gateway_webhooks;
DROP TABLE IF EXISTS payment_preferences;
DROP TABLE IF EXISTS payment_gateway_connections;
