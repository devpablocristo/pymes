import { expect, test, type Page } from '@playwright/test';

type PurchaseRow = {
  id: string;
  number: string;
  supplier_id: string;
  supplier_name: string;
  status: string;
  payment_status: string;
  total: number;
  currency: string;
  notes: string;
};

type UpdateCall = { id: string; body: Record<string, unknown> };

function seedPurchases(): PurchaseRow[] {
  return [
    {
      id: 'pur-001',
      number: 'CPA-SEED-001',
      supplier_id: 'sup-001',
      supplier_name: 'Proveedor Demo 1',
      status: 'received',
      payment_status: 'paid',
      total: 12100,
      currency: 'ARS',
      notes: 'Compra semilla recibida',
    },
    {
      id: 'pur-002',
      number: 'CPA-SEED-002',
      supplier_id: 'sup-001',
      supplier_name: 'Proveedor Demo 1',
      status: 'draft',
      payment_status: 'pending',
      total: 6050,
      currency: 'ARS',
      notes: 'Borrador de compra',
    },
  ];
}

async function installPurchasesApiMocks(page: Page) {
  let purchases = seedPurchases();
  const updateCalls: UpdateCall[] = [];
  const createCalls: Array<Record<string, unknown>> = [];

  await page.route('**/v1/**', async (route) => {
    const url = route.request().url();
    const method = route.request().method();

    if (url.includes('/v1/admin/tenant-settings') && method === 'GET') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
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
    }

    if (url.includes('/v1/session') && method === 'GET') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          auth: {
            org_id: 'e2e-org-001',
            org_name: 'E2E Test Org',
            product_role: 'admin',
            auth_method: 'api_key',
          },
        }),
      });
    }

    if (url.includes('/v1/purchases') && method === 'GET') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: purchases,
          total: purchases.length,
          has_more: false,
          next_cursor: null,
        }),
      });
    }

    if (/\/v1\/purchases\/[^/]+\/status$/.test(url) && method === 'PATCH') {
      const id = url.split('/').at(-2) ?? '';
      const body = route.request().postDataJSON() as Record<string, unknown>;
      updateCalls.push({ id, body });
      purchases = purchases.map((purchase) =>
        purchase.id === id ? { ...purchase, status: String(body.status ?? purchase.status) } : purchase,
      );
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(purchases.find((purchase) => purchase.id === id) ?? { ok: true }),
      });
    }

    if (url.includes('/v1/purchases') && method === 'POST') {
      const body = route.request().postDataJSON() as Record<string, unknown>;
      createCalls.push(body);
      const created: PurchaseRow = {
        id: `pur-${String(createCalls.length + 100).padStart(3, '0')}`,
        number: `CPA-E2E-${String(createCalls.length).padStart(3, '0')}`,
        supplier_id: String(body.supplier_id ?? ''),
        supplier_name: String(body.supplier_name ?? 'Proveedor E2E'),
        status: String(body.status ?? 'draft'),
        payment_status: String(body.payment_status ?? 'pending'),
        total: 2500,
        currency: 'ARS',
        notes: String(body.notes ?? ''),
      };
      purchases = [created, ...purchases];
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(created),
      });
    }

    if (/\/v1\/purchases\/[^/]+$/.test(url) && method === 'PUT') {
      const id = url.split('/').pop() ?? '';
      const body = route.request().postDataJSON() as Record<string, unknown>;
      updateCalls.push({ id, body });
      purchases = purchases.map((purchase) => (purchase.id === id ? { ...purchase, ...body } : purchase));
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(purchases.find((purchase) => purchase.id === id) ?? { ok: true }),
      });
    }

    if (method === 'GET') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], total: 0 }),
      });
    }

    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true }),
    });
  });

  return { updateCalls, createCalls };
}

