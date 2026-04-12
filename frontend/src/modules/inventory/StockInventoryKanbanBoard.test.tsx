/* eslint-disable @typescript-eslint/no-explicit-any -- mocks ligeros para superficie Kanban */
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../../lib/i18n';
import { StockInventoryKanbanBoard } from './StockInventoryKanbanBoard';

const apiMocks = vi.hoisted(() => ({
  apiRequest: vi.fn(),
  loadLazyCrudPageConfig: vi.fn(),
}));

vi.mock('../../lib/api', () => ({
  apiRequest: (...args: unknown[]) => apiMocks.apiRequest(...args),
}));

vi.mock('../../crud/lazyCrudPage', () => ({
  loadLazyCrudPageConfig: (...args: unknown[]) => apiMocks.loadLazyCrudPageConfig(...args),
}));

vi.mock('../../lib/useCrudListCreatedByMerge', () => ({
  useCrudListCreatedByMerge: () => ({}),
}));

vi.mock('@devpablocristo/modules-kanban-board', () => ({
  StatusKanbanBoard: ({ toolbarButtonRow }: any) => <div>{toolbarButtonRow}</div>,
}));

vi.mock('./StockLevelDetailModal', () => ({
  StockLevelDetailModal: () => null,
}));

function renderBoard() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <LanguageProvider initialLanguage="es">
          <StockInventoryKanbanBoard />
        </LanguageProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('StockInventoryKanbanBoard', () => {
  beforeEach(() => {
    apiMocks.apiRequest.mockReset();
    apiMocks.loadLazyCrudPageConfig.mockReset();

    apiMocks.apiRequest.mockResolvedValue({
      items: [
        {
          product_id: 'prod-1',
          product_name: 'Producto Demo A',
          sku: 'SKU-1',
          quantity: 10,
          min_quantity: 5,
          is_low_stock: false,
          updated_at: '2026-04-11T00:00:00Z',
        },
      ],
    });

    apiMocks.loadLazyCrudPageConfig.mockResolvedValue({
      supportsArchived: true,
      toolbarActions: [
        {
          id: 'stock-new-product',
          label: '+ Nuevo producto',
          kind: 'primary',
        },
      ],
    });
  });

  it('muestra una sola vez el botón + Nuevo producto y expone el toggle de archivados', async () => {
    renderBoard();

    const buttons = await screen.findAllByRole('button', { name: '+ Nuevo producto' });
    expect(buttons).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Ver archivados' })).toBeInTheDocument();
  });
});
