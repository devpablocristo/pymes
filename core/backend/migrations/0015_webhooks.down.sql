-- 0015_webhooks.down.sql
DROP TRIGGER IF EXISTS trg_webhook_endpoints_updated_at ON webhook_endpoints;
DROP TABLE IF EXISTS webhook_outbox;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
