BEGIN;

ALTER TABLE app.purchases
  ADD COLUMN IF NOT EXISTS issue_date date,
  ADD COLUMN IF NOT EXISTS net_amount numeric(20,2),
  ADD COLUMN IF NOT EXISTS exempt_amount numeric(20,2),
  ADD COLUMN IF NOT EXISTS vat_breakdown jsonb,
  ADD COLUMN IF NOT EXISTS exchange_rate numeric(20,6);

ALTER TABLE app.purchases
  ALTER COLUMN amount TYPE numeric(20,2) USING round(amount, 2);

-- V3 is greenfield, but this deterministic backfill keeps the migration
-- repeatable for local databases created before the richer purchase contract.
UPDATE app.purchases
SET issue_date = COALESCE(issue_date, created_at::date),
    net_amount = round(amount, 2),
    exempt_amount = 0,
    vat_breakdown = jsonb_build_array(
      jsonb_build_object(
        'rate', '0',
        'base_amount', round(amount, 2)::text,
        'tax_amount', '0'
      )
    )
WHERE issue_date IS NULL
   OR net_amount IS NULL
   OR exempt_amount IS NULL
   OR vat_breakdown IS NULL;

ALTER TABLE app.purchases
  ALTER COLUMN issue_date SET NOT NULL,
  ALTER COLUMN net_amount SET NOT NULL,
  ALTER COLUMN exempt_amount SET NOT NULL,
  ALTER COLUMN vat_breakdown SET NOT NULL;

CREATE OR REPLACE FUNCTION app.purchase_amounts_are_valid(
  breakdown jsonb,
  expected_net numeric,
  exempt numeric,
  expected_total numeric
)
RETURNS boolean
LANGUAGE plpgsql
IMMUTABLE
STRICT
AS $$
DECLARE
  item jsonb;
  rate_text text;
  base_text text;
  tax_text text;
  rate_value numeric;
  base_value numeric;
  tax_value numeric;
  base_total numeric := 0;
  tax_total numeric := 0;
  seen_rates text[] := ARRAY[]::text[];
BEGIN
  IF jsonb_typeof(breakdown) <> 'array'
     OR expected_net < 0
     OR exempt < 0
     OR expected_total <= 0 THEN
    RETURN false;
  END IF;

  FOR item IN SELECT value FROM jsonb_array_elements(breakdown)
  LOOP
    IF jsonb_typeof(item) <> 'object'
       OR NOT item ?& ARRAY['rate', 'base_amount', 'tax_amount']
       OR (SELECT count(*) FROM jsonb_object_keys(item)) <> 3 THEN
      RETURN false;
    END IF;
    rate_text := item->>'rate';
    base_text := item->>'base_amount';
    tax_text := item->>'tax_amount';
    IF rate_text NOT IN ('0', '2.5', '5', '10.5', '21', '27')
       OR base_text !~ '^(0|[1-9][0-9]{0,13})(\.[0-9]{1,2})?$'
       OR tax_text !~ '^(0|[1-9][0-9]{0,13})(\.[0-9]{1,2})?$'
       OR rate_text = ANY(seen_rates) THEN
      RETURN false;
    END IF;
    seen_rates := array_append(seen_rates, rate_text);
    rate_value := rate_text::numeric;
    base_value := base_text::numeric;
    tax_value := tax_text::numeric;
    IF base_value <= 0
       OR tax_value < 0
       OR round(base_value * rate_value / 100, 2) <> tax_value THEN
      RETURN false;
    END IF;
    base_total := base_total + base_value;
    tax_total := tax_total + tax_value;
  END LOOP;

  RETURN base_total = expected_net
     AND expected_total = expected_net + exempt + tax_total;
END
$$;

ALTER TABLE app.purchases
  DROP CONSTRAINT IF EXISTS purchases_tax_components_valid,
  DROP CONSTRAINT IF EXISTS purchases_exchange_rate_valid;

ALTER TABLE app.purchases
  ADD CONSTRAINT purchases_tax_components_valid
    CHECK (
      app.purchase_amounts_are_valid(
        vat_breakdown,
        net_amount,
        exempt_amount,
        amount
      )
    ),
  ADD CONSTRAINT purchases_exchange_rate_valid
    CHECK (
      (currency = 'ARS' AND (exchange_rate IS NULL OR exchange_rate = 1))
      OR
      (
        currency IN ('USD', 'EUR')
        AND exchange_rate IS NOT NULL
        AND exchange_rate > 0
      )
    );

COMMIT;
