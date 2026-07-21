-- 0025_fiscal.up.sql
-- Capa fiscal ARCA (ex-AFIP): emisión de comprobantes electrónicos con CAE + QR
-- vía WSAA -> WSFEv1. Emisión desde la venta. Los nombres de web services
-- (WSAA/WSFEv1) y dominios afip.gov.ar se conservan (son los reales de ARCA).

-- Config fiscal por org (emisor). La clave privada del certificado se guarda
-- CIFRADA (paymentgateway.Crypto, AES-256-GCM); nunca en texto plano.
CREATE TABLE IF NOT EXISTS fiscal_settings (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    cuit text NOT NULL DEFAULT '',
    environment text NOT NULL DEFAULT 'homologation'
        CONSTRAINT fiscal_settings_env_check CHECK (environment IN ('homologation','production')),
    tax_condition text NOT NULL DEFAULT '',
    cert_pem text NOT NULL DEFAULT '',
    key_encrypted text NOT NULL DEFAULT '',
    default_point_of_sale int NOT NULL DEFAULT 1,
    enabled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_fiscal_settings_updated_at
    BEFORE UPDATE ON fiscal_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Cache del Ticket de Acceso (TA) del WSAA por org + servicio (~12h de validez).
CREATE TABLE IF NOT EXISTS fiscal_auth_tickets (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    service text NOT NULL,
    token text NOT NULL,
    sign text NOT NULL,
    expires_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, service)
);

-- Comprobantes fiscales emitidos (documento legal de la venta).
CREATE TABLE IF NOT EXISTS fiscal_vouchers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    sale_id uuid,
    voucher_type int NOT NULL,
    point_of_sale int NOT NULL,
    cbte_nro bigint,
    concepto int NOT NULL DEFAULT 1,
    doc_tipo int NOT NULL DEFAULT 99,
    doc_nro text NOT NULL DEFAULT '0',
    condicion_iva_receptor int,
    currency text NOT NULL DEFAULT 'PES',
    exchange_rate numeric(18,6) NOT NULL DEFAULT 1,
    imp_neto numeric(15,2) NOT NULL DEFAULT 0,
    imp_iva numeric(15,2) NOT NULL DEFAULT 0,
    imp_trib numeric(15,2) NOT NULL DEFAULT 0,
    imp_op_ex numeric(15,2) NOT NULL DEFAULT 0,
    imp_tot_conc numeric(15,2) NOT NULL DEFAULT 0,
    imp_total numeric(15,2) NOT NULL DEFAULT 0,
    iva_breakdown jsonb NOT NULL DEFAULT '[]'::jsonb,
    cae text NOT NULL DEFAULT '',
    cae_vto date,
    qr_url text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT fiscal_vouchers_status_check
        CHECK (status IN ('pending','authorized','rejected','error')),
    afip_result text NOT NULL DEFAULT '',
    observations jsonb NOT NULL DEFAULT '[]'::jsonb,
    errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    request_payload jsonb,
    response_payload jsonb,
    emitted_at timestamptz,
    created_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
-- Correlatividad: un único comprobante autorizado por (org, pto vta, tipo, número).
CREATE UNIQUE INDEX IF NOT EXISTS uq_fiscal_vouchers_authorized
    ON fiscal_vouchers(org_id, point_of_sale, voucher_type, cbte_nro)
    WHERE status = 'authorized' AND cbte_nro IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_fiscal_vouchers_org ON fiscal_vouchers(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_fiscal_vouchers_sale ON fiscal_vouchers(org_id, sale_id) WHERE sale_id IS NOT NULL;
