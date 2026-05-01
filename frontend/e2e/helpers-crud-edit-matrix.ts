import type { Page } from '@playwright/test';
import { toCrudResourceSlug } from '../src/crud/crudResourceSlug';

/** Empata con `business_name` del tenant-settings mock (`E2E Test` → `e2e-test`). */
export const CRUD_EDIT_MATRIX_TENANT_SLUG = 'e2e-test';

const iso = () => new Date().toISOString();

function paginated(items: unknown[]): Record<string, unknown> {
  return { items, total: items.length, has_more: false, next_cursor: null };
}

/** Ruta del listado CRUD (StandalonePage o subruta `inventory/list`). */
export function crudEditMatrixListPath(resourceId: string): string {
  const tenant = CRUD_EDIT_MATRIX_TENANT_SLUG;
  const slug = toCrudResourceSlug(resourceId);
  if (resourceId === 'inventory') {
    return `/${tenant}/inventory/list`;
  }
  // Productos: vista por defecto = galería; la matriz necesita tabla con tbody.
  if (resourceId === 'products') {
    return `/${tenant}/products/list`;
  }
  return `/${tenant}/${slug}`;
}

/**
 * Mock HTTP para matriz CRUD: tenant-settings + session + respuestas GET sintéticas por pathname.
 * Intercepta cualquier URL que contenga `/v1/` (core, verticales en otros puertos incl.).
 */
