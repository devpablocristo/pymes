\set ON_ERROR_STOP on

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_roles WHERE rolname = 'pymes_v3_rls_test'
  ) THEN
    CREATE ROLE pymes_v3_rls_test;
  END IF;
END
$$;

GRANT USAGE ON SCHEMA app TO pymes_v3_rls_test;
GRANT SELECT, INSERT ON
  app.sales,
  app.organizations,
  app.accounting_reversals,
  app.organization_feature_flags
TO pymes_v3_rls_test;
GRANT SELECT ON
  app.organization_feature_flag_audit,
  app.scheduling_booking_status_configurations,
  app.scheduling_booking_substates
TO pymes_v3_rls_test;

INSERT INTO app.organizations(id,name,slug,status)
VALUES
  ('org_a','A','a','ready'),
  ('org_b','B','b','ready')
ON CONFLICT (id) DO NOTHING;

BEGIN;
SELECT set_config('app.org_id','org_a',true);
INSERT INTO app.organization_feature_flags(org_id,updated_by)
VALUES ('org_a','test')
ON CONFLICT (org_id) DO NOTHING;
COMMIT;

BEGIN;
SELECT set_config('app.org_id','org_b',true);
INSERT INTO app.organization_feature_flags(org_id,updated_by)
VALUES ('org_b','test')
ON CONFLICT (org_id) DO NOTHING;
COMMIT;

INSERT INTO app.sales(
  id,org_id,recipient_ref,point_of_sale,document_type,voucher_number,
  amount,currency,status,snapshot_digest,correlation_id,request_id,
  actor_ref,source_version
)
VALUES
  (
    'shared_sale','org_a','party',1,'FA',1,1,'ARS','posted',
    repeat('a',64),'test','request:rls:a','system:test',1
  ),
  (
    'shared_sale','org_b','party',1,'FA',1,1,'ARS','posted',
    repeat('b',64),'test','request:rls:b','system:test',1
  )
ON CONFLICT (org_id,id) DO NOTHING;

INSERT INTO app.accounting_reversals(
  id,org_id,document_kind,document_id,original_journal_entry_id,
  effective_at,reason,status,snapshot_digest,correlation_id,request_id,
  actor_ref,source_version
)
VALUES
  (
    'shared_reversal','org_a','purchase','purchase_a','journal_a',
    now(),'test','requested',repeat('a',64),'test','request:reversal:a',
    'system:test',1
  ),
  (
    'shared_reversal','org_b','purchase','purchase_b','journal_b',
    now(),'test','requested',repeat('b',64),'test','request:reversal:b',
    'system:test',1
  )
ON CONFLICT (org_id,id) DO NOTHING;

INSERT INTO app.scheduling_booking_status_configurations(
  org_id,status,label
)
VALUES
  ('org_a','confirmed','Confirmado A'),
  ('org_b','confirmed','Confirmado B')
ON CONFLICT (org_id,status) DO UPDATE SET label=EXCLUDED.label;

INSERT INTO app.scheduling_booking_substates(
  org_id,status,code,label,active,sort_order
)
VALUES
  ('org_a','confirmed','vip','VIP A',true,10),
  ('org_b','confirmed','vip','VIP B',true,10)
ON CONFLICT (org_id,status,code) DO UPDATE SET label=EXCLUDED.label;

SET ROLE pymes_v3_rls_test;
SET app.org_id = 'org_a';

DO $$
DECLARE
  sale_count bigint;
  reversal_count bigint;
  feature_count bigint;
  status_count bigint;
  substate_count bigint;
BEGIN
  SELECT count(*) INTO sale_count FROM app.sales;
  SELECT count(*) INTO reversal_count FROM app.accounting_reversals;
  SELECT count(*) INTO feature_count FROM app.organization_feature_flags;
  SELECT count(*) INTO status_count
    FROM app.scheduling_booking_status_configurations;
  SELECT count(*) INTO substate_count
    FROM app.scheduling_booking_substates;
  IF sale_count <> 1 OR
     reversal_count <> 1 OR
     feature_count <> 1 OR
     status_count <> 1 OR
     substate_count <> 1 THEN
    RAISE EXCEPTION
      'RLS isolation failed: sales=%, reversals=%, features=%, statuses=%, substates=%',
      sale_count,
      reversal_count,
      feature_count,
      status_count,
      substate_count;
  END IF;
END
$$;

RESET ROLE;
