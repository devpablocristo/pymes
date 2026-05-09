# Migrations Audit — Repo `pymes`

> **Snapshot Fase A** del plan de reconstrucción de la capa de migraciones (`.claude/plans/tengo-un-bug-en-melodic-river.md`). Inventario crudo + diagnóstico, sin parches. Fuente para diseñar el squash.
>
> Generado: 2026-05-09 · Total archivos auditados: **125** · Tablas creadas: **99** · Drift crítico cross-source: **5 tablas**.

## 1. Resumen ejecutivo

| Backend | Total | Idempotentes | Con .down completo | Sin / trivial |
|---|---|---|---|---|
| pymes-core | 78 | 12 | 74 | 4 |
| professionals | 8 | 1 | 8 | 0 |
| workshops | 22 | 3 | 22 | 0 |
| beauty | 5 | 0 | 5 | 0 |
| restaurants | 4 | 0 | 4 | 0 |
| seeds (pymes-core) | 6 | 6 | 6 | 0 |
| `core/saas/go` (lib externa) | 2 | 0 | 0 | 2 |
| **TOTAL** | **125** | **22** | **119** | **6** |

**Hallazgos clave**:
- **103 / 125 migraciones (82,4 %)** no son completamente idempotentes (re-run failure si DB está parcialmente migrada).
- **6 migraciones** sin `.down.sql` o con down trivial (no reversible).
- **Drift cross-source crítico**: 5 tablas creadas tanto por `pymes-core/0001..0003` como por `core/saas/go/0001` con schemas distintos (`users`, `tenant_settings`, `admin_activity_events`, `notification_log`, `notification_preferences`).
- **0 RLS policies** declaradas en todas las migraciones (multi-tenant isolation depende del filtrado en Go).
- **0 triggers / functions** definidos (`updated_at` se maneja en runtime Go, sin garantía consistencia).
- **0 transacciones explícitas** (`BEGIN; ... COMMIT;`) — golang-migrate envuelve cada archivo, pero ALTERs múltiples no quedan atómicas.
- **2 implementaciones de UUID** en paralelo (`uuid-ossp` + `gen_random_uuid()`).

---

## 2. Tabla maestra (125 migraciones)