export async function installCrudEditMatrixMocks(page: Page): Promise<void> {
  let tenantSettings: Record<string, unknown> = {
    business_name: 'E2E Test',
    team_size: 'medium',
    sells: 'both',
    client_label: 'clientes',
    scheduling_enabled: false,
    uses_billing: true,
    currency: 'ARS',
    payment_method: 'cash',
    vertical: 'workshops',
    onboarding_completed_at: '2026-01-01T00:00:00.000Z',
  };

  await page.route('**/*', async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    if (!url.pathname.includes('/v1/')) {
      await route.continue();
      return;
    }

    const method = req.method();

    if (url.pathname.includes('/v1/admin/tenant-settings') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(tenantSettings),
      });
      return;
    }

    if (url.pathname.includes('/v1/admin/tenant-settings') && (method === 'PATCH' || method === 'PUT')) {
      const body = req.postDataJSON() as Record<string, unknown> | undefined;
      tenantSettings = { ...tenantSettings, ...(body ?? {}) };
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(tenantSettings),
      });
      return;
    }

    if (url.pathname.includes('/v1/session') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          auth: {
            org_id: 'e2e-org-001',
            org_name: 'E2E Org',
            product_role: 'admin',
            auth_method: 'api_key',
          },
        }),
      });
      return;
    }

    if (method !== 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true }),
      });
      return;
    }

    const path = url.pathname.replace(/\/+$/, '') || url.pathname;

    const fulfillJson = async (body: unknown, status = 200) => {
      await route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify(body),
      });
    };

    const salePaymentsMatch = path.match(/^\/v1\/sales\/([^/]+)\/payments$/);
    if (salePaymentsMatch) {
      await fulfillJson({
        items: [
          {
            id: 'pay-e2e-1',
            sale_id: salePaymentsMatch[1],
            method: 'cash',
            amount: 100,
            received_at: iso(),
            notes: 'Mock',
          },
        ],
      });
      return;
    }

    if (path === '/v1/scheduling/branches') {
      await fulfillJson({
        items: [
          {
            id: 'br-e2e',
            name: 'Sucursal E2E',
            active: true,
          },
        ],
      });
      return;
    }

    // ── Listas que solo exponen `{ items }` ───────────────────────────────
    if (path === '/v1/credit-notes') {
      await fulfillJson({
        items: [
          {
            id: 'cn-e2e-1',
            number: 'NC-E2E-1',
            party_id: '00000000-0000-4000-8000-000000000001',
            status: 'active',
            amount: 1000,
            balance: 1000,
            used_amount: 0,
            return_id: '00000000-0000-4000-8000-000000000002',
            created_at: iso(),
          },
        ],
      });
      return;
    }

    if (path === '/v1/webhook-endpoints') {
      await fulfillJson({
        items: [
          {
            id: 'wh-e2e-1',
            url: 'https://example.test/hook',
            description: 'Hook E2E',
            is_active: true,
            created_at: iso(),
            updated_at: iso(),
          },
        ],
      });
      return;
    }

    if (path === '/v1/roles') {
      await fulfillJson({
        items: [
          {
            id: 'role-e2e-1',
            name: 'e2e_rol',
            description: 'Rol E2E',
            is_system: false,
            permissions: [{ resource: 'customers', action: 'read' }],
            updated_at: iso(),
          },
        ],
      });
      return;
    }

    if (path === '/v1/procurement-policies') {
      await fulfillJson({
        items: [
          {
            id: 'pol-e2e-1',
            name: 'Política E2E',
            expression: 'true',
            effect: 'allow',
            priority: 100,
            mode: 'enforce',
            enabled: true,
            action_filter: 'procurement.submit',
            system_filter: 'pymes',
          },
        ],
      });
      return;
    }

    if (path === '/v1/procurement-requests') {
      await fulfillJson({
        items: [
          {
            id: 'pr-e2e-1',
            title: 'Solicitud E2E',
            description: 'Mock',
            category: 'general',
            estimated_total: 500,
            currency: 'ARS',
            lines: [{ description: 'Item', quantity: 1, unit_price_estimate: 500 }],
            status: 'draft',
            requester_actor: 'e2e-user',
          },
        ],
      });
      return;
    }

    if (path === '/v1/teachers/professionals') {
      await fulfillJson({
        items: [
          {
            id: 'prof-e2e-1',
            party_id: '00000000-0000-4000-8000-000000000010',
            bio: 'Bio',
            headline: 'Teacher E2E',
            public_slug: 'teacher-e2e',
            is_public: true,
            is_bookable: true,
            accepts_new_clients: true,
            specialties: [],
            created_at: iso(),
            updated_at: iso(),
          },
        ],
      });
      return;
    }

    if (path === '/v1/teachers/specialties') {
      await fulfillJson({
        items: [
          {
            id: 'spec-e2e-1',
            code: 'E2E',
            name: 'Especialidad E2E',
            description: 'Mock',
            is_active: true,
            created_at: iso(),
            updated_at: iso(),
          },
        ],
      });
      return;
    }

    if (path === '/v1/teachers/intakes') {
      await fulfillJson({
        items: [
          {
            id: 'int-e2e-1',
            profile_id: 'prof-e2e-1',
            customer_party_id: '00000000-0000-4000-8000-000000000011',
            status: 'draft',
            payload: {},
            created_at: iso(),
            updated_at: iso(),
          },
        ],
      });
      return;
    }

    if (path.startsWith('/v1/teachers/sessions')) {
      if (path === '/v1/teachers/sessions') {
        await fulfillJson({
          items: [
            {
              id: 'sess-e2e-1',
              booking_id: '00000000-0000-4000-8000-000000000012',
              profile_id: 'prof-e2e-1',
              status: 'scheduled',
              summary: 'Sesión E2E',
              started_at: iso(),
              ended_at: iso(),
              created_at: iso(),
              updated_at: iso(),
            },
          ],
        });
        return;
      }
    }

    if (path === '/v1/restaurants/dining-areas') {
      await fulfillJson({
        items: [
          {
            id: 'area-e2e-1',
            name: 'Zona E2E',
            sort_order: 0,
            is_active: true,
            created_at: iso(),
            updated_at: iso(),
          },
        ],
      });
      return;
    }

    if (path.startsWith('/v1/restaurants/dining-tables')) {
      await fulfillJson({
        items: [
          {
            id: 'tbl-e2e-1',
            dining_area_id: 'area-e2e-1',
            label: 'Mesa E2E',
            capacity: 4,
            sort_order: 0,
            is_active: true,
            created_at: iso(),
            updated_at: iso(),
          },
        ],
      });
      return;
    }

    if (path.startsWith('/v1/inventory')) {
      if (path === '/v1/inventory') {
        await fulfillJson({
          items: [
            {
              product_id: 'prod-e2e-1',
              product_name: 'Producto E2E',
              sku: 'SKU-E2E',
              quantity: 10,
              min_quantity: 1,
              is_low_stock: false,
              updated_at: iso(),
            },
          ],
        });
        return;
      }
    }

    if (path === '/v1/auto-repair/vehicles' || path === '/v1/auto-repair/vehicles/archived') {
      await fulfillJson(
        paginated([
          {
            id: 'veh-e2e-1',
            license_plate: 'E2E001',
            make: 'Make',
            model: 'Model',
            year: 2020,
            kilometers: 10000,
            customer_name: 'Cliente',
            updated_at: iso(),
          },
        ]),
      );
      return;
    }

    if (
      path === '/v1/auto-repair/work-orders' ||
      path === '/v1/auto-repair/work-orders/archived' ||
      path === '/v1/bike-shop/work-orders' ||
      path === '/v1/bike-shop/work-orders/archived'
    ) {
      await fulfillJson(
        paginated([
          {
            id: 'wo-e2e-1',
            org_id: 'e2e-org-001',
            branch_id: 'br-e2e',
            number: 'OT-E2E-1',
            target_type: path.includes('bike-shop') ? 'bicycle' : 'vehicle',
            target_id: path.includes('bike-shop') ? 'bike-e2e-1' : 'veh-e2e-1',
            target_label: path.includes('bike-shop') ? 'Bici E2E' : 'ABC123',
            customer_id: '00000000-0000-4000-8000-000000000020',
            customer_name: 'Cliente E2E',
            status: 'draft',
            currency: 'ARS',
            items: [],
            created_at: iso(),
            updated_at: iso(),
          },
        ]),
      );
      return;
    }

    const purchaseDetail = (id: string) => ({
      id,
      number: 'COMP-E2E',
      supplier_name: 'Proveedor E2E',
      status: 'draft',
      payment_status: 'unpaid',
      total: 1000,
      currency: 'ARS',
      notes: '',
      items: [{ description: 'Ítem', quantity: 1, unit_cost: 1000, tax_rate: 21 }],
    });

    const saleLikeDetail = (id: string) => ({
      id,
      number: 'VTA-E2E',
      customer_name: 'Cliente',
      status: 'draft',
      payment_method: 'cash',
      items: [{ description: 'Prod', quantity: 1, unit_price: 500, tax_rate: 21 }],
      notes: '',
    });

    const quoteLikeDetail = (id: string) => ({
      id,
      number: 'PRE-E2E',
      customer_name: 'Cliente',
      status: 'draft',
      valid_until: '2026-12-31',
      items: [{ description: 'Srv', quantity: 1, unit_price: 800, tax_rate: 21 }],
      notes: '',
    });

    const detailMatch = path.match(/^\/v1\/(purchases|sales|quotes)\/([^/]+)$/);
    if (detailMatch) {
      const [, kind, id] = detailMatch;
      if (kind === 'purchases') await fulfillJson(purchaseDetail(id));
      else if (kind === 'sales') await fulfillJson(saleLikeDetail(id));
      else await fulfillJson(quoteLikeDetail(id));
      return;
    }

    const segMatch = path.match(/^\/v1\/([^/]+)(?:\/archived)?$/);
    if (!segMatch) {
      await fulfillJson({ items: [], total: 0 });
      return;
    }

    const segment = segMatch[1];

    // Parties list shares `/v1/parties` with different semantics via `role=employee`.
    if (segment === 'parties') {
      if (url.searchParams.get('role') === 'employee') {
        await fulfillJson(
          paginated([
            {
              id: 'emp-e2e-1',
              party_type: 'person',
              display_name: 'Empleado E2E',
              email: 'emp@test.local',
              roles: [{ role: 'employee', is_active: true }],
            },
          ]),
        );
        return;
      }
      await fulfillJson(
        paginated([
          {
            id: 'party-e2e-1',
            party_type: 'person',
            display_name: 'Party E2E',
            email: 'party@test.local',
            phone: '',
            tax_id: '',
            notes: '',
            tags: [],
            roles: [{ role: 'customer', is_active: true }],
          },
        ]),
      );
      return;
    }

    const rows: Record<string, () => Record<string, unknown>> = {
      services: () => ({
        id: 'svc-e2e-1',
        name: 'Servicio E2E',
        code: 'SVC-E2E',
        category_code: 'general',
        sale_price: 1500,
        cost_price: 300,
        tax_rate: 21,
        currency: 'ARS',
        default_duration_minutes: 60,
        is_active: true,
        tags: [],
        description: 'Mock',
      }),
      customers: () => ({
        id: 'cust-e2e-1',
        type: 'person',
        name: 'Cliente E2E',
        tax_id: '',
        email: 'e2e@test.local',
        phone: '',
        notes: '',
        tags: [],
      }),
      suppliers: () => ({
        id: 'sup-e2e-1',
        name: 'Proveedor E2E',
        tax_id: '',
        email: '',
        phone: '',
        contact_name: '',
        notes: '',
        tags: [],
      }),
      products: () => ({
        id: 'prod-e2e-1',
        sku: 'SKU-E2E',
        name: 'Producto E2E',
        description: '',
        unit: 'unidad',
        price: 100,
        currency: 'ARS',
        cost_price: 50,
        tax_rate: 21,
        track_stock: false,
        is_active: true,
        tags: [],
      }),
      'price-lists': () => ({
        id: 'pl-e2e-1',
        name: 'Lista E2E',
        currency: 'ARS',
        is_default: false,
        is_active: true,
        updated_at: iso(),
      }),
      quotes: () => quoteLikeDetail('quote-e2e-1'),
      sales: () => saleLikeDetail('sale-e2e-1'),
      purchases: () => purchaseDetail('pur-e2e-1'),
      returns: () => ({
        id: 'ret-e2e-1',
        number: 'DEV-E2E',
        sale_number: 'VTA-1',
        status: 'received',
        total: 100,
        currency: 'ARS',
        created_at: iso(),
      }),
      cashflow: () => ({
        id: 'cf-e2e-1',
        type: 'income',
        amount: 250,
        currency: 'ARS',
        category: 'other',
        description: 'Movimiento E2E',
        payment_method: 'cash',
        reference_type: 'manual',
        created_by: 'e2e',
        created_at: iso(),
      }),
      'recurring-expenses': () => ({
        id: 'rec-e2e-1',
        name: 'Gasto recurrente E2E',
        amount: 99,
        currency: 'ARS',
        cadence: 'monthly',
        next_due_date: '2026-02-01',
        is_active: true,
      }),
      accounts: () => ({
        id: 'acc-e2e-1',
        type: 'asset',
        entity_type: 'party',
        entity_id: '00000000-0000-4000-8000-000000000030',
        entity_name: 'Entidad E2E',
        balance: 0,
        currency: 'ARS',
        credit_limit: 0,
        updated_at: iso(),
      }),
    };

    const builder = rows[segment];
    if (builder) {
      await fulfillJson(paginated([builder()]));
      return;
    }

    await fulfillJson(paginated([]));
  });
}
