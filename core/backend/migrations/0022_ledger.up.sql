-- 0022_ledger.up.sql
-- Motor contable de partida doble (libro mayor + asientos + plan de cuentas).
-- Posteo desacoplado por outbox: el flujo comercial encola eventos y un worker
-- genera los asientos balanceados. Modelo inspirado conceptualmente en LedgerSMB
-- (acc_trans + account_link), reimplementado para el modelo multi-tenant org_id.

-- Plan de cuentas. type: A=Activo, L=Pasivo, Q=Patrimonio, I=Ingreso, E=Egreso.
CREATE TABLE IF NOT EXISTS ledger_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    code text NOT NULL,
    name text NOT NULL,
    type char(1) NOT NULL
        CONSTRAINT ledger_accounts_type_check CHECK (type IN ('A','L','Q','I','E')),
    parent_id uuid REFERENCES ledger_accounts(id) ON DELETE SET NULL,
    is_postable boolean NOT NULL DEFAULT true,
    archived_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_ledger_accounts_org_code
    ON ledger_accounts(org_id, code) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ledger_accounts_org
    ON ledger_accounts(org_id) WHERE archived_at IS NULL;
CREATE TRIGGER trg_ledger_accounts_updated_at
    BEFORE UPDATE ON ledger_accounts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Mapeo funcional rol -> cuenta (patrón account_link de LedgerSMB). Roles:
-- revenue, cash, bank, receivable, payable, cogs, vat_payable_<rate>,
-- vat_credit_<rate>, credit_note_payable, card_clearing, mp_clearing, ...
CREATE TABLE IF NOT EXISTS ledger_account_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    role text NOT NULL,
    account_id uuid NOT NULL REFERENCES ledger_accounts(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_ledger_account_links_org_role UNIQUE (org_id, role)
);
CREATE TRIGGER trg_ledger_account_links_updated_at
    BEFORE UPDATE ON ledger_account_links FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Secuencia gapless de numeración de asientos por org.
CREATE TABLE IF NOT EXISTS ledger_sequences (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    next_entry_number bigint NOT NULL DEFAULT 1
);

-- Asientos (cabecera). entry_number se asigna sólo al postear con éxito.
CREATE TABLE IF NOT EXISTS journal_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    entry_number text NOT NULL,
    entry_date date NOT NULL DEFAULT current_date,
    currency text NOT NULL DEFAULT 'ARS',
    exchange_rate numeric(18,6) NOT NULL DEFAULT 1,
    source_type text NOT NULL DEFAULT 'manual',
    source_id uuid,
    source_event text NOT NULL DEFAULT 'manual',
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'posted'
        CONSTRAINT journal_entries_status_check CHECK (status IN ('posted','reversed')),
    reversed_by_entry_id uuid REFERENCES journal_entries(id) ON DELETE SET NULL,
    created_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_journal_entries_org_number
    ON journal_entries(org_id, entry_number);
-- Idempotencia: un evento de un documento produce a lo sumo un asiento.
CREATE UNIQUE INDEX IF NOT EXISTS uq_journal_entries_idempotency
    ON journal_entries(org_id, source_type, source_id, source_event)
    WHERE source_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_journal_entries_org_date
    ON journal_entries(org_id, entry_date DESC);

-- Líneas del asiento (partida doble). Una línea es débito XOR crédito.
CREATE TABLE IF NOT EXISTS journal_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    entry_id uuid NOT NULL REFERENCES journal_entries(id) ON DELETE CASCADE,
    account_id uuid NOT NULL REFERENCES ledger_accounts(id),
    debit numeric(15,2) NOT NULL DEFAULT 0,
    credit numeric(15,2) NOT NULL DEFAULT 0,
    base_amount numeric(15,2) NOT NULL DEFAULT 0,
    party_id uuid,
    memo text NOT NULL DEFAULT '',
    line_no int NOT NULL DEFAULT 0,
    CONSTRAINT journal_lines_debit_xor_credit CHECK (NOT (debit > 0 AND credit > 0)),
    CONSTRAINT journal_lines_non_negative CHECK (debit >= 0 AND credit >= 0)
);
CREATE INDEX IF NOT EXISTS idx_journal_lines_entry ON journal_lines(entry_id);
CREATE INDEX IF NOT EXISTS idx_journal_lines_account ON journal_lines(org_id, account_id);
CREATE INDEX IF NOT EXISTS idx_journal_lines_party
    ON journal_lines(org_id, party_id) WHERE party_id IS NOT NULL;

-- Outbox de eventos contables (esquema propio: webhook_outbox no tiene
-- attempts/next_retry). Drenado por el scheduler con FOR UPDATE SKIP LOCKED.
CREATE TABLE IF NOT EXISTS ledger_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    reference_type text NOT NULL,
    reference_id uuid NOT NULL,
    source_event text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'pending'
        CONSTRAINT ledger_outbox_status_check
        CHECK (status IN ('pending','posted','failed','skipped','dead')),
    attempts int NOT NULL DEFAULT 0,
    max_attempts int NOT NULL DEFAULT 10,
    next_retry timestamptz,
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_ledger_outbox_idempotency
        UNIQUE (org_id, reference_type, reference_id, source_event)
);
CREATE INDEX IF NOT EXISTS idx_ledger_outbox_drain
    ON ledger_outbox(next_retry) WHERE status IN ('pending','failed');
CREATE TRIGGER trg_ledger_outbox_updated_at
    BEFORE UPDATE ON ledger_outbox FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Gate por org: la contabilidad arranca apagada. El flujo comercial nunca lee
-- este flag (siempre encola); el worker decide skipped/failed/posted.
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS ledger_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE org_settings ADD COLUMN IF NOT EXISTS journal_prefix text NOT NULL DEFAULT 'ASTO';

-- Marca de posteo por documento (la actualiza el worker; M2+).
ALTER TABLE sales     ADD COLUMN IF NOT EXISTS posting_status text NOT NULL DEFAULT 'pending';
ALTER TABLE purchases ADD COLUMN IF NOT EXISTS posting_status text NOT NULL DEFAULT 'pending';
ALTER TABLE payments  ADD COLUMN IF NOT EXISTS posting_status text NOT NULL DEFAULT 'pending';
ALTER TABLE returns   ADD COLUMN IF NOT EXISTS posting_status text NOT NULL DEFAULT 'pending';
