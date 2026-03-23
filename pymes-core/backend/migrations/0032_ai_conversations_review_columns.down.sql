DROP INDEX IF EXISTS idx_ai_conversations_contact_phone;
DROP INDEX IF EXISTS idx_ai_conversations_review_request;
ALTER TABLE ai_conversations DROP COLUMN IF EXISTS review_status;
ALTER TABLE ai_conversations DROP COLUMN IF EXISTS review_request_id;
ALTER TABLE ai_conversations DROP COLUMN IF EXISTS pending_action;
ALTER TABLE ai_conversations DROP COLUMN IF EXISTS party_id;
ALTER TABLE ai_conversations DROP COLUMN IF EXISTS contact_name;
ALTER TABLE ai_conversations DROP COLUMN IF EXISTS contact_phone;
ALTER TABLE ai_conversations DROP COLUMN IF EXISTS channel;
