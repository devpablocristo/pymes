import { chromium } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
const SOURCE_ROOT = '/home/pablo/.config/google-chrome';
const PROFILE_NAME = 'Profile 1';
const TARGET_ROOT = '/tmp/playwright-google-chrome-suppliers';
fs.rmSync(TARGET_ROOT, { recursive: true, force: true });
fs.mkdirSync(TARGET_ROOT, { recursive: true });
fs.cpSync(path.join(SOURCE_ROOT, 'Local State'), path.join(TARGET_ROOT, 'Local State'));
fs.cpSync(path.join(SOURCE_ROOT, PROFILE_NAME), path.join(TARGET_ROOT, PROFILE_NAME), { recursive: true });
const context = await chromium.launchPersistentContext(TARGET_ROOT, {
  executablePath: '/opt/google/chrome/chrome',
  headless: true,
  viewport: { width: 1365, height: 768 },
  args: [`--profile-directory=${PROFILE_NAME}`],
});
try {
  const page = context.pages()[0] ?? await context.newPage();
  page.on('request', req => {
    const u = req.url();
    if (u.includes('/v1/suppliers')) console.log('REQ', req.method(), u);
  });
  page.on('response', async res => {
    const u = res.url();
    if (u.includes('/v1/suppliers')) {
      let body=''; try { body = await res.text(); } catch {}
      console.log('RES', res.status(), u, body.slice(0, 500));
    }
  });
  page.on('console', msg => console.log('CONSOLE', msg.type(), msg.text()));
  await page.goto('http://127.0.0.1:5180/bicimax/suppliers?archived=1', { waitUntil: 'networkidle', timeout: 90000 });
  console.log('URL', page.url());
  console.log('BODY', (await page.locator('body').innerText()).slice(0, 3000));
  await page.screenshot({ path: '/tmp/suppliers-archived-real.png', fullPage: true });
} finally {
  await context.close();
}
