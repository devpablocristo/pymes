import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Branch, SchedulingClient } from '@devpablocristo/modules-scheduling';
import { BranchContext, type BranchContextValue } from '../../lib/branchSelectionContext';
import { BranchSchedulingDaySummary } from './BranchSchedulingDaySummary';

function buildBranch(id: string, name: string): Branch {
  return {
    id,
    org_id: 'org-demo',
    code: id,
    name,
    timezone: 'America/Argentina/Tucuman',
    address: `${name} 123`,
    active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  };
}

function buildBranchContextValue(overrides: Partial<BranchContextValue> = {}): BranchContextValue {
  return {
    orgId: 'org-demo',
    branches: [
      buildBranch('branch-a', 'Casa Central'),
      buildBranch('branch-b', 'Sucursal Norte'),
    ],
    availableBranches: [
      buildBranch('branch-a', 'Casa Central'),
      buildBranch('branch-b', 'Sucursal Norte'),
    ],
    selectedBranchId: 'branch-b',
    selectedBranch: buildBranch('branch-b', 'Sucursal Norte'),
    isLoading: false,
    isError: false,
    error: null,
    setSelectedBranchId: vi.fn(),
    ...overrides,
  };
}

function renderSummary(client: SchedulingClient, branchContextValue?: BranchContextValue) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <BranchContext.Provider value={branchContextValue ?? buildBranchContextValue()}>
        <BranchSchedulingDaySummary client={client} locale="es-AR" />
      </BranchContext.Provider>
    </QueryClientProvider>,
  );
}

describe('BranchSchedulingDaySummary', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('queries dashboard and day endpoints using the globally selected branch', async () => {
    const client = {
      getDashboard: vi.fn().mockResolvedValue({
        bookings_today: 3,
        confirmed_bookings_today: 2,
        active_queues: 1,
        waiting_tickets: 4,
      }),
      getDayAgenda: vi.fn().mockResolvedValue([]),
    } as unknown as SchedulingClient;

    renderSummary(client);

    await waitFor(() => {
      expect(screen.getByText('Reservas')).toBeInTheDocument();
      expect(screen.getByText('3')).toBeInTheDocument();
    });

    expect(client.getDashboard).toHaveBeenCalledWith('branch-b', expect.any(String));
    expect(client.getDayAgenda).toHaveBeenCalledWith('branch-b', expect.any(String));
    expect(screen.getByText(/Sucursal Norte/)).toBeInTheDocument();
  });

  it('shows loading state while branch context is still hydrating', () => {
    const client = {
      getDashboard: vi.fn(),
      getDayAgenda: vi.fn(),
    } as unknown as SchedulingClient;

    renderSummary(
      client,
      buildBranchContextValue({
        branches: [],
        availableBranches: [],
        selectedBranch: null,
        selectedBranchId: null,
        isLoading: true,
      }),
    );

    expect(screen.getByText('Cargando resumen de agenda…')).toBeInTheDocument();
    expect(client.getDashboard).not.toHaveBeenCalled();
    expect(client.getDayAgenda).not.toHaveBeenCalled();
  });
});
