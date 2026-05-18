import { defineConfig } from '@playwright/test';

const e2ePreviewPort = process.env.E2E_PREVIEW_PORT ?? '4173';
const e2ePreviewOrigin = `http://127.0.0.1:${e2ePreviewPort}`;

export default defineConfig({
  testDir: './e2e',
  timeout: 90_000,
  expect: { timeout: 20_000 },
  fullyParallel: true,
  workers: process.env.CI ? 2 : 4,
  retries: process.env.CI ? 1 : 0,
  use: {
    baseURL: e2ePreviewOrigin,
    trace: 'on-first-retry',
    video: 'off',
  },
  webServer: {
    command: 'bash ./scripts/e2e-preview.sh',
    url: e2ePreviewOrigin,
    timeout: 240_000,
    reuseExistingServer: process.env.PLAYWRIGHT_REUSE_SERVER === '1',
  },
});
