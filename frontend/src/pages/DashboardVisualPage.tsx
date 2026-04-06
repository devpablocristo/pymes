/**
 * Dashboard visual — gráficos, stats, tablas. Conectado a la API real.
 * Dashboard fijo con datos reales desde /v1/dashboard-data/*
 */
import { useQuery } from '@tanstack/react-query';
import { SchedulingDaySummary, createSchedulingClient } from '@devpablocristo/modules-scheduling';
import '@devpablocristo/modules-scheduling/styles.css';
import { HttpError } from '@devpablocristo/core-authn/http/fetch';
import { useDashboardDataEndpoint } from '../dashboard/hooks/useDashboardDataEndpoint';
import { HOME_DASHBOARD_CONTEXT } from '../dashboard/types';
import {
  formatDashboardDateTime,
  formatDashboardMoney,
  formatDashboardShortDate,
  localeForLanguage,
} from '../dashboard/utils/format';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n } from '../lib/i18n';
import { IconAlert, IconArrowDown, IconArrowUp } from '@devpablocristo/modules-ui-data-display/icons';
import { apiRequest } from '../lib/api';
import type {
  SalesSummaryData,
  CashflowSummaryData,
  QuotesPipelineData,
  RecentSalesData,
  TopProductsData,
  TopServicesData,
  AuditActivityData,
  LowStockData,
} from '../dashboard/types';
import './DashboardVisualPage.css';

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- datos genéricos del dashboard API
type DashItem = Record<string, any>;

const schedulingClient = createSchedulingClient(apiRequest);

// ─── Datos del dashboard fijo ───

function useVisualDashboardData<T>(slug: string) {
  return useDashboardDataEndpoint<T>(`/v1/dashboard-data/${slug}`, HOME_DASHBOARD_CONTEXT);
}

function DashboardSectionError({ message }: { message: string }) {
  return (
    <div className="card">
      <p className="dash__empty-hint">{message}</p>
    </div>
  );
}

// ─── Stat Cards ───

