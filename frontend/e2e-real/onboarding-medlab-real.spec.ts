import { expect, test } from '@playwright/test';

const tenantName = process.env.E2E_REAL_ONBOARDING_TENANT_NAME ?? `MedLab ${Date.now()}`;
const loginEmail = process.env.E2E_REAL_CLERK_EMAIL ?? 'devpablocristo@gmail.com';
const loginPassword = process.env.E2E_REAL_CLERK_PASSWORD ?? '12345';
const expectedSlug = tenantName
  .normalize('NFD')
  .replace(/[\u0300-\u036f]/g, '')
  .toLowerCase()
  .replace(/[^a-z0-9]+/g, '-')
  .replace(/^-+|-+$/g, '')
  .slice(0, 64);

const forbiddenDuringFinish = [
  '/v1/session',
  '/v1/scheduling/branches',
  '/v1/admin/tenant-settings',
];

async function loginWithClerk(page: import('@playwright/test').Page) {
  await page.goto('/login', { waitUntil: 'domcontentloaded' });
  if (!page.url().includes('/login')) {
    return;
  }

  const emailInput = page.locator('input[name="identifier"], input[name="emailAddress"], input[type="email"]').first();
  await expect(emailInput).toBeVisible({ timeout: 30_000 });
  await emailInput.fill(loginEmail);

  const firstContinue = page.getByRole('button', { name: /^(continuar|continue)$/i }).first();
  await firstContinue.click();

  const passwordInput = page.locator('input[name="password"]:not([aria-hidden="true"])').first();
  await expect(passwordInput).toBeVisible({ timeout: 30_000 });
  await expect(passwordInput).toBeEnabled({ timeout: 30_000 });
  await passwordInput.fill(loginPassword);

  await page.getByRole('button', { name: /^(continuar|continue)$/i }).first().click();
  await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 60_000 });
}

test.describe('Onboarding real MedLab', () => {
  test('completa medicina laboral con más de 20, ambos, pacientes, agenda, cobros y mixto', async ({ page }, testInfo) => {
    const failures: string[] = [];
    let finishStarted = false;

    page.on('console', (msg) => {
      const text = msg.text();
      if (
        text.includes('Download the React DevTools') ||
        text.includes('Clerk has been loaded with development keys') ||
        text.includes('/favicon.ico')
      ) {
        return;
      }
      if (msg.type() === 'error' || msg.type() === 'warning') {
        failures.push(`console.${msg.type()}: ${text}`);
      }
    });

    page.on('response', (response) => {
      if (!finishStarted) return;
      const status = response.status();
      if (status < 400) return;
      const url = response.url();
      if (forbiddenDuringFinish.some((fragment) => url.includes(fragment))) {
        failures.push(`http ${status} ${response.request().method()} ${url}`);
      }
    });

    page.on('requestfailed', (request) => {
      const failure = request.failure()?.errorText ?? 'unknown';
      if (failure !== 'net::ERR_ABORTED') {
        failures.push(`requestfailed ${request.method()} ${request.url()}: ${failure}`);
      }
    });

    await loginWithClerk(page);
    await page.goto('/onboarding', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('heading', { name: 'Configurá tu espacio' })).toBeVisible();

    await page.getByLabel('¿Cómo se llama tu negocio o actividad?').fill(tenantName);
    await page.getByRole('button', { name: /^Más de 20/i }).click();
    await page.getByRole('button', { name: /^Medicina\b/i }).click();
    await page.getByRole('button', { name: /^Medicina laboral/i }).click();
    await page.getByRole('button', { name: 'Siguiente' }).click();

    await page.getByRole('button', { name: /^Ambos/i }).click();
    await page.getByRole('button', { name: /^pacientes$/i }).click();
    await page.getByRole('button', { name: /^Sí$/i }).click();
    await page.getByRole('button', { name: /^Sí, quiero saber quién me debe/i }).click();
    await page.getByRole('button', { name: 'Siguiente' }).click();

    await page.getByLabel('¿En qué moneda operás?').selectOption('ARS');
    await page.getByRole('button', { name: /^Mixto \(varios\)/i }).click();
    await page.getByRole('button', { name: 'Siguiente' }).click();

    await expect(page.getByText('Todo listo')).toBeVisible();
    await expect(page.locator('.onboarding-summary')).toContainText(tenantName);
    await expect(page.locator('.onboarding-summary')).toContainText('Más de 20');
    await expect(page.locator('.onboarding-summary')).toContainText('Medicina laboral');
    await expect(page.locator('.onboarding-summary')).toContainText('Ambos');
    await expect(page.locator('.onboarding-summary')).toContainText('pacientes');
    await expect(page.locator('.onboarding-summary')).toContainText('ARS');
    await expect(page.locator('.onboarding-summary')).toContainText('Mixto (varios)');

    finishStarted = true;
    await page.getByRole('button', { name: 'Empezar' }).click();

    await expect(page.locator('.onboarding-finish-error')).toHaveCount(0, { timeout: 20_000 });
    await expect(page).toHaveURL(new RegExp(`/${expectedSlug}/dashboard$`), { timeout: 60_000 });
    await expect(page.getByRole('button', { name: 'Abrir menú' })).toBeVisible({ timeout: 60_000 });

    await testInfo.attach('runtime-failures', {
      body: failures.join('\n') || 'clean',
      contentType: 'text/plain',
    });
    expect(failures).toEqual([]);
  });
});
