CREATE TABLE fiscal_ar.catalog_versions (
    catalog text NOT NULL CHECK (btrim(catalog) <> ''),
    version integer NOT NULL CHECK (version > 0),
    source_url text NOT NULL CHECK (btrim(source_url) <> ''),
    source_checksum char(64) NOT NULL
        CHECK (source_checksum ~ '^[0-9a-f]{64}$'),
    effective_from date NOT NULL,
    effective_until date,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (catalog, version),
    CONSTRAINT fiscal_ar_catalog_versions_version_unique UNIQUE (version),
    CHECK (
        effective_until IS NULL
        OR effective_until >= effective_from
    )
);

CREATE TABLE fiscal_ar.voucher_types (
    catalog_version integer NOT NULL,
    code integer NOT NULL CHECK (code > 0),
    operation text NOT NULL
        CHECK (operation IN ('invoice', 'credit_note', 'debit_note')),
    letter char(1) NOT NULL CHECK (letter IN ('A', 'B', 'C')),
    description text NOT NULL CHECK (btrim(description) <> ''),
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY (catalog_version, code),
    CONSTRAINT fiscal_ar_voucher_types_catalog_fk
        FOREIGN KEY (catalog_version)
        REFERENCES fiscal_ar.catalog_versions(version)
        ON DELETE RESTRICT
);

CREATE TABLE fiscal_ar.document_types (
    catalog_version integer NOT NULL,
    code integer NOT NULL CHECK (code > 0),
    description text NOT NULL CHECK (btrim(description) <> ''),
    validation_kind text NOT NULL
        CHECK (validation_kind IN ('cuit', 'dni', 'consumer_final')),
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY (catalog_version, code),
    CONSTRAINT fiscal_ar_document_types_catalog_fk
        FOREIGN KEY (catalog_version)
        REFERENCES fiscal_ar.catalog_versions(version)
        ON DELETE RESTRICT
);

CREATE TABLE fiscal_ar.vat_rates (
    catalog_version integer NOT NULL,
    code integer NOT NULL CHECK (code > 0),
    rate numeric(9, 6) NOT NULL CHECK (rate >= 0),
    description text NOT NULL CHECK (btrim(description) <> ''),
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY (catalog_version, code),
    CONSTRAINT fiscal_ar_vat_rates_catalog_fk
        FOREIGN KEY (catalog_version)
        REFERENCES fiscal_ar.catalog_versions(version)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_ar_vat_rates_rate_unique
        UNIQUE (catalog_version, rate)
);

CREATE TABLE fiscal_ar.tax_conditions (
    catalog_version integer NOT NULL,
    code integer NOT NULL CHECK (code > 0),
    internal_code text NOT NULL CHECK (btrim(internal_code) <> ''),
    description text NOT NULL CHECK (btrim(description) <> ''),
    enabled boolean NOT NULL DEFAULT true,
    PRIMARY KEY (catalog_version, code),
    CONSTRAINT fiscal_ar_tax_conditions_catalog_fk
        FOREIGN KEY (catalog_version)
        REFERENCES fiscal_ar.catalog_versions(version)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_ar_tax_conditions_internal_unique
        UNIQUE (catalog_version, internal_code)
);

CREATE TABLE fiscal_ar.concepts (
    catalog_version integer NOT NULL,
    code integer NOT NULL CHECK (code BETWEEN 1 AND 3),
    internal_code text NOT NULL
        CHECK (internal_code IN ('products', 'services', 'mixed')),
    description text NOT NULL CHECK (btrim(description) <> ''),
    PRIMARY KEY (catalog_version, code),
    CONSTRAINT fiscal_ar_concepts_catalog_fk
        FOREIGN KEY (catalog_version)
        REFERENCES fiscal_ar.catalog_versions(version)
        ON DELETE RESTRICT,
    CONSTRAINT fiscal_ar_concepts_internal_unique
        UNIQUE (catalog_version, internal_code)
);

INSERT INTO fiscal_ar.catalog_versions (
    catalog,
    version,
    source_url,
    source_checksum,
    effective_from
)
VALUES (
    'wsfev1',
    1,
    'https://www.arca.gob.ar/fe/ayuda/webservice.asp',
    encode(digest('arca-wsfev1-catalog-v1', 'sha256'), 'hex'),
    DATE '2025-04-15'
);

