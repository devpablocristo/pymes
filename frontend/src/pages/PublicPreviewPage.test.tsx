/* eslint-disable @typescript-eslint/ban-ts-comment */
// @ts-nocheck — vitest mocks use dynamic types that tsc cannot verify
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import type { SessionResponse } from '../lib/types';

const apiMocks = vi.hoisted(() => ({
  getSession: vi.fn<[], Promise<SessionResponse>>(),
  apiRequest: vi.fn(),
}));

const schedulingMocks = vi.hoisted(() => {
  const capturedProps: Array<{ orgRef: string; locale?: string; client: unknown }> = [];
  const client = { kind: 'public-scheduling-client' };
  return {
    capturedProps,
    client,
    createPublicSchedulingClient: vi.fn(() => client),
  };
});

vi.mock('../lib/api', () => ({
  getSession: () => apiMocks.getSession(),
  apiRequest: (...args: unknown[]) => apiMocks.apiRequest(...args),
}));

vi.mock('../components/PageLayout', () => ({
  PageLayout: ({
    title,
    lead,
    children,
  }: {
    title: React.ReactNode;
    lead?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      {lead ? <p>{lead}</p> : null}
      {children}
    </div>
  ),
}));

vi.mock('@devpablocristo/modules-scheduling', () => ({
  createPublicSchedulingClient: (...args: unknown[]) => schedulingMocks.createPublicSchedulingClient(...args),
  PublicSchedulingFlow: (props: { orgRef: string; locale?: string; client: unknown }) => {
    schedulingMocks.capturedProps.push(props);
    return (
      <div data-testid="public-scheduling-flow">
        flow:{props.orgRef}:{props.locale}
      </div>
    );
  },
}));

async function renderPublicPreviewPage() {
  const { PublicPreviewPage } = await import('./PublicPreviewPage');
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
          <PublicPreviewPage />
        </LanguageProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('PublicPreviewPage', () => {
  beforeEach(() => {
    vi.resetModules();
    apiMocks.getSession.mockReset();
    apiMocks.apiRequest.mockReset();
    schedulingMocks.createPublicSchedulingClient.mockClear();
    schedulingMocks.capturedProps.length = 0;
  });

  it('hydrates orgRef from session and passes it to the public scheduling flow', async () => {
    apiMocks.getSession.mockResolvedValue({
      auth: {
        org_id: '00000000-0000-0000-0000-000000000001',
        tenant_id: '00000000-0000-0000-0000-000000000001',
        role: 'owner',
        product_role: 'admin',
        scopes: ['admin:console:write'],
        actor: 'owner@example.com',
        auth_method: 'jwt',
      },
    });

    await renderPublicPreviewPage();

    const input = await screen.findByLabelText('Referencia de organización');
    await waitFor(() => {
      expect(input).toHaveValue('00000000-0000-0000-0000-000000000001');
    });
    await waitFor(() => {
      expect(screen.getByTestId('public-scheduling-flow')).toHaveTextContent(
        'flow:00000000-0000-0000-0000-000000000001:es',
      );
    });
    expect(schedulingMocks.createPublicSchedulingClient).toHaveBeenCalledTimes(1);
    expect(typeof schedulingMocks.createPublicSchedulingClient.mock.calls[0][0]).toBe('function');
  });

  it('reloads the preview flow when the operator enters another org ref', async () => {
    apiMocks.getSession.mockResolvedValue({
      auth: {
        org_id: '00000000-0000-0000-0000-000000000001',
        tenant_id: '00000000-0000-0000-0000-000000000001',
        role: 'owner',
        product_role: 'admin',
        scopes: ['admin:console:write'],
        actor: 'owner@example.com',
        auth_method: 'jwt',
      },
    });

    await renderPublicPreviewPage();

    const input = await screen.findByLabelText('Referencia de organización');
    fireEvent.change(input, { target: { value: 'demo-public-org' } });
    fireEvent.click(screen.getByRole('button', { name: 'Cargar' }));

    await waitFor(() => {
      expect(screen.getByTestId('public-scheduling-flow')).toHaveTextContent('flow:demo-public-org:es');
    });
    expect(schedulingMocks.capturedProps.at(-1)).toEqual(
      expect.objectContaining({
        orgRef: 'demo-public-org',
        locale: 'es',
        client: schedulingMocks.client,
      }),
    );
  });
});
