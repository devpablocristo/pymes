CREATE SCHEMA IF NOT EXISTS fiscal;
CREATE SCHEMA IF NOT EXISTS fiscal_ar;

REVOKE CREATE ON SCHEMA fiscal FROM PUBLIC;
REVOKE CREATE ON SCHEMA fiscal_ar FROM PUBLIC;

CREATE OR REPLACE FUNCTION fiscal_ar.is_valid_cuit(requested_cuit text)
RETURNS boolean
LANGUAGE plpgsql
IMMUTABLE
STRICT
SET search_path = pg_catalog
AS $function$
DECLARE
    weights integer[] := ARRAY[5, 4, 3, 2, 7, 6, 5, 4, 3, 2];
    position integer;
    weighted_sum integer := 0;
    verifier integer;
BEGIN
    IF requested_cuit !~ '^[0-9]{11}$' THEN
        RETURN false;
    END IF;

    FOR position IN 1..10 LOOP
        weighted_sum := weighted_sum
            + substring(requested_cuit FROM position FOR 1)::integer
              * weights[position];
    END LOOP;

    verifier := 11 - (weighted_sum % 11);
    IF verifier = 11 THEN
        verifier := 0;
    ELSIF verifier = 10 THEN
        verifier := 9;
    END IF;

    RETURN verifier = right(requested_cuit, 1)::integer;
END
$function$;

REVOKE ALL ON FUNCTION fiscal_ar.is_valid_cuit(text) FROM PUBLIC;

CREATE TABLE fiscal.profiles (
    org_id uuid PRIMARY KEY
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    country_code char(2) NOT NULL DEFAULT 'AR'
        CHECK (country_code ~ '^[A-Z]{2}$'),
    legal_name text NOT NULL CHECK (btrim(legal_name) <> ''),
    legal_address jsonb NOT NULL,
    tax_condition text NOT NULL CHECK (btrim(tax_condition) <> ''),
    activity_start_date date NOT NULL,
    default_currency char(3) NOT NULL DEFAULT 'ARS'
        CHECK (default_currency ~ '^[A-Z]{3}$'),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE fiscal_ar.settings (
    org_id uuid NOT NULL,
    environment text NOT NULL
        CHECK (environment IN ('homologation', 'production')),
    cuit char(11) NOT NULL
        CHECK (fiscal_ar.is_valid_cuit(cuit::text)),
    iva_condition text NOT NULL CHECK (
        iva_condition IN (
            'responsable_inscripto',
            'monotributo',
            'exento',
            'no_responsable'
        )
    ),
    gross_income_number text,
    enabled boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, environment),
    CONSTRAINT fiscal_ar_settings_profile_fk
        FOREIGN KEY (org_id)
        REFERENCES fiscal.profiles(org_id)
        ON DELETE RESTRICT,
    CHECK (
        gross_income_number IS NULL
        OR btrim(gross_income_number) <> ''
    )
);

CREATE TABLE fiscal.certificates (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    country_code char(2) NOT NULL CHECK (country_code ~ '^[A-Z]{2}$'),
    environment text NOT NULL
        CHECK (environment IN ('homologation', 'production')),
    certificate_ref text NOT NULL CHECK (btrim(certificate_ref) <> ''),
    private_key_ref text NOT NULL CHECK (
        private_key_ref ~ '^(kms|secret|vault)://'
    ),
    fingerprint_sha256 char(64) NOT NULL
        CHECK (fingerprint_sha256 ~ '^[0-9a-f]{64}$'),
    subject_tax_id text NOT NULL CHECK (btrim(subject_tax_id) <> ''),
    valid_from timestamptz NOT NULL,
    valid_until timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'rotated', 'revoked', 'expired')),
    rotates_certificate_id uuid,
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_certificates_environment_identity_unique
        UNIQUE (org_id, environment, id),
    CONSTRAINT fiscal_certificates_fingerprint_unique
        UNIQUE (org_id, environment, fingerprint_sha256),
    CONSTRAINT fiscal_certificates_rotation_fk
        FOREIGN KEY (org_id, rotates_certificate_id)
        REFERENCES fiscal.certificates(org_id, id)
        ON DELETE RESTRICT,
    CHECK (valid_until > valid_from),
    CHECK (rotates_certificate_id IS NULL OR rotates_certificate_id <> id),
    CHECK (
        country_code <> 'AR'
        OR fiscal_ar.is_valid_cuit(subject_tax_id)
    )
);

CREATE INDEX fiscal_certificates_expiration_idx
    ON fiscal.certificates (org_id, environment, status, valid_until);

CREATE TABLE fiscal.points_of_sale (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    country_code char(2) NOT NULL CHECK (country_code ~ '^[A-Z]{2}$'),
    environment text NOT NULL
        CHECK (environment IN ('homologation', 'production')),
    code integer NOT NULL CHECK (code BETWEEN 1 AND 99999),
    issuing_system text NOT NULL DEFAULT 'wsfev1'
        CHECK (btrim(issuing_system) <> ''),
    name text NOT NULL CHECK (btrim(name) <> ''),
    enabled boolean NOT NULL DEFAULT true,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_points_of_sale_environment_identity_unique
        UNIQUE (org_id, environment, id),
    CONSTRAINT fiscal_points_of_sale_code_unique
        UNIQUE (org_id, country_code, environment, code)
);

CREATE TABLE fiscal.auth_requests (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    country_code char(2) NOT NULL CHECK (country_code ~ '^[A-Z]{2}$'),
    environment text NOT NULL
        CHECK (environment IN ('homologation', 'production')),
    service text NOT NULL CHECK (btrim(service) <> ''),
    certificate_id uuid NOT NULL,
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    unique_id bigint NOT NULL CHECK (unique_id > 0),
    generation_time timestamptz NOT NULL,
    expiration_time timestamptz NOT NULL,
    tra_sha256 char(64) NOT NULL CHECK (tra_sha256 ~ '^[0-9a-f]{64}$'),
    cms_sha256 char(64)
        CHECK (cms_sha256 IS NULL OR cms_sha256 ~ '^[0-9a-f]{64}$'),
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'processing', 'succeeded', 'failed')),
    error_code text,
    error_detail_redacted text,
    lease_owner text,
    lease_until timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_auth_requests_idempotency_unique
        UNIQUE (org_id, idempotency_key),
    CONSTRAINT fiscal_auth_requests_certificate_fk
        FOREIGN KEY (org_id, environment, certificate_id)
        REFERENCES fiscal.certificates(org_id, environment, id)
        ON DELETE RESTRICT,
    CHECK (expiration_time > generation_time),
    CHECK (
        (status IN ('queued', 'processing') AND completed_at IS NULL)
        OR
        (status IN ('succeeded', 'failed') AND completed_at IS NOT NULL)
    ),
    CHECK (
        (lease_owner IS NULL AND lease_until IS NULL)
        OR
        (
            lease_owner IS NOT NULL
            AND btrim(lease_owner) <> ''
            AND lease_until IS NOT NULL
        )
    )
);

CREATE INDEX fiscal_auth_requests_work_idx
    ON fiscal.auth_requests (org_id, status, lease_until, created_at);

