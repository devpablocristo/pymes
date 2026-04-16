import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ModulePage } from './ModulePage';

const crudMocks = vi.hoisted(() => ({
  hasLazyCrudResource: vi.fn<[string], Promise<boolean>>(),
}));

const apiMocks = vi.hoisted(() => ({
  apiRequest: vi.fn<[string], Promise<unknown>>(),
  getSession: vi.fn<[], Promise<unknown>>(),
  downloadAPIFile: vi.fn(),
}));

vi.mock('../crud/lazyCrudPage', () => ({
  hasLazyCrudResource: (...args: [string]) => crudMocks.hasLazyCrudResource(...args),
}));

vi.mock('../crud/configuredCrudViews', () => ({
  ConfiguredCrudStandalonePage: ({ resourceId }: { resourceId: string }) => (
    <div>
      standalone:{resourceId}
    </div>
  ),
}));

vi.mock('../lib/api', () => ({
  apiRequest: (...args: [string]) => apiMocks.apiRequest(...args),
  getSession: () => apiMocks.getSession(),
  downloadAPIFile: (...args: unknown[]) => apiMocks.downloadAPIFile(...args),
}));

function renderModulePage(initialPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/modules/:moduleId" element={<ModulePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe('ModulePage', () => {
  afterEach(() => {
    crudMocks.hasLazyCrudResource.mockReset();
    apiMocks.apiRequest.mockReset();
    apiMocks.getSession.mockReset();
    apiMocks.downloadAPIFile.mockReset();
  });

  it('muestra estado de carga mientras resuelve si el módulo es CRUD', () => {
    const pending = deferred<boolean>();
    crudMocks.hasLazyCrudResource.mockReturnValue(pending.promise);

    renderModulePage('/modules/customers');

    expect(screen.getByRole('heading', { level: 1, name: 'Módulo' })).toBeInTheDocument();
    expect(screen.getByText('Cargando modulo…')).toBeInTheDocument();
  });

  it('muestra fallback de error si falla la resolución del módulo', async () => {
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    crudMocks.hasLazyCrudResource.mockRejectedValueOnce(new Error('boom module'));

    renderModulePage('/modules/customers');

    await waitFor(() => {
      expect(screen.getByText('boom module')).toBeInTheDocument();
    });
    expect(screen.getByText('No se pudo resolver la configuración del módulo.')).toBeInTheDocument();

    consoleErrorSpy.mockRestore();
  });

  it('renderiza la shell CRUD standalone cuando el recurso pertenece al catálogo CRUD', async () => {
    crudMocks.hasLazyCrudResource.mockResolvedValueOnce(true);

    renderModulePage('/modules/customers');

    expect(await screen.findByText('standalone:customers')).toBeInTheDocument();
  });

  it('renderiza reportes como página de negocio y no como explorer técnico', async () => {
    crudMocks.hasLazyCrudResource.mockResolvedValueOnce(false);
    apiMocks.apiRequest.mockImplementation(async (path: string) => {
      if (path.includes('sales-summary')) {
        return { from: '2026-04-01', to: '2026-04-13', data: { total_sales: 120000, count_sales: 8, average_ticket: 15000 } };
      }
      if (path.includes('sales-by-product')) {
        return { items: [{ product_name: 'Semilla', quantity: 5, revenue: 50000 }] };
      }
      if (path.includes('sales-by-service')) {
        return { items: [] };
      }
      if (path.includes('sales-by-customer')) {
        return { items: [{ customer_name: 'Juan Pérez', total: 50000, count: 2 }] };
      }
      if (path.includes('sales-by-payment')) {
        return { items: [{ payment_method: 'Transferencia', total: 50000, count: 2 }] };
      }
      if (path.includes('inventory-valuation')) {
        return { total: 83000, items: [{ product_name: 'Semilla', quantity: 10, cost_price: 3000, valuation: 30000 }] };
      }
      if (path.includes('low-stock')) {
        return { items: [{ product_name: 'Fertilizante', quantity: 1, min_quantity: 5 }] };
      }
      if (path.includes('cashflow-summary')) {
        return { from: '2026-04-01', to: '2026-04-13', data: { total_income: 150000, total_expense: 70000, balance: 80000 } };
      }
      if (path.includes('profit-margin')) {
        return { from: '2026-04-01', to: '2026-04-13', data: { revenue: 120000, cost: 80000, gross_profit: 40000, margin_pct: 33.3 } };
      }
      return {};
    });

    renderModulePage('/modules/reports');

    expect(await screen.findByRole('heading', { level: 1, name: 'Reportes' })).toBeInTheDocument();
    expect(await screen.findByText('Ventas por producto')).toBeInTheDocument();
    expect(screen.getByText('Balance')).toBeInTheDocument();
    expect(screen.queryByText('Ruta en la consola')).not.toBeInTheDocument();
    expect(screen.queryByText('Org activa')).not.toBeInTheDocument();
    expect(screen.queryByText('/v1/reports/sales-summary')).not.toBeInTheDocument();
  });
});
