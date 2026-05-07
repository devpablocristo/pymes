DO $$
DECLARE
    rec record;
BEGIN
    FOR rec IN
        SELECT table_schema, table_name
          FROM information_schema.columns
         WHERE table_schema = 'workshops'
           AND column_name = 'tenant_id'
         ORDER BY table_name
    LOOP
        IF NOT EXISTS (
            SELECT 1
              FROM information_schema.columns
             WHERE table_schema = rec.table_schema
               AND table_name = rec.table_name
               AND column_name = 'org_id'
        ) THEN
            EXECUTE format(
                'ALTER TABLE %I.%I RENAME COLUMN tenant_id TO org_id',
                rec.table_schema,
                rec.table_name
            );
        END IF;
    END LOOP;
END $$;
