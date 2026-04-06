/* eslint-disable @typescript-eslint/ban-ts-comment */
// @ts-nocheck — vitest mocks use dynamic types that tsc cannot verify
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import type { NotificationPreference } from '../lib/types';
import { NotificationPreferencesPage } from './NotificationPreferencesPage';

const apiMocks = vi.hoisted(() => ({
  getNotificationPreferences: vi.fn(),
  updateNotificationPreference: vi.fn(),
}));

const pageSearchMocks = vi.hoisted(() => ({
  usePageSearch: vi.fn(),
}));

vi.mock('../lib/api', () => ({
  getNotificationPreferences: () => apiMocks.getNotificationPreferences(),
  updateNotificationPreference: (...args: unknown[]) => apiMocks.updateNotificationPreference(...args),
}));

vi.mock('../components/PageSearch', () => ({
  usePageSearch: () => pageSearchMocks.usePageSearch(),
}));

vi.mock('@devpablocristo/modules-search', () => ({
  useSearch: <T,>(items: T[], textFn: (item: T) => string, query: string) => {
    if (!query.trim()) return items;
    const normalized = query.trim().toLowerCase();
    return items.filter((item) => textFn(item).toLowerCase().includes(normalized));
  },
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

function buildPreference(overrides: Partial<NotificationPreference> = {}): NotificationPreference {
  return {
    user_id: 'user-1',
    notification_type: 'welcome',
    channel: 'email',
    enabled: true,
    ...overrides,
  };
}

function renderPage(options?: { language?: 'es' | 'en'; embedded?: boolean }) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <LanguageProvider initialLanguage={options?.language ?? 'es'}>
          <NotificationPreferencesPage embedded={options?.embedded ?? false} />
        </LanguageProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('NotificationPreferencesPage', () => {
  beforeEach(() => {
    apiMocks.getNotificationPreferences.mockReset();
    apiMocks.updateNotificationPreference.mockReset();
    pageSearchMocks.usePageSearch.mockReset();
    pageSearchMocks.usePageSearch.mockReturnValue('');
  });

  it('renders localized copy and toggles a preference with the canonical payload', async () => {
    apiMocks.getNotificationPreferences.mockResolvedValue({
      items: [buildPreference()],
    });
    apiMocks.updateNotificationPreference.mockResolvedValue(buildPreference({ enabled: false }));

    renderPage({ language: 'en' });

    expect(await screen.findByRole('heading', { level: 1, name: 'Notification preferences' })).toBeInTheDocument();
    expect(screen.getByText('Choose which notices you receive on each channel.')).toBeInTheDocument();
    expect(await screen.findByText('Welcome')).toBeInTheDocument();
    expect(screen.getByText('Email')).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Enable Welcome via Email'));

    await waitFor(() => {
      expect(apiMocks.updateNotificationPreference).toHaveBeenCalledWith({
        notification_type: 'welcome',
        channel: 'email',
        enabled: false,
      });
    });
  });

  it('shows the filtered empty state inside settings when no preference matches the search', async () => {
    apiMocks.getNotificationPreferences.mockResolvedValue({
      items: [buildPreference()],
    });
    pageSearchMocks.usePageSearch.mockReturnValue('sms');

    renderPage({ embedded: true, language: 'es' });

    await waitFor(() => {
      expect(screen.getByText('No hay preferencias que coincidan con la búsqueda.')).toBeInTheDocument();
    });
    expect(screen.queryByRole('heading', { level: 1, name: 'Preferencias de notificación' })).not.toBeInTheDocument();
  });
});
