CREATE TABLE IF NOT EXISTS payment_gateway_connections (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'mercadopago'
        CHECK (provider IN ('mercadopago')),
    external_user_id text NOT NULL,
    access_token_encrypted text NOT NULL,
    refresh_token_encrypted text NOT NULL,
    token_expires_at timestamptz NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    connected_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payment_preferences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'mercadopago',
    external_id text NOT NULL DEFAULT '',
    reference_type text NOT NULL CHECK (reference_type IN ('sale', 'quote')),
    reference_id uuid NOT NULL,
    amount numeric(15,2) NOT NULL,
    description text NOT NULL DEFAULT '',
    payment_url text NOT NULL DEFAULT '',
    qr_data text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'expired', 'refunded')),
    external_payer_id text NOT NULL DEFAULT '',
    paid_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payment_prefs_org
    ON payment_preferences(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_prefs_reference
    ON payment_preferences(org_id, reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_payment_prefs_external
    ON payment_preferences(provider, external_id)
    WHERE external_id != '';

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_method_check;
ALTER TABLE payments ADD CONSTRAINT payments_method_check
    CHECK (method IN ('cash', 'card', 'transfer', 'check', 'other', 'credit_note', 'mercadopago'));

ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS bank_holder text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_cbu text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_alias text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS show_qr_in_pdf boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS wa_payment_template text NOT NULL DEFAULT
        'Podes transferir a:\nAlias: {bank_alias}\nCBU: {bank_cbu}\nTitular: {bank_holder}\nBanco: {bank_name}\nMonto: {total}',
    ADD COLUMN IF NOT EXISTS wa_payment_link_template text NOT NULL DEFAULT
        'Hola {customer_name}, podes pagar {total} de tu compra {number} con este link: {payment_url}';
