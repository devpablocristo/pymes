/* eslint-disable @typescript-eslint/ban-ts-comment */
// @ts-nocheck — vitest mocks use dynamic types that tsc cannot verify
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import { BranchContext, type BranchContextValue } from '../lib/branchSelectionContext';

const pageSearchMocks = vi.hoisted(() => ({
  usePageSearch: vi.fn(),
}));

const schedulingMocks = vi.hoisted(() => {
  const capturedCalendarProps: Array<{
    client: unknown;
    locale?: string;
    searchQuery?: string;
    initialBranchId?: string;
  }> = [];
  const capturedQueueProps: Array<{ client: unknown; locale?: string; searchQuery?: string }> = [];
  const client = { kind: 'internal-scheduling-client' };
  return {
    client,
    capturedCalendarProps,
    capturedQueueProps,
    createSchedulingClient: vi.fn(() => client),
  };
});

vi.mock('../lib/api', () => ({
  apiRequest: vi.fn(),
}));

vi.mock('../components/PageSearch', () => ({
  usePageSearch: () => pageSearchMocks.usePageSearch(),
}));

vi.mock('../components/PageLayout', () => ({
  PageLayout: ({
    title,
    lead,
    inlineActions,
    actions,
    children,
  }: {
    title: React.ReactNode;
    lead?: React.ReactNode;
    inlineActions?: React.ReactNode;
    actions?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      {lead ? <p>{lead}</p> : null}
      {inlineActions}
      {actions}
      {children}
    </div>
  ),
}));

vi.mock('@devpablocristo/modules-scheduling/styles.next.css', () => ({}));

vi.mock('@devpablocristo/modules-scheduling/next', () => ({
  createSchedulingClient: (...args: unknown[]) => schedulingMocks.createSchedulingClient(...args),
  SchedulingCalendar: (props: { client: unknown; locale?: string; searchQuery?: string; initialBranchId?: string }) => {
    schedulingMocks.capturedCalendarProps.push(props);
    return (
      <div data-testid="scheduling-calendar">
        calendar:{props.locale}:{props.searchQuery}
      </div>
    );
  },
  QueueOperatorBoard: (props: { client: unknown; locale?: string; searchQuery?: string }) => {
    schedulingMocks.capturedQueueProps.push(props);
    return (
      <div data-testid="queue-operator-board">
        queue:{props.locale}:{props.searchQuery}
      </div>
    );
  },
}));

describe('CalendarPage', () => {
  beforeEach(() => {
    pageSearchMocks.usePageSearch.mockReset();
    schedulingMocks.createSchedulingClient.mockClear();
    schedulingMocks.capturedCalendarProps.length = 0;
    schedulingMocks.capturedQueueProps.length = 0;
    pageSearchMocks.usePageSearch.mockReturnValue('cliente demo');
  });

  it('mounts scheduling calendar with the shared client, locale and page search query', async () => {
    const { CalendarPage } = await import('./CalendarPage');
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    const branchContextValue: BranchContextValue = {
      orgId: 'org-demo',
      branches: [{ id: 'branch-central', name: 'Casa Central', active: true }],
      availableBranches: [{ id: 'branch-central', name: 'Casa Central', active: true }],
      selectedBranchId: 'branch-central',
      selectedBranch: { id: 'branch-central', name: 'Casa Central', active: true },
      isLoading: false,
      isError: false,
      error: null,
      setSelectedBranchId: vi.fn(),
    };

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
          <LanguageProvider initialLanguage="es">
            <BranchContext.Provider value={branchContextValue}>
              <CalendarPage />
            </BranchContext.Provider>
          </LanguageProvider>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('scheduling-calendar')).toHaveTextContent('calendar:es:cliente demo');
    });
    expect(pageSearchMocks.usePageSearch).toHaveBeenCalled();
    expect(schedulingMocks.createSchedulingClient).toHaveBeenCalledTimes(1);
      expect(schedulingMocks.capturedCalendarProps.at(-1)).toEqual(
        expect.objectContaining({
          client: schedulingMocks.client,
          locale: 'es',
          searchQuery: 'cliente demo',
          initialBranchId: 'branch-central',
        }),
      );
    // QueueOperatorBoard se removió de CalendarPage en Stage 3.
    // Si en el futuro vuelve, va en su propia página (no embebido en agenda).
    expect(screen.queryByTestId('queue-operator-board')).toBeNull();
  });
});
