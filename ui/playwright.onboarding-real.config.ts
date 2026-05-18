import { defineConfig } from '@playwright/test';

const baseURL = process.env.E2E_REAL_BASE_URL ?? 'http://127.0.0.1:5180';

export default defineConfig({
  testDir: './e2e-real',
  timeout: 300_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  use: {
    baseURL,
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
});
