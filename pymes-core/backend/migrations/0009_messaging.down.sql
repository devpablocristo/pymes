-- 0009_messaging.down.sql

DROP TRIGGER IF EXISTS trg_whatsapp_conversations_updated_at ON whatsapp_conversations;
DROP TRIGGER IF EXISTS trg_whatsapp_campaigns_updated_at ON whatsapp_campaigns;
DROP TRIGGER IF EXISTS trg_whatsapp_templates_updated_at ON whatsapp_templates;
DROP TRIGGER IF EXISTS trg_whatsapp_messages_updated_at ON whatsapp_messages;

DROP TABLE IF EXISTS whatsapp_conversations;
DROP TABLE IF EXISTS whatsapp_campaign_recipients;
DROP TABLE IF EXISTS whatsapp_campaigns;
DROP TABLE IF EXISTS whatsapp_opt_ins;
DROP TABLE IF EXISTS whatsapp_templates;
DROP TABLE IF EXISTS whatsapp_messages;
DROP TABLE IF EXISTS whatsapp_connections;
