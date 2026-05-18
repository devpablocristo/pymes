import { mkdir, stat } from 'node:fs/promises';
import path from 'node:path';
import { chromium, type FullConfig, type Page } from '@playwright/test';

const DEFAULT_EMAIL = 'devpablocristo@gmail.com';
const DEFAULT_PASSWORD = '12345';

async function exists(filePath: string): Promise<boolean> {
  try {
    await stat(filePath);
    return true;
  } catch {
    return false;
  }
}

async function maybeLoginWithClerk(page: Page, baseURL: string) {
  const email = process.env.E2E_REAL_CLERK_EMAIL ?? DEFAULT_EMAIL;
  const password = process.env.E2E_REAL_CLERK_PASSWORD ?? DEFAULT_PASSWORD;

  await page.goto(`${baseURL}/login`, { waitUntil: 'domcontentloaded' });

  const emailInput = page.locator('input[name="identifier"], input[name="emailAddress"], input[type="email"]').first();
  const passwordInput = page.locator('input[name="password"], input[type="password"]').first();
  await emailInput.fill(email);
  await passwordInput.fill(password);
  await page.getByRole('button', { name: /^continuar$/i }).click();

  await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 60_000 });
  await page.goto(`${baseURL}/bicimax/dashboard`, { waitUntil: 'domcontentloaded' });
  await page.waitForURL(/\/bicimax\/dashboard$/, { timeout: 60_000 });

  if (page.url().includes('/login')) {
    throw new Error('E2E auth failed: Clerk login did not leave /login');
  }
  await page.getByRole('button', { name: 'Abrir menú' }).waitFor({ state: 'visible', timeout: 60_000 });
  if (await page.getByText(/No se pudo cargar la configuración del tenant/i).isVisible().catch(() => false)) {
    throw new Error('E2E auth failed: authenticated session could not load tenant settings');
  }
}

export default async function globalSetup(config: FullConfig) {
  const statePath = process.env.E2E_REAL_STORAGE_STATE ?? path.join('e2e-real', '.auth', 'dev-owner.json');
  const resolvedStatePath = path.resolve(process.cwd(), statePath);
  if (process.env.E2E_REAL_REUSE_AUTH_STATE !== '0' && (await exists(resolvedStatePath))) {
    return;
  }

  await mkdir(path.dirname(resolvedStatePath), { recursive: true });
  const baseURL = process.env.E2E_REAL_BASE_URL ?? String(config.projects[0]?.use.baseURL ?? 'http://127.0.0.1:5180');
  const browser = await chromium.launch();
  const page = await browser.newPage();
  try {
    await maybeLoginWithClerk(page, baseURL);
    await page.context().storageState({ path: resolvedStatePath });
  } finally {
    await browser.close();
  }
}
