CREATE TABLE IF NOT EXISTS purchases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    supplier_id uuid REFERENCES suppliers(id),
    supplier_name text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'received', 'partial', 'voided')),
    payment_status text NOT NULL DEFAULT 'pending' CHECK (payment_status IN ('pending', 'partial', 'paid')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    notes text NOT NULL DEFAULT '',
    received_at timestamptz,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, number)
);

CREATE TABLE IF NOT EXISTS purchase_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_id uuid NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id),
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_cost numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_purchases_org ON purchases(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_purchases_supplier ON purchases(supplier_id) WHERE supplier_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_purchases_org_status ON purchases(org_id, status);

CREATE TABLE IF NOT EXISTS accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('receivable', 'payable')),
    entity_type text NOT NULL CHECK (entity_type IN ('customer', 'supplier')),
    entity_id uuid NOT NULL,
    entity_name text NOT NULL DEFAULT '',
    balance numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    credit_limit numeric(15,2) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_accounts_org ON accounts(org_id, type);
CREATE INDEX IF NOT EXISTS idx_accounts_entity ON accounts(org_id, entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_accounts_balance ON accounts(org_id) WHERE balance != 0;

CREATE TABLE IF NOT EXISTS account_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('charge', 'payment', 'adjustment', 'void')),
    amount numeric(15,2) NOT NULL,
    balance numeric(15,2) NOT NULL,
    description text NOT NULL DEFAULT '',
    reference_type text NOT NULL DEFAULT 'manual',
    reference_id uuid,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_account_movements_account ON account_movements(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_account_movements_org ON account_movements(org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    reference_type text NOT NULL CHECK (reference_type IN ('sale', 'purchase')),
    reference_id uuid NOT NULL,
    method text NOT NULL DEFAULT 'cash' CHECK (method IN ('cash', 'card', 'transfer', 'check', 'other', 'credit_note')),
    amount numeric(15,2) NOT NULL,
    notes text NOT NULL DEFAULT '',
    received_at timestamptz NOT NULL DEFAULT now(),
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payments_reference ON payments(org_id, reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_payments_org ON payments(org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS returns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    sale_id uuid NOT NULL REFERENCES sales(id),
    reason text NOT NULL DEFAULT 'other' CHECK (reason IN ('defective', 'wrong_item', 'changed_mind', 'other')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    refund_method text NOT NULL DEFAULT 'cash' CHECK (refund_method IN ('cash', 'credit_note', 'original_method')),
    status text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'voided')),
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, number)
);

CREATE TABLE IF NOT EXISTS return_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id uuid NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
    sale_item_id uuid NOT NULL REFERENCES sale_items(id),
    product_id uuid REFERENCES products(id),
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS credit_notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    customer_id uuid NOT NULL REFERENCES customers(id),
    return_id uuid NOT NULL REFERENCES returns(id),
    amount numeric(15,2) NOT NULL,
    used_amount numeric(15,2) NOT NULL DEFAULT 0,
    balance numeric(15,2) NOT NULL,
    expires_at timestamptz,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'used', 'expired', 'voided')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, number)
);

CREATE INDEX IF NOT EXISTS idx_returns_org ON returns(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_returns_sale ON returns(sale_id);
CREATE INDEX IF NOT EXISTS idx_credit_notes_customer ON credit_notes(org_id, customer_id) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS price_lists (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_default boolean NOT NULL DEFAULT false,
    markup numeric(5,2) NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, name)
);

