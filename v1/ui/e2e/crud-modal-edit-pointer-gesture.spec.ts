import { expect, test } from '@playwright/test';
import {
  CRUD_EDIT_MATRIX_TENANT_SLUG,
  installCrudEditMatrixMocks,
} from './helpers-crud-edit-matrix';

/**
 * Reproduce el gesto mouse real (down/up separados): si el DOM cambia tras down,
 * sin pointer capture el «up» puede ir al backdrop y cerrar el modal.
 */
test.describe('CRUD modal Editar — gesto ratón down/up', () => {
  test.beforeEach(async ({ page }) => {
    await installCrudEditMatrixMocks(page);
    await page.goto(`/${CRUD_EDIT_MATRIX_TENANT_SLUG}/customers`);
    await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 60_000 });
  });

  test('pointer down/up en Editar deja el diálogo abierto con Guardar', async ({ page }) => {
    await page.locator('tbody tr').first().click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible({ timeout: 30_000 });

    const editBtn = dialog.getByRole('button', { name: 'Editar' });
    await expect(editBtn).toBeVisible({ timeout: 30_000 });

    await editBtn.scrollIntoViewIfNeeded();
    await editBtn.hover();

    const box = await editBtn.boundingBox();
    expect(box).not.toBeNull();

    const x = box!.x + box!.width / 2;
    const y = box!.y + box!.height / 2;

    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.up();

    await expect(dialog).toBeVisible({ timeout: 30_000 });

    const saveSubmit = dialog.locator('button[type="submit"]');
    await expect(saveSubmit).toBeVisible({ timeout: 30_000 });
    await expect(saveSubmit).toHaveText(/Guardar/);
  });
});
