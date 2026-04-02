-- Campañas de WhatsApp (envíos masivos)
CREATE TABLE IF NOT EXISTS whatsapp_campaigns (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    template_name   TEXT NOT NULL DEFAULT '',
    template_language TEXT NOT NULL DEFAULT 'es',
    template_params JSONB NOT NULL DEFAULT '[]'::JSONB,
    tag_filter      TEXT NOT NULL DEFAULT '',
    status          VARCHAR(32) NOT NULL DEFAULT 'draft',
    total_recipients INT NOT NULL DEFAULT 0,
    sent_count      INT NOT NULL DEFAULT 0,
    delivered_count INT NOT NULL DEFAULT 0,
    read_count      INT NOT NULL DEFAULT 0,
    failed_count    INT NOT NULL DEFAULT 0,
    scheduled_at    TIMESTAMPTZ,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_by      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_campaigns_org_created
    ON whatsapp_campaigns (org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_whatsapp_campaigns_status
    ON whatsapp_campaigns (org_id, status);

-- Destinatarios individuales de cada campaña
CREATE TABLE IF NOT EXISTS whatsapp_campaign_recipients (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES whatsapp_campaigns(id) ON DELETE CASCADE,
    org_id      UUID NOT NULL,
    party_id    UUID NOT NULL,
    phone       TEXT NOT NULL DEFAULT '',
    party_name  TEXT NOT NULL DEFAULT '',
    status      VARCHAR(32) NOT NULL DEFAULT 'pending',
    wa_message_id TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    sent_at     TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_campaign_recipients_campaign
    ON whatsapp_campaign_recipients (campaign_id);

CREATE INDEX IF NOT EXISTS idx_whatsapp_campaign_recipients_status
    ON whatsapp_campaign_recipients (campaign_id, status);
