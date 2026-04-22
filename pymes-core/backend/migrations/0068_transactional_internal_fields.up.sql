-- Uniformiza los CRUDs transaccionales con el resto: is_favorite, tags y deleted_at
-- para soportar favoritos, etiquetas internas y soft delete (archive/restore).

ALTER TABLE cash_movements
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_cash_movements_org_deleted_at
    ON cash_movements (org_id, deleted_at);

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_payments_org_deleted_at
    ON payments (org_id, deleted_at);

ALTER TABLE returns
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_returns_org_deleted_at
    ON returns (org_id, deleted_at);
