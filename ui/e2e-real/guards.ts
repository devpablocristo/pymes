import { expect, type Page, type TestInfo } from '@playwright/test';

const ignoredConsoleFragments = [
  'Download the React DevTools',
  'React Router Future Flag Warning',
  'Clerk has been loaded with development keys',
];

export function installRuntimeGuards(page: Page, testInfo: TestInfo) {
  const failures: string[] = [];

  page.on('console', (msg) => {
    const text = msg.text();
    if (ignoredConsoleFragments.some((fragment) => text.includes(fragment))) {
      return;
    }
    if (msg.type() === 'error' || msg.type() === 'warning') {
      failures.push(`console.${msg.type()}: ${text}`);
    }
  });

  page.on('pageerror', (err) => {
    failures.push(`pageerror: ${err.message}`);
  });

  page.on('requestfailed', (request) => {
    const failure = request.failure()?.errorText ?? 'unknown';
    if (failure === 'net::ERR_ABORTED') {
      return;
    }
    failures.push(`requestfailed ${request.method()} ${request.url()}: ${failure}`);
  });

  page.on('response', (response) => {
    const status = response.status();
    if (status < 400) {
      return;
    }
    const request = response.request();
    const url = response.url();
    if (request.resourceType() === 'image' || url.includes('/favicon')) {
      return;
    }
    failures.push(`http ${status} ${request.method()} ${url}`);
  });

  return async function assertCleanRuntime() {
    await testInfo.attach('runtime-guard-failures', {
      body: failures.join('\n') || 'clean',
      contentType: 'text/plain',
    });
    expect(failures).toEqual([]);
  };
}

export async function expectNoCrudFailure(page: Page) {
  await expect(page.locator('text=/unexpected error|ERROR:|FORBIDDEN:|forbidden|relation .* does not exist/i')).toHaveCount(0);
  await expect(page.locator('text=/Cargando agenda|Cargando modulo|Cargando configuración/i')).toHaveCount(0, { timeout: 15_000 });
}
