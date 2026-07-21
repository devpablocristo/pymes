ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS supported_currencies jsonb NOT NULL DEFAULT '[]'::jsonb;

-- Bases de desarrollo sin columna secondary_currency (0012 no aplicada o drift): solo moneda principal.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'tenant_settings' AND column_name = 'secondary_currency'
  ) THEN
    UPDATE tenant_settings
    SET supported_currencies = CASE
        WHEN COALESCE(NULLIF(TRIM(secondary_currency), ''), '') = ''
            THEN to_jsonb(ARRAY[COALESCE(NULLIF(TRIM(currency), ''), 'ARS')])
        ELSE to_jsonb(ARRAY[
                COALESCE(NULLIF(TRIM(currency), ''), 'ARS'),
                TRIM(secondary_currency)
             ])
    END;
  ELSE
    UPDATE tenant_settings
    SET supported_currencies = to_jsonb(ARRAY[COALESCE(NULLIF(TRIM(currency), ''), 'ARS')]);
  END IF;
END $$;