| # | Backend | File | Propósito | Tablas creadas | Tablas alteradas | Idx/cstr | Idempotente | .down completo |
|---|---|---|---|---|---|---|---|---|
| 1 | pymes-core | `0001_base_schema.up.sql` | base schema (legacy tenants) | admin_activity_events, audit_log, tenant_api_keys, tenant_api_key_scopes, tenant_memberships, tenants, tenant_settings, tenant_usage_counters, users | — | 23 | parcial | sí |
| 2 | pymes-core | `0002_billing.up.sql` | billing fields | — | tenant_settings | 4 | parcial | sí |
| 3 | pymes-core | `0003_notifications.up.sql` | notifications | notification_log, notification_preferences | — | 6 | parcial | sí |
| 4 | pymes-core | `0004_local_seed.up.sql` | local seed | — | — | 0 | no | sí |
| 5 | pymes-core | `0005_core_business.up.sql` | core business (sales/customers/...) | cash_movements, customers, products, quote_items, quotes, sale_items, sales, stock_levels, stock_movements, suppliers | — | 39 | parcial | sí |
| 6 | pymes-core | `0006_tenant_business_settings.up.sql` | tenant business settings | — | tenant_settings | 0 | parcial | sí |
| 7 | pymes-core | `0007_core_seed.up.sql` | core seed | — | — | 0 | no | sí |
| 8 | pymes-core | `0008_sales_voided_at.up.sql` | sales soft-void | — | sales | 0 | parcial | sí |
| 9 | pymes-core | `0009_audit_log_fk.up.sql` | audit log FK | — | audit_log | 1 | no | sí |
| 10 | pymes-core | `0010_transversal_core.up.sql` | transversal core (accounts/payments/quotes/...) | account_movements, accounts, appointments, appointment_slots, credit_notes, payments, price_list_items, price_lists, purchase_items, purchases, recurring_expenses, return_items, returns | customers, products, quote_items, quotes, sale_items, sales | 40 | parcial | sí |
| 11 | pymes-core | `0011_transversal_infra.up.sql` | transversal infra (RBAC/scheduler/webhooks) | attachments, dashboard_configs, exchange_rates, role_permissions, roles, scheduler_runs, timeline_entries, user_roles, webhook_deliveries, webhook_endpoints | — | 24 | parcial | sí |
| 12 | pymes-core | `0012_tenant_settings_ext.up.sql` | tenant settings ext | — | tenant_settings | 0 | parcial | sí |
| 13 | pymes-core | `0013_rbac_seed.up.sql` | RBAC seed | — | — | 0 | no | sí |
| 14 | pymes-core | `0014_ai_tables.up.sql` | AI tables | ai_conversations, ai_dossiers, ai_usage_daily | — | 6 | parcial | sí |
| 15 | pymes-core | `0015_whatsapp_connections.up.sql` | whatsapp connections | whatsapp_connections | — | 2 | parcial | sí |
| 16 | pymes-core | `0016_payment_gateway.up.sql` | payment gateway | payment_gateway_connections, payment_gateway_webhooks | payments, tenant_settings | 6 | parcial | sí |
| 17 | pymes-core | `0017_party_model.up.sql` | party model unificado | parties, party_agents, party_organizations, party_classifications, party_contacts | accounts, ai_conversations, appointments, customers, payments, products, quotes, sales, suppliers | 43 | sí | sí |
| 18 | pymes-core | `0018_prompt02_upgrade.up.sql` | prompt02 upgrade | webhook_outbox | — | 2 | parcial | sí |
| 19 | pymes-core | `0019_payment_gateway_events.up.sql` | payment gateway events | payment_gateway_events | — | 3 | parcial | sí |
| 20 | pymes-core | `0020_ai_agent_events.up.sql` | AI agent events | ai_agent_events | — | 4 | parcial | sí |
| 21 | pymes-core | `0021_dashboard_personalizable.up.sql` | dashboard personalizable | dashboard_default_layouts, dashboard_layouts | — | 10 | parcial | sí |
| 22 | pymes-core | `0022_party_model_reconcile.up.sql` | party model reconcile | parties, party_agents, party_organizations | ai_conversations, audit_log, payment_gateway_events, payments, sales | 23 | sí | **no (`SELECT 1`)** |
| 23 | pymes-core | `0023_saas_core_schema_align.up.sql` | saas core schema align | — | tenant_api_key_scopes, tenant_api_keys, users | 0 | parcial | sí |
| 24 | pymes-core | `0024_whatsapp_full.up.sql` | whatsapp full | whatsapp_messages, whatsapp_opt_outs | whatsapp_connections | 10 | parcial | **no (file ausente)** |
| 25 | pymes-core | `0025_procurement_requests.up.sql` | procurement requests | procurement_policies, procurement_requests, procurement_request_items | — | 8 | sí | sí |
| 26 | pymes-core | `0026_procurement_policies_rbac.up.sql` | procurement policies RBAC | — | — | 0 | sí | sí |
| 27 | pymes-core | `0027_users_phone.up.sql` | users phone | — | users | 0 | parcial | sí |
| 28 | pymes-core | `0028_users_name_parts.up.sql` | users name parts | — | users | 0 | parcial | sí |
| 29 | pymes-core | `0029_tenant_supported_currencies.up.sql` | tenant supported currencies | — | tenant_settings | 0 | sí | sí |
| 30 | pymes-core | `0030_transversal_modules_seed.up.sql` | transversal modules seed | — | — | 0 | no | sí |
| 31 | pymes-core | `0031_drop_legacy_party_views.up.sql` | drop legacy party views | — | — | 0 | parcial | sí |
| 32 | pymes-core | `0032_ai_conversations_review_columns.up.sql` | AI conversations review | — | ai_conversations | 2 | parcial | sí |
| 33 | pymes-core | `0033_archive_support.up.sql` | archive support | — | appointments, quotes | 2 | parcial | sí |
| 34 | pymes-core | `0034_in_app_notifications.up.sql` | in-app notifications | in_app_notifications | — | 3 | parcial | sí |
| 35 | pymes-core | `0035_namespace_pymes_in_app_notifications.up.sql` | namespace in-app | — | in_app_notifications | 0 | parcial | sí |
| 36 | pymes-core | `0036_namespace_pymes_notifications.up.sql` | rename notification_log → pymes_notification_log | — | notification_log, notification_preferences | 0 | parcial | sí |
| 37 | pymes-core | `0037_namespace_pymes_notifications_constraints.up.sql` | rename constraints | — | notification_log, notification_preferences | 0 | parcial (rota en DB fresca) | sí |
| 38 | pymes-core | `0038_whatsapp_campaigns.up.sql` | whatsapp campaigns | whatsapp_campaign_recipients, whatsapp_campaigns | — | 6 | parcial | **no (file ausente)** |
| 39 | pymes-core | `0039_whatsapp_conversations.up.sql` | whatsapp conversations | whatsapp_conversations | whatsapp_messages | 5 | parcial | **no (file ausente)** |
| 40 | pymes-core | `0040_tenant_onboarding_profile.up.sql` | tenant onboarding profile | — | tenant_settings | 0 | parcial | sí |
| 41 | pymes-core | `0041_migrate_appointments_to_scheduling.up.sql` | migrate appointments | — | tenant_settings | 0 | parcial | sí |
| 42 | pymes-core | `0042_split_products_services.up.sql` | split products/services | catalog_services, service_price_lists, services | purchase_items, quote_items, sale_items, sales | 8 | parcial | sí |
| 43 | pymes-core | `0043_rename_service_tables.up.sql` | rename service tables | — | services | 0 | parcial | sí |
| 44 | pymes-core | `0044_dashboard_services_widget.up.sql` | dashboard services widget | — | — | 0 | no | sí |
| 45 | pymes-core | `0045_active_products_are_products.up.sql` | active products | — | products | 1 | no | sí |
| 46 | pymes-core | `0046_accounts_type_party_unique.up.sql` | accounts type party UNIQUE | — | — | 2 | parcial | sí |
| 47 | pymes-core | `0047_drop_legacy_dashboard_layouts.up.sql` | drop legacy dashboard | — | — | 0 | parcial | sí |
| 48 | pymes-core | `0048_drop_legacy_dashboard_catalog_and_preferences.up.sql` | drop legacy dashboard prefs | — | — | 0 | parcial | sí |
| 49 | pymes-core | `0049_products_services_is_active.up.sql` | products/services is_active | — | products, services | 0 | parcial | sí |
| 50 | pymes-core | `0050_drop_superseded_appointments.up.sql` | drop appointments | — | tenant_settings | 0 | parcial | **no (file ausente)** |
| 51 | pymes-core | `0051_english_role_names.up.sql` | rename role names | — | — | 0 | no | sí |
| 52 | pymes-core | `0052_products_image_url.up.sql` | products image_url | — | products | 0 | parcial | sí |
| 53 | pymes-core | `0053_credit_notes_optional_return.up.sql` | credit notes optional return | — | credit_notes | 0 | no | sí |
| 54 | pymes-core | `0054_calendar_export_tokens.up.sql` | calendar export tokens | calendar_export_tokens | — | 4 | parcial | sí |
| 55 | pymes-core | `0055_calendar_sync_connections.up.sql` | calendar sync connections | calendar_sync_connections, calendar_sync_errors | — | 4 | parcial | sí |
| 56 | pymes-core | `0056_products_image_urls.up.sql` | products image_urls (jsonb) | — | products | 0 | parcial | sí |
| 57 | pymes-core | `0057_business_insight_candidates.up.sql` | business insights | pymes_business_insight_candidates | — | 4 | parcial | sí |
| 58 | pymes-core | `0058_sales_branch.up.sql` | sales branch | — | sales | 1 | parcial | sí |
| 59 | pymes-core | `0059_quotes_branch.up.sql` | quotes branch | — | quotes | 1 | parcial | sí |
| 60 | pymes-core | `0060_purchases_branch.up.sql` | purchases branch | — | purchases | 1 | parcial | sí |
| 61 | pymes-core | `0061_quotes_purchases_tags_metadata.up.sql` | tags/metadata | — | purchases, quotes | 0 | parcial | sí |
| 62 | pymes-core | `0062_sales_tags_metadata.up.sql` | sales tags/metadata | — | sales | 0 | parcial | sí |
| 63 | pymes-core | `0063_favorites.up.sql` | favorites | — | parties, products, services | 0 | parcial | sí |
| 64 | pymes-core | `0064_purchases_internal_fields.up.sql` | purchases internal fields | — | purchases | 0 | parcial | sí |
| 65 | pymes-core | `0065_purchases_deleted_at.up.sql` | purchases deleted_at | — | purchases | 1 | parcial | sí |
| 66 | pymes-core | `0066_internal_fields_commercial.up.sql` | internal fields commercial | — | price_lists, quotes, recurring_expenses | 0 | parcial | sí |
| 67 | pymes-core | `0067_price_lists_recurring_deleted_at.up.sql` | deleted_at | — | price_lists, recurring_expenses | 2 | parcial | sí |
| 68 | pymes-core | `0068_transactional_internal_fields.up.sql` | transactional internal fields | — | cash_movements, payments, return_items, returns | 3 | parcial | sí |
| 69 | pymes-core | `0069_invoices.up.sql` | invoices | invoice_line_items, invoices | — | 7 | parcial | sí |
| 70 | pymes-core | `0070_employees.up.sql` | employees | employees | — | 5 | parcial | sí |
| 71 | pymes-core | `0071_agent_readiness.up.sql` | agent readiness | agent_confirmations, agent_idempotency_keys | audit_log, payments | 10 | parcial | sí |
| 72 | pymes-core | `0072_inventory_branch.up.sql` | inventory branch | — | stock_levels, stock_movements | 6 | parcial | sí |
| 73 | pymes-core | `0073_cashflow_branch.up.sql` | cashflow branch | — | cash_movements, sales | 2 | parcial | sí |
| 74 | pymes-core | `0074_employees_metadata.up.sql` | employees metadata | — | employees | 0 | parcial | sí |
| 75 | pymes-core | `0075_tenant_access_model.up.sql` | tenant access model | tenant_invitations | org_api_key_scopes, org_api_keys, users | 10 | sí | sí |
| 76 | pymes-core | `0076_drop_procurement_policies.up.sql` | drop procurement policies | — | — | 0 | parcial | sí |
| 77 | pymes-core | `0077_complete_tenant_schema_rename.up.sql` | tenant → org rename | — | org_api_key_scopes, org_api_keys, users, tenants, tenant_memberships, notification_preferences, notification_log | 6 | sí | sí |
| 78 | pymes-core | `0078_webhook_events_clerk.up.sql` | webhook events clerk | webhook_events_clerk | — | 6 | parcial | sí |
| 79 | prof | `0001_professionals_schema.up.sql` | professionals schema | (7 tablas en schema professionals) | — | 19 | parcial | sí (**`DROP SCHEMA CASCADE` — incompleto**) |
| 80 | prof | `0002_service_id_adoption.up.sql` | service_id adoption | — | (varias en schema professionals) | 3 | parcial | sí |
| 81 | prof | `0003_drop_legacy_product_refs.up.sql` | drop legacy product refs | — | (schema professionals) | 1 | parcial | sí |
| 82 | prof | `0004_rename_appointment_id_to_booking_id.up.sql` | rename booking_id | — | (schema professionals) | 3 | parcial | sí |
| 83 | prof | `0005_internal_fields.up.sql` | internal fields | — | (schema professionals) | 0 | parcial | sí |
| 84 | prof | `0006_specialties_metadata.up.sql` | specialties metadata | — | (schema professionals) | 0 | parcial | sí |
| 85 | prof | `0007_archive_columns.up.sql` | archive columns | — | (schema professionals) | 4 | parcial | sí |
| 86 | prof | `0008_complete_tenant_schema_rename.up.sql` | tenant → org rename | — | (schema professionals) | 23 | sí | sí |
| 87 | work | `0001_workshops_schema.up.sql` | workshops schema | (workshops/work_orders/...) | — | 9 | parcial | sí |
| 88 | work | `0002_bike_shop.up.sql` | bike shop | bicycles | (varias) | 8 | parcial | sí |
| 89 | work | `0003_auto_repair_seed.up.sql` | auto repair seed | — | — | 0 | no | **no (`SELECT 1`)** |
| 90 | work | `0004_work_orders_kanban.up.sql` | kanban states | — | work_orders | 0 | parcial | sí |
| 91 | work | `0005_vehicles_archive.up.sql` | vehicles archive | — | vehicles | 1 | parcial | sí |
| 92 | work | `0006_services_archive.up.sql` | services archive | — | services | 1 | parcial | sí |
| 93 | work | `0007_work_orders_archive.up.sql` | work_orders archive | — | work_orders | 1 | parcial | sí |
| 94 | work | `0008_bike_shop_archive.up.sql` | bike shop archive | — | bicycles | 2 | parcial | sí |
| 95 | work | `0009_services_catalog_link.up.sql` | services catalog link | — | services | 3 | sí | sí |
| 96 | work | `0010_drop_legacy_linked_product_id.up.sql` | drop legacy product_id | — | services | 0 | parcial | sí |
| 97 | work | `0011_rename_appointment_id_to_booking_id.up.sql` | rename booking_id | — | (varias) | 0 | no | sí |
| 98 | work | `0012_consolidate_services_into_core.up.sql` | consolidar services en core | — | (varias) | 0 | sí | sí |
| 99 | work | `0013_unified_work_orders.up.sql` | unified work_orders | work_orders (re-create) | — | 6 | parcial | sí |
| 100 | work | `0014_drop_superseded_work_orders.up.sql` | drop superseded | — | (varias) | 0 | parcial | sí |
| 101 | work | `0015_drop_bicycles.up.sql` | drop bicycles | — | — | 0 | parcial | sí |
| 102 | work | `0016_work_orders_branch.up.sql` | work_orders branch | — | work_orders | 1 | parcial | sí |
| 103 | work | `0017_restore_bicycles_module.up.sql` | restore bicycles | bicycles | (varias) | 3 | parcial | sí |
| 104 | work | `0018_internal_fields.up.sql` | internal fields | — | (varias) | 0 | parcial | sí |
| 105 | work | `0019_customer_assets.up.sql` | customer assets | customer_assets | — | 3 | parcial | sí |
| 106 | work | `0020_work_orders_assets.up.sql` | work_orders assets | — | work_orders | 1 | parcial | sí |
| 107 | work | `0021_drop_work_order_target_columns.up.sql` | drop target cols | — | work_orders | 0 | parcial | sí |
| 108 | work | `0022_complete_tenant_schema_rename.up.sql` | tenant → org rename | — | — | 0 | sí | sí |
| 109 | beauty | `0001_beauty_schema.up.sql` | beauty schema | beauty_salons, beauty_stylists, beauty_salon_services | — | 4 | parcial | sí |
| 110 | beauty | `0002_services_catalog_link.up.sql` | services catalog link | — | beauty_salon_services | 3 | sí | sí |
| 111 | beauty | `0003_drop_legacy_linked_product_id.up.sql` | drop legacy product_id | — | beauty_salon_services | 0 | parcial | sí |
| 112 | beauty | `0004_drop_salon_services.up.sql` | drop salon_services | — | — | 0 | parcial | sí |
| 113 | beauty | `0005_drop_staff_members.up.sql` | drop staff_members | — | — | 0 | parcial | sí |
| 114 | rest | `0001_restaurant_schema.up.sql` | restaurant schema | restaurant_locations, dining_areas, dining_tables, reservations | — | 8 | parcial | sí |
| 115 | rest | `0002_internal_fields.up.sql` | internal fields | — | reservations | 0 | parcial | sí |
| 116 | rest | `0003_dining_metadata.up.sql` | dining metadata | — | dining_areas, dining_tables | 0 | parcial | sí |
| 117 | rest | `0004_dining_archive.up.sql` | dining archive | — | dining_areas, dining_tables | 2 | parcial | sí |
| 118 | seed | `01_clerk_prereqs.sql` | Clerk org prereqs | — | — | 0 | sí | n/a |
| 119 | seed | `02_core_business.sql` | core business demo | — | — | 0 | sí | n/a |
| 120 | seed | `03_rbac.sql` | RBAC seed | — | — | 0 | sí | n/a |
| 121 | seed | `04_full_demo.sql` | full demo | — | — | 0 | sí | n/a |
| 122 | seed | `05_scheduling_demo.sql` | scheduling demo | — | — | 0 | sí | n/a |
| 123 | seed | `06_bulk_demo.sql` | bulk demo | — | — | 0 | sí | n/a |
| 124 | saas-lib | `0001_saas_core.up.sql` | saas core schema | admin_activity_events, in_app_notifications, notification_log, notification_preferences, org_api_key_scopes, org_api_keys, org_members, org_usage_counters, orgs, saas_usage_event_dedup, tenant_settings, users, protected_resources, restore_evidence | — | 38 | parcial | **no** |
| 125 | saas-lib | `0002_org_external_ids.up.sql` | org external_id | — | orgs | 1 | parcial | **no** |