async function openPurchasesBoard(page: Page) {
  await page.goto('/modules/purchases/board');
  const errorAlert = page.getByRole('alert');
  if (await errorAlert.isVisible().catch(() => false)) {
    throw new Error(`La vista de compras cayó al error boundary: ${await errorAlert.innerText()}`);
  }
  await expect(page.getByRole('heading', { name: 'Compras' })).toBeVisible();
}

async function dragCardToColumn(page: Page, cardShell: ReturnType<Page['locator']>, targetColumn: ReturnType<Page['locator']>) {
  const cardBox = await cardShell.boundingBox();
  const columnBox = await targetColumn.boundingBox();
  if (!cardBox || !columnBox) {
    throw new Error('No se pudo resolver el bounding box del kanban para la prueba de drag.');
  }

  await page.mouse.move(cardBox.x + cardBox.width / 2, cardBox.y + cardBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(
    columnBox.x + columnBox.width / 2,
    columnBox.y + Math.min(160, columnBox.height / 2),
    { steps: 24 },
  );
  await page.mouse.up();
}

test.describe('Purchases Kanban', () => {
  test('permite mover una compra draft entre columnas válidas', async ({ page }) => {
    const { updateCalls } = await installPurchasesApiMocks(page);
    await openPurchasesBoard(page);

    const draftColumn = page.locator('.m-kanban__column-body[data-column="draft"]');
    const receivedColumn = page.locator('.m-kanban__column-body[data-column="received"]');
    const draftCard = draftColumn
      .locator('.m-kanban__card-shell[data-row-draggable="true"]')
      .filter({ hasText: 'CPA-SEED-002' });

    await expect(draftCard).toBeVisible();
    await dragCardToColumn(page, draftCard, receivedColumn);

    await expect(receivedColumn.getByText('CPA-SEED-002', { exact: true })).toBeVisible();
    await expect(draftColumn.getByText('CPA-SEED-002', { exact: true })).toHaveCount(0);
    expect(updateCalls).toEqual([
      {
        id: 'pur-002',
        body: expect.objectContaining({ status: 'received' }),
      },
    ]);
  });

  test('permite mover una compra ya recibida a otra columna válida', async ({ page }) => {
    const { updateCalls } = await installPurchasesApiMocks(page);
    await openPurchasesBoard(page);

    const receivedColumn = page.locator('.m-kanban__column-body[data-column="received"]');
    const voidedColumn = page.locator('.m-kanban__column-body[data-column="voided"]');
    const receivedCard = receivedColumn
      .locator('.m-kanban__card-shell[data-row-draggable="true"]')
      .filter({ hasText: 'CPA-SEED-001' });

    await expect(receivedCard).toBeVisible();
    await dragCardToColumn(page, receivedCard, voidedColumn);

    await expect(voidedColumn.getByText('CPA-SEED-001', { exact: true })).toBeVisible();
    await expect(receivedColumn.getByText('CPA-SEED-001', { exact: true })).toHaveCount(0);
    expect(updateCalls).toEqual([
      {
        id: 'pur-001',
        body: expect.objectContaining({ status: 'voided' }),
      },
    ]);
  });

  test('crea una compra nueva desde el pie de una columna y respeta su estado por defecto', async ({ page }) => {
    const { createCalls } = await installPurchasesApiMocks(page);
    await openPurchasesBoard(page);

    const receivedColumn = page.locator('.m-kanban__column-body[data-column="received"]');
    await receivedColumn.getByRole('button', { name: 'Añadir compra' }).click();

    await page.getByLabel('Proveedor').fill('Proveedor Kanban');
    await page
      .getByLabel('Items JSON')
      .fill('[{"description":"Insumo e2e","quantity":1,"unit_cost":2500}]');
    await page.getByRole('button', { name: 'Crear' }).click();

    await expect(receivedColumn.getByText('Proveedor Kanban')).toBeVisible();
    expect(createCalls).toEqual([
      expect.objectContaining({
        supplier_name: 'Proveedor Kanban',
        status: 'received',
      }),
    ]);
  });
});
