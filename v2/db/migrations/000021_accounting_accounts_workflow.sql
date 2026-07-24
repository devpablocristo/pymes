-- Make the chart of accounts an operational, auditable directory.
-- Account identity and lifecycle remain tenant-owned; mapping definitions are
-- global product metadata and contain no tenant data.

DO $preflight$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM accounting.accounts
         GROUP BY org_id, lower(btrim(code))
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION
            'account codes collide after whitespace/case normalization'
            USING ERRCODE = '23505',
                  CONSTRAINT = 'accounting_accounts_code_ci_unique';
    END IF;
END
$preflight$;

UPDATE accounting.accounts
   SET code = btrim(code),
       name = btrim(name)
 WHERE code IS DISTINCT FROM btrim(code)
    OR name IS DISTINCT FROM btrim(name);

ALTER TABLE accounting.accounts
    ADD CONSTRAINT accounting_accounts_code_normalized CHECK (
        code = btrim(code)
        AND char_length(code) BETWEEN 1 AND 32
    ),
    ADD CONSTRAINT accounting_accounts_name_normalized CHECK (
        name = btrim(name)
        AND char_length(name) BETWEEN 1 AND 160
    ),
    ADD CONSTRAINT accounting_accounts_node_type_check CHECK (
        (
            posting_allowed
            AND parent_id IS NOT NULL
            AND monetary_class <> 'not_applicable'
        )
        OR
        (
            NOT posting_allowed
            AND monetary_class = 'not_applicable'
        )
    ) NOT VALID;

CREATE UNIQUE INDEX accounting_accounts_code_ci_uidx
    ON accounting.accounts (org_id, lower(code));

CREATE OR REPLACE FUNCTION accounting.account_code_sort_key(requested_code text)
RETURNS text
LANGUAGE sql
IMMUTABLE
STRICT
SET search_path = pg_catalog
RETURN (
    SELECT string_agg(
        CASE
            WHEN segment.value ~ '^[0-9]+$'
                THEN '0' || lpad(segment.value, 24, '0')
            ELSE '1' || lower(segment.value)
        END,
        '.'
        ORDER BY segment.ordinality
    )
      FROM regexp_split_to_table(btrim(requested_code), '[.]')
           WITH ORDINALITY AS segment(value, ordinality)
);

REVOKE ALL
ON FUNCTION accounting.account_code_sort_key(text)
FROM PUBLIC;

CREATE TABLE accounting.account_mapping_definitions (
    role text PRIMARY KEY CHECK (
        role = lower(btrim(role))
        AND role ~ '^[a-z0-9_]+$'
    ),
    label_es text NOT NULL CHECK (btrim(label_es) <> ''),
    label_en text NOT NULL CHECK (btrim(label_en) <> ''),
    description_es text NOT NULL CHECK (btrim(description_es) <> ''),
    description_en text NOT NULL CHECK (btrim(description_en) <> ''),
    required boolean NOT NULL DEFAULT false,
    compatible_account_classes text[] NOT NULL CHECK (
        cardinality(compatible_account_classes) > 0
        AND compatible_account_classes <@ ARRAY[
            'asset', 'liability', 'equity', 'revenue', 'cost', 'expense'
        ]::text[]
    ),
    compatible_normal_balances text[] NOT NULL CHECK (
        cardinality(compatible_normal_balances) > 0
        AND compatible_normal_balances <@ ARRAY['debit', 'credit']::text[]
    ),
    compatible_monetary_classes text[] NOT NULL CHECK (
        cardinality(compatible_monetary_classes) > 0
        AND compatible_monetary_classes <@ ARRAY[
            'monetary', 'non_monetary'
        ]::text[]
    ),
    canonical_role text,
    is_alias boolean NOT NULL DEFAULT false,
    display_order integer NOT NULL CHECK (display_order > 0),
    CONSTRAINT accounting_account_mapping_definitions_canonical_fk
        FOREIGN KEY (canonical_role)
        REFERENCES accounting.account_mapping_definitions(role)
        ON DELETE RESTRICT
        DEFERRABLE INITIALLY DEFERRED,
    CHECK (canonical_role IS NULL OR canonical_role <> role),
    CHECK (NOT required OR NOT is_alias)
);