---

## 3. Mapa tabla → migración creadora

(Solo tablas con drift / colisión o renombre relevante; el resto sigue patrón 1:1.)

| Tabla | Backend | Migración creadora | Nota |
|---|---|---|---|
| **users** | pymes-core, saas-lib | 0001_base_schema.up.sql + 0001_saas_core.up.sql | **DRIFT** — schemas distintos (ver §4) |
| **tenant_settings** | pymes-core, saas-lib | 0001_base_schema.up.sql + 0001_saas_core.up.sql | **DRIFT** — PK distinta + columnas distintas |
| **admin_activity_events** | pymes-core, saas-lib | 0001_base_schema.up.sql + 0001_saas_core.up.sql | **DRIFT** — `tenant_id` vs `org_id` + payload distinto |
| **notification_log** | pymes-core (luego renombrado) + saas-lib | 0003_notifications.up.sql + 0001_saas_core.up.sql | **DRIFT** — pymes lo renombra a `pymes_notification_log` en 0036 |
| **notification_preferences** | pymes-core (luego renombrado) + saas-lib | 0003_notifications.up.sql + 0001_saas_core.up.sql | **DRIFT** — pymes lo renombra a `pymes_notification_preferences` en 0036 |
| **in_app_notifications** | pymes-core, saas-lib | 0034_in_app_notifications.up.sql + 0001_saas_core.up.sql | Pymes lo renombra a `pymes_in_app_notifications` en 0035 |
| **tenants** | pymes-core | 0001_base_schema.up.sql | Renombrada a `orgs` en 0077; squash la elimina |
| **tenant_memberships** | pymes-core | 0001_base_schema.up.sql | Renombrada a `org_members` en 0077 |
| **tenant_api_keys** | pymes-core | 0001_base_schema.up.sql | Renombrada a `org_api_keys` en 0075 |
| **tenant_api_key_scopes** | pymes-core | 0001_base_schema.up.sql | Renombrada a `org_api_key_scopes` en 0075 |
| **tenant_usage_counters** | pymes-core | 0001_base_schema.up.sql | Será reemplazada por `org_usage_counters` (saas) |
| **orgs** | saas-lib | 0001_saas_core.up.sql | Identidad ganadora del squash |
| **org_members** | saas-lib | 0001_saas_core.up.sql | Identidad ganadora del squash |
| **org_api_keys / scopes** | saas-lib | 0001_saas_core.up.sql | Identidad ganadora |
| **org_usage_counters** | saas-lib | 0001_saas_core.up.sql | Identidad ganadora |
| customers | pymes-core | 0005_core_business.up.sql | Convertida a party (role) en 0017_party_model |
| suppliers | pymes-core | 0005_core_business.up.sql | Convertida a party (role) en 0017 |
| appointments | pymes-core | 0010_transversal_core.up.sql | Migrada a scheduling module en 0041; dropped en 0050 |
| dashboard_layouts | pymes-core | 0021_dashboard_personalizable.up.sql | Drop en 0047 |
| catalog_services | pymes-core | 0042_split_products_services.up.sql | Renamed a `services` en 0043 |
| procurement_policies | pymes-core | 0025_procurement_requests.up.sql | Drop en 0076 |
| salon_services | beauty | (legacy) | Drop en 0004 |
| staff_members | beauty | (legacy) | Drop en 0005 |

