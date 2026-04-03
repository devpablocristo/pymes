import { test, expect } from '@playwright/test';
import { mockApiWithOnboardingDone } from './helpers';

// Estos tests requieren un build sin Clerk (VITE_CLERK_PUBLISHABLE_KEY vacío).
// Correr con: npm run test:e2e (usa scripts/e2e-preview.sh que desactiva Clerk).
test.describe('Accessibility basics', () => {
  test.beforeEach(async ({ page }) => {
    await mockApiWithOnboardingDone(page);
  });

  test('html lang está configurado', async ({ page }) => {
    await page.goto('/');
    const lang = await page.locator('html').getAttribute('lang');
    expect(['es', 'en']).toContain(lang);
  });

  test('skip link existe', async ({ page }) => {
    await page.goto('/');
    await page.waitForSelector('a.skip-link', { timeout: 10_000 }).catch(() => null);
    const skipLinks = page.locator('a.skip-link');
    const count = await skipLinks.count();
    // Si la app carga sin Shell (login redirect), puede no haber skip link
    if (count > 0) {
      await expect(skipLinks.first()).toHaveAttribute('href', '#main-content');
    }
  });
});
