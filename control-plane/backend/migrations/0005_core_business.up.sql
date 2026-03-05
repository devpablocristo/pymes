CREATE TABLE IF NOT EXISTS customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL DEFAULT 'person' CHECK (type IN ('person', 'company')),
    name text NOT NULL,
    tax_id text,
    email text,
    phone text,
    address jsonb NOT NULL DEFAULT '{}'::jsonb,
    notes text NOT NULL DEFAULT '',
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_org_tax_unique
    ON customers(org_id, tax_id)
    WHERE deleted_at IS NULL AND tax_id IS NOT NULL AND tax_id != '';
CREATE INDEX IF NOT EXISTS idx_customers_org ON customers(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customers_org_name ON customers(org_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customers_org_email ON customers(org_id, email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customers_org_tax ON customers(org_id, tax_id) WHERE deleted_at IS NULL AND tax_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_customers_tags ON customers USING GIN(tags) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS suppliers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    tax_id text,
    email text,
    phone text,
    address jsonb NOT NULL DEFAULT '{}'::jsonb,
    contact_name text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_suppliers_org_tax_unique
    ON suppliers(org_id, tax_id)
    WHERE deleted_at IS NULL AND tax_id IS NOT NULL AND tax_id != '';
CREATE INDEX IF NOT EXISTS idx_suppliers_org ON suppliers(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_suppliers_org_name ON suppliers(org_id, name) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL DEFAULT 'product' CHECK (type IN ('product', 'service')),
    sku text,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    unit text NOT NULL DEFAULT 'unit',
    price numeric(15,2) NOT NULL DEFAULT 0,
    cost_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2),
    track_stock boolean NOT NULL DEFAULT true,
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_products_org_sku
    ON products(org_id, sku)
    WHERE deleted_at IS NULL AND sku IS NOT NULL AND sku != '';
CREATE INDEX IF NOT EXISTS idx_products_org ON products(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_products_org_name ON products(org_id, name) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS stock_levels (
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    quantity numeric(15,2) NOT NULL DEFAULT 0,
    min_quantity numeric(15,2) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, product_id)
);

CREATE TABLE IF NOT EXISTS stock_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id),
    type text NOT NULL CHECK (type IN ('in', 'out', 'adjustment')),
    quantity numeric(15,2) NOT NULL,
    reason text NOT NULL DEFAULT '',
    reference_id uuid,
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_stock_movements_org ON stock_movements(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_movements_product ON stock_movements(org_id, product_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_low ON stock_levels(org_id) WHERE quantity <= min_quantity AND min_quantity > 0;

CREATE TABLE IF NOT EXISTS quotes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    customer_id uuid REFERENCES customers(id),
    customer_name text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'sent', 'accepted', 'rejected', 'expired')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    notes text NOT NULL DEFAULT '',
    valid_until timestamptz,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, number)
);

CREATE TABLE IF NOT EXISTS quote_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id uuid NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id),
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_quotes_org ON quotes(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_quotes_org_status ON quotes(org_id, status);
CREATE INDEX IF NOT EXISTS idx_quotes_customer ON quotes(customer_id) WHERE customer_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS sales (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    customer_id uuid REFERENCES customers(id),
    customer_name text NOT NULL DEFAULT '',
    quote_id uuid REFERENCES quotes(id),
    status text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'voided')),
    payment_method text NOT NULL DEFAULT 'cash' CHECK (payment_method IN ('cash', 'card', 'transfer', 'other')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, number)
);

CREATE TABLE IF NOT EXISTS sale_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id uuid NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id),
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    cost_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_sales_org ON sales(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sales_org_date ON sales(org_id, created_at) WHERE status = 'completed';
CREATE INDEX IF NOT EXISTS idx_sales_customer ON sales(customer_id) WHERE customer_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS cash_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('income', 'expense')),
    amount numeric(15,2) NOT NULL,
    currency text NOT NULL DEFAULT 'ARS',
    category text NOT NULL DEFAULT 'other',
    description text NOT NULL DEFAULT '',
    payment_method text NOT NULL DEFAULT 'cash',
    reference_type text NOT NULL DEFAULT 'manual',
    reference_id uuid,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cash_movements_org ON cash_movements(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cash_movements_org_type ON cash_movements(org_id, type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cash_movements_org_date ON cash_movements(org_id, created_at);