Resto de tablas creadas una sola vez (no listadas para brevedad).

---

## 4. Drift cross-source crítico (saas vs pymes-core)

### 4.1 `users`

| Columna | pymes-core/0001 | saas-lib/0001 |
|---|---|---|
| `id` | `uuid PK DEFAULT gen_random_uuid()` | `uuid PK DEFAULT gen_random_uuid()` |
| `external_id` | `text UNIQUE NOT NULL` | `text NOT NULL UNIQUE` |
| `email` | `text UNIQUE NOT NULL` | `text NOT NULL UNIQUE` |
| `name` | `text NOT NULL DEFAULT ''` | `text NOT NULL DEFAULT ''` |
| `avatar_url` | **`text NOT NULL DEFAULT ''`** | **`text` (nullable)** |
| `deleted_at` | **`timestamptz` (existe)** | **(no existe)** |
| `created_at` / `updated_at` | `timestamptz NOT NULL DEFAULT now()` | igual |

### 4.2 `tenant_settings`

| Columna | pymes-core/0001 | saas-lib/0001 |
|---|---|---|
| **PK** | **`tenant_id` → `tenants(id)`** | **`org_id` → `orgs(id)`** |
| `plan_code` | `text NOT NULL DEFAULT 'starter'` | igual |
| `hard_limits` / `hard_limits_json` | `jsonb DEFAULT '{}'` | **renombrado** `hard_limits_json jsonb DEFAULT '{}'::jsonb` |
| `stripe_customer_id` | — | `text UNIQUE` |
| `stripe_subscription_id` | — | `text UNIQUE` |
| `billing_status` | — | `text DEFAULT 'trialing' CHECK (...)` |
| `past_due_since` | — | `timestamptz` |
| `status` | — | `text DEFAULT 'active' CHECK (...)` |
| `deleted_at` | — | `timestamptz` |
| `updated_by` | `text` | igual |
| `created_at` | `timestamptz` | igual |
| `updated_at` | `timestamptz` | igual |

