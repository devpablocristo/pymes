import { expect, test } from '@playwright/test';
import { installRuntimeGuards } from './guards';

const tenant = process.env.E2E_REAL_TENANT_SLUG ?? 'bicimax';

test.describe('Ajustes real', () => {
  test('mantiene avatar/header al navegar, refrescar y cambiar secciones', async ({ page }, testInfo) => {
    const assertCleanRuntime = installRuntimeGuards(page, testInfo);

    await page.goto(`/${tenant}/dashboard`);
    await expect(page.getByRole('button', { name: 'Abrir menú' })).toBeVisible();
    await page.waitForLoadState('networkidle');

    await page.goto(`/${tenant}/settings`);
    await expect(page.getByRole('heading', { name: 'Ajustes' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Abrir menú' })).toBeVisible();

    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('heading', { name: 'Ajustes' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Abrir menú' })).toBeVisible();

    await page.getByRole('button', { name: /Apariencia|Perfil|Empresa/i }).first().click();
    await expect(page.getByRole('button', { name: 'Abrir menú' })).toBeVisible();

    await assertCleanRuntime();
  });
});
