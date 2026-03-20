CREATE TABLE IF NOT EXISTS whatsapp_connections (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    phone_number_id text NOT NULL,
    waba_id text NOT NULL,
    access_token_encrypted text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_connections_phone
    ON whatsapp_connections(phone_number_id) WHERE is_active = true;