INSERT INTO fiscal_ar.voucher_types (
    catalog_version,
    code,
    operation,
    letter,
    description
)
VALUES
    (1, 1, 'invoice', 'A', 'Factura A'),
    (1, 2, 'debit_note', 'A', 'Nota de débito A'),
    (1, 3, 'credit_note', 'A', 'Nota de crédito A'),
    (1, 6, 'invoice', 'B', 'Factura B'),
    (1, 7, 'debit_note', 'B', 'Nota de débito B'),
    (1, 8, 'credit_note', 'B', 'Nota de crédito B'),
    (1, 11, 'invoice', 'C', 'Factura C'),
    (1, 12, 'debit_note', 'C', 'Nota de débito C'),
    (1, 13, 'credit_note', 'C', 'Nota de crédito C');

INSERT INTO fiscal_ar.document_types (
    catalog_version,
    code,
    description,
    validation_kind
)
VALUES
    (1, 80, 'CUIT', 'cuit'),
    (1, 86, 'CUIL', 'cuit'),
    (1, 96, 'DNI', 'dni'),
    (1, 99, 'Consumidor final', 'consumer_final');

INSERT INTO fiscal_ar.vat_rates (
    catalog_version,
    code,
    rate,
    description
)
VALUES
    (1, 3, 0.000000, 'IVA 0%'),
    (1, 9, 2.500000, 'IVA 2,5%'),
    (1, 8, 5.000000, 'IVA 5%'),
    (1, 4, 10.500000, 'IVA 10,5%'),
    (1, 5, 21.000000, 'IVA 21%'),
    (1, 6, 27.000000, 'IVA 27%');

INSERT INTO fiscal_ar.tax_conditions (
    catalog_version,
    code,
    internal_code,
    description
)
VALUES
    (1, 1, 'responsable_inscripto', 'IVA Responsable Inscripto'),
    (1, 4, 'exento', 'IVA Sujeto Exento'),
    (1, 5, 'consumidor_final', 'Consumidor Final'),
    (1, 6, 'monotributo', 'Responsable Monotributo'),
    (1, 7, 'no_categorizado', 'Sujeto No Categorizado'),
    (1, 8, 'proveedor_exterior', 'Proveedor del Exterior'),
    (1, 9, 'cliente_exterior', 'Cliente del Exterior'),
    (1, 10, 'iva_liberado', 'IVA Liberado - Ley 19.640'),
    (1, 13, 'monotributo_social', 'Monotributista Social'),
    (1, 15, 'iva_no_alcanzado', 'IVA No Alcanzado'),
    (
        1,
        16,
        'monotributo_promovido',
        'Monotributo Trabajador Independiente Promovido'
    );

INSERT INTO fiscal_ar.concepts (
    catalog_version,
    code,
    internal_code,
    description
)
VALUES
    (1, 1, 'products', 'Productos'),
    (1, 2, 'services', 'Servicios'),
    (1, 3, 'mixed', 'Productos y servicios');

INSERT INTO accounting.chart_templates (
    code,
    version,
    country_code,
    functional_currency,
    name,
    source,
    source_checksum
)
VALUES (
    'ar-pyme',
    1,
    'AR',
    'ARS',
    'Plan de cuentas PyME Argentina',
    'Pymes v2 ar-pyme',
    encode(digest('pymes-v2-ar-pyme-v1', 'sha256'), 'hex')
);