CREATE TABLE fiscal.auth_ticket_refs (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    auth_request_id uuid NOT NULL,
    environment text NOT NULL
        CHECK (environment IN ('homologation', 'production')),
    service text NOT NULL CHECK (btrim(service) <> ''),
    certificate_id uuid NOT NULL,
    secret_ref text NOT NULL CHECK (
        secret_ref ~ '^(kms|secret|vault)://'
    ),
    expires_at timestamptz NOT NULL,
    response_sha256 char(64) NOT NULL
        CHECK (response_sha256 ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_auth_ticket_refs_request_unique
        UNIQUE (org_id, auth_request_id),
    CONSTRAINT fiscal_auth_ticket_refs_request_fk
        FOREIGN KEY (org_id, auth_request_id)
        REFERENCES fiscal.auth_requests(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_auth_ticket_refs_certificate_fk
        FOREIGN KEY (org_id, environment, certificate_id)
        REFERENCES fiscal.certificates(org_id, environment, id)
        ON DELETE RESTRICT
);

CREATE INDEX fiscal_auth_ticket_refs_cache_idx
    ON fiscal.auth_ticket_refs (
        org_id,
        environment,
        service,
        certificate_id,
        expires_at DESC
    );

CREATE TABLE fiscal.vouchers (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    country_code char(2) NOT NULL DEFAULT 'AR'
        CHECK (country_code ~ '^[A-Z]{2}$'),
    environment text NOT NULL
        CHECK (environment IN ('homologation', 'production')),
    point_of_sale_id uuid NOT NULL,
    certificate_id uuid,
    operation text NOT NULL
        CHECK (operation IN ('invoice', 'credit_note', 'debit_note')),
    voucher_type integer NOT NULL CHECK (voucher_type > 0),
    source_type text NOT NULL CHECK (btrim(source_type) <> ''),
    source_id text NOT NULL CHECK (btrim(source_id) <> ''),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    command_fingerprint char(64) NOT NULL
        CHECK (command_fingerprint ~ '^[0-9a-f]{64}$'),
    status text NOT NULL DEFAULT 'queued' CHECK (
        status IN (
            'queued',
            'processing',
            'authorized',
            'rejected',
            'uncertain'
        )
    ),
    voucher_number bigint CHECK (voucher_number > 0),
    concept text NOT NULL
        CHECK (concept IN ('products', 'services', 'mixed')),
    issue_date date NOT NULL,
    service_from date,
    service_to date,
    payment_due_date date,
    currency_code char(3) NOT NULL
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    exchange_rate numeric(24, 10) NOT NULL CHECK (exchange_rate > 0),
    exchange_rate_date date NOT NULL,
    exchange_rate_source text NOT NULL CHECK (btrim(exchange_rate_source) <> ''),
    net_amount numeric(24, 6) NOT NULL CHECK (net_amount >= 0),
    exempt_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (exempt_amount >= 0),
    non_taxed_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (non_taxed_amount >= 0),
    vat_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (vat_amount >= 0),
    other_tributes_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (other_tributes_amount >= 0),
    total_amount numeric(24, 6) NOT NULL CHECK (total_amount > 0),
    authorization_code char(14)
        CHECK (
            authorization_code IS NULL
            OR authorization_code ~ '^[0-9]{14}$'
        ),
    authorization_expires_at date,
    arca_result text,
    request_sha256 char(64)
        CHECK (request_sha256 IS NULL OR request_sha256 ~ '^[0-9a-f]{64}$'),
    response_sha256 char(64)
        CHECK (response_sha256 IS NULL OR response_sha256 ~ '^[0-9a-f]{64}$'),
    response_storage_ref text,
    lease_owner text,
    lease_until timestamptz,
    last_error_code text,
    last_error_detail_redacted text,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    authorized_at timestamptz,
    rejected_at timestamptz,
    uncertain_at timestamptz,
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_vouchers_environment_identity_unique
        UNIQUE (org_id, environment, id),
    CONSTRAINT fiscal_vouchers_idempotency_unique
        UNIQUE (org_id, idempotency_key),
    CONSTRAINT fiscal_vouchers_source_unique
        UNIQUE (
            org_id,
            country_code,
            environment,
            source_type,
            source_id,
            operation
        ),
    CONSTRAINT fiscal_vouchers_point_of_sale_fk
        FOREIGN KEY (org_id, environment, point_of_sale_id)
        REFERENCES fiscal.points_of_sale(org_id, environment, id)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_vouchers_certificate_fk
        FOREIGN KEY (org_id, environment, certificate_id)
        REFERENCES fiscal.certificates(org_id, environment, id)
        ON DELETE RESTRICT,
    CHECK (
        total_amount
        = net_amount
        + exempt_amount
        + non_taxed_amount
        + vat_amount
        + other_tributes_amount
    ),
    CHECK (
        (
            concept = 'products'
            AND service_from IS NULL
            AND service_to IS NULL
            AND payment_due_date IS NULL
        )
        OR
        (
            concept IN ('services', 'mixed')
            AND service_from IS NOT NULL
            AND service_to IS NOT NULL
            AND payment_due_date IS NOT NULL
            AND service_to >= service_from
        )
    ),
    CHECK (
        (lease_owner IS NULL AND lease_until IS NULL)
        OR
        (
            lease_owner IS NOT NULL
            AND btrim(lease_owner) <> ''
            AND lease_until IS NOT NULL
        )
    ),
    CHECK (
        status <> 'authorized'
        OR
        (
            voucher_number IS NOT NULL
            AND authorization_code IS NOT NULL
            AND authorization_expires_at IS NOT NULL
            AND response_sha256 IS NOT NULL
            AND authorized_at IS NOT NULL
            AND rejected_at IS NULL
            AND uncertain_at IS NULL
        )
    ),
    CHECK (
        status <> 'rejected'
        OR
        (
            rejected_at IS NOT NULL
            AND authorized_at IS NULL
            AND uncertain_at IS NULL
        )
    ),
    CHECK (
        status <> 'uncertain'
        OR
        (
            voucher_number IS NOT NULL
            AND uncertain_at IS NOT NULL
            AND authorized_at IS NULL
            AND rejected_at IS NULL
        )
    )
);

CREATE UNIQUE INDEX fiscal_vouchers_number_uidx
    ON fiscal.vouchers (
        org_id,
        environment,
        point_of_sale_id,
        voucher_type,
        voucher_number
    )
    WHERE voucher_number IS NOT NULL AND status <> 'rejected';

CREATE INDEX fiscal_vouchers_work_idx
    ON fiscal.vouchers (
        org_id,
        environment,
        status,
        lease_until,
        created_at
    );

CREATE TABLE fiscal.voucher_snapshots (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    voucher_id uuid NOT NULL,
    snapshot_version integer NOT NULL DEFAULT 1 CHECK (snapshot_version > 0),
    issuer_tax_id text NOT NULL CHECK (btrim(issuer_tax_id) <> ''),
    issuer_legal_name text NOT NULL CHECK (btrim(issuer_legal_name) <> ''),
    issuer_tax_condition text NOT NULL CHECK (btrim(issuer_tax_condition) <> ''),
    issuer_address jsonb NOT NULL,
    issuer_activity_start_date date NOT NULL,
    recipient_document_type integer NOT NULL CHECK (recipient_document_type > 0),
    recipient_document_number text NOT NULL
        CHECK (btrim(recipient_document_number) <> ''),
    recipient_name text NOT NULL CHECK (btrim(recipient_name) <> ''),
    recipient_tax_condition text NOT NULL CHECK (btrim(recipient_tax_condition) <> ''),
    recipient_address jsonb,
    currency_code char(3) NOT NULL
        CHECK (currency_code ~ '^[A-Z]{3}$'),
    exchange_rate numeric(24, 10) NOT NULL CHECK (exchange_rate > 0),
    exchange_rate_date date NOT NULL,
    exchange_rate_source text NOT NULL CHECK (btrim(exchange_rate_source) <> ''),
    issue_date date NOT NULL,
    service_from date,
    service_to date,
    payment_due_date date,
    net_amount numeric(24, 6) NOT NULL CHECK (net_amount >= 0),
    exempt_amount numeric(24, 6) NOT NULL CHECK (exempt_amount >= 0),
    non_taxed_amount numeric(24, 6) NOT NULL CHECK (non_taxed_amount >= 0),
    vat_amount numeric(24, 6) NOT NULL CHECK (vat_amount >= 0),
    other_tributes_amount numeric(24, 6) NOT NULL
        CHECK (other_tributes_amount >= 0),
    total_amount numeric(24, 6) NOT NULL CHECK (total_amount > 0),
    canonical_json text NOT NULL CHECK (
        jsonb_typeof(canonical_json::jsonb) = 'object'
    ),
    snapshot_sha256 char(64) NOT NULL
        CHECK (snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_voucher_snapshots_voucher_unique
        UNIQUE (org_id, voucher_id),
    CONSTRAINT fiscal_voucher_snapshots_voucher_fk
        FOREIGN KEY (org_id, voucher_id)
        REFERENCES fiscal.vouchers(org_id, id)
        ON DELETE RESTRICT,
    CHECK (
        total_amount
        = net_amount
        + exempt_amount
        + non_taxed_amount
        + vat_amount
        + other_tributes_amount
    ),
    CHECK (
        recipient_document_type NOT IN (80, 86)
        OR fiscal_ar.is_valid_cuit(recipient_document_number)
    ),
    CHECK (
        recipient_document_type <> 96
        OR recipient_document_number ~ '^[0-9]{7,8}$'
    ),
    CHECK (
        recipient_document_type <> 99
        OR recipient_document_number = '0'
    ),
    CHECK (
        snapshot_sha256
        = encode(
            digest(convert_to(canonical_json, 'UTF8'), 'sha256'),
            'hex'
        )
    )
);

CREATE TABLE fiscal.voucher_lines (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    snapshot_id uuid NOT NULL,
    line_no integer NOT NULL CHECK (line_no > 0),
    product_reference text,
    description text NOT NULL CHECK (btrim(description) <> ''),
    quantity numeric(24, 6) NOT NULL CHECK (quantity > 0),
    unit_of_measure text NOT NULL CHECK (btrim(unit_of_measure) <> ''),
    unit_price numeric(24, 6) NOT NULL CHECK (unit_price >= 0),
    discount_amount numeric(24, 6) NOT NULL DEFAULT 0
        CHECK (discount_amount >= 0),
    tax_treatment text NOT NULL
        CHECK (tax_treatment IN ('taxable', 'exempt', 'non_taxed')),
    vat_rate numeric(9, 6) NOT NULL DEFAULT 0 CHECK (vat_rate >= 0),
    net_amount numeric(24, 6) NOT NULL CHECK (net_amount >= 0),
    vat_amount numeric(24, 6) NOT NULL CHECK (vat_amount >= 0),
    total_amount numeric(24, 6) NOT NULL CHECK (total_amount >= 0),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_voucher_lines_number_unique
        UNIQUE (org_id, snapshot_id, line_no),
    CONSTRAINT fiscal_voucher_lines_snapshot_fk
        FOREIGN KEY (org_id, snapshot_id)
        REFERENCES fiscal.voucher_snapshots(org_id, id)
        ON DELETE RESTRICT,
    CHECK (discount_amount <= round(quantity * unit_price, 6)),
    CHECK (total_amount = net_amount + vat_amount),
    CHECK (
        (tax_treatment = 'taxable')
        OR
        (vat_rate = 0 AND vat_amount = 0)
    )
);

CREATE TABLE fiscal.voucher_taxes (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    snapshot_id uuid NOT NULL,
    line_no integer NOT NULL CHECK (line_no > 0),
    tax_type text NOT NULL CHECK (
        tax_type IN ('vat', 'tribute', 'withholding', 'perception')
    ),
    authority_code text NOT NULL CHECK (btrim(authority_code) <> ''),
    description text NOT NULL CHECK (btrim(description) <> ''),
    taxable_base numeric(24, 6) NOT NULL CHECK (taxable_base >= 0),
    rate numeric(9, 6) NOT NULL CHECK (rate >= 0),
    amount numeric(24, 6) NOT NULL CHECK (amount >= 0),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_voucher_taxes_number_unique
        UNIQUE (org_id, snapshot_id, line_no),
    CONSTRAINT fiscal_voucher_taxes_snapshot_fk
        FOREIGN KEY (org_id, snapshot_id)
        REFERENCES fiscal.voucher_snapshots(org_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE fiscal.voucher_associations (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    voucher_id uuid NOT NULL,
    associated_voucher_id uuid NOT NULL,
    association_type text NOT NULL DEFAULT 'adjusts'
        CHECK (association_type = 'adjusts'),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_voucher_associations_voucher_unique
        UNIQUE (org_id, voucher_id),
    CONSTRAINT fiscal_voucher_associations_voucher_fk
        FOREIGN KEY (org_id, voucher_id)
        REFERENCES fiscal.vouchers(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_voucher_associations_original_fk
        FOREIGN KEY (org_id, associated_voucher_id)
        REFERENCES fiscal.vouchers(org_id, id)
        ON DELETE RESTRICT,
    CHECK (voucher_id <> associated_voucher_id)
);

CREATE TABLE fiscal.authorization_attempts (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    voucher_id uuid NOT NULL,
    attempt_no integer NOT NULL CHECK (attempt_no > 0),
    idempotency_key text NOT NULL CHECK (btrim(idempotency_key) <> ''),
    operation text NOT NULL
        CHECK (operation IN ('authorize', 'consult')),
    status text NOT NULL DEFAULT 'processing' CHECK (
        status IN (
            'processing',
            'succeeded',
            'rejected',
            'uncertain',
            'failed'
        )
    ),
    request_sha256 char(64) NOT NULL
        CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    request_storage_ref text,
    response_sha256 char(64)
        CHECK (response_sha256 IS NULL OR response_sha256 ~ '^[0-9a-f]{64}$'),
    response_storage_ref text,
    error_code text,
    error_detail_redacted text,
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_authorization_attempts_number_unique
        UNIQUE (org_id, voucher_id, attempt_no),
    CONSTRAINT fiscal_authorization_attempts_idempotency_unique
        UNIQUE (org_id, idempotency_key),
    CONSTRAINT fiscal_authorization_attempts_voucher_fk
        FOREIGN KEY (org_id, voucher_id)
        REFERENCES fiscal.vouchers(org_id, id)
        ON DELETE RESTRICT,
    CHECK (
        (status = 'processing' AND completed_at IS NULL)
        OR
        (status <> 'processing' AND completed_at IS NOT NULL)
    )
);

CREATE TABLE fiscal.voucher_number_reservations (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    voucher_id uuid NOT NULL,
    environment text NOT NULL
        CHECK (environment IN ('homologation', 'production')),
    point_of_sale_id uuid NOT NULL,
    voucher_type integer NOT NULL CHECK (voucher_type > 0),
    voucher_number bigint NOT NULL CHECK (voucher_number > 0),
    status text NOT NULL DEFAULT 'reserved'
        CHECK (status IN ('reserved', 'authorized', 'rejected', 'uncertain')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_voucher_number_reservations_voucher_unique
        UNIQUE (org_id, voucher_id),
    CONSTRAINT fiscal_voucher_number_reservations_voucher_fk
        FOREIGN KEY (org_id, environment, voucher_id)
        REFERENCES fiscal.vouchers(org_id, environment, id)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_voucher_number_reservations_point_fk
        FOREIGN KEY (org_id, environment, point_of_sale_id)
        REFERENCES fiscal.points_of_sale(org_id, environment, id)
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX fiscal_voucher_number_reservations_number_uidx
    ON fiscal.voucher_number_reservations (
        org_id,
        environment,
        point_of_sale_id,
        voucher_type,
        voucher_number
    )
    WHERE status <> 'rejected';

CREATE TABLE fiscal.voucher_artifacts (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    voucher_id uuid NOT NULL,
    artifact_type text NOT NULL CHECK (
        artifact_type IN (
            'pdf',
            'qr',
            'authority_request',
            'authority_response'
        )
    ),
    artifact_version integer NOT NULL DEFAULT 1 CHECK (artifact_version > 0),
    storage_ref text NOT NULL CHECK (btrim(storage_ref) <> ''),
    content_type text NOT NULL CHECK (btrim(content_type) <> ''),
    sha256 char(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_voucher_artifacts_version_unique
        UNIQUE (org_id, voucher_id, artifact_type, artifact_version),
    CONSTRAINT fiscal_voucher_artifacts_voucher_fk
        FOREIGN KEY (org_id, voucher_id)
        REFERENCES fiscal.vouchers(org_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE fiscal.voucher_accounting_links (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    voucher_id uuid NOT NULL,
    journal_entry_id uuid NOT NULL,
    created_by text NOT NULL CHECK (btrim(created_by) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_voucher_accounting_links_voucher_unique
        UNIQUE (org_id, voucher_id),
    CONSTRAINT fiscal_voucher_accounting_links_entry_unique
        UNIQUE (org_id, journal_entry_id),
    CONSTRAINT fiscal_voucher_accounting_links_voucher_fk
        FOREIGN KEY (org_id, voucher_id)
        REFERENCES fiscal.vouchers(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_voucher_accounting_links_entry_fk
        FOREIGN KEY (org_id, journal_entry_id)
        REFERENCES accounting.journal_entries(org_id, id)
        ON DELETE RESTRICT
);

CREATE TABLE fiscal.accounting_posting_intents (
    org_id uuid NOT NULL,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    voucher_id uuid NOT NULL,
    source_type text NOT NULL CHECK (btrim(source_type) <> ''),
    source_id text NOT NULL CHECK (btrim(source_id) <> ''),
    operation text NOT NULL
        CHECK (operation IN ('invoice', 'credit_note', 'debit_note')),
    snapshot_sha256 char(64) NOT NULL
        CHECK (snapshot_sha256 ~ '^[0-9a-f]{64}$'),
    authority_code char(14) NOT NULL
        CHECK (authority_code ~ '^[0-9]{14}$'),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'posted', 'failed')),
    journal_entry_id uuid,
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    last_error_code text
        CHECK (
            last_error_code IS NULL
            OR btrim(last_error_code) <> ''
        ),
    last_error_detail_redacted text
        CHECK (
            last_error_detail_redacted IS NULL
            OR btrim(last_error_detail_redacted) <> ''
        ),
    last_attempt_at timestamptz,
    posted_at timestamptz,
    failed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT fiscal_accounting_posting_intents_voucher_unique
        UNIQUE (org_id, voucher_id),
    CONSTRAINT fiscal_accounting_posting_intents_entry_unique
        UNIQUE (org_id, journal_entry_id),
    CONSTRAINT fiscal_accounting_posting_intents_voucher_fk
        FOREIGN KEY (org_id, voucher_id)
        REFERENCES fiscal.vouchers(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_accounting_posting_intents_entry_fk
        FOREIGN KEY (org_id, journal_entry_id)
        REFERENCES accounting.journal_entries(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_accounting_posting_intents_state_check CHECK (
        (
            status = 'pending'
            AND journal_entry_id IS NULL
            AND posted_at IS NULL
        )
        OR
        (
            status = 'failed'
            AND journal_entry_id IS NULL
            AND posted_at IS NULL
            AND failed_at IS NOT NULL
            AND attempt_count > 0
            AND (
                last_error_code IS NOT NULL
                OR last_error_detail_redacted IS NOT NULL
            )
        )
        OR
        (
            status = 'posted'
            AND journal_entry_id IS NOT NULL
            AND posted_at IS NOT NULL
            AND attempt_count > 0
            AND last_error_code IS NULL
            AND last_error_detail_redacted IS NULL
        )
    ),
    CHECK (updated_at >= created_at),
    CHECK (last_attempt_at IS NULL OR last_attempt_at >= created_at),
    CHECK (posted_at IS NULL OR posted_at >= created_at),
    CHECK (failed_at IS NULL OR failed_at >= created_at)
);

CREATE INDEX fiscal_accounting_posting_intents_work_idx
    ON fiscal.accounting_posting_intents (
        org_id,
        status,
        updated_at,
        created_at,
        id
    )
    WHERE status IN ('pending', 'failed');

CREATE OR REPLACE FUNCTION fiscal.reject_immutable_change()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
    RAISE EXCEPTION '% rows are immutable after insertion', TG_TABLE_NAME
        USING ERRCODE = '55000';
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.guard_operational_transition()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF (
            TG_TABLE_NAME = 'auth_requests'
            AND NEW.status <> 'queued'
        ) OR (
            TG_TABLE_NAME = 'authorization_attempts'
            AND NEW.status <> 'processing'
        ) THEN
            RAISE EXCEPTION 'invalid initial operational status'
                USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'fiscal operational history cannot be deleted'
            USING ERRCODE = '55000';
    END IF;

    IF OLD.status IN ('succeeded', 'failed', 'rejected', 'uncertain')
       AND NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION '% terminal rows are immutable', TG_TABLE_NAME
            USING ERRCODE = '55000';
    END IF;

    IF OLD.status IS DISTINCT FROM NEW.status
       AND NEW.version <> OLD.version + 1 THEN
        RAISE EXCEPTION 'status transition must increment version'
            USING ERRCODE = '40001';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.validate_accounting_posting_intent()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'pending'
           OR NEW.journal_entry_id IS NOT NULL
           OR NEW.attempt_count <> 0
           OR NEW.last_error_code IS NOT NULL
           OR NEW.last_error_detail_redacted IS NOT NULL
           OR NEW.last_attempt_at IS NOT NULL
           OR NEW.posted_at IS NOT NULL
           OR NEW.failed_at IS NOT NULL THEN
            RAISE EXCEPTION
                'a fiscal accounting posting intent must start pending'
                USING ERRCODE = '23514',
                      CONSTRAINT =
                          'fiscal_accounting_posting_intents_initial_state';
        END IF;
        RETURN NEW;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'fiscal accounting posting intents cannot be deleted'
            USING ERRCODE = '55000';
    END IF;

    IF ROW(
        OLD.org_id,
        OLD.id,
        OLD.voucher_id,
        OLD.source_type,
        OLD.source_id,
        OLD.operation,
        OLD.snapshot_sha256,
        OLD.authority_code,
        OLD.created_at
    ) IS DISTINCT FROM ROW(
        NEW.org_id,
        NEW.id,
        NEW.voucher_id,
        NEW.source_type,
        NEW.source_id,
        NEW.operation,
        NEW.snapshot_sha256,
        NEW.authority_code,
        NEW.created_at
    ) THEN
        RAISE EXCEPTION
            'fiscal accounting posting intent identity is immutable'
            USING ERRCODE = '55000';
    END IF;

    IF OLD.status = 'posted' AND NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'posted fiscal accounting intents are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF NEW.attempt_count < OLD.attempt_count
       OR (
           OLD.last_attempt_at IS NOT NULL
           AND NEW.last_attempt_at IS NOT NULL
           AND NEW.last_attempt_at < OLD.last_attempt_at
       ) THEN
        RAISE EXCEPTION 'posting intent attempt history cannot move backward'
            USING ERRCODE = '23514';
    END IF;

    IF OLD.status IS DISTINCT FROM NEW.status
       AND NOT (
           (OLD.status = 'pending' AND NEW.status IN ('posted', 'failed'))
           OR
           (OLD.status = 'failed' AND NEW.status IN ('pending', 'posted'))
       ) THEN
        RAISE EXCEPTION 'invalid posting intent transition % -> %',
            OLD.status,
            NEW.status
            USING ERRCODE = '23514';
    END IF;

    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.assert_authorized_posting_intent(
    target_org_id uuid,
    target_voucher_id uuid
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, fiscal
AS $function$
DECLARE
    voucher_status text;
    voucher_source_type text;
    voucher_source_id text;
    voucher_operation text;
    voucher_authority_code char(14);
    voucher_snapshot_sha256 char(64);
    intent_exists boolean;
    matching_intent_exists boolean;
BEGIN
    PERFORM app.assert_org_context(target_org_id);

    SELECT
        voucher.status,
        voucher.source_type,
        voucher.source_id,
        voucher.operation,
        voucher.authorization_code,
        snapshot.snapshot_sha256
      INTO
        voucher_status,
        voucher_source_type,
        voucher_source_id,
        voucher_operation,
        voucher_authority_code,
        voucher_snapshot_sha256
      FROM fiscal.vouchers AS voucher
      LEFT JOIN fiscal.voucher_snapshots AS snapshot
        ON snapshot.org_id = voucher.org_id
       AND snapshot.voucher_id = voucher.id
     WHERE voucher.org_id = target_org_id
       AND voucher.id = target_voucher_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT
        EXISTS (
            SELECT 1
              FROM fiscal.accounting_posting_intents AS intent
             WHERE intent.org_id = target_org_id
               AND intent.voucher_id = target_voucher_id
        ),
        EXISTS (
            SELECT 1
              FROM fiscal.accounting_posting_intents AS intent
             WHERE intent.org_id = target_org_id
               AND intent.voucher_id = target_voucher_id
               AND intent.source_type = voucher_source_type
               AND intent.source_id = voucher_source_id
               AND intent.operation = voucher_operation
               AND intent.snapshot_sha256 = voucher_snapshot_sha256
               AND intent.authority_code = voucher_authority_code
        )
      INTO intent_exists, matching_intent_exists;

    IF voucher_status = 'authorized' AND NOT matching_intent_exists THEN
        RAISE EXCEPTION
            'authorized fiscal voucher requires a matching posting intent'
            USING ERRCODE = '23514',
                  CONSTRAINT =
                      'fiscal_authorized_voucher_posting_intent_required';
    END IF;

    IF voucher_status <> 'authorized' AND intent_exists THEN
        RAISE EXCEPTION
            'posting intent requires an authorized fiscal voucher'
            USING ERRCODE = '23514',
                  CONSTRAINT =
                      'fiscal_posting_intent_authorized_voucher_required';
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.check_authorized_posting_intent_constraint()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    row_data jsonb;
    target_org_id uuid;
    target_voucher_id uuid;
BEGIN
    IF TG_OP = 'DELETE' THEN
        row_data := to_jsonb(OLD);
    ELSE
        row_data := to_jsonb(NEW);
    END IF;

    target_org_id := (row_data ->> 'org_id')::uuid;
    IF TG_TABLE_NAME = 'vouchers' THEN
        target_voucher_id := (row_data ->> 'id')::uuid;
    ELSE
        target_voucher_id := (row_data ->> 'voucher_id')::uuid;
    END IF;

    PERFORM fiscal.assert_authorized_posting_intent(
        target_org_id,
        target_voucher_id
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.assert_voucher_snapshot_valid(
    target_org_id uuid,
    target_voucher_id uuid
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    voucher_record fiscal.vouchers%ROWTYPE;
    snapshot_record fiscal.voucher_snapshots%ROWTYPE;
    taxable_total numeric(24, 6);
    exempt_total numeric(24, 6);
    non_taxed_total numeric(24, 6);
    line_vat_total numeric(24, 6);
    line_total numeric(24, 6);
    tax_vat_total numeric(24, 6);
    tribute_total numeric(24, 6);
    line_count integer;
    association_count integer;
BEGIN
    SELECT *
      INTO voucher_record
      FROM fiscal.vouchers
     WHERE org_id = target_org_id
       AND id = target_voucher_id;

    IF NOT FOUND THEN
        RETURN;
    END IF;

    SELECT *
      INTO snapshot_record
      FROM fiscal.voucher_snapshots
     WHERE org_id = target_org_id
       AND voucher_id = target_voucher_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'fiscal voucher requires an immutable snapshot'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_vouchers_snapshot_required';
    END IF;

    IF ROW(
        snapshot_record.currency_code,
        snapshot_record.exchange_rate,
        snapshot_record.exchange_rate_date,
        snapshot_record.exchange_rate_source,
        snapshot_record.issue_date,
        snapshot_record.service_from,
        snapshot_record.service_to,
        snapshot_record.payment_due_date,
        snapshot_record.net_amount,
        snapshot_record.exempt_amount,
        snapshot_record.non_taxed_amount,
        snapshot_record.vat_amount,
        snapshot_record.other_tributes_amount,
        snapshot_record.total_amount
    ) IS DISTINCT FROM ROW(
        voucher_record.currency_code,
        voucher_record.exchange_rate,
        voucher_record.exchange_rate_date,
        voucher_record.exchange_rate_source,
        voucher_record.issue_date,
        voucher_record.service_from,
        voucher_record.service_to,
        voucher_record.payment_due_date,
        voucher_record.net_amount,
        voucher_record.exempt_amount,
        voucher_record.non_taxed_amount,
        voucher_record.vat_amount,
        voucher_record.other_tributes_amount,
        voucher_record.total_amount
    ) THEN
        RAISE EXCEPTION 'voucher and fiscal snapshot totals differ'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_vouchers_snapshot_totals';
    END IF;

    SELECT
        count(*),
        coalesce(sum(line.net_amount)
            FILTER (WHERE line.tax_treatment = 'taxable'), 0),
        coalesce(sum(line.net_amount)
            FILTER (WHERE line.tax_treatment = 'exempt'), 0),
        coalesce(sum(line.net_amount)
            FILTER (WHERE line.tax_treatment = 'non_taxed'), 0),
        coalesce(sum(line.vat_amount), 0),
        coalesce(sum(line.total_amount), 0)
      INTO
        line_count,
        taxable_total,
        exempt_total,
        non_taxed_total,
        line_vat_total,
        line_total
      FROM fiscal.voucher_lines AS line
     WHERE line.org_id = target_org_id
       AND line.snapshot_id = snapshot_record.id;

    IF line_count = 0
       OR taxable_total <> voucher_record.net_amount
       OR exempt_total <> voucher_record.exempt_amount
       OR non_taxed_total <> voucher_record.non_taxed_amount
       OR line_vat_total <> voucher_record.vat_amount
       OR line_total + voucher_record.other_tributes_amount
            <> voucher_record.total_amount THEN
        RAISE EXCEPTION 'fiscal snapshot lines do not reconcile to voucher totals'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_voucher_lines_totals';
    END IF;

    SELECT
        coalesce(sum(tax.amount)
            FILTER (WHERE tax.tax_type = 'vat'), 0),
        coalesce(sum(tax.amount)
            FILTER (WHERE tax.tax_type IN ('tribute', 'perception')), 0)
      INTO tax_vat_total, tribute_total
      FROM fiscal.voucher_taxes AS tax
     WHERE tax.org_id = target_org_id
       AND tax.snapshot_id = snapshot_record.id;

    IF tax_vat_total <> voucher_record.vat_amount
       OR tribute_total <> voucher_record.other_tributes_amount THEN
        RAISE EXCEPTION 'fiscal snapshot taxes do not reconcile to voucher totals'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_voucher_taxes_totals';
    END IF;

    SELECT count(*)
      INTO association_count
      FROM fiscal.voucher_associations AS association
      JOIN fiscal.vouchers AS original
        ON original.org_id = association.org_id
       AND original.id = association.associated_voucher_id
     WHERE association.org_id = target_org_id
       AND association.voucher_id = target_voucher_id
       AND original.status = 'authorized';

    IF (voucher_record.operation = 'invoice' AND association_count <> 0)
       OR (
           voucher_record.operation IN ('credit_note', 'debit_note')
           AND association_count <> 1
       ) THEN
        RAISE EXCEPTION 'credit and debit notes require one authorized association'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_vouchers_association';
    END IF;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.check_voucher_snapshot_constraint()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
DECLARE
    target_voucher_id uuid;
BEGIN
    IF TG_TABLE_NAME = 'vouchers' THEN
        target_voucher_id := coalesce(NEW.id, OLD.id);
    ELSIF TG_TABLE_NAME = 'voucher_snapshots' THEN
        target_voucher_id := coalesce(NEW.voucher_id, OLD.voucher_id);
    ELSIF TG_TABLE_NAME IN ('voucher_lines', 'voucher_taxes') THEN
        SELECT snapshot.voucher_id
          INTO target_voucher_id
          FROM fiscal.voucher_snapshots AS snapshot
         WHERE snapshot.org_id = coalesce(NEW.org_id, OLD.org_id)
           AND snapshot.id = coalesce(NEW.snapshot_id, OLD.snapshot_id);
    ELSE
        target_voucher_id := coalesce(NEW.voucher_id, OLD.voucher_id);
    END IF;

    PERFORM fiscal.assert_voucher_snapshot_valid(
        coalesce(NEW.org_id, OLD.org_id),
        target_voucher_id
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.validate_voucher_update()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.status <> 'queued'
           OR NEW.voucher_number IS NOT NULL
           OR NEW.authorization_code IS NOT NULL THEN
            RAISE EXCEPTION 'a new fiscal voucher must be queued and unnumbered'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'fiscal_vouchers_initially_queued';
        END IF;
        RETURN NEW;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'fiscal vouchers cannot be deleted'
            USING ERRCODE = '55000';
    END IF;

    IF OLD.status IN ('authorized', 'rejected') THEN
        RAISE EXCEPTION 'terminal fiscal vouchers are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF ROW(
        OLD.org_id,
        OLD.country_code,
        OLD.environment,
        OLD.point_of_sale_id,
        OLD.operation,
        OLD.voucher_type,
        OLD.source_type,
        OLD.source_id,
        OLD.idempotency_key,
        OLD.command_fingerprint,
        OLD.concept,
        OLD.issue_date,
        OLD.service_from,
        OLD.service_to,
        OLD.payment_due_date,
        OLD.currency_code,
        OLD.exchange_rate,
        OLD.exchange_rate_date,
        OLD.exchange_rate_source,
        OLD.net_amount,
        OLD.exempt_amount,
        OLD.non_taxed_amount,
        OLD.vat_amount,
        OLD.other_tributes_amount,
        OLD.total_amount,
        OLD.created_by,
        OLD.created_at
    ) IS DISTINCT FROM ROW(
        NEW.org_id,
        NEW.country_code,
        NEW.environment,
        NEW.point_of_sale_id,
        NEW.operation,
        NEW.voucher_type,
        NEW.source_type,
        NEW.source_id,
        NEW.idempotency_key,
        NEW.command_fingerprint,
        NEW.concept,
        NEW.issue_date,
        NEW.service_from,
        NEW.service_to,
        NEW.payment_due_date,
        NEW.currency_code,
        NEW.exchange_rate,
        NEW.exchange_rate_date,
        NEW.exchange_rate_source,
        NEW.net_amount,
        NEW.exempt_amount,
        NEW.non_taxed_amount,
        NEW.vat_amount,
        NEW.other_tributes_amount,
        NEW.total_amount,
        NEW.created_by,
        NEW.created_at
    ) THEN
        RAISE EXCEPTION 'fiscal voucher snapshot fields are immutable'
            USING ERRCODE = '55000';
    END IF;

    IF OLD.voucher_number IS NOT NULL
       AND NEW.voucher_number IS DISTINCT FROM OLD.voucher_number THEN
        RAISE EXCEPTION 'a reserved fiscal voucher number cannot change'
            USING ERRCODE = '55000';
    END IF;

    IF OLD.status IS DISTINCT FROM NEW.status THEN
        IF NOT (
            (OLD.status = 'queued' AND NEW.status = 'processing')
            OR
            (
                OLD.status = 'processing'
                AND NEW.status IN (
                    'queued',
                    'authorized',
                    'rejected',
                    'uncertain'
                )
            )
            OR
            (
                OLD.status = 'uncertain'
                AND NEW.status IN (
                    'processing',
                    'authorized',
                    'rejected'
                )
            )
        ) THEN
            RAISE EXCEPTION 'invalid fiscal voucher transition % -> %',
                OLD.status,
                NEW.status
                USING ERRCODE = '23514',
                      CONSTRAINT = 'fiscal_vouchers_transition';
        END IF;

        IF NEW.version <> OLD.version + 1 THEN
            RAISE EXCEPTION 'fiscal voucher transition must increment version'
                USING ERRCODE = '40001',
                      CONSTRAINT = 'fiscal_vouchers_transition_version';
        END IF;

        PERFORM fiscal.assert_voucher_snapshot_valid(NEW.org_id, NEW.id);
    END IF;

    IF OLD.status = 'processing'
       AND NEW.status = 'queued'
       AND OLD.voucher_number IS NOT NULL THEN
        RAISE EXCEPTION 'a numbered voucher cannot return to the queue'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'fiscal_vouchers_numbered_not_requeued';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.sync_voucher_reservation_status()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, fiscal
AS $function$
BEGIN
    IF NEW.status IN ('authorized', 'rejected', 'uncertain') THEN
        UPDATE fiscal.voucher_number_reservations
           SET status = NEW.status,
               updated_at = now()
         WHERE org_id = NEW.org_id
           AND voucher_id = NEW.id;
    END IF;
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.lock_voucher_series(
    requested_org_id uuid,
    requested_environment text,
    requested_point_of_sale_id uuid,
    requested_voucher_type integer
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app
AS $function$
BEGIN
    PERFORM app.assert_org_context(requested_org_id);
    PERFORM pg_advisory_xact_lock(hashtextextended(
        requested_org_id::text
        || ':'
        || requested_environment
        || ':'
        || requested_point_of_sale_id::text
        || ':'
        || requested_voucher_type::text,
        0
    ));
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.reserve_voucher_number(
    requested_org_id uuid,
    requested_voucher_id uuid,
    requested_voucher_number bigint
)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, fiscal
AS $function$
DECLARE
    voucher_record fiscal.vouchers%ROWTYPE;
BEGIN
    PERFORM app.assert_org_context(requested_org_id);

    SELECT *
      INTO voucher_record
      FROM fiscal.vouchers
     WHERE org_id = requested_org_id
       AND id = requested_voucher_id
     FOR UPDATE;

    IF NOT FOUND OR voucher_record.status NOT IN ('processing', 'uncertain') THEN
        RAISE EXCEPTION 'voucher is not available for number reservation'
            USING ERRCODE = '55000';
    END IF;

    PERFORM fiscal.lock_voucher_series(
        voucher_record.org_id,
        voucher_record.environment,
        voucher_record.point_of_sale_id,
        voucher_record.voucher_type
    );

    IF EXISTS (
        SELECT 1
          FROM fiscal.vouchers AS pending
         WHERE pending.org_id = voucher_record.org_id
           AND pending.environment = voucher_record.environment
           AND pending.point_of_sale_id = voucher_record.point_of_sale_id
           AND pending.voucher_type = voucher_record.voucher_type
           AND pending.id <> voucher_record.id
           AND pending.voucher_number IS NOT NULL
           AND pending.status IN ('processing', 'uncertain')
    ) THEN
        RAISE EXCEPTION 'voucher series has an unresolved numbered voucher'
            USING ERRCODE = '55000',
                  CONSTRAINT = 'fiscal_voucher_series_unresolved';
    END IF;

    IF voucher_record.voucher_number IS NOT NULL
       AND voucher_record.voucher_number <> requested_voucher_number THEN
        RAISE EXCEPTION 'reserved voucher number cannot change'
            USING ERRCODE = '55000';
    END IF;

    UPDATE fiscal.vouchers
       SET voucher_number = requested_voucher_number,
           version = version + 1
     WHERE org_id = requested_org_id
       AND id = requested_voucher_id;

    INSERT INTO fiscal.voucher_number_reservations (
        org_id,
        voucher_id,
        environment,
        point_of_sale_id,
        voucher_type,
        voucher_number,
        status
    )
    VALUES (
        voucher_record.org_id,
        voucher_record.id,
        voucher_record.environment,
        voucher_record.point_of_sale_id,
        voucher_record.voucher_type,
        requested_voucher_number,
        CASE
            WHEN voucher_record.status = 'uncertain' THEN 'uncertain'
            ELSE 'reserved'
        END
    )
    ON CONFLICT (org_id, voucher_id) DO NOTHING;
END
$function$;

CREATE OR REPLACE FUNCTION fiscal.lease_voucher(
    requested_org_id uuid,
    requested_worker text,
    requested_lease interval
)
RETURNS uuid
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, fiscal
AS $function$
DECLARE
    leased_voucher_id uuid;
BEGIN
    PERFORM app.assert_org_context(requested_org_id);

    IF btrim(requested_worker) = ''
       OR requested_lease <= interval '0 seconds'
       OR requested_lease > interval '15 minutes' THEN
        RAISE EXCEPTION 'invalid fiscal voucher lease'
            USING ERRCODE = '22023';
    END IF;

    SELECT voucher.id
      INTO leased_voucher_id
      FROM fiscal.vouchers AS voucher
     WHERE voucher.org_id = requested_org_id
       AND voucher.status = 'queued'
       AND (
           voucher.lease_until IS NULL
           OR voucher.lease_until < now()
       )
     ORDER BY voucher.created_at, voucher.id
     FOR UPDATE SKIP LOCKED
     LIMIT 1;

    IF leased_voucher_id IS NULL THEN
        RETURN NULL;
    END IF;

    UPDATE fiscal.vouchers
       SET status = 'processing',
           lease_owner = requested_worker,
           lease_until = now() + requested_lease,
           version = version + 1
     WHERE org_id = requested_org_id
       AND id = leased_voucher_id;

    RETURN leased_voucher_id;
END
$function$;

REVOKE ALL ON FUNCTION fiscal.reject_immutable_change() FROM PUBLIC;
REVOKE ALL ON FUNCTION fiscal.guard_operational_transition() FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.validate_accounting_posting_intent()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.assert_authorized_posting_intent(uuid, uuid)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.check_authorized_posting_intent_constraint()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.assert_voucher_snapshot_valid(uuid, uuid)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.check_voucher_snapshot_constraint()
FROM PUBLIC;
REVOKE ALL ON FUNCTION fiscal.validate_voucher_update() FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.sync_voucher_reservation_status()
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.lock_voucher_series(uuid, text, uuid, integer)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.reserve_voucher_number(uuid, uuid, bigint)
FROM PUBLIC;
REVOKE ALL
ON FUNCTION fiscal.lease_voucher(uuid, text, interval)
FROM PUBLIC;

CREATE TRIGGER fiscal_auth_requests_transition
BEFORE INSERT OR UPDATE OR DELETE ON fiscal.auth_requests
FOR EACH ROW
EXECUTE FUNCTION fiscal.guard_operational_transition();

CREATE TRIGGER fiscal_authorization_attempts_transition
BEFORE INSERT OR UPDATE OR DELETE ON fiscal.authorization_attempts
FOR EACH ROW
EXECUTE FUNCTION fiscal.guard_operational_transition();

CREATE TRIGGER fiscal_vouchers_update_guard
BEFORE INSERT OR UPDATE OR DELETE ON fiscal.vouchers
FOR EACH ROW
EXECUTE FUNCTION fiscal.validate_voucher_update();

CREATE TRIGGER fiscal_vouchers_reservation_sync
AFTER UPDATE OF status ON fiscal.vouchers
FOR EACH ROW
WHEN (OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION fiscal.sync_voucher_reservation_status();

CREATE TRIGGER fiscal_accounting_posting_intents_guard
BEFORE INSERT OR UPDATE OR DELETE ON fiscal.accounting_posting_intents
FOR EACH ROW
EXECUTE FUNCTION fiscal.validate_accounting_posting_intent();

CREATE CONSTRAINT TRIGGER fiscal_vouchers_posting_intent_valid
AFTER INSERT OR UPDATE ON fiscal.vouchers
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_authorized_posting_intent_constraint();

CREATE CONSTRAINT TRIGGER fiscal_accounting_posting_intents_valid
AFTER INSERT OR UPDATE OR DELETE ON fiscal.accounting_posting_intents
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_authorized_posting_intent_constraint();

CREATE CONSTRAINT TRIGGER fiscal_vouchers_snapshot_valid
AFTER INSERT OR UPDATE ON fiscal.vouchers
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_voucher_snapshot_constraint();

CREATE CONSTRAINT TRIGGER fiscal_voucher_snapshots_valid
AFTER INSERT OR UPDATE OR DELETE ON fiscal.voucher_snapshots
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_voucher_snapshot_constraint();

CREATE CONSTRAINT TRIGGER fiscal_voucher_lines_valid
AFTER INSERT OR UPDATE OR DELETE ON fiscal.voucher_lines
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_voucher_snapshot_constraint();

CREATE CONSTRAINT TRIGGER fiscal_voucher_taxes_valid
AFTER INSERT OR UPDATE OR DELETE ON fiscal.voucher_taxes
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_voucher_snapshot_constraint();

CREATE CONSTRAINT TRIGGER fiscal_voucher_associations_valid
AFTER INSERT OR UPDATE OR DELETE ON fiscal.voucher_associations
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fiscal.check_voucher_snapshot_constraint();

CREATE TRIGGER fiscal_auth_ticket_refs_immutable
BEFORE UPDATE OR DELETE ON fiscal.auth_ticket_refs
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_voucher_snapshots_immutable
BEFORE UPDATE OR DELETE ON fiscal.voucher_snapshots
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_voucher_lines_immutable
BEFORE UPDATE OR DELETE ON fiscal.voucher_lines
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_voucher_taxes_immutable
BEFORE UPDATE OR DELETE ON fiscal.voucher_taxes
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_voucher_associations_immutable
BEFORE UPDATE OR DELETE ON fiscal.voucher_associations
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_voucher_artifacts_immutable
BEFORE UPDATE OR DELETE ON fiscal.voucher_artifacts
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

CREATE TRIGGER fiscal_voucher_accounting_links_immutable
BEFORE UPDATE OR DELETE ON fiscal.voucher_accounting_links
FOR EACH ROW
EXECUTE FUNCTION fiscal.reject_immutable_change();

DO $rls$
DECLARE
    schema_name text;
    table_name text;
BEGIN
    FOR schema_name, table_name IN
        SELECT *
        FROM (
            VALUES
                ('fiscal', 'profiles'),
                ('fiscal_ar', 'settings'),
                ('fiscal', 'certificates'),
                ('fiscal', 'points_of_sale'),
                ('fiscal', 'auth_requests'),
                ('fiscal', 'auth_ticket_refs'),
                ('fiscal', 'vouchers'),
                ('fiscal', 'voucher_snapshots'),
                ('fiscal', 'voucher_lines'),
                ('fiscal', 'voucher_taxes'),
                ('fiscal', 'voucher_associations'),
                ('fiscal', 'authorization_attempts'),
                ('fiscal', 'voucher_number_reservations'),
                ('fiscal', 'voucher_artifacts'),
                ('fiscal', 'voucher_accounting_links'),
                ('fiscal', 'accounting_posting_intents')
        ) AS tenant_tables(schema_name, table_name)
    LOOP
        EXECUTE format(
            'ALTER TABLE %I.%I ENABLE ROW LEVEL SECURITY',
            schema_name,
            table_name
        );
        EXECUTE format(
            'ALTER TABLE %I.%I FORCE ROW LEVEL SECURITY',
            schema_name,
            table_name
        );
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON %I.%I
             USING (
                 org_id = nullif(
                     current_setting(''app.org_id'', true),
                     ''''
                 )::uuid
             )
             WITH CHECK (
                 org_id = nullif(
                     current_setting(''app.org_id'', true),
                     ''''
                 )::uuid
             )',
            schema_name,
            table_name
        );
    END LOOP;
END
$rls$;

REVOKE ALL ON ALL TABLES IN SCHEMA fiscal FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA fiscal_ar FROM PUBLIC;

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT USAGE ON SCHEMA fiscal, fiscal_ar TO pymes_backend;
        GRANT EXECUTE ON FUNCTION fiscal_ar.is_valid_cuit(text)
        TO pymes_backend;

        GRANT SELECT, INSERT, UPDATE ON
            fiscal.profiles,
            fiscal_ar.settings,
            fiscal.certificates,
            fiscal.points_of_sale,
            fiscal.auth_requests,
            fiscal.vouchers,
            fiscal.accounting_posting_intents
        TO pymes_backend;

        GRANT SELECT, INSERT ON
            fiscal.auth_ticket_refs,
            fiscal.voucher_snapshots,
            fiscal.voucher_lines,
            fiscal.voucher_taxes,
            fiscal.voucher_associations,
            fiscal.authorization_attempts,
            fiscal.voucher_artifacts,
            fiscal.voucher_accounting_links
        TO pymes_backend;

        GRANT SELECT ON fiscal.voucher_number_reservations
        TO pymes_backend;

        GRANT UPDATE ON fiscal.authorization_attempts TO pymes_backend;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_roles WHERE rolname = 'pymes_fiscal_worker'
    ) THEN
        GRANT USAGE ON SCHEMA app, fiscal, fiscal_ar
        TO pymes_fiscal_worker;
        GRANT EXECUTE ON FUNCTION app.current_org_id()
        TO pymes_fiscal_worker;
        GRANT EXECUTE ON FUNCTION app.assert_org_context(uuid)
        TO pymes_fiscal_worker;
        GRANT EXECUTE ON FUNCTION fiscal.lock_voucher_series(
            uuid,
            text,
            uuid,
            integer
        ) TO pymes_fiscal_worker;
        GRANT EXECUTE
        ON FUNCTION fiscal.reserve_voucher_number(uuid, uuid, bigint)
        TO pymes_fiscal_worker;
        GRANT EXECUTE
        ON FUNCTION fiscal.lease_voucher(uuid, text, interval)
        TO pymes_fiscal_worker;

        GRANT SELECT ON
            fiscal.profiles,
            fiscal_ar.settings,
            fiscal.certificates,
            fiscal.points_of_sale,
            fiscal.voucher_snapshots,
            fiscal.voucher_lines,
            fiscal.voucher_taxes,
            fiscal.voucher_associations,
            fiscal.auth_ticket_refs
        TO pymes_fiscal_worker;

        GRANT SELECT, INSERT, UPDATE ON
            fiscal.auth_requests,
            fiscal.authorization_attempts
        TO pymes_fiscal_worker;

        GRANT SELECT, UPDATE ON fiscal.vouchers
        TO pymes_fiscal_worker;

        GRANT SELECT, INSERT ON
            fiscal.auth_ticket_refs,
            fiscal.voucher_artifacts,
            fiscal.accounting_posting_intents
        TO pymes_fiscal_worker;

        GRANT SELECT ON fiscal.voucher_number_reservations
        TO pymes_fiscal_worker;
    END IF;

    IF EXISTS (
        SELECT 1
          FROM pg_roles
         WHERE rolname = 'pymes_fiscal_accounting_worker'
    ) THEN
        GRANT USAGE ON SCHEMA app, accounting, fiscal
        TO pymes_fiscal_accounting_worker;

        GRANT SELECT ON
            fiscal.accounting_posting_intents,
            fiscal.vouchers,
            fiscal.points_of_sale,
            fiscal.voucher_snapshots,
            fiscal.voucher_associations,
            fiscal.voucher_accounting_links
        TO pymes_fiscal_accounting_worker;

        GRANT UPDATE ON fiscal.accounting_posting_intents
        TO pymes_fiscal_accounting_worker;

        GRANT INSERT ON fiscal.voucher_accounting_links
        TO pymes_fiscal_accounting_worker;
    END IF;
END
$grant$;