WITH alias_metadata(role, canonical_role) AS (
    VALUES
        ('checks_receivable', 'checks_clearing'),
        ('accounts_receivable', 'receivable'),
        ('vat_input', NULL),
        ('supplier_advances', NULL),
        ('accounts_payable', 'payable'),
        ('customer_advances', NULL),
        ('vat_output', NULL),
        ('current_year_result', 'current_result'),
        ('sales_goods', 'revenue'),
        ('sales_services', 'revenue'),
        ('general_expense', 'purchase_expense'),
        ('payment_fees', NULL),
        ('rounding', 'rounding_difference')
),
source AS (
    SELECT
        mapping.mapping_key AS role,
        min(mapping.description) AS label_es,
        initcap(replace(mapping.mapping_key, '_', ' ')) AS label_en,
        array_agg(DISTINCT account.account_class ORDER BY account.account_class)
            AS compatible_account_classes,
        array_agg(DISTINCT account.normal_balance ORDER BY account.normal_balance)
            AS compatible_normal_balances,
        array_agg(DISTINCT account.monetary_class ORDER BY account.monetary_class)
            FILTER (WHERE account.monetary_class <> 'not_applicable')
            AS compatible_monetary_classes,
        alias.canonical_role,
        alias.role IS NOT NULL AS is_alias
      FROM accounting.chart_template_mappings AS mapping
      JOIN accounting.chart_template_accounts AS account
        ON account.template_code = mapping.template_code
       AND account.template_version = mapping.template_version
       AND account.code = mapping.account_code
      LEFT JOIN alias_metadata AS alias
        ON alias.role = mapping.mapping_key
     GROUP BY mapping.mapping_key, alias.role, alias.canonical_role
),
ordered AS (
    SELECT
        source.*,
        row_number() OVER (
            ORDER BY source.is_alias, source.role
        )::integer AS display_order
      FROM source
)
INSERT INTO accounting.account_mapping_definitions (
    role,
    label_es,
    label_en,
    description_es,
    description_en,
    required,
    compatible_account_classes,
    compatible_normal_balances,
    compatible_monetary_classes,
    canonical_role,
    is_alias,
    display_order
)
SELECT
    role,
    label_es,
    label_en,
    'Cuenta utilizada para ' || lower(label_es),
    'Account used for ' || lower(label_en),
    NOT is_alias,
    compatible_account_classes,
    compatible_normal_balances,
    compatible_monetary_classes,
    canonical_role,
    is_alias,
    display_order
FROM ordered;

-- Preserve installations that created a private mapping before the canonical
-- catalog existed. They become read-only legacy aliases, never new roles.
INSERT INTO accounting.account_mapping_definitions (
    role,
    label_es,
    label_en,
    description_es,
    description_en,
    required,
    compatible_account_classes,
    compatible_normal_balances,
    compatible_monetary_classes,
    canonical_role,
    is_alias,
    display_order
)
SELECT
    mapping.mapping_key,
    initcap(replace(mapping.mapping_key, '_', ' ')),
    initcap(replace(mapping.mapping_key, '_', ' ')),
    'Mapping heredado conservado por compatibilidad',
    'Legacy mapping retained for compatibility',
    false,
    array_agg(DISTINCT account.account_class ORDER BY account.account_class),
    array_agg(DISTINCT account.normal_balance ORDER BY account.normal_balance),
    array_agg(DISTINCT account.monetary_class ORDER BY account.monetary_class),
    NULL,
    true,
    10000 + row_number() OVER (ORDER BY mapping.mapping_key)
FROM accounting.account_mappings AS mapping
JOIN accounting.accounts AS account
  ON account.org_id = mapping.org_id
 AND account.id = mapping.account_id
WHERE NOT EXISTS (
    SELECT 1
      FROM accounting.account_mapping_definitions AS definition
     WHERE definition.role = mapping.mapping_key
)
GROUP BY mapping.mapping_key;

-- Aliases are catalog metadata only. The v2 posting engine uses canonical
-- roles, so physical alias mappings would make their accounts impossible to
-- remap/archive and would duplicate configuration state.
DELETE FROM accounting.account_mappings AS mapping
USING accounting.account_mapping_definitions AS definition
 WHERE definition.role = mapping.mapping_key
   AND definition.is_alias;