INSERT INTO accounting.chart_template_accounts (
    template_code,
    template_version,
    code,
    name,
    account_class,
    parent_code,
    normal_balance,
    monetary_class,
    posting_allowed,
    display_order
)
VALUES
    ('ar-pyme', 1, '1', 'Activo', 'asset', NULL, 'debit', 'not_applicable', false, 100),
    ('ar-pyme', 1, '1.1', 'Disponibilidades', 'asset', '1', 'debit', 'not_applicable', false, 110),
    ('ar-pyme', 1, '1.1.01', 'Caja', 'asset', '1.1', 'debit', 'monetary', true, 111),
    ('ar-pyme', 1, '1.1.02', 'Bancos', 'asset', '1.1', 'debit', 'monetary', true, 112),
    ('ar-pyme', 1, '1.1.03', 'Tarjetas a cobrar', 'asset', '1.1', 'debit', 'monetary', true, 113),
    ('ar-pyme', 1, '1.1.04', 'Billeteras a cobrar', 'asset', '1.1', 'debit', 'monetary', true, 114),
    ('ar-pyme', 1, '1.1.05', 'Valores a depositar', 'asset', '1.1', 'debit', 'monetary', true, 115),
    ('ar-pyme', 1, '1.2', 'Créditos', 'asset', '1', 'debit', 'not_applicable', false, 120),
    ('ar-pyme', 1, '1.2.01', 'Clientes', 'asset', '1.2', 'debit', 'monetary', true, 121),
    ('ar-pyme', 1, '1.2.02', 'IVA crédito fiscal', 'asset', '1.2', 'debit', 'monetary', true, 122),
    ('ar-pyme', 1, '1.2.03', 'Retenciones a favor', 'asset', '1.2', 'debit', 'monetary', true, 123),
    ('ar-pyme', 1, '1.2.04', 'Percepciones a favor', 'asset', '1.2', 'debit', 'monetary', true, 124),
    ('ar-pyme', 1, '1.2.05', 'Anticipos a proveedores', 'asset', '1.2', 'debit', 'monetary', true, 125),
    ('ar-pyme', 1, '1.2.10', 'IVA crédito fiscal 21%', 'asset', '1.2', 'debit', 'monetary', true, 126),
    ('ar-pyme', 1, '1.2.11', 'IVA crédito fiscal 10,5%', 'asset', '1.2', 'debit', 'monetary', true, 127),
    ('ar-pyme', 1, '1.2.12', 'IVA crédito fiscal 27%', 'asset', '1.2', 'debit', 'monetary', true, 128),
    ('ar-pyme', 1, '1.2.13', 'IVA crédito fiscal 5%', 'asset', '1.2', 'debit', 'monetary', true, 129),
    ('ar-pyme', 1, '1.2.14', 'IVA crédito fiscal 2,5%', 'asset', '1.2', 'debit', 'monetary', true, 130),
    ('ar-pyme', 1, '1.3', 'Bienes de cambio', 'asset', '1', 'debit', 'not_applicable', false, 130),
    ('ar-pyme', 1, '1.3.01', 'Mercaderías', 'asset', '1.3', 'debit', 'non_monetary', true, 131),
    ('ar-pyme', 1, '1.4', 'Bienes de uso', 'asset', '1', 'debit', 'not_applicable', false, 140),
    ('ar-pyme', 1, '1.4.01', 'Bienes de uso', 'asset', '1.4', 'debit', 'non_monetary', true, 141),
    ('ar-pyme', 1, '2', 'Pasivo', 'liability', NULL, 'credit', 'not_applicable', false, 200),
    ('ar-pyme', 1, '2.1', 'Deudas comerciales', 'liability', '2', 'credit', 'not_applicable', false, 210),
    ('ar-pyme', 1, '2.1.01', 'Proveedores', 'liability', '2.1', 'credit', 'monetary', true, 211),
    ('ar-pyme', 1, '2.1.02', 'Anticipos de clientes', 'liability', '2.1', 'credit', 'monetary', true, 212),
    ('ar-pyme', 1, '2.1.03', 'Notas de crédito a clientes', 'liability', '2.1', 'credit', 'monetary', true, 213),
    ('ar-pyme', 1, '2.2', 'Deudas fiscales', 'liability', '2', 'credit', 'not_applicable', false, 220),
    ('ar-pyme', 1, '2.2.01', 'IVA débito fiscal', 'liability', '2.2', 'credit', 'monetary', true, 221),
    ('ar-pyme', 1, '2.2.02', 'Impuestos por pagar', 'liability', '2.2', 'credit', 'monetary', true, 222),
    ('ar-pyme', 1, '2.2.10', 'IVA débito fiscal 21%', 'liability', '2.2', 'credit', 'monetary', true, 223),
    ('ar-pyme', 1, '2.2.11', 'IVA débito fiscal 10,5%', 'liability', '2.2', 'credit', 'monetary', true, 224),
    ('ar-pyme', 1, '2.2.12', 'IVA débito fiscal 27%', 'liability', '2.2', 'credit', 'monetary', true, 225),
    ('ar-pyme', 1, '2.2.13', 'IVA débito fiscal 5%', 'liability', '2.2', 'credit', 'monetary', true, 226),
    ('ar-pyme', 1, '2.2.14', 'IVA débito fiscal 2,5%', 'liability', '2.2', 'credit', 'monetary', true, 227),
    ('ar-pyme', 1, '3', 'Patrimonio neto', 'equity', NULL, 'credit', 'not_applicable', false, 300),
    ('ar-pyme', 1, '3.1', 'Capital', 'equity', '3', 'credit', 'non_monetary', true, 310),
    ('ar-pyme', 1, '3.2', 'Ajuste de capital', 'equity', '3', 'credit', 'non_monetary', true, 320),
    ('ar-pyme', 1, '3.3', 'Resultados no asignados', 'equity', '3', 'credit', 'non_monetary', true, 330),
    ('ar-pyme', 1, '3.4', 'Resultado del ejercicio', 'equity', '3', 'credit', 'non_monetary', true, 340),
    ('ar-pyme', 1, '4', 'Ingresos', 'revenue', NULL, 'credit', 'not_applicable', false, 400),
    ('ar-pyme', 1, '4.1', 'Ventas de mercaderías', 'revenue', '4', 'credit', 'non_monetary', true, 410),
    ('ar-pyme', 1, '4.2', 'Ingresos por servicios', 'revenue', '4', 'credit', 'non_monetary', true, 420),
    ('ar-pyme', 1, '4.3', 'Diferencias de cambio ganadas', 'revenue', '4', 'credit', 'monetary', true, 430),
    ('ar-pyme', 1, '5', 'Costos', 'cost', NULL, 'debit', 'not_applicable', false, 500),
    ('ar-pyme', 1, '5.1', 'Costo de mercaderías vendidas', 'cost', '5', 'debit', 'non_monetary', true, 510),
    ('ar-pyme', 1, '6', 'Gastos', 'expense', NULL, 'debit', 'not_applicable', false, 600),
    ('ar-pyme', 1, '6.1', 'Gastos generales', 'expense', '6', 'debit', 'non_monetary', true, 610),
    ('ar-pyme', 1, '6.2', 'Comisiones de medios de pago', 'expense', '6', 'debit', 'monetary', true, 620),
    ('ar-pyme', 1, '6.3', 'Diferencias de cambio perdidas', 'expense', '6', 'debit', 'monetary', true, 630),
    ('ar-pyme', 1, '6.4', 'Diferencias de redondeo', 'expense', '6', 'debit', 'monetary', true, 640),
    ('ar-pyme', 1, '6.5', 'RECPAM', 'expense', '6', 'debit', 'monetary', true, 650);

