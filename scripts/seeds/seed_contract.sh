#!/usr/bin/env bash
# Central contract for visible demo seed coverage.

SEED_MIN_VISIBLE="${SEED_MIN_VISIBLE:-10}"

# Format: name|min|sql
# Use __ORG_ID__ as a placeholder resolved by verify.sh.
SEED_DB_CHECKS=(
  "customers|$SEED_MIN_VISIBLE|SELECT count(*) FROM party_roles pr JOIN parties p ON p.id = pr.party_id WHERE pr.tenant_id = '__ORG_ID__'::uuid AND pr.role = 'customer' AND pr.is_active = true AND p.deleted_at IS NULL"
  "suppliers|$SEED_MIN_VISIBLE|SELECT count(*) FROM party_roles pr JOIN parties p ON p.id = pr.party_id WHERE pr.tenant_id = '__ORG_ID__'::uuid AND pr.role = 'supplier' AND pr.is_active = true AND p.deleted_at IS NULL"
  "products|$SEED_MIN_VISIBLE|SELECT count(*) FROM products WHERE tenant_id = '__ORG_ID__'::uuid AND deleted_at IS NULL"
  "services|$SEED_MIN_VISIBLE|SELECT count(*) FROM services WHERE tenant_id = '__ORG_ID__'::uuid AND deleted_at IS NULL"
  "quotes|$SEED_MIN_VISIBLE|SELECT count(*) FROM quotes WHERE tenant_id = '__ORG_ID__'::uuid AND archived_at IS NULL"
  "sales|$SEED_MIN_VISIBLE|SELECT count(*) FROM sales WHERE tenant_id = '__ORG_ID__'::uuid AND voided_at IS NULL"
  "purchases|$SEED_MIN_VISIBLE|SELECT count(*) FROM purchases WHERE tenant_id = '__ORG_ID__'::uuid AND deleted_at IS NULL"
  "invoices|$SEED_MIN_VISIBLE|SELECT count(*) FROM invoices WHERE tenant_id = '__ORG_ID__'::uuid AND deleted_at IS NULL"
  "employees|$SEED_MIN_VISIBLE|SELECT count(*) FROM employees WHERE tenant_id = '__ORG_ID__'::uuid AND deleted_at IS NULL"
  "inventory|$SEED_MIN_VISIBLE|SELECT count(*) FROM stock_levels sl JOIN products p ON p.id = sl.product_id WHERE sl.tenant_id = '__ORG_ID__'::uuid AND p.deleted_at IS NULL"
  "cashflow|$SEED_MIN_VISIBLE|SELECT count(*) FROM cash_movements WHERE tenant_id = '__ORG_ID__'::uuid AND deleted_at IS NULL"
  "returns|$SEED_MIN_VISIBLE|SELECT count(*) FROM returns WHERE tenant_id = '__ORG_ID__'::uuid AND deleted_at IS NULL"
  "creditNotes|$SEED_MIN_VISIBLE|SELECT count(*) FROM credit_notes WHERE tenant_id = '__ORG_ID__'::uuid"
  "recurring|$SEED_MIN_VISIBLE|SELECT count(*) FROM recurring_expenses WHERE tenant_id = '__ORG_ID__'::uuid AND is_active = true AND deleted_at IS NULL"
  "payments|$SEED_MIN_VISIBLE|SELECT count(*) FROM payments WHERE tenant_id = '__ORG_ID__'::uuid AND deleted_at IS NULL"
  "accounts|$SEED_MIN_VISIBLE|SELECT count(*) FROM accounts WHERE tenant_id = '__ORG_ID__'::uuid"
  "notifications|$SEED_MIN_VISIBLE|SELECT count(*) FROM pymes_in_app_notifications WHERE tenant_id = '__ORG_ID__'::uuid"
  "agenda|$SEED_MIN_VISIBLE|SELECT count(*) FROM scheduling_bookings WHERE org_id = '__ORG_ID__'::uuid AND start_at >= CURRENT_DATE - interval '1 day' AND start_at < CURRENT_DATE + interval '14 days'"
  "vehiclesAuto|$SEED_MIN_VISIBLE|SELECT count(*) FROM workshops.customer_assets WHERE tenant_id = '__ORG_ID__'::uuid AND asset_type = 'vehicle' AND archived_at IS NULL"
  "workOrdersAuto|$SEED_MIN_VISIBLE|SELECT count(*) FROM workshops.work_orders WHERE tenant_id = '__ORG_ID__'::uuid AND asset_type = 'vehicle' AND archived_at IS NULL"
  "bicyclesBike|$SEED_MIN_VISIBLE|SELECT count(*) FROM workshops.customer_assets WHERE tenant_id = '__ORG_ID__'::uuid AND asset_type = 'bicycle' AND archived_at IS NULL"
  "workOrdersBike|$SEED_MIN_VISIBLE|SELECT count(*) FROM workshops.work_orders WHERE tenant_id = '__ORG_ID__'::uuid AND asset_type = 'bicycle' AND archived_at IS NULL"
  "restaurantDiningAreas|$SEED_MIN_VISIBLE|SELECT count(*) FROM restaurant.dining_areas WHERE tenant_id = '__ORG_ID__'::uuid"
  "restaurantDiningTables|$SEED_MIN_VISIBLE|SELECT count(*) FROM restaurant.dining_tables WHERE tenant_id = '__ORG_ID__'::uuid"
  "restaurantTableSessions|$SEED_MIN_VISIBLE|SELECT count(*) FROM restaurant.table_sessions WHERE tenant_id = '__ORG_ID__'::uuid"
  "professionalsProfiles|$SEED_MIN_VISIBLE|SELECT count(*) FROM professionals.professional_profiles WHERE tenant_id = '__ORG_ID__'::uuid"
  "professionalsSpecialties|$SEED_MIN_VISIBLE|SELECT count(*) FROM professionals.specialties WHERE tenant_id = '__ORG_ID__'::uuid"
  "professionalsIntakes|$SEED_MIN_VISIBLE|SELECT count(*) FROM professionals.intakes WHERE tenant_id = '__ORG_ID__'::uuid"
  "professionalsSessions|$SEED_MIN_VISIBLE|SELECT count(*) FROM professionals.sessions WHERE tenant_id = '__ORG_ID__'::uuid"
)

