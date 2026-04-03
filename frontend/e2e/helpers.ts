import type { Page } from '@playwright/test';
import { expect } from '@playwright/test';

/**
 * Intercepta las llamadas a la API de pymes para que funcionen sin backend real.
 * DEBE llamarse ANTES de page.goto() para interceptar las requests iniciales.
 */
export async function mockApiForE2E(page: Page) {
  // Estado mutable: el mock de tenant-settings recuerda si se completó el onboarding
  let tenantSettings: Record<string, unknown> = { onboarding_completed_at: null };

  await page.route('**/v1/**', (route) => {
    const url = route.request().url();
    const method = route.request().method();

    // GET /v1/admin/tenant-settings
    if (url.includes('/v1/admin/tenant-settings') && method === 'GET') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(tenantSettings),
      });
    }

    // PATCH /v1/admin/tenant-settings — actualiza el estado in-memory
    if (url.includes('/v1/admin/tenant-settings') && (method === 'PATCH' || method === 'PUT')) {
      const body = route.request().postDataJSON() ?? {};
      tenantSettings = {
        ...tenantSettings,
        ...body,
        onboarding_completed_at: body.onboarding_completed_at ?? new Date().toISOString(),
      };
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(tenantSettings),
      });
    }

    // GET /v1/session
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

    // GET genérico — lista vacía
    if (method === 'GET') {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], total: 0 }),
      });
    }

    // POST/PUT/PATCH/DELETE genérico
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true }),
    });
  });
}

/**
 * Mock para simular que el onboarding ya fue completado.
 */
export async function mockApiWithOnboardingDone(page: Page) {
  await page.route('**/v1/**', (route) => {
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
          uses_billing: false,
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
}

/**
 * Completa el onboarding mínimo para acceder a la app.
 */
export async function completeOnboarding(page: Page) {
  await mockApiForE2E(page);
  await page.goto('/');
  await page.waitForSelector('#onboarding-business-name');

  await page.fill('#onboarding-business-name', 'E2E Test Business');
  await page.locator('.onboarding-options .onboarding-option').first().click();
  await page.locator('.onboarding-options-vertical .onboarding-option').first().click();
  await page.click('.onboarding-btn-next');

  await page.locator('.onboarding-options .onboarding-option').first().click();
  await page.locator('.onboarding-field').nth(2).locator('.onboarding-chip').first().click();
  await page.locator('.onboarding-field').nth(3).locator('.onboarding-chip').first().click();
  await page.click('.onboarding-btn-next');

  await page.locator('.onboarding-options-row .onboarding-option').first().click();
  await page.click('.onboarding-btn-next');

  await page.click('.onboarding-btn-finish');
  await expect(page).not.toHaveURL(/onboarding/);
}
