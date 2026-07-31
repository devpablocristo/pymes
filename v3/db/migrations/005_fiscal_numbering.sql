BEGIN;

ALTER TABLE app.sales
  ADD COLUMN IF NOT EXISTS fiscal_environment text NOT NULL DEFAULT 'homologation'
  CHECK (fiscal_environment IN ('homologation', 'production'));

ALTER TABLE app.sales
  DROP CONSTRAINT IF EXISTS sales_org_id_point_of_sale_document_type_voucher_number_key;

CREATE UNIQUE INDEX IF NOT EXISTS sales_fiscal_number_unique
  ON app.sales (org_id, fiscal_environment, point_of_sale, document_type, voucher_number);

CREATE TABLE IF NOT EXISTS app.fiscal_number_sequences (
  org_id text NOT NULL REFERENCES app.organizations(id),
  fiscal_environment text NOT NULL CHECK (fiscal_environment IN ('homologation', 'production')),
  point_of_sale integer NOT NULL CHECK (point_of_sale > 0),
  document_type text NOT NULL,
  last_number integer NOT NULL CHECK (last_number > 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, fiscal_environment, point_of_sale, document_type)
);

ALTER TABLE app.fiscal_number_sequences ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.fiscal_number_sequences FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS fiscal_number_sequences_org_isolation ON app.fiscal_number_sequences;
CREATE POLICY fiscal_number_sequences_org_isolation ON app.fiscal_number_sequences
  USING (org_id = current_setting('app.org_id', true))
  WITH CHECK (org_id = current_setting('app.org_id', true));

COMMIT;
