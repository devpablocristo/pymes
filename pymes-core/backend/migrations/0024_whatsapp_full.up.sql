-- Ampliar whatsapp_connections con campos de Embedded Signup y calidad
ALTER TABLE whatsapp_connections
    ADD COLUMN IF NOT EXISTS display_phone_number text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS verified_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS quality_rating text NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS messaging_limit text NOT NULL DEFAULT 'TIER_NOT_SET',
    ADD COLUMN IF NOT EXISTS connected_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS disconnected_at timestamptz;

-- Historial de mensajes enviados y recibidos
CREATE TABLE IF NOT EXISTS whatsapp_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    phone_number_id text NOT NULL,
    direction text NOT NULL CHECK (direction IN ('inbound', 'outbound')),
    wa_message_id text NOT NULL DEFAULT '',
    to_phone text NOT NULL,
    from_phone text NOT NULL DEFAULT '',
    message_type text NOT NULL DEFAULT 'text',
    body text NOT NULL DEFAULT '',
    template_name text NOT NULL DEFAULT '',
    template_language text NOT NULL DEFAULT '',
    template_params jsonb NOT NULL DEFAULT '[]',
    media_url text NOT NULL DEFAULT '',
    media_mime_type text NOT NULL DEFAULT '',
    media_caption text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'delivered', 'read', 'failed')),
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}',
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
    ON whatsapp_messages(org_id, status) WHERE status NOT IN ('delivered', 'read');

-- Templates de WhatsApp (sincronizados con Meta)
CREATE TABLE IF NOT EXISTS whatsapp_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    meta_template_id text NOT NULL DEFAULT '',
    name text NOT NULL,
    language text NOT NULL DEFAULT 'es',
    category text NOT NULL DEFAULT 'UTILITY' CHECK (category IN ('UTILITY', 'MARKETING', 'AUTHENTICATION')),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'pending', 'approved', 'rejected', 'paused', 'disabled')),
    header_type text NOT NULL DEFAULT '' CHECK (header_type IN ('', 'text', 'image', 'document', 'video')),
    header_text text NOT NULL DEFAULT '',
    body_text text NOT NULL,
    footer_text text NOT NULL DEFAULT '',
    buttons jsonb NOT NULL DEFAULT '[]',
    example_params jsonb NOT NULL DEFAULT '[]',
    rejection_reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_templates_org_name_lang
    ON whatsapp_templates(org_id, name, language);

-- Opt-in de contactos para WhatsApp
CREATE TABLE IF NOT EXISTS whatsapp_opt_ins (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    phone text NOT NULL,
    status text NOT NULL DEFAULT 'opted_in' CHECK (status IN ('opted_in', 'opted_out')),
    source text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'form', 'import', 'whatsapp_reply')),
    opted_in_at timestamptz NOT NULL DEFAULT now(),
    opted_out_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_opt_ins_org_party
    ON whatsapp_opt_ins(org_id, party_id) WHERE status = 'opted_in';
CREATE INDEX IF NOT EXISTS idx_wa_opt_ins_phone
    ON whatsapp_opt_ins(org_id, phone);
