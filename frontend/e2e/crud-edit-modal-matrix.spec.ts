import { expect, test } from '@playwright/test';
import {
  CRUD_EDIT_MATRIX_TENANT_SLUG,
  crudEditMatrixListPath,
  installCrudEditMatrixMocks,
} from './helpers-crud-edit-matrix';

/**
 * Matriz: cada recurso con listado estándar debe abrir el modal visor y pasar
 * a modo edición (botón Guardar visible) al pulsar Editar.
 */
const CRUD_EDIT_MATRIX_RESOURCES = [
  'invoices',
  'customers',
  'suppliers',
  'products',
  'services',
  'priceLists',
  'quotes',
  'sales',
  'purchases',
  'returns',
  'creditNotes',
  'cashflow',
  'inventory',
  'payments',
  'recurring',
  'procurementRequests',
  'procurementPolicies',
  'accounts',
  'roles',
  'parties',
  'employees',
  'professionals',
  'specialties',
  'intakes',
  'sessions',
  'workshopVehicles',
  'carWorkOrders',
  'bikeWorkOrders',
  'restaurantDiningAreas',
  'restaurantDiningTables',
] as const;

test.describe('CRUD modal Editar → Guardar (matriz)', () => {
  test.beforeEach(async ({ page }) => {
    await installCrudEditMatrixMocks(page);
    await page.goto(`/${CRUD_EDIT_MATRIX_TENANT_SLUG}/customers`);
    await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 60_000 });
  });

  for (const resourceId of CRUD_EDIT_MATRIX_RESOURCES) {
    test(`recurso ${resourceId}`, async ({ page }) => {
      if (resourceId === 'invoices') {
        await page.evaluate(() => {
          const rows = [
            {
              id: 'inv-e2e-1',
              number: 'INV-E2E',
              customer: 'Cliente demo',
              initials: 'CD',
              issuedDate: '2026-01-01',
              dueDate: '2026-02-01',
              status: 'pending',
              items: [{ id: 'l1', description: 'Item', qty: 1, unit: 'unidad', unitPrice: 100 }],
              discount: 0,
              tax: 21,
            },
          ];
          localStorage.setItem('pymes.billing.demo.invoices.v2', JSON.stringify(rows));
        });
      }

      let path = crudEditMatrixListPath(resourceId);
      if (resourceId === 'payments') {
        path = `${path}?sale_id=sale-e2e-1`;
      }

      await page.goto(path);
      await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 60_000 });

      await page.locator('tbody tr').first().click();

      const dialog = page.getByRole('dialog');
      await expect(dialog).toBeVisible({ timeout: 30_000 });

      const editBtn = dialog.getByRole('button', { name: 'Editar' });
      await expect(editBtn).toBeVisible({ timeout: 30_000 });
      await editBtn.click();

      // Modo edición: el submit del formulario lleva el texto «Guardar» (no depender del árbol fuera del modal).
      const saveSubmit = dialog.locator('button[type="submit"]');
      await expect(saveSubmit).toBeVisible({ timeout: 30_000 });
      await expect(saveSubmit).toHaveText(/Guardar/);
    });
  }
});
