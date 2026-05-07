import { expect, test, type Page } from '@playwright/test';

const loginEmail = process.env.E2E_REAL_CLERK_EMAIL ?? 'devpablocristo@gmail.com';
const loginPassword = process.env.E2E_REAL_CLERK_PASSWORD ?? '12345';

async function loginWithClerk(page: Page) {
  await page.goto('/login', { waitUntil: 'domcontentloaded' });
  if (!page.url().includes('/login')) return;

  const emailInput = page.locator('input[name="identifier"], input[name="emailAddress"], input[type="email"]').first();
  await expect(emailInput).toBeVisible({ timeout: 30_000 });
  await emailInput.fill(loginEmail);
  await page.getByRole('button', { name: /^(continuar|continue)$/i }).first().click();

  const passwordInput = page.locator('input[name="password"]:not([aria-hidden="true"])').first();
  await expect(passwordInput).toBeVisible({ timeout: 30_000 });
  await expect(passwordInput).toBeEnabled({ timeout: 30_000 });
  await passwordInput.fill(loginPassword);
  await page.getByRole('button', { name: /^(continuar|continue)$/i }).first().click();
  await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 60_000 });
}

test.describe('Medical real MedLab', () => {
  test('abre examenes laborales sin failed fetch ni errores HTTP visibles', async ({ page }, testInfo) => {
    const failures: string[] = [];
    page.on('console', (msg) => {
      const text = msg.text();
      if (text.includes('Download the React DevTools') || text.includes('Clerk has been loaded with development keys') || text.includes('/favicon.ico')) return;
      if (msg.type() === 'error' || msg.type() === 'warning') failures.push(`console.${msg.type()}: ${text}`);
    });
    page.on('requestfailed', (request) => {
      const failure = request.failure()?.errorText ?? 'unknown';
      if (failure !== 'net::ERR_ABORTED') failures.push(`requestfailed ${request.method()} ${request.url()}: ${failure}`);
    });
    page.on('response', (response) => {
      const url = response.url();
      if (url.includes('/v1/medical/occupational-health/exams') && response.status() >= 400) {
        failures.push(`http ${response.status()} ${response.request().method()} ${url}`);
      }
    });

    await loginWithClerk(page);
    await page.goto('/medlab/medical/occupational-health/exams/list', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('heading', { name: 'Medicina laboral' })).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText('Failed to fetch')).toHaveCount(0, { timeout: 20_000 });
    await expect(page.locator('body')).not.toContainText(/failed to fetch|forbidden|tenant slug/i, { timeout: 20_000 });
    await testInfo.attach('runtime-failures', { body: failures.join('\n') || 'clean', contentType: 'text/plain' });
    expect(failures).toEqual([]);
  });
});
