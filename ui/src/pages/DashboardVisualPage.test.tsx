/* eslint-disable @typescript-eslint/ban-ts-comment */
// @ts-nocheck — vitest mocks use dynamic types that tsc cannot verify
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { HttpError } from '@devpablocristo/core-authn/http/fetch';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';

const apiMocks = vi.hoisted(() => ({
  apiRequest: vi.fn(),
}));

const pageSearchMocks = vi.hoisted(() => ({
  usePageSearch: vi.fn(),
}));

const schedulingMocks = vi.hoisted(() => {
  const capturedProps: Array<{ client: unknown; locale?: string }> = [];
  const client = { kind: 'internal-scheduling-client' };
  return {
    capturedProps,
    client,
    createSchedulingClient: vi.fn(() => client),
  };
});

vi.mock('../lib/api', () => ({
  apiRequest: (...args: unknown[]) => apiMocks.apiRequest(...args),
}));

vi.mock('../components/PageSearch', () => ({
  usePageSearch: () => pageSearchMocks.usePageSearch(),
}));

vi.mock('../components/PageLayout', () => ({
  PageLayout: ({
    title,
    lead,
    actions,
    children,
  }: {
    title: React.ReactNode;
    lead?: React.ReactNode;
    actions?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      {lead ? <p>{lead}</p> : null}
      {actions}
      {children}
    </div>
  ),
}));

vi.mock('@devpablocristo/modules-scheduling', () => ({
  createSchedulingClient: (...args: unknown[]) => schedulingMocks.createSchedulingClient(...args),
}));

vi.mock('../modules/scheduling/BranchSchedulingDaySummary', () => ({
  BranchSchedulingDaySummary: (props: { client: unknown; locale?: string }) => {
    schedulingMocks.capturedProps.push(props);
    return <div data-testid="scheduling-day-summary">summary:{props.locale}</div>;
  },
}));

async function renderDashboardVisualPage(initialLanguage: 'es' | 'en' = 'es') {
  const { DashboardVisualPage } = await import('./DashboardVisualPage');
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <LanguageProvider initialLanguage={initialLanguage}>
          <DashboardVisualPage />
        </LanguageProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('DashboardVisualPage', () => {
  beforeEach(() => {
    vi.resetModules();
    apiMocks.apiRequest.mockReset();
    pageSearchMocks.usePageSearch.mockReset();
    schedulingMocks.createSchedulingClient.mockClear();
    schedulingMocks.capturedProps.length = 0;
    window.localStorage.clear();

    apiMocks.apiRequest.mockImplementation((path: string) => {
      if (String(path).includes('/v1/accounts/debtors')) {
        return Promise.resolve({ items: [] });
      }
      return Promise.resolve({ items: [] });
    });
  });

  it('registers page search and mounts the dashboard sections', async () => {
    await renderDashboardVisualPage();

    await waitFor(() => {
      expect(screen.getByText('Agenda de hoy')).toBeInTheDocument();
    });
    expect(pageSearchMocks.usePageSearch).toHaveBeenCalled();
    expect(schedulingMocks.createSchedulingClient).toHaveBeenCalledTimes(1);
    expect(typeof schedulingMocks.createSchedulingClient.mock.calls[0][0]).toBe('function');
    expect(schedulingMocks.capturedProps).toHaveLength(0);
    expect(apiMocks.apiRequest).toHaveBeenCalledWith('/v1/dashboard-data/sales-summary?context=home');
    expect(apiMocks.apiRequest).toHaveBeenCalledWith('/v1/dashboard-data/cashflow-summary?context=home');
    expect(apiMocks.apiRequest).toHaveBeenCalledWith('/v1/dashboard-data/quotes-pipeline?context=home');
    expect(apiMocks.apiRequest).toHaveBeenCalledWith('/v1/dashboard-data/recent-sales?context=home');
    expect(apiMocks.apiRequest).toHaveBeenCalledWith('/v1/dashboard-data/top-services?context=home');
  });

  it('does not crash when dashboard payloads are incomplete', async () => {
    apiMocks.apiRequest.mockImplementation((path: string) => {
      if (String(path).includes('/v1/dashboard-data/sales-summary')) {
        return Promise.resolve({});
      }
      if (String(path).includes('/v1/dashboard-data/recent-sales')) {
        return Promise.resolve({ items: [{ id: 'sale-without-customer', total: 1200, status: 'paid' }] });
      }
      if (String(path).includes('/v1/dashboard-data/top-customers')) {
        return Promise.resolve({ items: [{ id: 'customer-without-name', visit_count: 2 }] });
      }
      if (String(path).includes('/v1/purchases')) {
        return Promise.resolve({ items: [{ id: 'purchase-without-supplier', amount: 900, status: 'pending' }] });
      }
      if (String(path).includes('/v1/accounts/debtors')) {
        return Promise.resolve({ items: [{ party_id: 'debtor-without-name', total_debt: 500 }] });
      }
      return Promise.resolve({ items: [] });
    });

    await renderDashboardVisualPage();

    await waitFor(() => {
      expect(screen.getByText('Ventas del mes')).toBeInTheDocument();
      expect(screen.queryByText('undefined')).not.toBeInTheDocument();
    });
  });

  it('treats debtors server failures as an empty state', async () => {
    apiMocks.apiRequest.mockImplementation((path: string) => {
      if (String(path).includes('/v1/accounts/debtors')) {
        return Promise.reject(new HttpError('boom', 500, '{"error":"boom"}'));
      }
      return Promise.resolve({ items: [] });
    });

    await renderDashboardVisualPage();

    await waitFor(() => {
      expect(apiMocks.apiRequest).toHaveBeenCalledWith('/v1/accounts/debtors');
      expect(screen.queryByText('No pudimos cargar el dashboard.')).not.toBeInTheDocument();
    });
  });

  it('appends branch_id to dashboard endpoints when a branch is active', async () => {
    window.localStorage.setItem('pymes-ui:branch-selection:active', 'branch-active');

    await renderDashboardVisualPage();

    await waitFor(() => {
      expect(apiMocks.apiRequest).toHaveBeenCalledWith(
        '/v1/dashboard-data/recent-sales?context=home&branch_id=branch-active',
      );
    });
    expect(apiMocks.apiRequest).toHaveBeenCalledWith(
      '/v1/dashboard-data/sales-summary?context=home&branch_id=branch-active',
    );
    expect(apiMocks.apiRequest).toHaveBeenCalledWith(
      '/v1/dashboard-data/cashflow-summary?context=home&branch_id=branch-active',
    );
    expect(apiMocks.apiRequest).toHaveBeenCalledWith(
      '/v1/dashboard-data/quotes-pipeline?context=home&branch_id=branch-active',
    );
    expect(apiMocks.apiRequest).toHaveBeenCalledWith(
      '/v1/dashboard-data/top-services?context=home&branch_id=branch-active',
    );
  });
});