DELETE FROM accounting.chart_template_mappings AS mapping
USING accounting.account_mapping_definitions AS definition
 WHERE definition.role = mapping.mapping_key
   AND definition.is_alias;

ALTER TABLE accounting.account_mappings
    ADD CONSTRAINT accounting_account_mappings_definition_fk
        FOREIGN KEY (mapping_key)
        REFERENCES accounting.account_mapping_definitions(role)
        ON DELETE RESTRICT;

-- Root protection is derived from the installed template, not from a loose
-- code convention. This marks precisely the six roots of ar-pyme today and
-- remains valid for future templates.
UPDATE accounting.accounts AS account
   SET system_key =
        'chart-root:'
        || setting.chart_template_code
        || ':'
        || template_account.account_class
  FROM accounting.organization_settings AS setting
  JOIN accounting.chart_template_accounts AS template_account
    ON template_account.template_code = setting.chart_template_code
   AND template_account.template_version = setting.chart_template_version
   AND template_account.parent_code IS NULL
 WHERE account.org_id = setting.org_id
   AND account.code = template_account.code
   AND account.account_class = template_account.account_class
   AND account.parent_id IS NULL
   AND NOT account.posting_allowed
   AND account.system_key IS NULL;

CREATE OR REPLACE FUNCTION accounting.assign_template_root_system_key()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    template_code text;
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);
    IF NEW.system_key IS NOT NULL
       OR NEW.parent_id IS NOT NULL
       OR NEW.posting_allowed THEN
        RETURN NEW;
    END IF;

    SELECT setting.chart_template_code
      INTO template_code
      FROM accounting.organization_settings AS setting
      JOIN accounting.chart_template_accounts AS template_account
        ON template_account.template_code = setting.chart_template_code
       AND template_account.template_version = setting.chart_template_version
       AND template_account.parent_code IS NULL
       AND template_account.code = NEW.code
       AND template_account.account_class = NEW.account_class
     WHERE setting.org_id = NEW.org_id;

    IF FOUND THEN
        NEW.system_key :=
            'chart-root:' || template_code || ':' || NEW.account_class;
    END IF;
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.account_has_dependencies(
    requested_org_id uuid,
    requested_account_id uuid
)
RETURNS boolean
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
BEGIN
    PERFORM app.assert_org_context(requested_org_id);
    RETURN (
        SELECT
        EXISTS (
            SELECT 1 FROM accounting.journal_lines
             WHERE org_id = requested_org_id
               AND account_id = requested_account_id
        )
        OR EXISTS (
            SELECT 1 FROM accounting.draft_lines
             WHERE org_id = requested_org_id
               AND account_id = requested_account_id
        )
        OR EXISTS (
            SELECT 1 FROM accounting.account_mappings
             WHERE org_id = requested_org_id
               AND account_id = requested_account_id
        )
        OR EXISTS (
            SELECT 1 FROM accounting.accounts
             WHERE org_id = requested_org_id
               AND parent_id = requested_account_id
        )
        OR EXISTS (
            SELECT 1 FROM accounting.financial_accounts
             WHERE org_id = requested_org_id
               AND ledger_account_id = requested_account_id
        )
        OR EXISTS (
            SELECT 1 FROM accounting.open_items
             WHERE org_id = requested_org_id
               AND account_id = requested_account_id
        )
        OR EXISTS (
            SELECT 1 FROM accounting.inflation_run_lines
             WHERE org_id = requested_org_id
               AND account_id = requested_account_id
        )
        OR EXISTS (
            SELECT 1 FROM accounting.currency_revaluation_lines
             WHERE org_id = requested_org_id
               AND account_id = requested_account_id
        )
    );
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_account_workflow()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    parent_record accounting.accounts%ROWTYPE;
    structure_changed boolean;
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);
    NEW.code := btrim(NEW.code);
    NEW.name := btrim(NEW.name);

    IF TG_OP = 'UPDATE' THEN
        IF OLD.system_key IS NOT NULL
           AND (
                OLD.code IS DISTINCT FROM NEW.code
                OR OLD.name IS DISTINCT FROM NEW.name
                OR OLD.account_class IS DISTINCT FROM NEW.account_class
                OR OLD.parent_id IS DISTINCT FROM NEW.parent_id
                OR OLD.normal_balance IS DISTINCT FROM NEW.normal_balance
                OR OLD.monetary_class IS DISTINCT FROM NEW.monetary_class
                OR OLD.posting_allowed IS DISTINCT FROM NEW.posting_allowed
                OR OLD.archived_at IS DISTINCT FROM NEW.archived_at
                OR OLD.trashed_at IS DISTINCT FROM NEW.trashed_at
           ) THEN
            RAISE EXCEPTION 'system account is read-only'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_accounts_system_protected';
        END IF;

        IF OLD.system_key IS DISTINCT FROM NEW.system_key THEN
            RAISE EXCEPTION 'system account identity is immutable'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_accounts_system_key_immutable';
        END IF;

        IF OLD.posting_allowed IS DISTINCT FROM NEW.posting_allowed THEN
            RAISE EXCEPTION 'account node type is immutable'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_accounts_node_type_immutable';
        END IF;

        structure_changed :=
            OLD.code IS DISTINCT FROM NEW.code
            OR OLD.account_class IS DISTINCT FROM NEW.account_class
            OR OLD.parent_id IS DISTINCT FROM NEW.parent_id
            OR OLD.normal_balance IS DISTINCT FROM NEW.normal_balance
            OR OLD.monetary_class IS DISTINCT FROM NEW.monetary_class;

        IF structure_changed
           AND accounting.account_has_dependencies(NEW.org_id, NEW.id) THEN
            RAISE EXCEPTION 'account structure is locked after first use'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_accounts_structure_locked';
        END IF;

        IF OLD.archived_at IS NULL AND NEW.archived_at IS NOT NULL THEN
            IF EXISTS (
                SELECT 1 FROM accounting.account_mappings
                 WHERE org_id = NEW.org_id AND account_id = NEW.id
            ) THEN
                RAISE EXCEPTION 'mapped account must be remapped before archive'
                    USING ERRCODE = '23514',
                          CONSTRAINT = 'accounting_accounts_mapping_blocks_archive';
            END IF;
            IF EXISTS (
                SELECT 1 FROM accounting.financial_accounts
                 WHERE org_id = NEW.org_id
                   AND ledger_account_id = NEW.id
                   AND archived_at IS NULL
            ) THEN
                RAISE EXCEPTION 'active financial account blocks archive'
                    USING ERRCODE = '23514',
                          CONSTRAINT = 'accounting_accounts_financial_blocks_archive';
            END IF;
            IF EXISTS (
                SELECT 1 FROM accounting.accounts
                 WHERE org_id = NEW.org_id
                   AND parent_id = NEW.id
                   AND archived_at IS NULL
                   AND trashed_at IS NULL
            ) THEN
                RAISE EXCEPTION 'active child accounts must be archived first'
                    USING ERRCODE = '23514',
                          CONSTRAINT = 'accounting_accounts_active_children';
            END IF;
        END IF;

        IF OLD.trashed_at IS NULL AND NEW.trashed_at IS NOT NULL
           AND accounting.account_has_dependencies(NEW.org_id, NEW.id) THEN
            RAISE EXCEPTION 'only unused and unlinked accounts can be trashed'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_accounts_trash_unused';
        END IF;
    END IF;

    IF NEW.parent_id IS NOT NULL THEN
        SELECT *
          INTO parent_record
          FROM accounting.accounts
         WHERE org_id = NEW.org_id
           AND id = NEW.parent_id
         FOR SHARE;
        IF NOT FOUND
           OR parent_record.posting_allowed
           OR parent_record.account_class <> NEW.account_class THEN
            RAISE EXCEPTION 'invalid account parent'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_accounts_invalid_parent';
        END IF;
        IF parent_record.archived_at IS NOT NULL
           OR parent_record.trashed_at IS NOT NULL THEN
            RAISE EXCEPTION 'account parent must be active'
                USING ERRCODE = '23514',
                      CONSTRAINT = 'accounting_accounts_parent_inactive';
        END IF;
    END IF;

    IF TG_OP = 'UPDATE'
       AND (
            (OLD.archived_at IS NOT NULL AND NEW.archived_at IS NULL)
            OR (OLD.trashed_at IS NOT NULL AND NEW.trashed_at IS NULL)
       )
       AND EXISTS (
            WITH RECURSIVE ancestors AS (
                SELECT parent.id, parent.parent_id,
                       parent.archived_at, parent.trashed_at
                  FROM accounting.accounts AS parent
                 WHERE parent.org_id = NEW.org_id
                   AND parent.id = NEW.parent_id
                UNION ALL
                SELECT parent.id, parent.parent_id,
                       parent.archived_at, parent.trashed_at
                  FROM accounting.accounts AS parent
                  JOIN ancestors ON ancestors.parent_id = parent.id
                 WHERE parent.org_id = NEW.org_id
            )
            SELECT 1 FROM ancestors
             WHERE archived_at IS NOT NULL OR trashed_at IS NOT NULL
       ) THEN
        RAISE EXCEPTION 'restore parent accounts first'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_accounts_parent_inactive';
    END IF;

    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.validate_account_mapping_definition()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    definition accounting.account_mapping_definitions%ROWTYPE;
    account_record accounting.accounts%ROWTYPE;
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);
    SELECT * INTO definition
      FROM accounting.account_mapping_definitions
     WHERE role = NEW.mapping_key;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'unknown functional mapping role'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_account_mappings_unknown_role';
    END IF;
    IF definition.is_alias THEN
        RAISE EXCEPTION 'legacy mapping aliases are read-only'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_account_mappings_alias_read_only';
    END IF;

    SELECT * INTO account_record
      FROM accounting.accounts
     WHERE org_id = NEW.org_id AND id = NEW.account_id
     FOR SHARE;
    IF NOT FOUND
       OR NOT account_record.posting_allowed
       OR account_record.archived_at IS NOT NULL
       OR account_record.trashed_at IS NOT NULL
       OR NOT account_record.account_class =
            ANY(definition.compatible_account_classes)
       OR NOT account_record.normal_balance =
            ANY(definition.compatible_normal_balances)
       OR NOT account_record.monetary_class =
            ANY(definition.compatible_monetary_classes) THEN
        RAISE EXCEPTION 'account is incompatible with functional mapping'
            USING ERRCODE = '23514',
                  CONSTRAINT = 'accounting_account_mappings_incompatible';
    END IF;
    RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.account_event_snapshot_hash(
    p_org_id uuid,
    p_subject_type text,
    p_account_id uuid,
    p_mapping_key text,
    p_action text,
    p_actor text,
    p_reason text,
    p_version bigint,
    p_before_snapshot jsonb,
    p_after_snapshot jsonb
)
RETURNS text
LANGUAGE sql
IMMUTABLE
SET search_path = pg_catalog, accounting
RETURN encode(
    public.digest(
        convert_to(
            jsonb_build_object(
                'org_id', p_org_id,
                'subject_type', p_subject_type,
                'account_id', p_account_id,
                'mapping_key', p_mapping_key,
                'action', p_action,
                'actor', p_actor,
                'reason', p_reason,
                'version', p_version,
                'before_snapshot', p_before_snapshot,
                'after_snapshot', p_after_snapshot
            )::text,
            'UTF8'
        ),
        'sha256'
    ),
    'hex'
);

