#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=scripts/seeds/lib.sh
source "$ROOT_DIR/scripts/seeds/lib.sh"

TARGET_ORG_UUID="$(resolve_target_org_uuid)"
LOCAL_USER_UUID="00000000-0000-0000-0000-000000000002"
LOCAL_API_KEY_UUID="00000000-0000-0000-0000-000000000004"

run_pymes_sql_inline "
DO \$\$
DECLARE
    v_org uuid := '${TARGET_ORG_UUID}';
    p1 uuid;
    p2 uuid;
    p3 uuid;
    svc1 uuid;
    svc2 uuid;
    legacy_demo_service_product_1 uuid;
    legacy_demo_service_product_2 uuid;
    c1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/1');
    c2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/2');
    c3 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/customer/3');
    s1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/1');
    s2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/supplier/2');
    q1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/quote/1');
    sale1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/1');
    sale2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/sale/2');
    pl_default uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/price-list/default');
    rec1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/recurring/1');
    rec2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/recurring/2');
    pur1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/purchase/1');
    pur2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/purchase/2');
    pr1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/procurement/1');
    wh1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/webhook/1');
    r_admin uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/role/admin');
    r_seller uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/role/vendedor');
    r_cashier uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/role/cajero');
    r_accountant uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/role/contador');
    r_warehouse uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/role/almacenero');
    notif_id uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/in-app-notif/demo-welcome');
    sched_branch uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/branch/central');
    sched_service uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/service/general_consultation');
    sched_catchall_service uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/service/general_appointment');
    sched_resource uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/resource/professional-1');
    sched_queue uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/queue/frontdesk');