### 4.3 `admin_activity_events`

| Columna | pymes-core/0001 | saas-lib/0001 |
|---|---|---|
| `id` | `uuid PK DEFAULT gen_random_uuid()` | `uuid PK` (sin default) |
| **owner FK** | **`tenant_id → tenants`** | **`org_id → orgs`** |
| `actor` | `text` (nullable) | `text` (nullable) |
| `action` | `text NOT NULL` | igual |
| `resource_type` | `text NOT NULL DEFAULT ''` | `text NOT NULL` (sin default) |
| `resource_id` | `text` | `text` |
| `payload` / `payload_json` | **`payload jsonb` (nullable)** | **`payload_json jsonb NOT NULL DEFAULT '{}'::jsonb`** |
| `created_at` | `timestamptz NOT NULL DEFAULT now()` | igual |

### 4.4 `notification_log`

| Columna | pymes-core/0003 | saas-lib/0001 |
|---|---|---|
| `id` | `uuid PK DEFAULT gen_random_uuid()` | igual |
| **owner FK** | **`tenant_id NOT NULL → tenants`** | **`org_id NOT NULL → orgs`** |
| `user_id` | `NOT NULL → users CASCADE` | `nullable → users SET NULL` |
| `notification_type` | `text NOT NULL` | igual |
| `channel` | `text NOT NULL` (sin default) | `text NOT NULL DEFAULT 'email'` |
| `recipient` | — | `text NOT NULL` |
| `subject` | — | `text NOT NULL` |
| `status` | `text NOT NULL` | `text NOT NULL DEFAULT 'sent'` |
| `provider_message_id` | `text` | — |
| `dedup_key` | `text NOT NULL UNIQUE` | `text` (nullable) |
| `error_message` | — | `text` |
| `created_at` | `timestamptz` | igual |