CREATE TABLE IF NOT EXISTS price_list_items (
    price_list_id uuid NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    price numeric(15,2) NOT NULL,
    PRIMARY KEY (price_list_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_price_lists_org ON price_lists(org_id) WHERE is_active = true;

CREATE TABLE IF NOT EXISTS recurring_expenses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    description text NOT NULL,
    amount numeric(15,2) NOT NULL,
    currency text NOT NULL DEFAULT 'ARS',
    category text NOT NULL DEFAULT 'other',
    payment_method text NOT NULL DEFAULT 'transfer'
        CHECK (payment_method IN ('cash', 'card', 'transfer', 'debit', 'check', 'other')),
    frequency text NOT NULL DEFAULT 'monthly'
        CHECK (frequency IN ('weekly', 'biweekly', 'monthly', 'quarterly', 'yearly')),
    day_of_month int NOT NULL DEFAULT 1 CHECK (day_of_month BETWEEN 1 AND 28),
    supplier_id uuid REFERENCES suppliers(id),
    is_active boolean NOT NULL DEFAULT true,
    next_due_date date NOT NULL,
    last_paid_date date,
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_recurring_expenses_org ON recurring_expenses(org_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_recurring_expenses_due ON recurring_expenses(next_due_date) WHERE is_active = true;

CREATE TABLE IF NOT EXISTS appointments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    customer_id uuid REFERENCES customers(id),
    customer_name text NOT NULL DEFAULT '',
    customer_phone text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'scheduled'
        CHECK (status IN ('scheduled', 'confirmed', 'in_progress', 'completed', 'cancelled', 'no_show')),
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    CHECK (end_at > start_at),
    duration int NOT NULL DEFAULT 60,
    location text NOT NULL DEFAULT '',
    assigned_to text NOT NULL DEFAULT '',
    color text NOT NULL DEFAULT '#3B82F6',
    notes text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_appointments_org_date ON appointments(org_id, start_at);
CREATE INDEX IF NOT EXISTS idx_appointments_org_status ON appointments(org_id, status, start_at);
CREATE INDEX IF NOT EXISTS idx_appointments_customer ON appointments(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_appointments_assigned ON appointments(org_id, assigned_to, start_at) WHERE assigned_to != '';

CREATE TABLE IF NOT EXISTS appointment_slots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    day_of_week int NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    start_time time NOT NULL,
    end_time time NOT NULL,
    slot_minutes int NOT NULL DEFAULT 60,
    max_per_slot int NOT NULL DEFAULT 1,
    UNIQUE(org_id, day_of_week, start_time)
);

ALTER TABLE sales
    ADD COLUMN IF NOT EXISTS amount_paid numeric(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS payment_status text NOT NULL DEFAULT 'paid'
        CHECK (payment_status IN ('pending', 'partial', 'paid'));

ALTER TABLE sale_items
    ADD COLUMN IF NOT EXISTS discount_type text NOT NULL DEFAULT 'none'
        CHECK (discount_type IN ('none', 'percentage', 'fixed')),
    ADD COLUMN IF NOT EXISTS discount_value numeric(15,2) NOT NULL DEFAULT 0;

ALTER TABLE quote_items
    ADD COLUMN IF NOT EXISTS discount_type text NOT NULL DEFAULT 'none'
        CHECK (discount_type IN ('none', 'percentage', 'fixed')),
    ADD COLUMN IF NOT EXISTS discount_value numeric(15,2) NOT NULL DEFAULT 0;

ALTER TABLE sales
    ADD COLUMN IF NOT EXISTS discount_type text NOT NULL DEFAULT 'none'
        CHECK (discount_type IN ('none', 'percentage', 'fixed')),
    ADD COLUMN IF NOT EXISTS discount_value numeric(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS discount_total numeric(15,2) NOT NULL DEFAULT 0;

ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS discount_type text NOT NULL DEFAULT 'none'
        CHECK (discount_type IN ('none', 'percentage', 'fixed')),
    ADD COLUMN IF NOT EXISTS discount_value numeric(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS discount_total numeric(15,2) NOT NULL DEFAULT 0;

ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS price_list_id uuid REFERENCES price_lists(id);

ALTER TABLE sales DROP CONSTRAINT IF EXISTS sales_payment_method_check;
ALTER TABLE sales ADD CONSTRAINT sales_payment_method_check
    CHECK (payment_method IN ('cash', 'card', 'transfer', 'check', 'other', 'credit', 'mixed'));

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS price_currency text NOT NULL DEFAULT 'ARS';

ALTER TABLE sales
    ADD COLUMN IF NOT EXISTS exchange_rate numeric(15,4),
    ADD COLUMN IF NOT EXISTS exchange_rate_type text;
