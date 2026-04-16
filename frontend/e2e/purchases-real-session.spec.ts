import { chromium, expect, test } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';

const SOURCE_ROOT = '/home/pablo/.config/google-chrome';
const PROFILE_NAME = 'Profile 1';
const TARGET_ROOT = '/tmp/playwright-google-chrome-purchases';

function resetTargetProfile(): void {
  fs.rmSync(TARGET_ROOT, { recursive: true, force: true });
  fs.mkdirSync(TARGET_ROOT, { recursive: true });
  fs.cpSync(path.join(SOURCE_ROOT, 'Local State'), path.join(TARGET_ROOT, 'Local State'));
  fs.cpSync(path.join(SOURCE_ROOT, PROFILE_NAME), path.join(TARGET_ROOT, PROFILE_NAME), {
    recursive: true,
  });
}

test('abre compras con la sesion real de chrome', async () => {
  resetTargetProfile();

  const context = await chromium.launchPersistentContext(TARGET_ROOT, {
    executablePath: '/opt/google/chrome/chrome',
    headless: false,
    viewport: null,
    args: [`--profile-directory=${PROFILE_NAME}`],
    slowMo: 250,
  });

  try {
    const page = context.pages()[0] ?? (await context.newPage());
    await page.goto('http://127.0.0.1:5180/modules/purchases/board', {
      waitUntil: 'domcontentloaded',
    });

    if (page.url().includes('accounts.google.com')) {
      const nextButton = page.getByRole('button', { name: 'Next' });
      await expect(nextButton).toBeVisible({ timeout: 15_000 });
      await nextButton.click();
      await page.waitForTimeout(5_000);
    }

    await expect(page.getByRole('heading', { name: 'Compras' })).toBeVisible({
      timeout: 30_000,
    });

    await page.screenshot({
      path: '/tmp/purchases-real-session-board.png',
      fullPage: true,
    });
  } finally {
    await context.close();
  }
});
