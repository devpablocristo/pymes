/* eslint-disable @typescript-eslint/no-explicit-any -- mocks de test usan any para props de componentes */
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PageSearchProvider } from '../components/PageSearch';
import { LanguageProvider } from '../lib/i18n';
import { queryKeys } from '../lib/queryKeys';
import type { WorkOrder as AutoRepairWorkOrder } from '../lib/workOrdersApi';
import { WorkOrdersKanbanPanel } from './WorkOrdersKanbanPanel';

const apiMocks = vi.hoisted(() => ({
  getAllWorkOrders: vi.fn<[], Promise<AutoRepairWorkOrder[]>>(),
  getWorkOrdersArchived: vi.fn<[], Promise<AutoRepairWorkOrder[]>>(),
  patchWorkOrder: vi.fn(),
  loadLazyCrudPageConfig: vi.fn(),
}));

vi.mock('@clerk/react', () => ({
  useUser: () => ({ user: null, isLoaded: true }),
}));

vi.mock('../lib/auth', () => ({
  clerkEnabled: false,
}));

vi.mock('../lib/workOrdersApi', () => ({
  getAllWorkOrders: () => apiMocks.getAllWorkOrders(),
  getWorkOrdersArchived: () => apiMocks.getWorkOrdersArchived(),
  patchWorkOrder: (...args: unknown[]) => apiMocks.patchWorkOrder(...args),
}));

vi.mock('../crud/lazyCrudPage', () => ({
  loadLazyCrudPageConfig: (...args: unknown[]) => apiMocks.loadLazyCrudPageConfig(...args),
}));

vi.mock('@devpablocristo/modules-kanban-board', () => ({
  StatusKanbanBoard: ({ items, onCardOpen, toolbarButtonRow }: any) => (
    <div>
      <div>{toolbarButtonRow}</div>
      {items.map((row: AutoRepairWorkOrder) => (
        <button key={row.id} type="button" onClick={() => onCardOpen(row)}>
          {row.number} - {row.customer_name}
        </button>
      ))}
    </div>
  ),
}));

vi.mock('../components/WorkOrderKanbanDetailModal', () => ({
  WorkOrderKanbanDetailModal: ({ orderId, onSaved, onRecordRemoved }: any) => {
    if (!orderId) return null;
    return (
      <div>
        <button
          type="button"
          onClick={() =>
            onSaved({
              id: orderId,
              number: 'OT-001',
              customer_name: 'Cliente actualizado',
              status: 'in_progress',
            } as AutoRepairWorkOrder)
          }
        >
          Guardar modal
        </button>
        <button type="button" onClick={() => onRecordRemoved?.(orderId)}>
          Eliminar modal
        </button>
      </div>
    );
  },
}));

function buildWorkOrder(overrides?: Partial<AutoRepairWorkOrder>): AutoRepairWorkOrder {
  return {
    id: 'wo-1',
    org_id: 'org-1',
    number: 'OT-001',
    target_type: 'vehicle',
    target_id: 'veh-1',
    target_label: 'AAA111',
    metadata: {},
    vehicle_id: 'veh-1',
    vehicle_plate: 'AAA111',
    customer_id: 'cust-1',
    customer_name: 'Cliente original',
    booking_id: undefined,
    quote_id: undefined,
    sale_id: undefined,
    status: 'received',
    requested_work: 'Cambio de aceite',
    diagnosis: '',
    notes: '',
    internal_notes: '',
    currency: 'ARS',
    subtotal_services: 0,
    subtotal_parts: 0,
    tax_total: 0,
    total: 0,
    opened_at: '2026-04-02T10:00:00Z',
    promised_at: undefined,
    ready_at: undefined,
    delivered_at: undefined,
    ready_pickup_notified_at: undefined,
    created_by: 'tech-1',
    archived_at: null,
    created_at: '2026-04-02T10:00:00Z',
    updated_at: '2026-04-02T10:00:00Z',
    items: [],
    ...overrides,
  };
}

function renderKanban() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <LanguageProvider initialLanguage="es">
          <PageSearchProvider>
            <WorkOrdersKanbanPanel />
          </PageSearchProvider>
        </LanguageProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { queryClient };
}

describe('WorkOrdersKanbanPanel', () => {
  beforeEach(() => {
    apiMocks.getAllWorkOrders.mockReset();
    apiMocks.getWorkOrdersArchived.mockReset();
    apiMocks.patchWorkOrder.mockReset();
    apiMocks.loadLazyCrudPageConfig.mockReset();

    apiMocks.getAllWorkOrders.mockResolvedValue([
      buildWorkOrder(),
      buildWorkOrder({ id: 'wo-2', number: 'OT-002', customer_name: 'Cliente secundario' }),
    ]);
    apiMocks.getWorkOrdersArchived.mockResolvedValue([]);
    apiMocks.loadLazyCrudPageConfig.mockResolvedValue({
      toolbarActions: [],
      formFields: [],
      allowCreate: false,
      supportsArchived: false,
    });
  });

  it('sincroniza la caché de Query cuando el modal guarda una orden', async () => {
    const { queryClient } = renderKanban();

    expect(
      await screen.findByRole('button', { name: 'OT-001 - Cliente original' }, { timeout: 10_000 }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'OT-001 - Cliente original' }));
    fireEvent.click(screen.getByRole('button', { name: 'Guardar modal' }));

    await waitFor(
      () => {
        expect(screen.getByRole('button', { name: 'OT-001 - Cliente actualizado' })).toBeInTheDocument();
      },
      { timeout: 10_000 },
    );

    const cached = queryClient.getQueryData<AutoRepairWorkOrder[]>(queryKeys.carWorkOrders.kanban(false)) ?? [];
    expect(cached.find((row) => row.id === 'wo-1')?.customer_name).toBe('Cliente actualizado');
  });

  it('sincroniza la caché de Query cuando el modal elimina una orden', async () => {
    const { queryClient } = renderKanban();

    expect(
      await screen.findByRole('button', { name: 'OT-001 - Cliente original' }, { timeout: 10_000 }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'OT-001 - Cliente original' }));
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar modal' }));

    await waitFor(
      () => {
        expect(screen.queryByRole('button', { name: 'OT-001 - Cliente original' })).not.toBeInTheDocument();
      },
      { timeout: 10_000 },
    );

    const cached = queryClient.getQueryData<AutoRepairWorkOrder[]>(queryKeys.carWorkOrders.kanban(false)) ?? [];
    expect(cached.some((row) => row.id === 'wo-1')).toBe(false);
  });

  it('vuelve a pintar las órdenes cuando la query se recarga con nuevos datos', async () => {
    const { queryClient } = renderKanban();

    expect(
      await screen.findByRole('button', { name: 'OT-001 - Cliente original' }, { timeout: 10_000 }),
    ).toBeInTheDocument();

    apiMocks.getAllWorkOrders.mockResolvedValueOnce([
      buildWorkOrder({ id: 'wo-3', number: 'OT-003', customer_name: 'Cliente recargado' }),
    ]);

    await queryClient.invalidateQueries({ queryKey: queryKeys.carWorkOrders.kanban(false) });

    await waitFor(
      () => {
        expect(screen.getByRole('button', { name: 'OT-003 - Cliente recargado' })).toBeInTheDocument();
      },
      { timeout: 10_000 },
    );

    expect(screen.queryByRole('button', { name: 'OT-001 - Cliente original' })).not.toBeInTheDocument();
  });
});
