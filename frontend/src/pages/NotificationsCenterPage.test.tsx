/* eslint-disable @typescript-eslint/ban-ts-comment */
// @ts-nocheck — vitest mocks use dynamic types that tsc cannot verify
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import { NOTIFICATION_CHAT_HANDOFF_KEY } from '../lib/notificationChatHandoff';
import type { InAppNotificationItem } from '../lib/api';

const apiMocks = vi.hoisted(() => ({
  listInAppNotifications: vi.fn(),
  markInAppNotificationRead: vi.fn(),
  createInsightNotifications: vi.fn(),
}));

const reviewMocks = vi.hoisted(() => ({
  approveRequest: vi.fn(),
  rejectRequest: vi.fn(),
}));

const navigationMocks = vi.hoisted(() => ({
  navigate: vi.fn(),
}));

const pageSearchMocks = vi.hoisted(() => ({
  usePageSearch: vi.fn(),
}));

const shareMocks = vi.hoisted(() => ({
  openWhatsAppPrefilledShare: vi.fn(),
  buildApprovalShareText: vi.fn(() => 'share approval'),
  buildInAppNotificationShareText: vi.fn(() => 'share notification'),
}));

vi.mock('../lib/api', () => ({
  listInAppNotifications: () => apiMocks.listInAppNotifications(),
  markInAppNotificationRead: (...args: unknown[]) => apiMocks.markInAppNotificationRead(...args),
}));

vi.mock('../lib/aiApi', () => ({
  createInsightNotifications: (...args: unknown[]) => apiMocks.createInsightNotifications(...args),
}));

vi.mock('../lib/reviewApi', () => ({
  approveRequest: (...args: unknown[]) => reviewMocks.approveRequest(...args),
  rejectRequest: (...args: unknown[]) => reviewMocks.rejectRequest(...args),
}));

vi.mock('../components/PageSearch', () => ({
  usePageSearch: () => pageSearchMocks.usePageSearch(),
}));

