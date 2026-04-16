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
    notif_welcome uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/in-app-notif/demo-welcome');
    notif_sales_weekly uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/in-app-notif/sales-weekly');
    notif_collections uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/in-app-notif/collections-followup');
    notif_stock uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/in-app-notif/stock-alert');
    notif_review uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/in-app-notif/review-approval');
    notif_customer_winback uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/in-app-notif/customer-winback');
    sched_branch uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/branch/central');
    sched_service uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/service/general_consultation');
    sched_catchall_service uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/service/general_appointment');
    sched_resource uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/resource/professional-1');
    sched_queue uuid := uuid_generate_v5(v_org, 'modules-scheduling/v1/queue/frontdesk');
    emp_party uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/employee/1');
    prof_party uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/party/1');
    prof_profile uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/profile/1');
    spec1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/clinical');
    spec2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/specialty/pediatrics');
    intake1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/intake/1');
    sess1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/professional/session/1');
    area_main uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/main');
    area_terrace uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/area/terrace');
    tbl_m1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/main-1');
    tbl_m2 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/main-2');
    tbl_t1 uuid := uuid_generate_v5(v_org, 'pymes-seed/v1/restaurant/table/terrace-1');
BEGIN
    SELECT id INTO p1 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-001' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO p2 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-002' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO p3 FROM products WHERE org_id = v_org AND sku = 'DEMO-PROD-003' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO svc1 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-001' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO svc2 FROM services WHERE org_id = v_org AND code = 'DEMO-SVC-002' AND deleted_at IS NULL LIMIT 1;
    SELECT id INTO legacy_demo_service_product_1 FROM products WHERE org_id = v_org AND sku = 'DEMO-SVC-001' LIMIT 1;
    SELECT id INTO legacy_demo_service_product_2 FROM products WHERE org_id = v_org AND sku = 'DEMO-SVC-002' LIMIT 1;

    DELETE FROM pymes_in_app_notifications
    WHERE id IN (
      notif_welcome,
      notif_sales_weekly,
      notif_collections,
      notif_stock,
      notif_review,
      notif_customer_winback
    );

    DELETE FROM org_members
    WHERE org_id = v_org
      AND user_id = '${LOCAL_USER_UUID}';

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

    -- Incluye tanto los ids hardcodeados en los SQL como los que genera el backend
    -- al procesar ventas/compras semilla (misma marca created_by='seed').
    DELETE FROM cash_movements WHERE org_id = v_org AND created_by = 'seed';
    DELETE FROM stock_movements WHERE org_id = v_org AND created_by = 'seed';

    -- Notas de crédito + devoluciones + cobros (dependen de sales/purchases)
    DELETE FROM credit_notes
    WHERE id IN (
      uuid_generate_v5(v_org, 'pymes-seed/v1/credit-note/1'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/credit-note/2')
    );
    DELETE FROM returns WHERE id = uuid_generate_v5(v_org, 'pymes-seed/v1/return/1');
    DELETE FROM payments WHERE id IN (
      uuid_generate_v5(v_org, 'pymes-seed/v1/payment/sale/1'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/payment/sale/2'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/payment/purchase/1')
    );

    -- Governance + control demo
    DELETE FROM attachments WHERE id IN (
      uuid_generate_v5(v_org, 'pymes-seed/v1/attachment/1'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/attachment/2')
    );
    DELETE FROM timeline_entries WHERE id IN (
      uuid_generate_v5(v_org, 'pymes-seed/v1/timeline/1'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/timeline/2')
    );
    DELETE FROM audit_log WHERE id IN (
      uuid_generate_v5(v_org, 'pymes-seed/v1/audit/1'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/audit/2')
    );
    DELETE FROM procurement_policies WHERE id IN (
      uuid_generate_v5(v_org, 'pymes-seed/v1/procurement-policy/1'),
      uuid_generate_v5(v_org, 'pymes-seed/v1/procurement-policy/2')
    );

    -- Professionals vertical
    DELETE FROM professionals.session_notes WHERE session_id = sess1;
    DELETE FROM professionals.sessions WHERE id = sess1;
    DELETE FROM professionals.intakes WHERE id = intake1;
    DELETE FROM professionals.professional_specialties
    WHERE org_id = v_org AND profile_id = prof_profile;
    DELETE FROM professionals.professional_profiles WHERE id = prof_profile;
    DELETE FROM professionals.specialties WHERE id IN (spec1, spec2);

    -- Restaurants vertical
    DELETE FROM restaurant.dining_tables WHERE id IN (tbl_m1, tbl_m2, tbl_t1);
    DELETE FROM restaurant.dining_areas WHERE id IN (area_main, area_terrace);

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

    DELETE FROM party_roles WHERE org_id = v_org AND party_id IN (c1, c2, c3, s1, s2, emp_party, prof_party);
    DELETE FROM party_persons WHERE party_id IN (c1, c3, emp_party, prof_party);
    DELETE FROM party_organizations WHERE party_id IN (c2, s1, s2);
    DELETE FROM parties WHERE id IN (c1, c2, c3, s1, s2, emp_party, prof_party);
END \$\$;
"

run_pymes_sql_inline "
DELETE FROM users WHERE id = '${LOCAL_USER_UUID}';
"

if [[ -z "${PYMES_SEED_DEMO_ORG_EXTERNAL_ID:-}" ]]; then
  run_pymes_sql_inline "
  DELETE FROM tenant_settings WHERE org_id = '${TARGET_ORG_UUID}';
  DELETE FROM orgs WHERE id = '${TARGET_ORG_UUID}';
  "
fi
