import { expect, test } from '@playwright/test';

test('debug purchases runtime board', async ({ page }) => {
  await page.goto('/modules/purchases/board');
  await page.waitForLoadState('networkidle');
  await expect(page.getByRole('heading', { name: 'Compras' })).toBeVisible();
  const board = page.locator('.m-kanban__board');
  await expect(board).toBeVisible();
  const text = await board.innerText();
  console.log('\n--- BOARD TEXT ---\n' + text + '\n--- END BOARD TEXT ---\n');
  await page.screenshot({ path: '/tmp/purchases-runtime-board.png', fullPage: true });
});