### 4.5 `notification_preferences`

| Columna | pymes-core/0003 | saas-lib/0001 |
|---|---|---|
| `channel` | `text NOT NULL` (sin default) | `text NOT NULL DEFAULT 'email'` |
| Resto | iguales | iguales |

**Resolución 0036/0037**: pymes-core renombra `notification_log` → `pymes_notification_log` y `notification_preferences` → `pymes_notification_preferences` para evitar la colisión con saas. Pero NO hace lo mismo con `users`, `tenant_settings`, `admin_activity_events` — esas tres siguen colisionando. Por eso `CREATE TABLE IF NOT EXISTS` produce comportamiento dependiente del orden de ejecución.

---

## 5. Migraciones sin `.down.sql` completo

| # | File | Razón |
|---|---|---|
| 22 | pymes-core `0022_party_model_reconcile.down.sql` | `SELECT 1;` (no-op) |
| 24 | pymes-core `0024_whatsapp_full.down.sql` | archivo ausente |
| 38 | pymes-core `0038_whatsapp_campaigns.down.sql` | archivo ausente |
| 39 | pymes-core `0039_whatsapp_conversations.down.sql` | archivo ausente |
| 50 | pymes-core `0050_drop_superseded_appointments.down.sql` | archivo ausente |
| 79 | prof `0001_professionals_schema.down.sql` | `DROP SCHEMA CASCADE` (no-op por tabla, pierde datos) |
| 89 | work `0003_auto_repair_seed.down.sql` | `SELECT 1;` (no-op) |
| 124 | saas-lib `0001_saas_core.down.sql` | archivo ausente |
| 125 | saas-lib `0002_org_external_ids.down.sql` | archivo ausente |

**Impacto**: 9 migraciones no son reversibles. Si una rompe, no se puede `migrate down`.

---

## 6. Migraciones no idempotentes

Operaciones en estos archivos pueden fallar si la DB ya tiene estado parcial:

| # | File | Operación riesgosa |
|---|---|---|
| 4 | `0004_local_seed.up.sql` | `INSERT` directo sin `ON CONFLICT` |
| 7 | `0007_core_seed.up.sql` | igual |
| 9 | `0009_audit_log_fk.up.sql` | `ALTER TABLE ADD CONSTRAINT` sin guard |
| 13 | `0013_rbac_seed.up.sql` | `INSERT` directo |
| 30 | `0030_transversal_modules_seed.up.sql` | `INSERT` directo |
| 36 | `0036_namespace_pymes_notifications.up.sql` | `ALTER TABLE RENAME` sin guard (falla si saas creó la tabla) |
| 37 | `0037_namespace_pymes_notifications_constraints.up.sql` | `ALTER TABLE RENAME CONSTRAINT` sin guard (rota en DB fresca) |
| 44 | `0044_dashboard_services_widget.up.sql` | `INSERT` directo |
| 45 | `0045_active_products_are_products.up.sql` | `UPDATE` masivo sin condición segura |
| 51 | `0051_english_role_names.up.sql` | `UPDATE/INSERT` directos |
| 53 | `0053_credit_notes_optional_return.up.sql` | `ALTER TABLE DROP CONSTRAINT` sin guard |
| 89 | work `0003_auto_repair_seed.up.sql` | `INSERT` directo |
| 97 | work `0011_rename_appointment_id_to_booking_id.up.sql` | `ALTER TABLE RENAME COLUMN` sin guard |

> 82 % de migraciones tienen alguna operación parcialmente idempotente (mezcla de `IF NOT EXISTS` con sentencias que no lo soportan). El squash las reescribe envolviendo en bloques `DO $$ ... END $$` con guards explícitos.

---

## 7. Tablas que desaparecen en el squash (legacy)

### 7.1 Eliminadas completamente (datos perdidos):

- `tenants` → reemplazada por `orgs`
- `tenant_memberships` → reemplazada por `org_members`
- `tenant_api_keys` → reemplazada por `org_api_keys`
- `tenant_api_key_scopes` → reemplazada por `org_api_key_scopes`
- `tenant_usage_counters` → reemplazada por `org_usage_counters`
- `procurement_policies` (ya eliminada en 0076; el squash no la recrea)
- `appointments`, `appointment_slots` (migradas a scheduling module en 0041)
- `dashboard_layouts`, `dashboard_default_layouts`, `dashboard_configs` (eliminadas en 0047/0048)
- `customers`, `suppliers` (convertidas a `parties` con role)
- `catalog_services` (renamed a `services`)

### 7.2 FK renombrada `tenant_id → org_id`:

`audit_log`, `admin_activity_events`, `notification_log` (squash usa schema saas), `tenant_settings` (PK cambia), `pymes_in_app_notifications` (mantener si pymes lo usa además de saas), parties, products, services, sales, sale_items, quotes, quote_items, purchases, purchase_items, payments, accounts, account_movements, cash_movements, recurring_expenses, returns, return_items, credit_notes, invoices, invoice_line_items, stock_levels, stock_movements, attachments, timeline_entries, calendar_export_tokens, calendar_sync_connections, calendar_sync_errors, employees, ai_dossiers, ai_conversations, ai_usage_daily, ai_agent_events, agent_confirmations, agent_idempotency_keys, whatsapp_*, webhook_endpoints, webhook_deliveries, webhook_outbox, webhook_events_clerk, role_permissions (no), roles (no — no es multi-tenant), user_roles, exchange_rates, scheduler_runs, pymes_business_insight_candidates, parties + party_*.

### 7.3 Schema/columna que cambia:

- `users.avatar_url` → `text` nullable (alineado a saas).
- `users.deleted_at` → conservar (decisión documentada en plan).
- `tenant_settings` → estructura saas completa (billing, stripe, status).
- `notification_log` → estructura saas completa (recipient, subject, error_message).
- `admin_activity_events.payload` → renombrado `payload_json` (alineado a saas).

---

## 8. Extensiones requeridas

| Extensión | Origen | Propósito |
|---|---|---|
| `pgcrypto` | pymes-core/0001 + saas-lib/0001 | `gen_random_uuid()` |
| `uuid-ossp` | pymes-core/0001 | `uuid_generate_v4()` (legacy, solo declarada — ningún archivo la usa) |
| `btree_gist` | scheduling lib (modules/scheduling/go) | exclusion constraint en `scheduling_bookings.no_overlap` |

> El squash declara solo `pgcrypto` y `btree_gist`. `uuid-ossp` se elimina (no se usa).

---

## 9. Anti-patterns y deuda detectada

### 9.1 FK sin `ON DELETE` explícito (~30 referencias)

Ejemplos: `quote_items.product_id REFERENCES products(id)`, `sale_items.product_id`, `purchase_items.product_id`, `auto_repair_vehicles.vehicle_id`, `bicycles.workshop_id`, varios `*_id` en migraciones 0010, 0011, 0017, 0019, 0042. Sin `ON DELETE`, el default es `RESTRICT` — borrar un product que tiene line items falla con error opaco. **El squash explicita `ON DELETE` (CASCADE / RESTRICT / SET NULL) en cada FK según semántica del dominio.**

### 9.2 Soft-delete dual

