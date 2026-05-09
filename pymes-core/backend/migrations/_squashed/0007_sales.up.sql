-- 0007_sales.up.sql
-- Ciclo comercial completo: quotes, sales, purchases, payments, returns,
-- credit_notes, invoices, cash_movements, accounts.
--
-- Sales usa `voided_at` (excepción documentada al patrón archived_at —
-- semántica contable: una venta no se "archiva", se anula).
-- Purchases usa `archived_at` (no es contablemente sensitivo).
--
-- branch_id sin FK a scheduling_branches (referencia lógica, ver 0006).

CREATE TABLE IF NOT EXISTS quotes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    party_name text NOT NULL DEFAULT '',
    branch_id uuid,
    status text NOT NULL DEFAULT 'draft'
        CONSTRAINT quotes_status_check
        CHECK (status IN ('draft','sent','accepted','rejected','expired')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    discount_type text NOT NULL DEFAULT 'none'
        CONSTRAINT quotes_discount_type_check
        CHECK (discount_type IN ('none','percentage','fixed')),
    discount_value numeric(15,2) NOT NULL DEFAULT 0,
    discount_total numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    valid_until timestamptz,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    CONSTRAINT quotes_org_number_uniq UNIQUE (org_id, number)
);
CREATE INDEX IF NOT EXISTS idx_quotes_org ON quotes(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_quotes_org_status ON quotes(org_id, status);
CREATE INDEX IF NOT EXISTS idx_quotes_org_branch_date
    ON quotes(org_id, branch_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_quotes_party
    ON quotes(party_id) WHERE party_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_quotes_org_archived_at
    ON quotes(org_id, archived_at);

CREATE TRIGGER trg_quotes_updated_at
    BEFORE UPDATE ON quotes FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS quote_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id uuid NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id) ON DELETE SET NULL,
    service_id uuid REFERENCES services(id) ON DELETE SET NULL,
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    discount_type text NOT NULL DEFAULT 'none'
        CONSTRAINT quote_items_discount_type_check
        CHECK (discount_type IN ('none','percentage','fixed')),
    discount_value numeric(15,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_quote_items_quote ON quote_items(quote_id);
CREATE INDEX IF NOT EXISTS idx_quote_items_service
    ON quote_items(service_id) WHERE service_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS sales (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    party_name text NOT NULL DEFAULT '',
    quote_id uuid REFERENCES quotes(id) ON DELETE SET NULL,
    branch_id uuid,
    status text NOT NULL DEFAULT 'completed'
        CONSTRAINT sales_status_check
        CHECK (status IN ('completed','voided')),
    payment_method text NOT NULL DEFAULT 'cash'
        CONSTRAINT sales_payment_method_check
        CHECK (payment_method IN ('cash','card','transfer','check','other','credit','mixed')),
    payment_status text NOT NULL DEFAULT 'paid'
        CONSTRAINT sales_payment_status_check
        CHECK (payment_status IN ('pending','partial','paid')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    amount_paid numeric(15,2) NOT NULL DEFAULT 0,
    discount_type text NOT NULL DEFAULT 'none'
        CONSTRAINT sales_discount_type_check
        CHECK (discount_type IN ('none','percentage','fixed')),
    discount_value numeric(15,2) NOT NULL DEFAULT 0,
    discount_total numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    exchange_rate numeric(15,4),
    exchange_rate_type text,
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    voided_at timestamptz,
    CONSTRAINT sales_org_number_uniq UNIQUE (org_id, number)
);
CREATE INDEX IF NOT EXISTS idx_sales_org ON sales(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sales_org_completed
    ON sales(org_id, created_at) WHERE status = 'completed';
CREATE INDEX IF NOT EXISTS idx_sales_org_branch_date
    ON sales(org_id, branch_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sales_party
    ON sales(party_id) WHERE party_id IS NOT NULL;

CREATE TRIGGER trg_sales_updated_at
    BEFORE UPDATE ON sales FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS sale_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id uuid NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id) ON DELETE SET NULL,
    service_id uuid REFERENCES services(id) ON DELETE SET NULL,
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    cost_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    discount_type text NOT NULL DEFAULT 'none'
        CONSTRAINT sale_items_discount_type_check
        CHECK (discount_type IN ('none','percentage','fixed')),
    discount_value numeric(15,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sale_items_sale ON sale_items(sale_id);
CREATE INDEX IF NOT EXISTS idx_sale_items_service
    ON sale_items(service_id) WHERE service_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS purchases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    party_name text NOT NULL DEFAULT '',
    branch_id uuid,
    status text NOT NULL DEFAULT 'draft'
        CONSTRAINT purchases_status_check
        CHECK (status IN ('draft','received','partial','voided')),
    payment_status text NOT NULL DEFAULT 'pending'
        CONSTRAINT purchases_payment_status_check
        CHECK (payment_status IN ('pending','partial','paid')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    received_at timestamptz,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    CONSTRAINT purchases_org_number_uniq UNIQUE (org_id, number)
);
CREATE INDEX IF NOT EXISTS idx_purchases_org ON purchases(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_purchases_org_status ON purchases(org_id, status);
CREATE INDEX IF NOT EXISTS idx_purchases_party
    ON purchases(party_id) WHERE party_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_purchases_org_archived_at
    ON purchases(org_id, archived_at);

CREATE TRIGGER trg_purchases_updated_at
    BEFORE UPDATE ON purchases FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS purchase_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_id uuid NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id) ON DELETE SET NULL,
    service_id uuid REFERENCES services(id) ON DELETE SET NULL,
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_cost numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_purchase_items_purchase ON purchase_items(purchase_id);
CREATE INDEX IF NOT EXISTS idx_purchase_items_service
    ON purchase_items(service_id) WHERE service_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    reference_type text NOT NULL
        CONSTRAINT payments_reference_type_check
        CHECK (reference_type IN ('sale','purchase')),
    reference_id uuid NOT NULL,
    method text NOT NULL DEFAULT 'cash'
        CONSTRAINT payments_method_check
        CHECK (method IN ('cash','card','transfer','check','other','credit_note')),
    amount numeric(15,2) NOT NULL,
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    received_at timestamptz NOT NULL DEFAULT now(),
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_payments_reference
    ON payments(org_id, reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_payments_org ON payments(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_org_archived_at
    ON payments(org_id, archived_at);

CREATE TABLE IF NOT EXISTS returns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    sale_id uuid NOT NULL REFERENCES sales(id) ON DELETE RESTRICT,
    reason text NOT NULL DEFAULT 'other'
        CONSTRAINT returns_reason_check
        CHECK (reason IN ('defective','wrong_item','changed_mind','other')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    refund_method text NOT NULL DEFAULT 'cash'
        CONSTRAINT returns_refund_method_check
        CHECK (refund_method IN ('cash','credit_note','original_method')),
    status text NOT NULL DEFAULT 'completed'
        CONSTRAINT returns_status_check
        CHECK (status IN ('completed','voided')),
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    CONSTRAINT returns_org_number_uniq UNIQUE (org_id, number)
);
CREATE INDEX IF NOT EXISTS idx_returns_org ON returns(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_returns_sale ON returns(sale_id);

CREATE TABLE IF NOT EXISTS return_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id uuid NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
    sale_item_id uuid NOT NULL REFERENCES sale_items(id) ON DELETE RESTRICT,
    product_id uuid REFERENCES products(id) ON DELETE SET NULL,
    service_id uuid REFERENCES services(id) ON DELETE SET NULL,
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_return_items_return ON return_items(return_id);

CREATE TABLE IF NOT EXISTS credit_notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE RESTRICT,
    return_id uuid REFERENCES returns(id) ON DELETE RESTRICT,  -- opcional (no todo crédito viene de un return)
    amount numeric(15,2) NOT NULL,
    used_amount numeric(15,2) NOT NULL DEFAULT 0,
    balance numeric(15,2) NOT NULL,
    expires_at timestamptz,
    status text NOT NULL DEFAULT 'active'
        CONSTRAINT credit_notes_status_check
        CHECK (status IN ('active','used','expired','voided')),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT credit_notes_org_number_uniq UNIQUE (org_id, number)
);
CREATE INDEX IF NOT EXISTS idx_credit_notes_org_party
    ON credit_notes(org_id, party_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_credit_notes_org_status
    ON credit_notes(org_id, status);

CREATE TABLE IF NOT EXISTS invoices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    customer_name text NOT NULL DEFAULT '',
    issued_date date NOT NULL,
    due_date date NOT NULL,
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT invoices_status_check
        CHECK (status IN ('paid','pending','overdue')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    discount_percent numeric(6,2) NOT NULL DEFAULT 0,
    tax_percent numeric(6,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    notes text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    CONSTRAINT invoices_org_number_uniq UNIQUE (org_id, number)
);
CREATE INDEX IF NOT EXISTS idx_invoices_org ON invoices(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_org_status ON invoices(org_id, status);
CREATE INDEX IF NOT EXISTS idx_invoices_org_archived_at
    ON invoices(org_id, archived_at);

CREATE TRIGGER trg_invoices_updated_at
    BEFORE UPDATE ON invoices FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS invoice_line_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description text NOT NULL,
    qty numeric(15,4) NOT NULL DEFAULT 0,
    unit text NOT NULL DEFAULT 'unidad',
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    line_total numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_invoice_line_items_invoice
    ON invoice_line_items(invoice_id);

CREATE TABLE IF NOT EXISTS cash_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    branch_id uuid,
    type text NOT NULL
        CONSTRAINT cash_movements_type_check
        CHECK (type IN ('income','expense')),
    amount numeric(15,2) NOT NULL,
    currency text NOT NULL DEFAULT 'ARS',
    category text NOT NULL DEFAULT 'other',
    description text NOT NULL DEFAULT '',
    payment_method text NOT NULL DEFAULT 'cash',
    reference_type text NOT NULL DEFAULT 'manual',
    reference_id uuid,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_cash_movements_org
    ON cash_movements(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cash_movements_org_type
    ON cash_movements(org_id, type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cash_movements_org_branch
    ON cash_movements(org_id, branch_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cash_movements_org_archived_at
    ON cash_movements(org_id, archived_at);

CREATE TABLE IF NOT EXISTS recurring_expenses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    description text NOT NULL,
    amount numeric(15,2) NOT NULL,
    currency text NOT NULL DEFAULT 'ARS',
    category text NOT NULL DEFAULT 'other',
    payment_method text NOT NULL DEFAULT 'transfer'
        CONSTRAINT recurring_expenses_payment_method_check
        CHECK (payment_method IN ('cash','card','transfer','debit','check','other')),
    frequency text NOT NULL DEFAULT 'monthly'
        CONSTRAINT recurring_expenses_frequency_check
        CHECK (frequency IN ('weekly','biweekly','monthly','quarterly','yearly')),
    day_of_month int NOT NULL DEFAULT 1
        CONSTRAINT recurring_expenses_day_of_month_check
        CHECK (day_of_month BETWEEN 1 AND 28),
    party_id uuid REFERENCES parties(id) ON DELETE SET NULL,
    is_active boolean NOT NULL DEFAULT true,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}',
    next_due_date date NOT NULL,
    last_paid_date date,
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_recurring_expenses_org
    ON recurring_expenses(org_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_recurring_expenses_due
    ON recurring_expenses(next_due_date) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_recurring_expenses_org_archived_at
    ON recurring_expenses(org_id, archived_at);

CREATE TRIGGER trg_recurring_expenses_updated_at
    BEFORE UPDATE ON recurring_expenses FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL
        CONSTRAINT accounts_type_check
        CHECK (type IN ('receivable','payable')),
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    party_name text NOT NULL DEFAULT '',
    balance numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    credit_limit numeric(15,2) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT accounts_org_type_party_uniq UNIQUE (org_id, type, party_id)
);
CREATE INDEX IF NOT EXISTS idx_accounts_org ON accounts(org_id, type);
CREATE INDEX IF NOT EXISTS idx_accounts_party ON accounts(org_id, party_id);
CREATE INDEX IF NOT EXISTS idx_accounts_balance
    ON accounts(org_id) WHERE balance != 0;

CREATE TRIGGER trg_accounts_updated_at
    BEFORE UPDATE ON accounts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS account_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL
        CONSTRAINT account_movements_type_check
        CHECK (type IN ('charge','payment','adjustment','void')),
    amount numeric(15,2) NOT NULL,
    balance numeric(15,2) NOT NULL,
    description text NOT NULL DEFAULT '',
    reference_type text NOT NULL DEFAULT 'manual',
    reference_id uuid,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_account_movements_account
    ON account_movements(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_account_movements_org
    ON account_movements(org_id, created_at DESC);