vi.mock('@devpablocristo/modules-search', () => ({
  useSearch: <T,>(items: T[]) => items,
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');
  return {
    ...actual,
    useNavigate: () => navigationMocks.navigate,
  };
});

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

vi.mock('@devpablocristo/modules-ui-notification-feed', () => ({
  NotificationFeed: ({
    items,
    summary,
    loading,
    error,
    emptyMessage,
  }: {
    items: Array<{
      id: string;
      title: React.ReactNode;
      body: React.ReactNode;
      actions?: React.ReactNode;
      extra?: React.ReactNode;
      timestamp?: React.ReactNode;
    }>;
    summary?: React.ReactNode;
    loading?: boolean;
    error?: React.ReactNode;
    emptyMessage?: React.ReactNode;
  }) => (
    <div>
      {summary ? <div data-testid="notifications-summary">{summary}</div> : null}
      {loading ? <div data-testid="notifications-loading">loading</div> : null}
      {error}
      {items.length === 0 ? emptyMessage : null}
      {items.map((item) => (
        <article key={item.id} data-testid={`notification-${item.id}`}>
          <div>{item.title}</div>
          <div>{item.body}</div>
          <div>{item.timestamp}</div>
          <div>{item.extra}</div>
          <div>{item.actions}</div>
        </article>
      ))}
    </div>
  ),
}));

vi.mock('../lib/whatsappPrefillShare', () => ({
  buildApprovalShareText: (...args: unknown[]) => shareMocks.buildApprovalShareText(...args),
  buildInAppNotificationShareText: (...args: unknown[]) => shareMocks.buildInAppNotificationShareText(...args),
  openWhatsAppPrefilledShare: (...args: unknown[]) => shareMocks.openWhatsAppPrefilledShare(...args),
}));

function buildNotification(overrides: Partial<InAppNotificationItem> = {}): InAppNotificationItem {
  return {
    id: 'notif-1',
    title: 'Insight nuevo',
    body: 'Conviene revisar las ventas del día.',
    kind: 'insight',
    entity_type: 'insight',
    entity_id: 'entity-1',
    chat_context: {
      scope: 'sales',
      routed_agent: 'commercial',
      suggested_user_message: 'Explicame este insight',
    },
    read_at: null,
    created_at: '2026-04-03T12:00:00Z',
    ...overrides,
  };
}

describe('NotificationsCenterPage', () => {
  beforeEach(async () => {
    vi.resetModules();
    apiMocks.listInAppNotifications.mockReset();
    apiMocks.markInAppNotificationRead.mockReset();
    apiMocks.createInsightNotifications.mockReset();
    reviewMocks.approveRequest.mockReset();
    reviewMocks.rejectRequest.mockReset();
    navigationMocks.navigate.mockReset();
    pageSearchMocks.usePageSearch.mockReset();
    shareMocks.openWhatsAppPrefilledShare.mockReset();
    shareMocks.buildApprovalShareText.mockClear();
    shareMocks.buildInAppNotificationShareText.mockClear();
    pageSearchMocks.usePageSearch.mockReturnValue('');
    apiMocks.markInAppNotificationRead.mockResolvedValue({ id: 'notif-1', read_at: '2026-04-03T12:05:00Z' });
    apiMocks.createInsightNotifications.mockResolvedValue({
      request_id: 'req-1',
      service_kind: 'insight_service',
      output_kind: 'insight_notification',
      content_language: 'es',
      items: [],
    });
    reviewMocks.approveRequest.mockResolvedValue(undefined);
    reviewMocks.rejectRequest.mockResolvedValue(undefined);
    sessionStorage.clear();
  });

  it('opens a regular notification in chat and stores the handoff payload', async () => {
    const { NotificationsCenterPage } = await import('./NotificationsCenterPage');
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    apiMocks.listInAppNotifications.mockResolvedValue({
      items: [buildNotification()],
      unread_count: 1,
    });

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
          <LanguageProvider initialLanguage="es">
            <NotificationsCenterPage embedded />
          </LanguageProvider>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('notifications-summary')).toHaveTextContent('1 sin leer');
    });
    fireEvent.click(screen.getByRole('button', { name: /explicar en chat/i }));

    await waitFor(() => {
      expect(apiMocks.markInAppNotificationRead).toHaveBeenCalled();
      expect(apiMocks.markInAppNotificationRead.mock.calls[0][0]).toBe('notif-1');
      expect(navigationMocks.navigate).toHaveBeenCalledWith('/chat');
    });
    expect(JSON.parse(sessionStorage.getItem(NOTIFICATION_CHAT_HANDOFF_KEY) ?? '{}')).toEqual(
      expect.objectContaining({
        notificationId: 'notif-1',
        title: 'Insight nuevo',
        body: 'Conviene revisar las ventas del día.',
        source: 'in_app_notification',
        notification_id: 'notif-1',
        routedAgent: 'commercial',
        chatContext: expect.objectContaining({
          suggested_user_message: 'Explicame este insight',
        }),
      }),
    );
  });

  it('submits approval decisions from the inbox feed', async () => {
    const { NotificationsCenterPage } = await import('./NotificationsCenterPage');
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    apiMocks.listInAppNotifications.mockResolvedValue({
      items: [
        buildNotification({
          id: 'notif-approval',
          title: 'Aprobación pendiente',
          kind: 'approval',
          chat_context: {
            source: 'review_approval',
            approval: {
              id: 'approval-1',
              request_id: 'request-1',
              action_type: 'delete_customer',
              target_resource: 'customer:123',
              reason: 'La operación es sensible',
              risk_level: 'high',
              status: 'pending',
              created_at: '2026-04-03T12:00:00Z',
            },
          },
        }),
      ],
      unread_count: 1,
    });

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
          <LanguageProvider initialLanguage="es">
            <NotificationsCenterPage embedded />
          </LanguageProvider>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('notifications-summary')).toHaveTextContent('1 sin leer');
      expect(screen.getByTestId('notifications-summary')).toHaveTextContent('1 decisión');
    });

    fireEvent.change(screen.getByLabelText(/nota para/i), { target: { value: 'Aprobado por soporte' } });
    fireEvent.click(screen.getByRole('button', { name: /aprobar/i }));

    await waitFor(() => {
      expect(reviewMocks.approveRequest).toHaveBeenCalledWith('approval-1', 'Aprobado por soporte');
    });
  });

  it('genera insights automáticamente cuando la bandeja está vacía', async () => {
    const { NotificationsCenterPage } = await import('./NotificationsCenterPage');
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    apiMocks.listInAppNotifications.mockResolvedValue({
      items: [],
      unread_count: 0,
    });

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
          <LanguageProvider initialLanguage="es">
            <NotificationsCenterPage embedded />
          </LanguageProvider>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(apiMocks.createInsightNotifications).toHaveBeenCalledWith({
        period: 'week',
        compare: true,
        top_limit: 5,
        preferred_language: 'es',
      });
    });
  });
});