function StatCards() {
  const { t, language } = useI18n();
  const sales = useVisualDashboardData<SalesSummaryData>('sales-summary');
  const cashflow = useVisualDashboardData<CashflowSummaryData>('cashflow-summary');
  const quotes = useVisualDashboardData<QuotesPipelineData>('quotes-pipeline');
  const loadError = sales.error || cashflow.error || quotes.error;

  if (loadError) {
    return <DashboardSectionError message={formatFetchErrorForUser(loadError, t('dashboard.errors.load'))} />;
  }

  const stats = [
    {
      label: t('dashboard.visual.sales'),
      value: sales.data ? formatDashboardMoney(sales.data.total_sales, language) : '—',
      sub: sales.data && typeof sales.data.count_sales === 'number'
        ? `${sales.data.count_sales} ${t('dashboard.visual.operations')}`
        : '',
      tone: 'blue' as const,
      loading: sales.isLoading,
    },
    {
      label: t('dashboard.visual.averageTicket'),
      value: sales.data ? formatDashboardMoney(sales.data.average_ticket, language) : '—',
      sub: sales.data?.period ?? '',
      tone: 'green' as const,
      loading: sales.isLoading,
    },
    {
      label: t('dashboard.visual.income'),
      value: cashflow.data ? formatDashboardMoney(cashflow.data.total_income, language) : '—',
      sub: cashflow.data?.period ?? '',
      tone: 'purple' as const,
      loading: cashflow.isLoading,
    },
    {
      label: t('dashboard.visual.expense'),
      value: cashflow.data ? formatDashboardMoney(cashflow.data.total_expense, language) : '—',
      sub: cashflow.data?.period ?? '',
      tone: 'red' as const,
      loading: cashflow.isLoading,
    },
    {
      label: t('dashboard.visual.pendingQuotes'),
      value: quotes.data && typeof quotes.data.pending_total === 'number' ? String(quotes.data.pending_total) : '—',
      sub: quotes.data && typeof quotes.data.accepted === 'number'
        ? `${quotes.data.accepted} ${t('dashboard.visual.accepted')}`
        : '',
      tone: 'amber' as const,
      loading: quotes.isLoading,
    },
  ];

  return (
    <div className="dash__stats">
      {stats.map((s) => (
        <div key={s.label} className="dash__stat-card">
          <div className={`dash__stat-icon dash__stat-icon--${s.tone}`}>{s.loading ? '…' : s.label.charAt(0)}</div>
          <div className="dash__stat-info">
            <div className="dash__stat-value">{s.loading ? <span className="spinner" /> : s.value}</div>
            <div className="dash__stat-label">{s.label}</div>
            {s.sub && <div className="dash__stat-trend dash__stat-trend--muted">{s.sub}</div>}
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── Cashflow Chart (barras CSS con datos reales) ───

function CashflowChart() {
  const { t, language } = useI18n();
  const { data, isLoading, error } = useVisualDashboardData<CashflowSummaryData>('cashflow-summary');
  if (isLoading)
    return (
      <div className="card">
        <div className="spinner" />
      </div>
    );
  if (error) {
    return <DashboardSectionError message={formatFetchErrorForUser(error, t('dashboard.errors.load'))} />;
  }
  if (!data) {
    return (
      <div className="card">
        <p className="dash__empty-hint">{t('dashboard.visual.noCashflow')}</p>
      </div>
    );
  }

  const max = Math.max(data.total_income, data.total_expense, 1);
  const balance = data.total_income - data.total_expense;

  return (
    <div className="card">
      <div className="dash__chart-header">
        <div>
          <div className="dash__chart-metric">{formatDashboardMoney(balance, language)}</div>
          <span className={`dash__stat-trend ${balance >= 0 ? 'dash__stat-trend--up' : 'dash__stat-trend--down'}`}>
            {balance >= 0 ? <IconArrowUp /> : <IconArrowDown />} {t('dashboard.visual.balance')} {data.period}
          </span>
        </div>
      </div>
      <div className="dash__bars dash__bars--dashboard">
        <div className="dash__bar-col">
          <div className="dash__bar dash__bar--success" style={{ height: `${(data.total_income / max) * 100}%` }} />
          <span className="dash__bar-label">{t('dashboard.visual.income')}</span>
        </div>
        <div className="dash__bar-col">
          <div className="dash__bar dash__bar--amber" style={{ height: `${(data.total_expense / max) * 100}%` }} />
          <span className="dash__bar-label">{t('dashboard.visual.expense')}</span>
        </div>
      </div>
    </div>
  );
}

// ─── Quotes Pipeline (donut CSS) ───

function QuotesPipeline() {
  const { t } = useI18n();
  const { data, isLoading, error } = useVisualDashboardData<QuotesPipelineData>('quotes-pipeline');
  if (isLoading)
    return (
      <div className="card">
        <div className="spinner" />
      </div>
    );
  if (error) {
    return <DashboardSectionError message={formatFetchErrorForUser(error, t('dashboard.errors.load'))} />;
  }
  if (!data) {
    return (
      <div className="card">
        <p className="dash__empty-hint">{t('dashboard.visual.noQuotes')}</p>
      </div>
    );
  }

  const segments = [
    { label: t('dashboard.visual.quoteDraft'), value: data.draft, color: 'var(--color-text-muted)' },
    { label: t('dashboard.visual.quoteSent'), value: data.sent, color: 'var(--color-primary)' },
    { label: t('dashboard.visual.quoteAccepted'), value: data.accepted, color: 'var(--color-success)' },
    { label: t('dashboard.visual.quoteRejected'), value: data.rejected, color: 'var(--color-danger)' },
  ];
  const total = segments.reduce((s, seg) => s + seg.value, 0) || 1;
  let accum = 0;
  const gradientParts = segments
    .filter((s) => s.value > 0)
    .map((seg) => {
      const start = (accum / total) * 360;
      accum += seg.value;
      const end = (accum / total) * 360;
      return `${seg.color} ${start}deg ${end}deg`;
    });

  return (
    <div className="card">
      <div className="dash__donut-wrap">
        <div
          className="dash__donut"
          style={{
            background: gradientParts.length ? `conic-gradient(${gradientParts.join(', ')})` : 'var(--color-border)',
          }}
        >
          <div className="dash__donut-center">{data.pending_total}</div>
        </div>
        <div className="dash__donut-legend">
          {segments.map((seg) => (
            <div key={seg.label} className="dash__donut-legend-item">
              <span className="dash__donut-legend-dot" style={{ background: seg.color }} />
              <span>
                {seg.label}: <strong>{seg.value}</strong>
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ─── Recent Sales ───

function RecentSales() {
  const { t, language } = useI18n();
  const { data, isLoading, error } = useVisualDashboardData<RecentSalesData>('recent-sales');
  if (isLoading)
    return (
      <div className="card">
        <div className="spinner" />
      </div>
    );
  if (error) {
    return <DashboardSectionError message={formatFetchErrorForUser(error, t('dashboard.errors.load'))} />;
  }
  const items = data?.items ?? [];

  return (
    <div className="card">
      {items.length === 0 ? (
        <p className="dash__empty-hint">{t('dashboard.visual.noRecentSales')}</p>
      ) : (
        <table className="dash__table">
          <thead>
            <tr>
              <th>{t('dashboard.visual.tableNumber')}</th>
              <th>{t('dashboard.visual.tableCustomer')}</th>
              <th>{t('dashboard.visual.tableTotal')}</th>
              <th>{t('dashboard.visual.tableDate')}</th>
            </tr>
          </thead>
          <tbody>
            {items.slice(0, 6).map((s: DashItem) => (
              <tr key={s.id ?? s.number}>
                <td className="dash__table-cell--strong">{s.number ?? s.id?.slice(0, 8)}</td>
                <td>{s.party_name ?? s.customer_name ?? '—'}</td>
                <td className="dash__table-cell--strong">{formatDashboardMoney(s.total, language)}</td>
                <td className="dash__table-cell--meta">{formatDashboardShortDate(s.created_at, language)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

// ─── Top Products ───

function TopProducts() {
  const { t } = useI18n();
  const { data, isLoading, error } = useVisualDashboardData<TopProductsData>('top-products');
  if (isLoading)
    return (
      <div className="card">
        <div className="spinner" />
      </div>
    );
  if (error) {
    return <DashboardSectionError message={formatFetchErrorForUser(error, t('dashboard.errors.load'))} />;
  }
  const items = data?.items ?? [];

  const maxQty = Math.max(...items.map((p: DashItem) => p.quantity ?? p.count ?? 1), 1);

  return (
    <div className="card">
      {items.length === 0 ? (
        <p className="dash__empty-hint">{t('dashboard.visual.noTopProducts')}</p>
      ) : (
        items.slice(0, 5).map((p: DashItem, i: number) => {
          const qty = p.quantity ?? p.count ?? 0;
          const pct = (qty / maxQty) * 100;
          return (
            <div key={p.id ?? p.name ?? i} className="dash__product-row">
              <div className="dash__product-row-head">
                <span className="dash__product-name">{p.name ?? p.display_name}</span>
                <span className="dash__product-meta">{qty} {t('dashboard.visual.salesCount')}</span>
              </div>
              <div className="dash__progress-wrap">
                <div className="dash__progress">
                  <div className="dash__progress-fill dash__progress-fill--primary" style={{ width: `${pct}%` }} />
                </div>
                <span className="dash__progress-pct">{Math.round(pct)}%</span>
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}

function TopServices() {
  const { t } = useI18n();
  const { data, isLoading, error } = useVisualDashboardData<TopServicesData>('top-services');
  if (isLoading)
    return (
      <div className="card">
        <div className="spinner" />
      </div>
    );
  if (error) {
    return <DashboardSectionError message={formatFetchErrorForUser(error, t('dashboard.errors.load'))} />;
  }
  const items = data?.items ?? [];

  const maxQty = Math.max(...items.map((s: DashItem) => s.quantity ?? s.count ?? 1), 1);

  return (
    <div className="card">
      {items.length === 0 ? (
        <p className="dash__empty-hint">{t('dashboard.visual.noTopServices')}</p>
      ) : (
        items.slice(0, 5).map((s: DashItem, i: number) => {
          const qty = s.quantity ?? s.count ?? 0;
          const pct = (qty / maxQty) * 100;
          return (
            <div key={s.service_id ?? s.id ?? s.name ?? i} className="dash__product-row">
              <div className="dash__product-row-head">
                <span className="dash__product-name">{s.name ?? s.display_name}</span>
                <span className="dash__product-meta">{qty} {t('dashboard.visual.salesCount')}</span>
              </div>
              <div className="dash__progress-wrap">
                <div className="dash__progress">
                  <div className="dash__progress-fill dash__progress-fill--primary" style={{ width: `${pct}%` }} />
                </div>
                <span className="dash__progress-pct">{Math.round(pct)}%</span>
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}

// ─── Audit Activity ───

function AuditActivity() {
  const { t, language } = useI18n();
  const { data, isLoading, error } = useVisualDashboardData<AuditActivityData>('audit-activity');
  if (isLoading)
    return (
      <div className="card">
        <div className="spinner" />
      </div>
    );
  if (error) {
    return <DashboardSectionError message={formatFetchErrorForUser(error, t('dashboard.errors.load'))} />;
  }
  const items = data?.items ?? [];

  return (
    <div className="card">
      {items.length === 0 ? (
        <p className="dash__empty-hint">{t('dashboard.visual.noRecentActivity')}</p>
      ) : (
        <table className="dash__table">
          <thead>
            <tr>
              <th>{t('dashboard.visual.tableAction')}</th>
              <th>{t('dashboard.visual.tableResource')}</th>
              <th>{t('dashboard.visual.tableActor')}</th>
              <th>{t('dashboard.visual.tableDate')}</th>
            </tr>
          </thead>
          <tbody>
            {items.slice(0, 8).map((a: DashItem, i: number) => (
              <tr key={a.id ?? i}>
                <td className="dash__table-cell--action">{a.action}</td>
                <td>
                  {a.resource_type} {a.resource_id?.slice(0, 8)}
                </td>
                <td>{a.actor ?? '—'}</td>
                <td className="dash__table-cell--meta">{formatDashboardDateTime(a.created_at, language)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

// ─── Stock bajo ───

function LowStockAlerts() {
  const { t } = useI18n();
  const { data, isLoading, error } = useVisualDashboardData<LowStockData>('low-stock');
  if (error) {
    return <DashboardSectionError message={formatFetchErrorForUser(error, t('dashboard.errors.load'))} />;
  }
  const items = data?.items ?? [];

  return (
    <div className="card">
      <div className="dash__chart-header">
        {!isLoading && items.length > 0 && (
          <span className="dash__chart-header-alert">
            <IconAlert /> {items.length}
          </span>
        )}
      </div>
      {isLoading ? (
        <div className="spinner" />
      ) : items.length === 0 ? (
        <p className="dash__empty-hint">{t('dashboard.visual.stockHealthy')}</p>
      ) : (
        items.slice(0, 6).map((p) => (
          <div key={p.product_id} className="dash__performer">
            <div className="dash__performer-info">
              <div className="dash__performer-name">{p.product_name}</div>
              <div className="dash__performer-role">{p.sku ? `SKU: ${p.sku}` : ''}</div>
            </div>
            <div className="dash__performer-money-wrap">
              <div
                className={`dash__performer-money ${p.quantity <= 0 ? 'dash__performer-money--danger' : 'dash__performer-money--warning'}`}
              >
                {p.quantity}
              </div>
              <div className="dash__performer-money-label">
                {t('dashboard.visual.minLabel')}: {p.min_quantity}
              </div>
            </div>
          </div>
        ))
      )}
    </div>
  );
}

// ─── Deudores ───

type DebtorItem = {
  party_id: string;
  party_name: string;
  total_debt: number;
  oldest_date?: string;
};

function isServerHttpError(error: unknown): error is HttpError {
  return error instanceof HttpError && (error.status ?? 0) >= 500;
}

function Debtors() {
  const { t, language } = useI18n();
  const { data, isLoading, error } = useQuery({
    queryKey: ['dashboard-debtors', HOME_DASHBOARD_CONTEXT],
    queryFn: async () => {
      try {
        return await apiRequest<{ items?: DebtorItem[] }>('/v1/accounts/debtors');
      } catch (loadError) {
        if (isServerHttpError(loadError)) {
          return { items: [] };
        }
        throw loadError;
      }
    },
    staleTime: 30_000,
    retry: 1,
  });
  if (error) {
    return <DashboardSectionError message={formatFetchErrorForUser(error, t('dashboard.errors.load'))} />;
  }
  const items = data?.items ?? [];

  return (
    <div className="card">
      <div className="dash__chart-header">
        {!isLoading && items.length > 0 && (
          <span className="dash__chart-header-danger">
            {formatDashboardMoney(items.reduce((s, d) => s + d.total_debt, 0), language)}
          </span>
        )}
      </div>
      {isLoading ? (
        <div className="spinner" />
      ) : items.length === 0 ? (
        <p className="dash__empty-hint">{t('dashboard.visual.noDebts')}</p>
      ) : (
        items.slice(0, 6).map((d) => (
          <div key={d.party_id} className="dash__performer">
            <div className="dash__performer-info">
              <div className="dash__performer-name">{d.party_name}</div>
              {d.oldest_date && (
                <div className="dash__performer-role">
                  {t('dashboard.visual.since')}{' '}
                  {formatDashboardShortDate(d.oldest_date, language)}
                </div>
              )}
            </div>
            <div className="dash__performer-money dash__performer-money--danger">
              {formatDashboardMoney(d.total_debt, language)}
            </div>
          </div>
        ))
      )}
    </div>
  );
}

// ─── Página ───

export function DashboardVisualPage() {
  const { t, language } = useI18n();
  // Registra el search para que aparezca el input. Filtrado de gráficos pendiente.
  usePageSearch();
  return (
    <PageLayout
      className="dash"
      title={t('shell.dashboard.panelTitle')}
      lead={t('shell.dashboard.panelLead')}
    >
      <StatCards />

      <div className="dash__grid--3">
        <CashflowChart />
        <QuotesPipeline />
        <SchedulingDaySummary client={schedulingClient} locale={localeForLanguage(language)} />
      </div>

      <div className="dash__grid--3">
        <RecentSales />
        <Debtors />
        <LowStockAlerts />
      </div>

      <div className="dash__grid--3">
        <TopProducts />
        <TopServices />
        <AuditActivity />
      </div>
    </PageLayout>
  );
}

export default DashboardVisualPage;
