-- 0009_messaging.up.sql
-- WhatsApp + customer messaging completo: connections, messages, templates,
-- opt_ins/outs, campaigns, recipients, conversations.
--
-- Consolida: 0015_whatsapp_connections, 0024_whatsapp_full,
-- 0038_whatsapp_campaigns, 0039_whatsapp_conversations.

CREATE TABLE IF NOT EXISTS whatsapp_connections (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    phone_number_id text NOT NULL,
    waba_id text NOT NULL,
    access_token_encrypted text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    display_phone_number text NOT NULL DEFAULT '',
    verified_name text NOT NULL DEFAULT '',
    quality_rating text NOT NULL DEFAULT 'unknown',
    messaging_limit text NOT NULL DEFAULT 'TIER_NOT_SET',
    connected_at timestamptz NOT NULL DEFAULT now(),
    disconnected_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_connections_phone_active
    ON whatsapp_connections(phone_number_id) WHERE is_active = true;

CREATE TABLE IF NOT EXISTS whatsapp_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    phone_number_id text NOT NULL,
    direction text NOT NULL
        CONSTRAINT wa_messages_direction_check
        CHECK (direction IN ('inbound','outbound')),
    wa_message_id text NOT NULL DEFAULT '',
    to_phone text NOT NULL,
    from_phone text NOT NULL DEFAULT '',
    message_type text NOT NULL DEFAULT 'text',
    body text NOT NULL DEFAULT '',
    template_name text NOT NULL DEFAULT '',
    template_language text NOT NULL DEFAULT '',
    template_params jsonb NOT NULL DEFAULT '[]'::jsonb,
    media_url text NOT NULL DEFAULT '',
    media_mime_type text NOT NULL DEFAULT '',
    media_caption text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT wa_messages_status_check
        CHECK (status IN ('pending','sent','delivered','read','failed')),
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    conversation_id uuid,
    created_by text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_wa_messages_org_created
    ON whatsapp_messages(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_wa_messages_party
    ON whatsapp_messages(party_id) WHERE party_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_wa_messages_wa_msg_id
    ON whatsapp_messages(wa_message_id) WHERE wa_message_id != '';
CREATE INDEX IF NOT EXISTS idx_wa_messages_status
    ON whatsapp_messages(org_id, status) WHERE status NOT IN ('delivered','read');
CREATE INDEX IF NOT EXISTS idx_wa_messages_conversation
    ON whatsapp_messages(conversation_id) WHERE conversation_id IS NOT NULL;

CREATE TRIGGER trg_whatsapp_messages_updated_at
    BEFORE UPDATE ON whatsapp_messages FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS whatsapp_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    meta_template_id text NOT NULL DEFAULT '',
    name text NOT NULL,
    language text NOT NULL DEFAULT 'es',
    category text NOT NULL DEFAULT 'UTILITY'
        CONSTRAINT wa_templates_category_check
        CHECK (category IN ('UTILITY','MARKETING','AUTHENTICATION')),
    status text NOT NULL DEFAULT 'draft'
        CONSTRAINT wa_templates_status_check
        CHECK (status IN ('draft','pending','approved','rejected','paused','disabled')),
    header_type text NOT NULL DEFAULT ''
        CONSTRAINT wa_templates_header_type_check
        CHECK (header_type IN ('','text','image','document','video')),
    header_text text NOT NULL DEFAULT '',
    body_text text NOT NULL,
    footer_text text NOT NULL DEFAULT '',
    buttons jsonb NOT NULL DEFAULT '[]'::jsonb,
    example_params jsonb NOT NULL DEFAULT '[]'::jsonb,
    rejection_reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_templates_org_name_lang
    ON whatsapp_templates(org_id, name, language);

CREATE TRIGGER trg_whatsapp_templates_updated_at
    BEFORE UPDATE ON whatsapp_templates FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS whatsapp_opt_ins (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    phone text NOT NULL,
    status text NOT NULL DEFAULT 'opted_in'
        CONSTRAINT wa_opt_ins_status_check
        CHECK (status IN ('opted_in','opted_out')),
    source text NOT NULL DEFAULT 'manual'
        CONSTRAINT wa_opt_ins_source_check
        CHECK (source IN ('manual','form','import','whatsapp_reply')),
    opted_in_at timestamptz NOT NULL DEFAULT now(),
    opted_out_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_opt_ins_org_party_active
    ON whatsapp_opt_ins(org_id, party_id) WHERE status = 'opted_in';
CREATE INDEX IF NOT EXISTS idx_wa_opt_ins_org_phone
    ON whatsapp_opt_ins(org_id, phone);

CREATE TABLE IF NOT EXISTS whatsapp_campaigns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL DEFAULT '',
    template_name text NOT NULL DEFAULT '',
    template_language text NOT NULL DEFAULT 'es',
    template_params jsonb NOT NULL DEFAULT '[]'::jsonb,
    tag_filter text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft',
    total_recipients int NOT NULL DEFAULT 0,
    sent_count int NOT NULL DEFAULT 0,
    delivered_count int NOT NULL DEFAULT 0,
    read_count int NOT NULL DEFAULT 0,
    failed_count int NOT NULL DEFAULT 0,
    scheduled_at timestamptz,
    started_at timestamptz,
    completed_at timestamptz,
    created_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_wa_campaigns_org_created
    ON whatsapp_campaigns(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_wa_campaigns_status
    ON whatsapp_campaigns(org_id, status);

CREATE TRIGGER trg_whatsapp_campaigns_updated_at
    BEFORE UPDATE ON whatsapp_campaigns FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS whatsapp_campaign_recipients (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id uuid NOT NULL REFERENCES whatsapp_campaigns(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    phone text NOT NULL DEFAULT '',
    party_name text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending',
    wa_message_id text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    sent_at timestamptz,
    delivered_at timestamptz,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_wa_campaign_recipients_campaign
    ON whatsapp_campaign_recipients(campaign_id);
CREATE INDEX IF NOT EXISTS idx_wa_campaign_recipients_status
    ON whatsapp_campaign_recipients(campaign_id, status);

CREATE TABLE IF NOT EXISTS whatsapp_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    phone text NOT NULL DEFAULT '',
    party_name text NOT NULL DEFAULT '',
    assigned_to text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open',
    last_message_at timestamptz,
    last_message_preview text NOT NULL DEFAULT '',
    unread_count int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT wa_conversations_org_party_uniq UNIQUE (org_id, party_id)
);
CREATE INDEX IF NOT EXISTS idx_wa_conversations_org_status
    ON whatsapp_conversations(org_id, status);
CREATE INDEX IF NOT EXISTS idx_wa_conversations_assigned
    ON whatsapp_conversations(org_id, assigned_to) WHERE assigned_to != '';

CREATE TRIGGER trg_whatsapp_conversations_updated_at
    BEFORE UPDATE ON whatsapp_conversations FOR EACH ROW EXECUTE FUNCTION set_updated_at();
