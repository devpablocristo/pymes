#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

ensure_seed_dbs_ready

run_pymes_sql_inline "
-- Preservar bootstrap del tenant: orgs/users/members/settings/api keys.
-- Solo se limpian tablas de CRUD/demo. org_members.party_id referencia
-- parties, por eso se desacopla antes del clear.
UPDATE org_members
   SET party_id = NULL
 WHERE party_id IS NOT NULL;

DO \$\$
DECLARE
    tables_to_truncate text;
BEGIN
    WITH RECURSIVE base_targets AS (
        SELECT
            cls.oid,
            nsp.nspname AS schemaname,
            cls.relname AS tablename,
            format('%I.%I', nsp.nspname, cls.relname) AS fqtn
        FROM pg_class cls
        JOIN pg_namespace nsp ON nsp.oid = cls.relnamespace
        WHERE cls.relkind = 'r'
          AND (
            nsp.nspname IN ('workshops', 'professionals', 'restaurant')
            OR (
                nsp.nspname = 'public'
                AND (
                    (cls.relname LIKE 'scheduling\_%' ESCAPE '\' AND cls.relname <> 'scheduling_branches')
                    OR cls.relname IN (
                        'parties',
                        'pymes_in_app_notifications',
                        'webhook_endpoints',
                        'timeline_entries',
                        'audit_log',
                        'whatsapp_messages',
                        'whatsapp_opt_ins',
                        'payment_preferences',
                        'ai_conversations',
                        'credit_notes',
                        'returns',
                        'payments',
                        'procurement_requests',
                        'procurement_policies',
                        'recurring_expenses',
                        'accounts',
                        'purchases',
                        'price_lists',
                        'cash_movements',
                        'sales',
                        'quotes',
                        'stock_levels',
                        'stock_movements',
                        'services',
                        'products',
                        'roles'
                    )
                )
            )
          )
    ),
    clear_graph AS (
        SELECT oid, fqtn
        FROM base_targets
        UNION
        SELECT child.oid, format('%I.%I', child_nsp.nspname, child.relname) AS fqtn
        FROM clear_graph parent
        JOIN pg_constraint fk
          ON fk.confrelid = parent.oid
         AND fk.contype = 'f'
        JOIN pg_class child
          ON child.oid = fk.conrelid
         AND child.relkind = 'r'
        JOIN pg_namespace child_nsp
          ON child_nsp.oid = child.relnamespace
    )
    SELECT string_agg(DISTINCT fqtn, ', ' ORDER BY fqtn)
      INTO tables_to_truncate
    FROM clear_graph
    WHERE fqtn NOT IN ('public.org_members', 'public.parties');

    IF tables_to_truncate IS NOT NULL THEN
        EXECUTE 'TRUNCATE TABLE ' || tables_to_truncate || ' RESTART IDENTITY';
    END IF;
END \$\$;

DELETE FROM parties;

INSERT INTO scheduling_branches (id, org_id, code, name, timezone, address, active, created_at, updated_at)
SELECT
    uuid_generate_v5(o.id, 'pymes-bootstrap/scheduling/branch/principal'),
    o.id,
    'principal',
    'Principal',
    'America/Argentina/Tucuman',
    '',
    true,
    now(),
    now()
FROM orgs o
WHERE NOT EXISTS (
    SELECT 1
    FROM scheduling_branches sb
    WHERE sb.org_id = o.id
);
"

run_review_sql_inline "
DO \$\$
DECLARE
    tables_to_truncate text;
BEGIN
    SELECT string_agg(format('%I.%I', schemaname, tablename), ', ' ORDER BY schemaname, tablename)
      INTO tables_to_truncate
    FROM pg_tables
    WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
      AND NOT (schemaname = 'public' AND tablename = 'schema_migrations');

    IF tables_to_truncate IS NOT NULL THEN
        EXECUTE 'TRUNCATE TABLE ' || tables_to_truncate || ' RESTART IDENTITY CASCADE';
    END IF;
END \$\$;
"