INSERT INTO accounting.chart_template_mappings (
    template_code,
    template_version,
    mapping_key,
    account_code,
    description
)
VALUES
    ('ar-pyme', 1, 'cash', '1.1.01', 'Caja predeterminada'),
    ('ar-pyme', 1, 'bank', '1.1.02', 'Banco predeterminado'),
    ('ar-pyme', 1, 'card_clearing', '1.1.03', 'Clearing de tarjetas'),
    ('ar-pyme', 1, 'wallet_clearing', '1.1.04', 'Clearing de billeteras'),
    ('ar-pyme', 1, 'checks_clearing', '1.1.05', 'Clearing de valores'),
    ('ar-pyme', 1, 'receivable', '1.2.01', 'Deudores por ventas'),
    ('ar-pyme', 1, 'withholdings_receivable', '1.2.03', 'Retenciones a favor'),
    ('ar-pyme', 1, 'perceptions_receivable', '1.2.04', 'Percepciones a favor'),
    ('ar-pyme', 1, 'vat_credit_21', '1.2.10', 'IVA crédito fiscal 21%'),
    ('ar-pyme', 1, 'vat_credit_105', '1.2.11', 'IVA crédito fiscal 10,5%'),
    ('ar-pyme', 1, 'vat_credit_27', '1.2.12', 'IVA crédito fiscal 27%'),
    ('ar-pyme', 1, 'vat_credit_5', '1.2.13', 'IVA crédito fiscal 5%'),
    ('ar-pyme', 1, 'vat_credit_25', '1.2.14', 'IVA crédito fiscal 2,5%'),
    ('ar-pyme', 1, 'inventory', '1.3.01', 'Mercaderías'),
    ('ar-pyme', 1, 'payable', '2.1.01', 'Proveedores'),
    ('ar-pyme', 1, 'credit_note_payable', '2.1.03', 'Notas de crédito a clientes'),
    ('ar-pyme', 1, 'vat_payable_21', '2.2.10', 'IVA débito fiscal 21%'),
    ('ar-pyme', 1, 'vat_payable_105', '2.2.11', 'IVA débito fiscal 10,5%'),
    ('ar-pyme', 1, 'vat_payable_27', '2.2.12', 'IVA débito fiscal 27%'),
    ('ar-pyme', 1, 'vat_payable_5', '2.2.13', 'IVA débito fiscal 5%'),
    ('ar-pyme', 1, 'vat_payable_25', '2.2.14', 'IVA débito fiscal 2,5%'),
    ('ar-pyme', 1, 'taxes_payable', '2.2.02', 'Impuestos por pagar'),
    ('ar-pyme', 1, 'capital', '3.1', 'Capital'),
    ('ar-pyme', 1, 'retained_earnings', '3.3', 'Resultados no asignados'),
    ('ar-pyme', 1, 'current_result', '3.4', 'Resultado del ejercicio'),
    ('ar-pyme', 1, 'revenue', '4.1', 'Ventas'),
    ('ar-pyme', 1, 'fx_gain', '4.3', 'Diferencias de cambio ganadas'),
    ('ar-pyme', 1, 'cogs', '5.1', 'Costo de mercaderías vendidas'),
    ('ar-pyme', 1, 'purchase_expense', '6.1', 'Compras de servicios y gastos'),
    ('ar-pyme', 1, 'fx_loss', '6.3', 'Diferencias de cambio perdidas'),
    ('ar-pyme', 1, 'rounding_difference', '6.4', 'Diferencias de redondeo'),
    ('ar-pyme', 1, 'recpam', '6.5', 'RECPAM'),

    -- Legacy aliases retained only so existing configured organizations can
    -- migrate without losing their functional links.
    ('ar-pyme', 1, 'checks_receivable', '1.1.05', 'Valores a depositar'),
    ('ar-pyme', 1, 'accounts_receivable', '1.2.01', 'Clientes'),
    ('ar-pyme', 1, 'vat_input', '1.2.02', 'IVA crédito fiscal'),
    ('ar-pyme', 1, 'supplier_advances', '1.2.05', 'Anticipos a proveedores'),
    ('ar-pyme', 1, 'accounts_payable', '2.1.01', 'Proveedores'),
    ('ar-pyme', 1, 'customer_advances', '2.1.02', 'Anticipos de clientes'),
    ('ar-pyme', 1, 'vat_output', '2.2.01', 'IVA débito fiscal'),
    ('ar-pyme', 1, 'current_year_result', '3.4', 'Resultado del ejercicio'),
    ('ar-pyme', 1, 'sales_goods', '4.1', 'Ventas de mercaderías'),
    ('ar-pyme', 1, 'sales_services', '4.2', 'Ingresos por servicios'),
    ('ar-pyme', 1, 'general_expense', '6.1', 'Gastos generales'),
    ('ar-pyme', 1, 'payment_fees', '6.2', 'Comisiones'),
    ('ar-pyme', 1, 'rounding', '6.4', 'Diferencias de redondeo');

