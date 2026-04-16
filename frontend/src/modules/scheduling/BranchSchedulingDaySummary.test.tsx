import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { SchedulingClient } from '@devpablocristo/modules-scheduling';
import { BranchSchedulingDaySummary } from './BranchSchedulingDaySummary';

const branchMocks = vi.hoisted(() => ({
  useBranchSelection: vi.fn(),
}));

vi.mock('../../lib/branchContext', () => ({
  useBranchSelection: () => branchMocks.useBranchSelection(),
}));

function renderSummary(client: SchedulingClient) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <BranchSchedulingDaySummary client={client} locale="es-AR" />
    </QueryClientProvider>,
  );
}

describe('BranchSchedulingDaySummary', () => {
  beforeEach(() => {
    branchMocks.useBranchSelection.mockReset();
    branchMocks.useBranchSelection.mockReturnValue({
      availableBranches: [
        { id: 'branch-a', name: 'Casa Central' },
        { id: 'branch-b', name: 'Sucursal Norte' },
      ],
      selectedBranch: { id: 'branch-b', name: 'Sucursal Norte' },
      selectedBranchId: 'branch-b',
      isLoading: false,
    });
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
    branchMocks.useBranchSelection.mockReturnValue({
      availableBranches: [],
      selectedBranch: null,
      selectedBranchId: null,
      isLoading: true,
    });

    const client = {
      getDashboard: vi.fn(),
      getDayAgenda: vi.fn(),
    } as unknown as SchedulingClient;

    renderSummary(client);

    expect(screen.getByText('Cargando resumen de agenda…')).toBeInTheDocument();
    expect(client.getDashboard).not.toHaveBeenCalled();
    expect(client.getDayAgenda).not.toHaveBeenCalled();
  });
});
