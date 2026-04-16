import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Branch } from '@devpablocristo/modules-scheduling/next';
import type { SessionResponse } from './types';
import { BranchProvider, useBranchSelection } from './branchContext';

const apiMocks = vi.hoisted(() => ({
  apiRequest: vi.fn(),
  getSession: vi.fn<[], Promise<SessionResponse>>(),
}));

vi.mock('./api', () => ({
  apiRequest: (...args: unknown[]) => apiMocks.apiRequest(...args),
  getSession: () => apiMocks.getSession(),
}));

function buildBranch(id: string, name: string, active = true): Branch {
  return {
    id,
    org_id: 'org-1',
    code: id,
    name,
    timezone: 'America/Argentina/Tucuman',
    address: '',
    active,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  };
}

function Probe() {
  const branch = useBranchSelection();

  return (
    <div>
      <div data-testid="selected-branch">{branch.selectedBranchId ?? ''}</div>
      <div data-testid="branch-count">{String(branch.availableBranches.length)}</div>
      <button type="button" onClick={() => branch.setSelectedBranchId('branch-b')}>
        change
      </button>
    </div>
  );
}

describe('BranchProvider', () => {
  beforeEach(() => {
    window.localStorage.clear();
    apiMocks.apiRequest.mockReset();
    apiMocks.getSession.mockReset();
    apiMocks.getSession.mockResolvedValue({
      auth: {
        org_id: 'org-1',
        org_name: 'Org Demo',
        tenant_id: 'org-1',
        role: 'admin',
        product_role: 'admin',
        scopes: [],
        actor: 'user-1',
        auth_method: 'jwt',
      },
    });
  });

  it('rehydrates the stored branch selection for the current org', async () => {
    window.localStorage.setItem('pymes-ui:branch-selection:org-1', 'branch-b');
    apiMocks.apiRequest.mockResolvedValue({
      items: [buildBranch('branch-a', 'Casa Central'), buildBranch('branch-b', 'Sucursal Norte')],
    });

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <BranchProvider>
          <Probe />
        </BranchProvider>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('selected-branch')).toHaveTextContent('branch-b');
    });
    expect(screen.getByTestId('branch-count')).toHaveTextContent('2');
  });

  it('falls back to the first available branch and persists manual changes', async () => {
    apiMocks.apiRequest.mockResolvedValue({
      items: [buildBranch('branch-a', 'Casa Central'), buildBranch('branch-b', 'Sucursal Norte')],
    });

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <BranchProvider>
          <Probe />
        </BranchProvider>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('selected-branch')).toHaveTextContent('branch-a');
    });

    fireEvent.click(screen.getByRole('button', { name: 'change' }));

    await waitFor(() => {
      expect(screen.getByTestId('selected-branch')).toHaveTextContent('branch-b');
    });
    expect(window.localStorage.getItem('pymes-ui:branch-selection:org-1')).toBe('branch-b');
  });
});
