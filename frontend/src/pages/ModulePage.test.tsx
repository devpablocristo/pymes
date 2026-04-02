import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ModulePage } from './ModulePage';

const crudMocks = vi.hoisted(() => ({
  hasLazyCrudResource: vi.fn<[string], Promise<boolean>>(),
}));

vi.mock('../crud/lazyCrudPage', () => ({
  hasLazyCrudResource: (...args: [string]) => crudMocks.hasLazyCrudResource(...args),
  LazyConfiguredCrudPage: ({ resourceId }: { resourceId: string }) => <div>CRUD {resourceId}</div>,
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

  it('renderiza el CRUD lazy cuando el recurso pertenece al catálogo CRUD', async () => {
    crudMocks.hasLazyCrudResource.mockResolvedValueOnce(true);

    renderModulePage('/modules/customers');

    expect(await screen.findByText('CRUD customers')).toBeInTheDocument();
  });
});