BEGIN
    SELECT id INTO p1 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-001' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO p2 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-002' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO p3 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-003' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO svc1 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-001' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO svc2 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-002' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO legacy_demo_service_product_1 FROM products WHERE org_id = v_org AND sku = 'DEMO-SVC-001' LIMIT 1;
    SELECT id INTO legacy_demo_service_product_2 FROM products WHERE org_id = v_org AND sku = 'DEMO-SVC-002' LIMIT 1;

    DELETE FROM pymes_in_app_notifications WHERE id = notif_id;

    DELETE FROM procurement_request_lines WHERE request_id = pr1;
    DELETE FROM procurement_requests WHERE id = pr1;

    DELETE FROM purchase_items WHERE purchase_id IN (pur1, pur2);
    DELETE FROM purchases WHERE id IN (pur1, pur2);

    DELETE FROM recurring_expenses WHERE id IN (rec1, rec2);
    DELETE FROM webhook_endpoints WHERE id = wh1;
    DELETE FROM accounts
    WHERE id IN (
      uuid_generate_v5(v_org, 'pymes-seed/v1/account/receivable-c1'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/account/payable-s1')
    );

    DELETE FROM cash_movements
    WHERE id IN (
      uuid_generate_v5(v_org, 'pymes-seed/v1/cash-move/1'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/cash-move/2')
    );
    DELETE FROM stock_movements
    WHERE id IN (
      uuid_generate_v5(v_org, 'pymes-seed/v1/stock-move/1'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/stock-move/2')
    );

    DELETE FROM sale_items WHERE sale_id IN (sale1, sale2);
    DELETE FROM sales WHERE id IN (sale1, sale2);
    DELETE FROM quote_items WHERE quote_id = q1;
    DELETE FROM quotes WHERE id = q1;

    DELETE FROM stock_levels WHERE org_id = v_org AND product_id IN (p1, p2, p3);

    DELETE FROM price_list_items WHERE price_list_id = pl_default;
    DELETE FROM price_lists WHERE id = pl_default;

    DELETE FROM user_roles WHERE org_id = v_org;
    DELETE FROM role_permissions WHERE role_id IN (r_admin, r_seller, r_cashier, r_accountant, r_warehouse);
    DELETE FROM roles WHERE id IN (r_admin, r_seller, r_cashier, r_accountant, r_warehouse);

    DELETE FROM scheduling_queue_tickets WHERE id IN (
      uuid_generate_v5(v_org, 'modules-scheduling/v1/ticket/demo-1'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/ticket/demo-2'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/ticket/demo-3')
    );
    -- Turnos que usan catálogo semilla (demo + p. ej. copia migración 0041 appointments → scheduling_bookings)
    IF to_regclass('public.scheduling_booking_action_tokens') IS NOT NULL THEN
      DELETE FROM scheduling_booking_action_tokens
      WHERE booking_id IN (
        SELECT id FROM scheduling_bookings
        WHERE org_id = v_org
          AND (
            resource_id = sched_resource
            OR branch_id = sched_branch
            OR service_id = sched_service
          )
      );
    END IF;
    DELETE FROM scheduling_bookings
    WHERE org_id = v_org
      AND (
        resource_id = sched_resource
        OR branch_id = sched_branch
        OR service_id = sched_service
      );
    DELETE FROM scheduling_service_resources
    WHERE service_id IN (sched_service, sched_catchall_service)
       OR resource_id = sched_resource;
    DELETE FROM scheduling_availability_rules
    WHERE id IN (
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/0'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/1'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/2'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/3'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/4'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/5'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/6'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/0'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/1'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/2'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/3'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/4'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/5'),
      uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/6')
    ) OR id IN (
      SELECT uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/weekday/' || gs::text || '/am')
      FROM generate_series(1, 5) AS gs
      UNION ALL
      SELECT uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/branch/weekday/' || gs::text || '/pm')
      FROM generate_series(1, 5) AS gs
      UNION ALL
      SELECT uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/weekday/' || gs::text || '/am')
      FROM generate_series(1, 5) AS gs
      UNION ALL
      SELECT uuid_generate_v5(v_org, 'modules-scheduling/v1/rule/resource/weekday/' || gs::text || '/pm')
      FROM generate_series(1, 5) AS gs
    );
    DELETE FROM scheduling_queues WHERE id = sched_queue;
    DELETE FROM scheduling_resources WHERE id = sched_resource;
    DELETE FROM scheduling_services WHERE id IN (sched_service, sched_catchall_service);
    DELETE FROM scheduling_branches WHERE id = sched_branch;

    DELETE FROM org_api_key_scopes WHERE api_key_id = '${LOCAL_API_KEY_UUID}';
    DELETE FROM org_api_keys WHERE id = '${LOCAL_API_KEY_UUID}';

    DELETE FROM services WHERE org_id = v_org AND id IN (svc1, svc2);
    DELETE FROM products
    WHERE org_id = v_org
      AND (
        id IN (p1, p2, p3, legacy_demo_service_product_1, legacy_demo_service_product_2)
        OR sku IN ('DEMO-PROD-001', 'DEMO-PROD-002', 'DEMO-PROD-003', 'DEMO-SVC-001', 'DEMO-SVC-002')
      );

    DELETE FROM party_roles WHERE org_id = v_org AND party_id IN (c1, c2, c3, s1, s2);
    DELETE FROM party_persons WHERE party_id IN (c1, c3);
    DELETE FROM party_organizations WHERE party_id IN (c2, s1, s2);
    DELETE FROM parties WHERE id IN (c1, c2, c3, s1, s2);
END \$\$;
"

if [[ -z "${PYMES_SEED_DEMO_ORG_EXTERNAL_ID:-}" ]]; then
  run_pymes_sql_inline "
  DELETE FROM org_members WHERE org_id = '${TARGET_ORG_UUID}' AND user_id = '${LOCAL_USER_UUID}';
  DELETE FROM tenant_settings WHERE org_id = '${TARGET_ORG_UUID}';
  DELETE FROM users WHERE id = '${LOCAL_USER_UUID}';
  DELETE FROM orgs WHERE id = '${TARGET_ORG_UUID}';
  "
fi
