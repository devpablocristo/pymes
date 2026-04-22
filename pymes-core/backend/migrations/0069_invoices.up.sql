-- Invoices: baja del demo frontend-only a core con CRUD completo.
-- Sigue el patrón sales + sale_items (parent + line items), con lifecycle de archive,
-- favoritos y tags como el resto de los CRUDs uniformizados.

CREATE TABLE IF NOT EXISTS invoices (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number           text NOT NULL,
    party_id         uuid REFERENCES parties(id),
    customer_name    text NOT NULL DEFAULT '',
    issued_date      date NOT NULL,
    due_date         date NOT NULL,
    status           text NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('paid','pending','overdue')),
    subtotal         numeric(15,2) NOT NULL DEFAULT 0,
    discount_percent numeric(6,2)  NOT NULL DEFAULT 0,
    tax_percent      numeric(6,2)  NOT NULL DEFAULT 0,
    total            numeric(15,2) NOT NULL DEFAULT 0,
    notes            text NOT NULL DEFAULT '',
    is_favorite      boolean NOT NULL DEFAULT false,
    tags             text[]  NOT NULL DEFAULT '{}',
    created_by       text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at       timestamptz,
    UNIQUE (org_id, number)
);

CREATE INDEX IF NOT EXISTS idx_invoices_org             ON invoices(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_org_status      ON invoices(org_id, status);
CREATE INDEX IF NOT EXISTS idx_invoices_org_deleted_at  ON invoices(org_id, deleted_at);

CREATE TABLE IF NOT EXISTS invoice_line_items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id  uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description text NOT NULL,
    qty         numeric(15,4) NOT NULL DEFAULT 0,
    unit        text NOT NULL DEFAULT 'unidad',
    unit_price  numeric(15,2) NOT NULL DEFAULT 0,
    line_total  numeric(15,2) NOT NULL DEFAULT 0,
    sort_order  int  NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_invoice_line_items_invoice ON invoice_line_items(invoice_id);