CREATE TABLE accounting.account_events (
    org_id uuid NOT NULL
        REFERENCES iam.organizations(id) ON DELETE RESTRICT,
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    subject_type text NOT NULL CHECK (
        subject_type IN ('account', 'mapping')
    ),
    account_id uuid,
    mapping_key text,
    action text NOT NULL CHECK (
        action IN (
            'create', 'update', 'archive', 'restore', 'trash', 'mapping_set'
        )
    ),
    actor text NOT NULL CHECK (btrim(actor) <> ''),
    reason text NOT NULL CHECK (btrim(reason) <> ''),
    version bigint NOT NULL CHECK (version > 0),
    before_snapshot jsonb,
    after_snapshot jsonb NOT NULL CHECK (
        jsonb_typeof(after_snapshot) = 'object'
    ),
    snapshot_hash text NOT NULL CHECK (snapshot_hash ~ '^[0-9a-f]{64}$'),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, id),
    CONSTRAINT accounting_account_events_account_fk
        FOREIGN KEY (org_id, account_id)
        REFERENCES accounting.accounts(org_id, id)
        ON DELETE RESTRICT,
    CONSTRAINT accounting_account_events_definition_fk
        FOREIGN KEY (mapping_key)
        REFERENCES accounting.account_mapping_definitions(role)
        ON DELETE RESTRICT,
    CHECK (
        (subject_type = 'account' AND account_id IS NOT NULL
            AND mapping_key IS NULL AND action <> 'mapping_set')
        OR
        (subject_type = 'mapping' AND account_id IS NULL
            AND mapping_key IS NOT NULL AND action = 'mapping_set')
    ),
    CHECK (
        before_snapshot IS NULL
        OR jsonb_typeof(before_snapshot) = 'object'
    ),
    CHECK (
        snapshot_hash = accounting.account_event_snapshot_hash(
            org_id, subject_type, account_id, mapping_key, action,
            actor, reason, version, before_snapshot, after_snapshot
        )
    )
);

