import { expect, test } from '@playwright/test';

test('purchases board renders on no-clerk frontend at 5191', async ({ page }) => {
  await page.goto('http://127.0.0.1:5191/modules/purchases/board');
  await expect(page.getByRole('heading', { name: 'Compras' })).toBeVisible({ timeout: 30000 });
  await expect(page.locator('.m-kanban__board')).toBeVisible();
});
