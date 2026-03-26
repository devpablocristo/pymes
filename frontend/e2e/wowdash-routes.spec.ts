import { expect, test, type Page } from '@playwright/test';
import {
  WOWDASH_TEMPLATE_FEATURE_COUNT,
  WOWDASH_TEMPLATE_FEATURES,
  type WowdashTemplateFeature,
} from './wowdashRoutePaths';

const TENANT_PROFILE_KEY = 'pymes-ui:pymes:tenant_profile';
const TENANT_PROFILE_JSON = JSON.stringify({
  businessName: 'E2E',
  teamSize: 'solo',
  sells: 'services',
  clientLabel: 'Cliente',
  usesScheduling: false,
  usesBilling: false,
  currency: 'USD',
  paymentMethod: 'cash',
  vertical: 'none',
  completedAt: new Date().toISOString(),
});

/** Orden fijo de bloques en el reporte (1 feature = 1 test dentro del bloque). */
const CATEGORY_ORDER: string[] = [
  'Dashboards',
  'Usuarios y perfiles',
  'Componentes UI',
  'Formularios',
  'Tablas',
  'Gráficos y widgets',
  'Aplicación',
  'Facturas',
  'IA (demo)',
  'Cripto / marketplace',
  'Galería y blog',
  'Páginas de contenido',
  'Estado y errores',
  'Ajustes (demo)',
  'Catch-all',
];

test.beforeEach(async ({ context }) => {
  await context.addInitScript((payload: { key: string; value: string }) => {
    window.localStorage.setItem(payload.key, payload.value);
  }, { key: TENANT_PROFILE_KEY, value: TENANT_PROFILE_JSON });
});

test.describe('Inventario template Wowdash', () => {
  test(`hay ${WOWDASH_TEMPLATE_FEATURE_COUNT} features mapeados 1:1 (incl. catch-all E2E)`, () => {
    expect(WOWDASH_TEMPLATE_FEATURES.length).toBe(WOWDASH_TEMPLATE_FEATURE_COUNT);
    const segments = new Set(WOWDASH_TEMPLATE_FEATURES.map((f) => f.segment));
    expect(segments.size).toBe(WOWDASH_TEMPLATE_FEATURE_COUNT);
  });
});

async function assertFeatureLoads(page: Page, f: WowdashTemplateFeature) {
  const path = f.segment === '' ? '/console/wowdash' : `/console/wowdash/${f.segment}`;
  const pageErrors: string[] = [];
  page.on('pageerror', (err) => {
    pageErrors.push(err.message);
  });

  const response = await page.goto(path, { waitUntil: 'load', timeout: 85_000 });
  expect(response, `HTTP ${path}`).not.toBeNull();
  expect(response!.status(), `status ${path}`).toBeLessThan(400);

  await expect(page.locator('#wowdash-template-root')).toBeVisible({ timeout: 45_000 });
  await expect(page.locator('#wowdash-template-root')).not.toBeEmpty();
  await page.waitForTimeout(2500);

  await expect(page.getByRole('heading', { name: 'Something went wrong' })).not.toBeVisible();
  await expect(page.locator('.error-boundary-fallback')).not.toBeVisible();

  expect(pageErrors, `pageerror ${path}\n${pageErrors.join('\n')}`).toEqual([]);
}

for (const category of CATEGORY_ORDER) {
  const items = WOWDASH_TEMPLATE_FEATURES.filter((f) => f.category === category);
  if (items.length === 0) {
    continue;
  }

  test.describe(`Feature template — ${category}`, () => {
    for (const f of items) {
      const slug = f.segment === '' ? 'index' : f.segment;
      test(`${slug} — ${f.titleEs}`, async ({ page }) => {
        await assertFeatureLoads(page, f);
      });
    }
  });
}
