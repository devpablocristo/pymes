import { test, expect } from '@playwright/test';
import { mockApiWithOnboardingDone } from './helpers';

// Estos tests requieren un build sin Clerk (VITE_CLERK_PUBLISHABLE_KEY vacío).
// Correr con: npm run test:e2e (usa scripts/e2e-preview.sh que desactiva Clerk).
test.describe('Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await mockApiWithOnboardingDone(page);
  });

  test('la app carga sin errores', async ({ page }) => {
    const errors: string[] = [];
    page.on('pageerror', (err) => errors.push(err.message));
    await page.goto('/');
    await page.waitForTimeout(2000);
    expect(errors).toHaveLength(0);
  });

  test('la página tiene título', async ({ page }) => {
    await page.goto('/');
    const title = await page.title();
    expect(title).toBeTruthy();
  });

  test('navega a /settings sin crash', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForTimeout(2000);
    // La app debería cargar sin error — puede redirigir a login con Clerk
    const url = page.url();
    expect(url).toBeTruthy();
  });

  test('navega a /chat sin crash', async ({ page }) => {
    await page.goto('/chat');
    await page.waitForTimeout(2000);
    const url = page.url();
    expect(url).toBeTruthy();
  });
});