| Patrón | Tablas | Convención |
|---|---|---|
| `deleted_at timestamptz NULL` | ~45 tablas (parties, products, customers, suppliers, employees, sales, payments, returns, …) | dominante |
| `archived_at timestamptz NULL` | ~8 tablas (appointments, quotes, vehicles, work_orders, dining_areas, dining_tables, customer_assets, procurement_requests) | minoría |
| `voided_at timestamptz NULL` | `sales` (0008) | excepción contable |
| `status text` con valores `'active'/'archived'` | algunos `tenant_settings`, `whatsapp_connections` | inconsistente |

**El squash unifica en `archived_at`** (semántica más explícita: "se archivó", no "se borró lógicamente"). Excepción documentada: `users.deleted_at` (anonimización GDPR; no es archive).

### 9.3 Naming inconsistente de constraints e índices

- Algunos índices llamados `idx_<table>_<col>`, otros `<table>_<col>_idx`.
- FK auto-generadas (sin `CONSTRAINT <name>`) en 60+ casos; nombradas a mano en otras (`sessions_org_id_booking_id_key`).
- UNIQUE: a veces `_key` (auto-generado), a veces `_uniq` o sin sufijo.
- Algunos índices con `IF NOT EXISTS`, otros sin.

**El squash impone**: `idx_<tabla>_<columnas>` para todos los indexes; `<tabla>_<col>_fkey` para FK; `<tabla>_<col>_uniq` para UNIQUE; `<tabla>_<col>_check` para CHECK. Todos con `IF NOT EXISTS` cuando aplica.

### 9.4 Mezcla de UUID generators

- `uuid_generate_v4()`: declarada en pymes-core/0001 pero no usada.
- `gen_random_uuid()`: usada universalmente.

**El squash usa solo `gen_random_uuid()`** (de `pgcrypto`). Drop `uuid-ossp`.

### 9.5 Cero RLS

**0 `CREATE POLICY`** en todas las migraciones. Multi-tenant isolation depende de filtrado en código Go (`WHERE org_id = ?`).

**Decisión del squash**: NO introducir RLS en este refactor (out of scope; requiere re-architecture de auth/middleware). Documentar limitación en `docs/DATABASE_INIT.md`.

### 9.6 Cero triggers / functions

**0 `CREATE TRIGGER`, 0 `CREATE FUNCTION`** en migraciones. `updated_at` se updatea desde Go (GORM hooks) — pero no es consistente: SQL directo lo bypassea.

**Squash agrega**:
- Function `set_updated_at()` (PL/pgSQL).
- Trigger `BEFORE UPDATE` que la invoca, aplicado a cada tabla con `updated_at`.

### 9.7 Cero transacciones explícitas

`golang-migrate` envuelve cada archivo en una transacción implícita, pero algunos archivos contienen operaciones no transaccionables (CREATE INDEX CONCURRENTLY, etc.) que el wrapper rompería. El squash usa `BEGIN; ... COMMIT;` explícito en todas y documenta cuáles requieren ejecución fuera de transacción (índices concurrentes, si los hay).

### 9.8 Cuatro tablas distintas de schema_migrations

| Tabla | Backend que la usa | Estructura |
|---|---|---|
| `schema_migrations` | golang-migrate default (varios) | `(version bigint, dirty bool)` |
| `pymes_core_schema_migrations` | pymes-core (runner.go) | `(version bigint, dirty bool)` |
| `schema_migrations_professionals` | professionals (gorm gormdb) | `(version text)` — heterogéneo |
| `schema_migrations_beauty` | beauty (gorm gormdb) | `(version text)` |
| `saas_core_schema_migrations` | saas-go-lib (custom) | `(scope text, version text, applied_at, PRIMARY KEY(scope, version))` |

**El squash unifica** a una sola tabla:

```sql
CREATE TABLE schema_migrations (
    scope text NOT NULL,
    version text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now(),
    dirty boolean NOT NULL DEFAULT false,
    PRIMARY KEY (scope, version)
);
```

Cada componente (pymes-core, scheduling, professionals, workshops, beauty, restaurants) usa su `scope` propio. Patrón portado de `core/saas/go/migrations/migrations.go`.

---

## 10. Próximos pasos

Plan canónico: [`.claude/plans/tengo-un-bug-en-melodic-river.md`](../.claude/plans/tengo-un-bug-en-melodic-river.md).

Resumen del cronograma:
- **Fase A** (este documento): inventario congelado ✅
- **Fase B**: scaffolding (scripts/migrations-validate.sh, scripts/migrations-snapshot.sh, dir _squashed/)
- **Fase C**: snapshot del schema actual (pg_dump baseline)
- **Fase D**: diseñar nuevas migraciones 0001..0017 de pymes-core
- **Fase E**: convenciones SQL (trigger updated_at)
- **Fase F**: squash de 4 verticales
- **Fase G**: runner único + cleanup de bootstrap.go
- **Fase H**: eliminar código muerto + seeds limpios
- **Fase I**: tests de migración + CI
- **Fase J**: validaciones post-cut (build + smoke + E2E)
- **Fase K**: documentación final (DATABASE_INIT.md, actualizar CLAUDE.md)

Total: ~10 días dedicados.