CREATE INDEX accounting_account_events_subject_idx
    ON accounting.account_events (
        org_id, subject_type, account_id, mapping_key, occurred_at DESC
    );

CREATE OR REPLACE FUNCTION accounting.audit_account_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    event_action text;
    event_actor text;
    event_reason text;
    before_state jsonb;
    after_state jsonb;
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);
    event_actor := coalesce(
        nullif(current_setting('app.user_id', true), ''),
        nullif(current_setting('app.actor_subject', true), ''),
        'system'
    );
    IF TG_OP = 'INSERT' THEN
        event_action := 'create';
        event_reason := coalesce(
            nullif(current_setting('app.accounting_reason', true), ''),
            'Creación de cuenta'
        );
        before_state := NULL;
    ELSIF OLD.trashed_at IS NULL AND NEW.trashed_at IS NOT NULL THEN
        event_action := 'trash';
        event_reason := coalesce(
            nullif(current_setting('app.accounting_reason', true), ''),
            'Envío a papelera'
        );
        before_state := to_jsonb(OLD) - 'org_id';
    ELSIF (OLD.archived_at IS NOT NULL OR OLD.trashed_at IS NOT NULL)
          AND NEW.archived_at IS NULL AND NEW.trashed_at IS NULL THEN
        event_action := 'restore';
        event_reason := coalesce(
            nullif(current_setting('app.accounting_reason', true), ''),
            'Restauración de cuenta'
        );
        before_state := to_jsonb(OLD) - 'org_id';
    ELSIF OLD.archived_at IS NULL AND NEW.archived_at IS NOT NULL THEN
        event_action := 'archive';
        event_reason := coalesce(
            nullif(current_setting('app.accounting_reason', true), ''),
            'Archivo de cuenta'
        );
        before_state := to_jsonb(OLD) - 'org_id';
    ELSE
        event_action := 'update';
        event_reason := coalesce(
            nullif(current_setting('app.accounting_reason', true), ''),
            'Actualización de cuenta'
        );
        before_state := to_jsonb(OLD) - 'org_id';
    END IF;
    after_state := to_jsonb(NEW) - 'org_id';

    INSERT INTO accounting.account_events (
        org_id, subject_type, account_id, action, actor, reason, version,
        before_snapshot, after_snapshot, snapshot_hash
    )
    VALUES (
        NEW.org_id, 'account', NEW.id, event_action, event_actor,
        event_reason, NEW.version, before_state, after_state,
        accounting.account_event_snapshot_hash(
            NEW.org_id, 'account', NEW.id, NULL, event_action, event_actor,
            event_reason, NEW.version, before_state, after_state
        )
    );
    RETURN NULL;
