import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { ShellRoutes } from './ShellRoutes';
import { TenantAccessProvider } from '../lib/TenantAccessProvider';
import type { TenantAccess } from '../lib/tenantAccessContext';

vi.mock('./lazyRoutes', () => ({
  CalendarPage: () => <div>agenda</div>,
  ConfiguredCrudSectionPage: ({ children }: { children?: React.ReactNode }) => <div>crud-section{children}</div>,
  ConfiguredCrudIndexRedirect: () => <div>crud-index</div>,
  CrudUiConfigurePage: () => <div>crud-configure</div>,
  DashboardVisualPage: () => <div>dashboard</div>,
  ModulePage: () => <div>module-page</div>,
  NotificationsCenterPage: () => <div>notifications</div>,
  RestaurantTableSessionsPage: () => <div>restaurant-sessions</div>,
  SettingsHubPage: () => <div>settings</div>,
  ConfiguredCrudModePage: () => <div>crud-mode</div>,
  ConfiguredCrudRouteModePage: () => <div>crud-route-mode</div>,
  ConfiguredCrudNestedRouteModePage: () => <div>crud-nested-mode</div>,
  UnifiedChatPage: () => <div>chat</div>,
  CustomerMessagingCampaignsPage: () => <div>campaigns</div>,
  CustomerMessagingInboxPage: () => <div>inbox</div>,
  AutomationRulesPage: () => <div>automation</div>,
  WatcherConfigPage: () => <div>watchers</div>,
}));

function LocationProbe() {
  const location = useLocation();
  return <output aria-label="current-path">{`${location.pathname}${location.search}`}</output>;
}

function renderShellRoutes(path: string) {
  const access: TenantAccess = {
    tenantId: '00000000-0000-0000-0000-000000000001',
    tenantSlug: 'bicimax',
    tenantName: 'Bicimax',
    role: 'owner',
    session: {
      auth: {
        org_id: '00000000-0000-0000-0000-000000000001',
        tenant_name: 'Bicimax',
        tenant_slug: 'bicimax',
        role: 'owner',
        product_role: 'admin',
        scopes: [],
        actor: 'user-1',
        auth_method: 'jwt',
      },
      tenant: { id: '00000000-0000-0000-0000-000000000001', slug: 'bicimax', name: 'Bicimax' },
      membership: { role: 'owner' },
    },
    settings: {
      org_id: '00000000-0000-0000-0000-000000000001',
      plan_code: 'starter',
      hard_limits: {},
      billing_status: 'trialing',
      currency: 'ARS',
      supported_currencies: ['ARS'],
      tax_rate: 21,
      quote_prefix: 'PRE',
      sale_prefix: 'VTA',
      next_quote_number: 1,
      next_sale_number: 1,
      allow_negative_stock: true,
      purchase_prefix: 'CPA',
      next_purchase_number: 1,
      return_prefix: 'DEV',
      credit_note_prefix: 'NC',
      next_return_number: 1,
      next_credit_note_number: 1,
      business_name: 'Bicimax',
      business_tax_id: '',
      business_address: '',
      business_phone: '',
      business_email: '',
      team_size: 'small',
      sells: 'both',
      client_label: 'clientes',
      uses_billing: true,
      payment_method: 'mixed',
      vertical: 'medical',
      onboarding_completed_at: '2026-05-07T00:00:00.000Z',
      wa_quote_template: '',
      wa_receipt_template: '',
      wa_default_country_code: '54',
      scheduling_enabled: true,
      scheduling_label: 'Turno',
      scheduling_reminder_hours: 24,
      secondary_currency: '',
      default_rate_type: 'blue',
      auto_fetch_rates: false,
      show_dual_prices: false,
      bank_holder: '',
      bank_cbu: '',
      bank_alias: '',
      bank_name: '',
      show_qr_in_pdf: false,
      wa_payment_template: '',
      wa_payment_link_template: '',
      updated_at: '2026-05-07T00:00:00.000Z',
    },
  };
  return render(
    <MemoryRouter initialEntries={[path]} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <TenantAccessProvider value={access}>
        <LocationProbe />
        <ShellRoutes />
      </TenantAccessProvider>
    </MemoryRouter>,
  );
}

describe('ShellRoutes tenant isolation', () => {
  it('renderiza rutas tenant-scoped ya validadas por TenantAccessBoundary', async () => {
    renderShellRoutes('/bicimax/invoices/list?foo=bar');

    await waitFor(() => {
      expect(screen.getByLabelText('current-path')).toHaveTextContent('/bicimax/invoices/list?foo=bar');
    });
    expect(screen.getByText('crud-route-mode')).toBeInTheDocument();
  });

  it('redirige la raiz al dashboard del tenant activo', async () => {
    renderShellRoutes('/');

    await waitFor(() => {
      expect(screen.getByLabelText('current-path')).toHaveTextContent('/bicimax/dashboard');
    });
    expect(screen.getByText('dashboard')).toBeInTheDocument();
  });
});
