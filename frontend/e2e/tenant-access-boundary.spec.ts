import { expect, test } from '@playwright/test';

test('blocks a manipulated tenant slug before mounting shell or CRUD requests', async ({ page }) => {
  const crudRequests: string[] = [];

  await page.route('**/v1/**', async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    const method = req.method();

    if (url.pathname.includes('/v1/invoices')) {
      crudRequests.push(`${method} ${url.pathname}`);
    }

    if (url.pathname.includes('/v1/session') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          auth: {
            tenant_id: 'e2e-org-001',
            tenant_name: 'E2E Test Tenant',
            tenant_slug: 'e2e-test',
            role: 'admin',
            product_role: 'admin',
            scopes: [],
            actor: 'user-e2e',
            auth_method: 'api_key',
          },
          tenant: {
            id: 'e2e-org-001',
            slug: 'e2e-test',
            name: 'E2E Test Tenant',
          },
          membership: {
            role: 'admin',
          },
        }),
      });
      return;
    }

    if (url.pathname.includes('/v1/admin/tenant-settings') && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          tenant_id: 'e2e-org-001',
          business_name: 'E2E Test',
          team_size: 'solo',
          sells: 'products',
          client_label: 'clientes',
          scheduling_enabled: false,
          uses_billing: true,
          currency: 'ARS',
          payment_method: 'cash',
          vertical: 'none',
          onboarding_completed_at: '2026-01-01T00:00:00.000Z',
        }),
      });
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items: [], total: 0 }),
    });
  });

  await page.goto('/medlab/invoices/list');

  await expect(page.getByRole('heading', { name: 'Acceso al tenant denegado' })).toBeVisible();
  await expect(page.getByText('Facturación')).toHaveCount(0);
  expect(crudRequests).toEqual([]);
});
