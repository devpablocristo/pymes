import { expect, test } from '@playwright/test';
import { expectNoCrudFailure, installRuntimeGuards } from './guards';

const tenant = process.env.E2E_REAL_TENANT_SLUG ?? 'bicimax';

const crudRoutes = [
  { id: 'invoices', path: 'invoices', title: 'Facturación' },
  { id: 'customers', path: 'customers', title: 'Clientes' },
  { id: 'suppliers', path: 'suppliers', title: 'Proveedores' },
  { id: 'products', path: 'products', title: 'Productos' },
  { id: 'services', path: 'services', title: 'Servicios' },
  { id: 'priceLists', path: 'price-lists', title: 'Listas de precios' },
  { id: 'quotes', path: 'quotes', title: 'Presupuestos' },
  { id: 'sales', path: 'sales', title: 'Ventas' },
  { id: 'purchases', path: 'purchases', title: 'Compras' },
  { id: 'returns', path: 'returns', title: 'Devoluciones' },
  { id: 'creditNotes', path: 'credit-notes', title: 'Notas de crédito' },
  { id: 'inventory', path: 'inventory', title: 'Inventario' },
  { id: 'cashflow', path: 'cashflow', title: 'Caja' },
  { id: 'payments', path: 'payments', title: 'Pagos' },
  { id: 'recurring', path: 'recurring', title: 'Gastos recurrentes' },
  { id: 'employees', path: 'employees', title: 'Empleados' },
  { id: 'procurementRequests', path: 'procurement-requests', title: 'Solicitudes de compra internas' },
  { id: 'accounts', path: 'accounts', title: 'Cuentas corrientes' },
  { id: 'roles', path: 'roles', title: 'Roles' },
  { id: 'parties', path: 'parties', title: 'Entidades' },
  { id: 'webhooks', path: 'webhooks', title: 'Webhooks' },
  { id: 'professionals', path: 'professionals', title: 'Profesionales' },
  { id: 'specialties', path: 'specialties', title: 'Especialidades' },
  { id: 'intakes', path: 'intakes', title: 'Ingresos' },
  { id: 'sessions', path: 'sessions', title: 'Sesiones' },
  { id: 'occupationalHealthExams', path: 'medical/occupational-health/exams', title: 'Medicina laboral' },
  { id: 'workshopVehicles', path: 'workshop-vehicles', title: 'Vehículos' },
  { id: 'carWorkOrders', path: 'car-work-orders', title: 'Órdenes de trabajo' },
  { id: 'bikeWorkOrders', path: 'bike-work-orders', title: 'Órdenes de trabajo' },
  { id: 'restaurantDiningAreas', path: 'restaurant-dining-areas', title: 'Zonas del salón' },
  { id: 'restaurantDiningTables', path: 'restaurant-dining-tables', title: 'Mesas' },
];

async function confirmDialog(page: import('@playwright/test').Page, dialogName: RegExp, buttonName: RegExp) {
  const dialog = page.getByRole('dialog', { name: dialogName });
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: buttonName }).click();
}

function crudModeUrlPattern(path: string, modePath = '(list|gallery|board)') {
  return new RegExp(`/[^/]+/${path}/${modePath}$`);
}

