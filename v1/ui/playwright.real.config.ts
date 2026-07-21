import { defineConfig } from '@playwright/test';

const baseURL = process.env.E2E_REAL_BASE_URL ?? 'http://127.0.0.1:5180';

export default defineConfig({
  testDir: './e2e-real',
  timeout: 120_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  globalSetup: './e2e-real/global.setup.ts',
  use: {
    baseURL,
    storageState: process.env.E2E_REAL_STORAGE_STATE ?? './e2e-real/.auth/dev-owner.json',
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