END
$function$;

CREATE OR REPLACE FUNCTION accounting.audit_account_mapping_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, accounting
AS $function$
DECLARE
    event_actor text;
    event_reason text;
    before_state jsonb;
    after_state jsonb;
BEGIN
    PERFORM app.assert_org_context(NEW.org_id);
    event_actor := coalesce(
        nullif(current_setting('app.user_id', true), ''),
        nullif(current_setting('app.actor_subject', true), ''),
        'system'
    );
    event_reason := coalesce(
        nullif(current_setting('app.accounting_reason', true), ''),
        'Actualización de mapping funcional'
    );
    before_state := CASE
        WHEN TG_OP = 'INSERT' THEN NULL
        ELSE to_jsonb(OLD) - 'org_id'
    END;
    after_state := to_jsonb(NEW) - 'org_id';
    INSERT INTO accounting.account_events (
        org_id, subject_type, mapping_key, action, actor, reason, version,
        before_snapshot, after_snapshot, snapshot_hash
    )
    VALUES (
        NEW.org_id, 'mapping', NEW.mapping_key, 'mapping_set', event_actor,
        event_reason, NEW.version, before_state, after_state,
        accounting.account_event_snapshot_hash(
            NEW.org_id, 'mapping', NULL, NEW.mapping_key, 'mapping_set',
            event_actor, event_reason, NEW.version, before_state, after_state
        )
    );
    RETURN NULL;