# Format: name|min|base-env-var|path
# Paths may use __FROM__ and __TO__, replaced by verify.sh.
SEED_API_CHECKS=(
  "customers|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/customers?limit=50"
  "suppliers|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/suppliers?limit=50"
  "products|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/products?limit=50"
  "services|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/services?limit=50"
  "quotes|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/quotes?limit=50"
  "sales|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/sales?limit=50"
  "purchases|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/purchases?limit=50"
  "invoices|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/invoices?limit=50"
  "employees|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/employees?limit=50"
  "inventory|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/inventory?limit=50"
  "cashflow|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/cashflow?limit=50"
  "returns|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/returns?limit=50"
  "creditNotes|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/credit-notes?limit=50"
  "recurring|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/recurring-expenses?limit=50"
  "agenda|$SEED_MIN_VISIBLE|SEED_VERIFY_CORE_URL|/v1/scheduling/bookings?from=__FROM__&to=__TO__&limit=100"
  "vehiclesAuto|$SEED_MIN_VISIBLE|SEED_VERIFY_WORKSHOPS_URL|/v1/auto-repair/vehicles?limit=50"
  "workOrdersAuto|$SEED_MIN_VISIBLE|SEED_VERIFY_WORKSHOPS_URL|/v1/auto-repair/work-orders?asset_type=vehicle&limit=50"
  "bicyclesBike|$SEED_MIN_VISIBLE|SEED_VERIFY_WORKSHOPS_URL|/v1/bike-shop/bicycles?limit=50"
  "workOrdersBike|$SEED_MIN_VISIBLE|SEED_VERIFY_WORKSHOPS_URL|/v1/bike-shop/work-orders?asset_type=bicycle&limit=50"
  "restaurantDiningAreas|$SEED_MIN_VISIBLE|SEED_VERIFY_RESTAURANTS_URL|/v1/restaurants/dining-areas?limit=50"
  "restaurantDiningTables|$SEED_MIN_VISIBLE|SEED_VERIFY_RESTAURANTS_URL|/v1/restaurants/dining-tables?limit=50"
  "restaurantTableSessions|$SEED_MIN_VISIBLE|SEED_VERIFY_RESTAURANTS_URL|/v1/restaurants/table-sessions?open_only=false&limit=50"
  "professionalsProfiles|$SEED_MIN_VISIBLE|SEED_VERIFY_PROFESSIONALS_URL|/v1/teachers/professionals?limit=50"
  "professionalsSpecialties|$SEED_MIN_VISIBLE|SEED_VERIFY_PROFESSIONALS_URL|/v1/teachers/specialties?limit=50"
  "professionalsIntakes|$SEED_MIN_VISIBLE|SEED_VERIFY_PROFESSIONALS_URL|/v1/teachers/intakes?limit=50"
  "professionalsSessions|$SEED_MIN_VISIBLE|SEED_VERIFY_PROFESSIONALS_URL|/v1/teachers/sessions?limit=50"
)
