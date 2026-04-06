CREATE TABLE IF NOT EXISTS catalog_services (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    code text,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    category_code text NOT NULL DEFAULT '',
    sale_price numeric(15,2) NOT NULL DEFAULT 0,
    cost_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2),
    currency text NOT NULL DEFAULT 'ARS',
    default_duration_minutes integer,
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_catalog_services_org_code
    ON catalog_services(org_id, code)
    WHERE deleted_at IS NULL AND code IS NOT NULL AND code != '';
CREATE INDEX IF NOT EXISTS idx_catalog_services_org ON catalog_services(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_catalog_services_org_name ON catalog_services(org_id, name) WHERE deleted_at IS NULL;

ALTER TABLE sale_items
    ADD COLUMN IF NOT EXISTS service_id uuid REFERENCES catalog_services(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_sale_items_service_id ON sale_items(service_id) WHERE service_id IS NOT NULL;

ALTER TABLE quote_items
    ADD COLUMN IF NOT EXISTS service_id uuid REFERENCES catalog_services(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_quote_items_service_id ON quote_items(service_id) WHERE service_id IS NOT NULL;

ALTER TABLE purchase_items
    ADD COLUMN IF NOT EXISTS service_id uuid REFERENCES catalog_services(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_purchase_items_service_id ON purchase_items(service_id) WHERE service_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS service_price_list_items (
    price_list_id uuid NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    service_id uuid NOT NULL REFERENCES catalog_services(id) ON DELETE CASCADE,
    price numeric(15,2) NOT NULL DEFAULT 0,
    PRIMARY KEY (price_list_id, service_id)
);

INSERT INTO catalog_services (
    id,
    org_id,
    code,
    name,
    description,
    category_code,
    sale_price,
    cost_price,
    tax_rate,
    currency,
    default_duration_minutes,
    tags,
    metadata,
    created_at,
    updated_at,
    deleted_at
)
SELECT
    p.id,
    p.org_id,
    NULLIF(p.sku, ''),
    p.name,
    p.description,
    COALESCE(p.metadata->>'category_code', ''),
    p.price,
    p.cost_price,
    p.tax_rate,
    COALESCE(p.price_currency, 'ARS'),
    NULL,
    p.tags,
    p.metadata,
    p.created_at,
    p.updated_at,
    p.deleted_at
FROM products p
WHERE p.type = 'service'
ON CONFLICT (id) DO NOTHING;

UPDATE sale_items si
SET service_id = si.product_id
FROM products p
WHERE si.product_id = p.id
  AND p.type = 'service'
  AND si.service_id IS NULL;

UPDATE quote_items qi
SET service_id = qi.product_id
FROM products p
WHERE qi.product_id = p.id
  AND p.type = 'service'
  AND qi.service_id IS NULL;

UPDATE purchase_items pi
SET service_id = pi.product_id
FROM products p
WHERE pi.product_id = p.id
  AND p.type = 'service'
  AND pi.service_id IS NULL;

INSERT INTO service_price_list_items (price_list_id, service_id, price)
SELECT pli.price_list_id, pli.product_id, pli.price
FROM price_list_items pli
JOIN products p ON p.id = pli.product_id
WHERE p.type = 'service'
ON CONFLICT (price_list_id, service_id) DO NOTHING;
