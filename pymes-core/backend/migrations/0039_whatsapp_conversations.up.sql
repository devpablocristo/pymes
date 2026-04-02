-- Conversaciones WhatsApp (thread por contacto + asignación de operador)
CREATE TABLE IF NOT EXISTS whatsapp_conversations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    party_id        UUID NOT NULL,
    phone           TEXT NOT NULL DEFAULT '',
    party_name      TEXT NOT NULL DEFAULT '',
    assigned_to     TEXT NOT NULL DEFAULT '',
    status          VARCHAR(32) NOT NULL DEFAULT 'open',
    last_message_at TIMESTAMPTZ,
    last_message_preview TEXT NOT NULL DEFAULT '',
    unread_count    INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(org_id, party_id)
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_conversations_org_status
    ON whatsapp_conversations (org_id, status);

CREATE INDEX IF NOT EXISTS idx_whatsapp_conversations_assigned
    ON whatsapp_conversations (org_id, assigned_to) WHERE assigned_to != '';

-- Agregar campos de operador a mensajes existentes
ALTER TABLE whatsapp_messages
    ADD COLUMN IF NOT EXISTS conversation_id UUID,
    ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_conversation
    ON whatsapp_messages (conversation_id) WHERE conversation_id IS NOT NULL;