END
$function$;

REVOKE ALL ON FUNCTION accounting.assign_template_root_system_key() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.account_has_dependencies(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.validate_account_workflow() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.validate_account_mapping_definition() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.account_event_snapshot_hash(
    uuid, text, uuid, text, text, text, text, bigint, jsonb, jsonb
) FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.audit_account_change() FROM PUBLIC;
REVOKE ALL ON FUNCTION accounting.audit_account_mapping_change() FROM PUBLIC;

DROP TRIGGER IF EXISTS accounting_accounts_template_root ON accounting.accounts;
CREATE TRIGGER accounting_accounts_template_root
BEFORE INSERT ON accounting.accounts
FOR EACH ROW EXECUTE FUNCTION accounting.assign_template_root_system_key();

DROP TRIGGER IF EXISTS accounting_accounts_workflow ON accounting.accounts;
CREATE TRIGGER accounting_accounts_workflow
BEFORE INSERT OR UPDATE ON accounting.accounts
FOR EACH ROW EXECUTE FUNCTION accounting.validate_account_workflow();

DROP TRIGGER IF EXISTS accounting_account_mappings_definition_guard
ON accounting.account_mappings;
CREATE TRIGGER accounting_account_mappings_definition_guard
BEFORE INSERT OR UPDATE ON accounting.account_mappings
FOR EACH ROW EXECUTE FUNCTION accounting.validate_account_mapping_definition();

CREATE TRIGGER accounting_accounts_audit
AFTER INSERT OR UPDATE ON accounting.accounts
FOR EACH ROW EXECUTE FUNCTION accounting.audit_account_change();

CREATE TRIGGER accounting_account_mappings_audit
AFTER INSERT OR UPDATE ON accounting.account_mappings
FOR EACH ROW EXECUTE FUNCTION accounting.audit_account_mapping_change();

CREATE TRIGGER accounting_account_events_immutable
BEFORE UPDATE OR DELETE ON accounting.account_events
FOR EACH ROW EXECUTE FUNCTION accounting.reject_immutable_change();

ALTER TABLE accounting.account_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounting.account_events FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation
ON accounting.account_events
USING (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
)
WITH CHECK (
    org_id = nullif(current_setting('app.org_id', true), '')::uuid
);

REVOKE ALL ON accounting.account_mapping_definitions FROM PUBLIC;
REVOKE ALL ON accounting.account_events FROM PUBLIC;

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        REVOKE DELETE ON accounting.accounts, accounting.account_mappings
        FROM pymes_backend;
        GRANT SELECT ON
            accounting.account_mapping_definitions,
            accounting.account_events
        TO pymes_backend;
        GRANT EXECUTE ON FUNCTION accounting.account_code_sort_key(text)
        TO pymes_backend;
        GRANT EXECUTE ON FUNCTION accounting.account_event_snapshot_hash(
            uuid, text, uuid, text, text, text, text, bigint, jsonb, jsonb
        ) TO pymes_backend;
    END IF;
END
$grant$;