test.describe('CRUD uniforme real', () => {
  test.afterEach(async ({ page }, testInfo) => {
    await testInfo.attach('final-url', { body: page.url(), contentType: 'text/plain' });
  });

  for (const crud of crudRoutes) {
    test(`${crud.id}: navega Lista/Galería/Tablero sin errores`, async ({ page }, testInfo) => {
      const assertCleanRuntime = installRuntimeGuards(page, testInfo);

      await page.goto(`/${tenant}/${crud.path}`);
      await expect(page.getByRole('heading', { name: crud.title })).toBeVisible();
      await expect(page).toHaveURL(crudModeUrlPattern(crud.path));
      await expectNoCrudFailure(page);

      for (const view of [
        { label: 'Lista', path: 'list' },
        { label: 'Galería', path: 'gallery' },
        { label: 'Tablero', path: 'board' },
      ]) {
        await page.getByRole('link', { name: view.label }).click();
        await expect(page).toHaveURL(crudModeUrlPattern(crud.path, view.path));
        await expect(page.getByRole('heading', { name: crud.title })).toBeVisible();
        await expectNoCrudFailure(page);
        await page.reload({ waitUntil: 'domcontentloaded' });
        await expect(page).toHaveURL(crudModeUrlPattern(crud.path, view.path));
        await expect(page.getByRole('heading', { name: crud.title })).toBeVisible();
        await expectNoCrudFailure(page);
      }

      await page.goBack();
      await expect(page).toHaveURL(crudModeUrlPattern(crud.path, 'gallery'));
      await page.goForward();
      await expect(page).toHaveURL(crudModeUrlPattern(crud.path, 'board'));
      await assertCleanRuntime();
    });
  }

  test('services: create/update/archive/restore/hard-delete real CRUD lifecycle', async ({ page }, testInfo) => {
    const assertCleanRuntime = installRuntimeGuards(page, testInfo);
    const unique = `Servicio QA ${Date.now()}`;
    const updated = `${unique} editado`;

    await page.goto(`/${tenant}/services/list`);
    await page.getByRole('button', { name: /\+ Nuevo servicio|Nuevo servicio/i }).click();
    await page.getByLabel(/Nombre/i).fill(unique);
    await page.getByLabel(/Código/i).fill(`QA-${Date.now()}`);
    await page.getByLabel(/Precio/i).fill('1000');
    await page.getByRole('button', { name: /Guardar|Crear|Agregar/i }).click();
    await expect(page.getByText(unique)).toBeVisible();

    await page.getByText(unique).first().click();
    await page.getByRole('button', { name: /Editar/i }).click();
    await page.getByLabel(/Nombre/i).fill(updated);
    await page.getByRole('button', { name: /Guardar/i }).click();
    await expect(page.getByText(updated)).toBeVisible();

    await page.getByText(updated).first().click();
    await page.getByRole('button', { name: /Archivar|Eliminar/i }).click();
    await confirmDialog(page, /Archivar servicio/i, /^Archivar$/i);
    await expect(page.getByText(updated)).toHaveCount(0);

    await page.getByRole('button', { name: /Ver archivados|Archivados/i }).click();
    await expect(page.getByText(updated)).toBeVisible();
    await page.getByText(updated).first().click();
    await page.getByRole('button', { name: /Restaurar/i }).click();
    await expect(page.getByText(updated)).toHaveCount(0);

    await page.getByRole('button', { name: /Ver activos|Activos/i }).click();
    await expect(page.getByText(updated)).toBeVisible();
    await page.getByText(updated).first().click();
    await page.getByRole('button', { name: /Archivar|Eliminar/i }).click();
    await confirmDialog(page, /Archivar servicio/i, /^Archivar$/i);
    await page.getByRole('button', { name: /Ver archivados|Archivados/i }).click();
    await page.getByText(updated).first().click();
    await page.getByRole('button', { name: /Eliminar definitivo/i }).click();
    await confirmDialog(page, /Eliminar servicio/i, /^Eliminar definitivo$/i);
    await expect(page.getByText(updated)).toHaveCount(0);

    await expectNoCrudFailure(page);
    await assertCleanRuntime();
  });

  test('occupationalHealthExams: create/update/archive/restore/hard-delete real CRUD lifecycle', async ({ page }, testInfo) => {
    const assertCleanRuntime = installRuntimeGuards(page, testInfo);
    const unique = `Trabajador QA ${Date.now()}`;
    const updated = `${unique} editado`;

    await page.goto(`/${tenant}/medical/occupational-health/exams/list`);
    await expect(page.getByRole('heading', { name: 'Medicina laboral' })).toBeVisible();
    await expect(page).toHaveURL(crudModeUrlPattern('medical/occupational-health/exams', 'list'));

    await page.getByRole('button', { name: /\+ Nuevo examen|Nuevo examen/i }).click();
    await page.getByLabel(/Trabajador/i).fill(unique);
    await page.getByLabel(/Documento/i).fill(`QA-${Date.now()}`);
    await page.getByLabel(/Empresa/i).fill('Empresa QA');
    await page.getByRole('button', { name: /Crear|Guardar|Agregar/i }).click();
    await expect(page.getByText(unique)).toBeVisible();

    await page.getByText(unique).first().click();
    await page.getByRole('button', { name: /Editar/i }).click();
    await page.getByLabel(/Trabajador/i).fill(updated);
    await page.getByRole('button', { name: /Guardar/i }).click();
    await expect(page.getByText(updated)).toBeVisible();

    await page.getByText(updated).first().click();
    await page.getByRole('button', { name: /Archivar|Eliminar/i }).click();
    await confirmDialog(page, /Archivar examen laboral/i, /^Archivar$/i);
    await expect(page.getByText(updated)).toHaveCount(0);

    await page.getByRole('button', { name: /Ver archivados|Archivados/i }).click();
    await expect(page.getByText(updated)).toBeVisible();
    await page.getByText(updated).first().click();
    await page.getByRole('button', { name: /Restaurar/i }).click();
    await expect(page.getByText(updated)).toHaveCount(0);

    await page.getByRole('button', { name: /Ver activos|Activos/i }).click();
    await expect(page.getByText(updated)).toBeVisible();
    await page.getByText(updated).first().click();
    await page.getByRole('button', { name: /Archivar|Eliminar/i }).click();
    await confirmDialog(page, /Archivar examen laboral/i, /^Archivar$/i);
    await page.getByRole('button', { name: /Ver archivados|Archivados/i }).click();
    await page.getByText(updated).first().click();
    await page.getByRole('button', { name: /Eliminar definitivo/i }).click();
    await confirmDialog(page, /Eliminar examen laboral/i, /^Eliminar definitivo$/i);
    await expect(page.getByText(updated)).toHaveCount(0);

    await expectNoCrudFailure(page);
    await assertCleanRuntime();
  });
});
