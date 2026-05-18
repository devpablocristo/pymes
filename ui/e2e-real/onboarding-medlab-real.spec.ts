import { expect, test } from '@playwright/test';
import fs from 'node:fs/promises';

const tenantName = process.env.E2E_REAL_ONBOARDING_TENANT_NAME ?? `MedLab ${Date.now()}`;
const loginEmail = process.env.E2E_REAL_CLERK_EMAIL ?? 'devpablocristo@gmail.com';
const loginPassword = process.env.E2E_REAL_CLERK_PASSWORD ?? '';
const loginEmailCode = process.env.E2E_REAL_CLERK_CODE ?? '';
const loginEmailCodeFile = process.env.E2E_REAL_CLERK_CODE_FILE ?? '';
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
  const waitForEmailCode = async () => {
    if (loginEmailCode.trim()) {
      return loginEmailCode.trim();
    }
    if (!loginEmailCodeFile.trim()) {
      throw new Error('Clerk requested an email verification code. Set E2E_REAL_CLERK_CODE or E2E_REAL_CLERK_CODE_FILE and rerun.');
    }
    const deadline = Date.now() + 10 * 60_000;
    while (Date.now() < deadline) {
      const code = await fs
        .readFile(loginEmailCodeFile, 'utf8')
        .then((value) => value.trim())
        .catch(() => '');
      if (/^\d{6}$/.test(code)) {
        return code;
      }
      await page.waitForTimeout(1000);
    }
    throw new Error(`Timed out waiting for Clerk code file: ${loginEmailCodeFile}`);
  };

  const completeEmailCodeIfPresent = async (timeout = 5_000) => {
    const hasCodeScreen = await page
      .waitForFunction(
        () => {
          const text = document.body.innerText;
          const codeLikeInput = Array.from(document.querySelectorAll('input')).some((input) => {
            const label = `${input.getAttribute('aria-label') ?? ''} ${input.getAttribute('name') ?? ''} ${
              input.getAttribute('autocomplete') ?? ''
            }`;
            return /verification|code|codigo|código|one-time/i.test(label);
          });
          return /Revise su correo electrónico|Check your email|verification code|código/i.test(text) || codeLikeInput;
        },
        undefined,
        { timeout },
      )
      .then(() => true)
      .catch(() => false);
    if (!hasCodeScreen) {
      return false;
    }
    const code = await waitForEmailCode();
    const codeInput = page.getByRole('textbox', { name: 'Enter verification code' }).first();
    await codeInput.click();
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+A' : 'Control+A');
    await page.keyboard.press('Backspace');
    await codeInput.pressSequentially(code, { delay: 35 });
    const leftLogin = page
      .waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 90_000 })
      .then(() => true)
      .catch(() => false);
    const continueButton = page.getByRole('button', { name: /^(continuar|continue)$/i }).first();
    if (await continueButton.isVisible({ timeout: 2_000 }).catch(() => false)) {
      await continueButton.click().catch(() => undefined);
    }
    await expect(await leftLogin, 'Clerk should leave the login flow after entering the email code').toBe(true);
    return true;
  };

  await page.goto('/login', { waitUntil: 'domcontentloaded' });
  if (!page.url().includes('/login')) {
    return;
  }

  const emailInput = page.locator('input[name="identifier"], input[name="emailAddress"], input[type="email"]').first();
  await expect(emailInput).toBeVisible({ timeout: 30_000 });
  await emailInput.fill(loginEmail);

  const firstContinue = page.getByRole('button', { name: /^(continuar|continue)$/i }).first();
  await firstContinue.click();

  if (await completeEmailCodeIfPresent(10_000)) {
    return;
  }

  const otherMethod = page.getByRole('link', { name: /usar otro método|use another method/i }).first();
  if (await otherMethod.isVisible({ timeout: 2_000 }).catch(() => false)) {
    await otherMethod.click();
    if (await completeEmailCodeIfPresent(loginEmailCode.trim() ? 30_000 : 10_000)) {
      return;
    }
    const passwordMethod = page.getByRole('button', { name: /contraseña|password/i }).first();
    if (await passwordMethod.isVisible({ timeout: 5_000 }).catch(() => false)) {
      await passwordMethod.click();
    }
  }

  if (await completeEmailCodeIfPresent(loginEmailCode.trim() ? 30_000 : 2_000)) {
    return;
  }

  const passwordInput = page.locator('input[name="password"]:not([aria-hidden="true"])').first();
  await expect(passwordInput).toBeVisible({ timeout: 30_000 });
  if (!loginPassword.trim()) {
    throw new Error('Clerk requested a password. Set E2E_REAL_CLERK_PASSWORD explicitly or use an email-code flow.');
  }
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