CREATE OR REPLACE FUNCTION accounting.install_chart_template(
    requested_org_id uuid,
    requested_template_code text,
    requested_template_version integer
)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    template_record accounting.chart_templates%ROWTYPE;
    template_account accounting.chart_template_accounts%ROWTYPE;
    inserted_accounts integer := 0;
BEGIN
    PERFORM app.assert_org_context(requested_org_id);

    SELECT *
      INTO template_record
      FROM accounting.chart_templates
     WHERE code = requested_template_code
       AND version = requested_template_version;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'accounting chart template does not exist'
            USING ERRCODE = '22023';
    END IF;

    INSERT INTO accounting.organization_settings (
        org_id,
        country_code,
        functional_currency,
        chart_template_code,
        chart_template_version
    )
    VALUES (
        requested_org_id,
        template_record.country_code,
        template_record.functional_currency,
        template_record.code,
        template_record.version
    )
    ON CONFLICT (org_id) DO UPDATE
       SET chart_template_code = excluded.chart_template_code,
           chart_template_version = excluded.chart_template_version,
           updated_at = now(),
           version = accounting.organization_settings.version + 1
     WHERE accounting.organization_settings.chart_template_code IS NULL;

    IF NOT EXISTS (
        SELECT 1
          FROM accounting.organization_settings AS setting
         WHERE setting.org_id = requested_org_id
           AND setting.chart_template_code = template_record.code
           AND setting.chart_template_version = template_record.version
    ) THEN
        RAISE EXCEPTION 'organization already uses a different chart template'
            USING ERRCODE = '23505';
    END IF;

    FOR template_account IN
        SELECT account.*
          FROM accounting.chart_template_accounts AS account
         WHERE account.template_code = template_record.code
           AND account.template_version = template_record.version
         ORDER BY
            array_length(string_to_array(account.code, '.'), 1),
            account.display_order,
            account.code
    LOOP
        INSERT INTO accounting.accounts (
            org_id,
            code,
            name,
            account_class,
            parent_id,
            normal_balance,
            monetary_class,
            posting_allowed
        )
        SELECT
            requested_org_id,
            template_account.code,
            template_account.name,
            template_account.account_class,
            parent.id,
            template_account.normal_balance,
            template_account.monetary_class,
            template_account.posting_allowed
        FROM (VALUES (1)) AS seed(ignored)
        LEFT JOIN accounting.accounts AS parent
          ON parent.org_id = requested_org_id
         AND parent.code = template_account.parent_code
        ON CONFLICT (org_id, code) DO NOTHING;

        IF FOUND THEN
            inserted_accounts := inserted_accounts + 1;
        END IF;
    END LOOP;

    INSERT INTO accounting.account_mappings (
        org_id,
        mapping_key,
        account_id,
        description
    )
    SELECT
        requested_org_id,
        mapping.mapping_key,
        account.id,
        mapping.description
    FROM accounting.chart_template_mappings AS mapping
    JOIN accounting.accounts AS account
      ON account.org_id = requested_org_id
     AND account.code = mapping.account_code
    WHERE mapping.template_code = template_record.code
      AND mapping.template_version = template_record.version
    ON CONFLICT (org_id, mapping_key) DO NOTHING;

    RETURN inserted_accounts;
END
$function$;

REVOKE ALL
ON FUNCTION accounting.install_chart_template(uuid, text, integer)
FROM PUBLIC;

REVOKE ALL ON ALL TABLES IN SCHEMA fiscal_ar FROM PUBLIC;

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT SELECT ON
            fiscal_ar.catalog_versions,
            fiscal_ar.voucher_types,
            fiscal_ar.document_types,
            fiscal_ar.vat_rates,
            fiscal_ar.tax_conditions,
            fiscal_ar.concepts
        TO pymes_backend;
        GRANT EXECUTE
        ON FUNCTION accounting.install_chart_template(uuid, text, integer)
        TO pymes_backend;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_roles WHERE rolname = 'pymes_fiscal_worker'
    ) THEN
        GRANT SELECT ON
            fiscal_ar.catalog_versions,
            fiscal_ar.voucher_types,
            fiscal_ar.document_types,
            fiscal_ar.vat_rates,
            fiscal_ar.tax_conditions,
            fiscal_ar.concepts
        TO pymes_fiscal_worker;
    END IF;
END
$grant$;
