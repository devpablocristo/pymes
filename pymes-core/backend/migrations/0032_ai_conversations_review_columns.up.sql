-- Columnas para atención al cliente gobernada (WhatsApp + Nexus Review)
ALTER TABLE ai_conversations ADD COLUMN IF NOT EXISTS channel VARCHAR(32);
ALTER TABLE ai_conversations ADD COLUMN IF NOT EXISTS contact_phone VARCHAR(32);
ALTER TABLE ai_conversations ADD COLUMN IF NOT EXISTS contact_name VARCHAR(255);
ALTER TABLE ai_conversations ADD COLUMN IF NOT EXISTS party_id UUID;
ALTER TABLE ai_conversations ADD COLUMN IF NOT EXISTS pending_action JSONB;
ALTER TABLE ai_conversations ADD COLUMN IF NOT EXISTS review_request_id UUID;
ALTER TABLE ai_conversations ADD COLUMN IF NOT EXISTS review_status VARCHAR(32);

CREATE INDEX IF NOT EXISTS idx_ai_conversations_review_request
    ON ai_conversations (review_request_id)
    WHERE review_request_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ai_conversations_contact_phone
    ON ai_conversations (org_id, contact_phone)
    WHERE contact_phone IS NOT NULL;
