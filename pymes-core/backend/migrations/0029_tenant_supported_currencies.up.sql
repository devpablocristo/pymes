ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS supported_currencies jsonb NOT NULL DEFAULT '[]'::jsonb;

UPDATE tenant_settings
SET supported_currencies = CASE
    WHEN COALESCE(NULLIF(TRIM(secondary_currency), ''), '') = ''
        THEN to_jsonb(ARRAY[COALESCE(NULLIF(TRIM(currency), ''), 'ARS')])
    ELSE to_jsonb(ARRAY[
            COALESCE(NULLIF(TRIM(currency), ''), 'ARS'),
            TRIM(secondary_currency)
         ])
END;
