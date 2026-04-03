import { test, expect } from '@playwright/test';
import { mockApiForE2E } from './helpers';

test.describe('Onboarding', () => {
  test.beforeEach(async ({ page }) => {
    await page.evaluate(() => localStorage.clear()).catch(() => {});
    await mockApiForE2E(page);
    await page.goto('/');
  });

  test('redirige a /onboarding si no hay perfil', async ({ page }) => {
    await expect(page).toHaveURL(/onboarding/);
  });

  test('muestra el wizard con título y primer step', async ({ page }) => {
    await page.waitForSelector('h1');
    await expect(page.locator('h1')).toBeVisible();
    await expect(page.locator('#onboarding-business-name')).toBeVisible();
  });

  // TODO: requiere build sin Clerk + mock completo de tenant-settings PATCH
  test.skip('completa el onboarding y llega al dashboard', async ({ page }) => {
    await page.waitForSelector('#onboarding-business-name');

    // Step 1
    await page.fill('#onboarding-business-name', 'Mi Negocio Test');
    await page.locator('.onboarding-options .onboarding-option').first().click();
    await page.locator('.onboarding-options-vertical .onboarding-option').first().click();
    await page.click('.onboarding-btn-next');

    // Step 2
    await page.waitForSelector('h2');
    await page.locator('.onboarding-options .onboarding-option').first().click();
    await page.locator('.onboarding-field').nth(2).locator('.onboarding-chip').first().click();
    await page.locator('.onboarding-field').nth(3).locator('.onboarding-chip').first().click();
    await page.click('.onboarding-btn-next');

    // Step 3
    await page.waitForSelector('#onboarding-currency');
    await page.locator('.onboarding-options-row .onboarding-option').first().click();
    await page.click('.onboarding-btn-next');

    // Step 4
    const summaryRows = page.locator('.onboarding-summary-row');
    await expect(summaryRows).toHaveCount(9);
    await expect(page.locator('.onboarding-summary-row strong').first()).toHaveText('Mi Negocio Test');

    // Esperar a que el botón finish se habilite y clickear
    const finishBtn = page.locator('.onboarding-btn-finish');
    await expect(finishBtn).toBeEnabled();
    await finishBtn.click();

    // El finish hace PATCH + navigate — esperar la navegación o quedarse en onboarding con error
    await page.waitForTimeout(3000);
    const currentUrl = page.url();
    // Si hay un error visible, el test falla con ese mensaje
    const errorEl = page.locator('.onboarding-finish-error');
    const hasError = await errorEl.isVisible().catch(() => false);
    if (hasError) {
      const errorText = await errorEl.textContent();
      throw new Error(`Onboarding finish error: ${errorText}`);
    }
    expect(currentUrl).not.toMatch(/onboarding/);
  });

  test('botón Siguiente deshabilitado sin datos requeridos', async ({ page }) => {
    await page.waitForSelector('.onboarding-btn-next');
    await expect(page.locator('.onboarding-btn-next')).toBeDisabled();
  });

  test('botón Atrás navega al step anterior', async ({ page }) => {
    await page.waitForSelector('#onboarding-business-name');
    await page.fill('#onboarding-business-name', 'Test Negocio');
    await page.locator('.onboarding-options .onboarding-option').first().click();
    await page.locator('.onboarding-options-vertical .onboarding-option').first().click();
    await page.click('.onboarding-btn-next');

    await page.waitForSelector('.onboarding-btn-back');
    await page.click('.onboarding-btn-back');
    await expect(page.locator('#onboarding-business-name')).toHaveValue('Test Negocio');
  });
});
