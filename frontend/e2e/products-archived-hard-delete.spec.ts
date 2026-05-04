import { expect, test } from '@playwright/test';
import {
  CRUD_EDIT_MATRIX_TENANT_SLUG,
  installCrudEditMatrixMocks,
} from './helpers-crud-edit-matrix';

/**
 * Productos archivados: «Eliminar» debe llamar DELETE `/v1/products/{id}` (sin `/hard`), coherente con buildRestCrudDataSource.
 */
test.describe('Productos archivados — eliminar definitivo', () => {
  test.beforeEach(async ({ page }) => {
    await installCrudEditMatrixMocks(page);
  });

  test('confirmación Eliminar dispara DELETE sobre el ítem (sin /hard)', async ({ page }) => {
    await page.goto(`/${CRUD_EDIT_MATRIX_TENANT_SLUG}/products/list?archived=1`);

    await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 60_000 });

    const deletePromise = page.waitForRequest((req) => {
      if (req.method() !== 'DELETE') return false;
      const u = req.url();
      return u.includes('/v1/products/prod-arch-e2e-1') && !u.includes('/hard');
    });

    await page.locator('tbody tr').first().click();

    const detailDialog = page.getByRole('dialog');
    await expect(detailDialog).toBeVisible({ timeout: 30_000 });
    await detailDialog.getByRole('button', { name: 'Eliminar' }).click();

    const confirmDialog = page.getByRole('dialog').filter({ hasText: /elimina definitivamente/i });
    await expect(confirmDialog).toBeVisible({ timeout: 15_000 });
    await confirmDialog.getByRole('button', { name: 'Eliminar' }).click();

    await deletePromise;
  });
});
