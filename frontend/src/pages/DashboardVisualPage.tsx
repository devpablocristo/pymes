/**
 * Dashboard visual — gráficos, stats, tablas. Conectado a la API real.
 * Usa los mismos endpoints que el sistema de widgets: /v1/dashboard-data/*
 */
import { useQuery } from '@tanstack/react-query';
import {
  IconAlert,
  IconArrowDown,
  IconArrowUp,
  IconCalendar,
} from '@devpablocristo/modules-ui-data-display/icons';
import { apiRequest } from '../lib/api';
import type {
  SalesSummaryData,
  CashflowSummaryData,
  QuotesPipelineData,
  RecentSalesData,
  TopProductsData,
  AuditActivityData,
  LowStockData,
} from '../dashboard/types';
import './DashboardVisualPage.css';

// ─── Hooks de datos ───

function useWidgetData<T>(key: string) {
  return useQuery({
    queryKey: ['dashboard-visual', key],
    queryFn: () => apiRequest<T>(`/v1/dashboard-data/${key}?context=home`),
    staleTime: 30_000,
    retry: 1,
  });
}

function fmtMoney(n: number): string {
  if (n >= 1_000_000) return `$${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `$${(n / 1_000).toFixed(0)}K`;
  return `$${n.toLocaleString('es-AR')}`;
}

// ─── Stat Cards ───

function StatCards() {
  const sales = useWidgetData<SalesSummaryData>('sales-summary');
  const cashflow = useWidgetData<CashflowSummaryData>('cashflow-summary');
  const quotes = useWidgetData<QuotesPipelineData>('quotes-pipeline');

  const stats = [
    {
      label: 'Ventas',
      value: sales.data ? fmtMoney(sales.data.total_sales) : '—',
      sub: sales.data ? `${sales.data.count_sales} operaciones` : '',
      tone: 'blue' as const,
      loading: sales.isLoading,
    },
    {
      label: 'Ticket promedio',
      value: sales.data ? fmtMoney(sales.data.average_ticket) : '—',
      sub: sales.data?.period ?? '',
      tone: 'green' as const,
      loading: sales.isLoading,
    },
    {
      label: 'Ingresos',
      value: cashflow.data ? fmtMoney(cashflow.data.total_income) : '—',
      sub: cashflow.data?.period ?? '',
      tone: 'purple' as const,
      loading: cashflow.isLoading,
    },
    {
      label: 'Egresos',
      value: cashflow.data ? fmtMoney(cashflow.data.total_expense) : '—',
      sub: cashflow.data?.period ?? '',
      tone: 'red' as const,
      loading: cashflow.isLoading,
    },
    {
      label: 'Presupuestos pendientes',
      value: quotes.data ? String(quotes.data.pending_total) : '—',
      sub: quotes.data ? `${quotes.data.accepted} aceptados` : '',
      tone: 'amber' as const,
      loading: quotes.isLoading,
    },
  ];

  return (
    <div className="dash__stats">
      {stats.map((s) => (
        <div key={s.label} className="dash__stat-card">
          <div className={`dash__stat-icon dash__stat-icon--${s.tone}`}>
            {s.loading ? '…' : s.value.charAt(0)}
          </div>
          <div className="dash__stat-info">
            <div className="dash__stat-value">{s.loading ? <span className="spinner" style={{ width: 16, height: 16 }} /> : s.value}</div>
            <div className="dash__stat-label">{s.label}</div>
            {s.sub && <div className="dash__stat-trend dash__stat-trend--up" style={{ color: 'var(--color-text-muted)' }}>{s.sub}</div>}
          </div>
        </div>
      ))}
    </div>
  );
}

// ─── Cashflow Chart (barras CSS con datos reales) ───

function CashflowChart() {
  const { data, isLoading } = useWidgetData<CashflowSummaryData>('cashflow-summary');
  if (isLoading) return <div className="card"><div className="spinner" /></div>;
  if (!data) return <div className="card"><p style={{ color: 'var(--color-text-muted)' }}>Sin datos de cashflow</p></div>;

  const max = Math.max(data.total_income, data.total_expense, 1);
  const balance = data.total_income - data.total_expense;

  return (
    <div className="card">
      <div className="dash__chart-header">
        <div>          <div className="dash__chart-metric">{fmtMoney(balance)}</div>
          <span className={`dash__stat-trend ${balance >= 0 ? 'dash__stat-trend--up' : 'dash__stat-trend--down'}`}>
            {balance >= 0 ? <IconArrowUp /> : <IconArrowDown />} Balance {data.period}
          </span>
        </div>
      </div>
      <div className="dash__bars" style={{ height: 120 }}>
        <div className="dash__bar-col">
          <div className="dash__bar dash__bar--success" style={{ height: `${(data.total_income / max) * 100}%` }} />
          <span className="dash__bar-label">Ingresos</span>
        </div>
        <div className="dash__bar-col">
          <div className="dash__bar dash__bar--amber" style={{ height: `${(data.total_expense / max) * 100}%` }} />
          <span className="dash__bar-label">Egresos</span>
        </div>
      </div>
    </div>
  );
}

// ─── Quotes Pipeline (donut CSS) ───

function QuotesPipeline() {
  const { data, isLoading } = useWidgetData<QuotesPipelineData>('quotes-pipeline');
  if (isLoading) return <div className="card"><div className="spinner" /></div>;
  if (!data) return <div className="card"><p style={{ color: 'var(--color-text-muted)' }}>Sin datos de presupuestos</p></div>;

  const segments = [
    { label: 'Borrador', value: data.draft, color: 'var(--color-text-muted)' },
    { label: 'Enviados', value: data.sent, color: 'var(--color-primary)' },
    { label: 'Aceptados', value: data.accepted, color: 'var(--color-success)' },
    { label: 'Rechazados', value: data.rejected, color: 'var(--color-danger)' },
  ];
  const total = segments.reduce((s, seg) => s + seg.value, 0) || 1;
  let accum = 0;
  const gradientParts = segments.filter(s => s.value > 0).map((seg) => {
    const start = (accum / total) * 360;
    accum += seg.value;
    const end = (accum / total) * 360;
    return `${seg.color} ${start}deg ${end}deg`;
  });

  return (
    <div className="card">
      <div className="dash__chart-header">      </div>
      <div className="dash__donut-wrap">
        <div className="dash__donut" style={{ background: gradientParts.length ? `conic-gradient(${gradientParts.join(', ')})` : 'var(--color-border)' }}>
          <div className="dash__donut-center">{data.pending_total}</div>
        </div>
        <div className="dash__donut-legend">
          {segments.map((seg) => (
            <div key={seg.label} className="dash__donut-legend-item">
              <span className="dash__donut-legend-dot" style={{ background: seg.color }} />
              <span>{seg.label}: <strong>{seg.value}</strong></span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ─── Recent Sales ───

function RecentSales() {
  const { data, isLoading } = useWidgetData<RecentSalesData>('sales-recent');
  if (isLoading) return <div className="card"><div className="spinner" /></div>;
  const items = data?.items ?? [];

  return (
    <div className="card">
      <div className="dash__chart-header">      </div>
      {items.length === 0 ? (
        <p style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-sm)' }}>Sin ventas recientes</p>
      ) : (
        <table className="dash__table">
          <thead><tr><th>N°</th><th>Cliente</th><th>Total</th><th>Fecha</th></tr></thead>
          <tbody>
            {items.slice(0, 6).map((s: any) => (
              <tr key={s.id ?? s.number}>
                <td style={{ fontWeight: 600 }}>{s.number ?? s.id?.slice(0, 8)}</td>
                <td>{s.party_name ?? s.customer_name ?? '—'}</td>
                <td style={{ fontWeight: 600 }}>{fmtMoney(s.total ?? 0)}</td>
                <td style={{ fontSize: 'var(--text-xs)', color: 'var(--color-text-muted)' }}>
                  {s.created_at ? new Date(s.created_at).toLocaleDateString('es-AR', { day: '2-digit', month: 'short' }) : '—'}
                </td>
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
  const { data, isLoading } = useWidgetData<TopProductsData>('products-top');
  if (isLoading) return <div className="card"><div className="spinner" /></div>;
  const items = data?.items ?? [];

  const maxQty = Math.max(...items.map((p: any) => p.quantity ?? p.count ?? 1), 1);

  return (
    <div className="card">
      <div className="dash__chart-header">      </div>
      {items.length === 0 ? (
        <p style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-sm)' }}>Sin datos de productos</p>
      ) : (
        items.slice(0, 5).map((p: any, i: number) => {
          const qty = p.quantity ?? p.count ?? 0;
          const pct = (qty / maxQty) * 100;
          return (
            <div key={p.id ?? p.name ?? i} style={{ marginBottom: 'var(--space-3)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.25rem', fontSize: 'var(--text-sm)' }}>
                <span style={{ fontWeight: 600 }}>{p.name ?? p.display_name}</span>
                <span style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-xs)' }}>{qty} ventas</span>
              </div>
              <div className="dash__progress-wrap">
                <div className="dash__progress">
                  <div className="dash__progress-fill" style={{ width: `${pct}%`, background: 'var(--color-primary)' }} />
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
  const { data, isLoading } = useWidgetData<AuditActivityData>('audit-activity');
  if (isLoading) return <div className="card"><div className="spinner" /></div>;
  const items = data?.items ?? [];

  return (
    <div className="card">
      <div className="dash__chart-header">      </div>
      {items.length === 0 ? (
        <p style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-sm)' }}>Sin actividad reciente</p>
      ) : (
        <table className="dash__table">
          <thead><tr><th>Acción</th><th>Recurso</th><th>Actor</th><th>Fecha</th></tr></thead>
          <tbody>
            {items.slice(0, 8).map((a: any, i: number) => (
              <tr key={a.id ?? i}>
                <td style={{ fontWeight: 600, fontSize: 'var(--text-xs)' }}>{a.action}</td>
                <td>{a.resource_type} {a.resource_id?.slice(0, 8)}</td>
                <td>{a.actor ?? '—'}</td>
                <td style={{ fontSize: 'var(--text-xs)', color: 'var(--color-text-muted)' }}>
                  {a.created_at ? new Date(a.created_at).toLocaleDateString('es-AR', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' }) : '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

// ─── Turnos de hoy ───

type AppointmentItem = {
  id: string;
  title: string;
  customer_name: string;
  start_at: string;
  end_at: string;
  status: string;
  assigned_to?: string;
  color?: string;
};

function TodayAppointments() {
  const today = new Date().toISOString().slice(0, 10);
  const { data, isLoading } = useQuery({
    queryKey: ['dashboard-appointments-today'],
    queryFn: () => apiRequest<{ items?: AppointmentItem[] }>(`/v1/appointments?from=${today}&to=${today}&limit=10`),
    staleTime: 30_000,
    retry: 1,
  });

  const items = data?.items ?? [];
  const now = new Date();
  const upcoming = items
    .filter((a) => new Date(a.start_at) >= now)
    .sort((a, b) => new Date(a.start_at).getTime() - new Date(b.start_at).getTime());
  const next = upcoming[0];

  return (
    <div className="card">
      <div className="dash__chart-header">        <span style={{ fontSize: 'var(--text-xl)', fontWeight: 700, color: 'var(--color-primary)' }}>
          {isLoading ? '…' : items.length}
        </span>
      </div>
      {isLoading ? (
        <div className="spinner" />
      ) : items.length === 0 ? (
        <p style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-sm)' }}>Sin turnos hoy</p>
      ) : (
        <>
          {next && (
            <div style={{ padding: 'var(--space-3)', background: 'var(--color-primary-subtle)', borderRadius: 'var(--radius-sm)', marginBottom: 'var(--space-3)', display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
              <IconCalendar />
              <div>
                <div style={{ fontWeight: 600, fontSize: 'var(--text-sm)' }}>Próximo: {next.title || next.customer_name}</div>
                <div style={{ fontSize: 'var(--text-xs)', color: 'var(--color-text-secondary)' }}>
                  {new Date(next.start_at).toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' })}
                  {next.assigned_to ? ` · ${next.assigned_to}` : ''}
                </div>
              </div>
            </div>
          )}
          {items.slice(0, 5).map((a) => (
            <div key={a.id} className="dash__performer">
              <div style={{ width: 4, height: 28, borderRadius: 2, background: a.color || 'var(--color-primary)', flexShrink: 0 }} />
              <div className="dash__performer-info">
                <div className="dash__performer-name">{a.title || a.customer_name}</div>
                <div className="dash__performer-role">
                  {new Date(a.start_at).toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' })}
                  {' — '}
                  {new Date(a.end_at).toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' })}
                </div>
              </div>
              <span className={`badge ${a.status === 'confirmed' ? 'badge-success' : a.status === 'cancelled' ? 'badge-danger' : 'badge-neutral'}`} style={{ fontSize: 'var(--text-xs)' }}>
                {a.status}
              </span>
            </div>
          ))}
        </>
      )}
    </div>
  );
}

// ─── Stock bajo ───

function LowStockAlerts() {
  const { data, isLoading } = useWidgetData<LowStockData>('inventory-low-stock');
  const items = data?.items ?? [];

  return (
    <div className="card">
      <div className="dash__chart-header">        {!isLoading && items.length > 0 && (
          <span style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-1)', color: 'var(--color-warning)', fontSize: 'var(--text-sm)', fontWeight: 600 }}>
            <IconAlert /> {items.length}
          </span>
        )}
      </div>
      {isLoading ? (
        <div className="spinner" />
      ) : items.length === 0 ? (
        <p style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-sm)' }}>Todo el stock está en orden</p>
      ) : (
        items.slice(0, 6).map((p) => (
          <div key={p.product_id} className="dash__performer">
            <div className="dash__performer-info">
              <div className="dash__performer-name">{p.product_name}</div>
              <div className="dash__performer-role">{p.sku ? `SKU: ${p.sku}` : ''}</div>
            </div>
            <div style={{ textAlign: 'right' }}>
              <div style={{ fontWeight: 700, color: p.quantity <= 0 ? 'var(--color-danger)' : 'var(--color-warning)', fontSize: 'var(--text-sm)' }}>
                {p.quantity}
              </div>
              <div style={{ fontSize: 'var(--text-xs)', color: 'var(--color-text-muted)' }}>mín: {p.min_quantity}</div>
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

function Debtors() {
  const { data, isLoading } = useQuery({
    queryKey: ['dashboard-debtors'],
    queryFn: () => apiRequest<{ items?: DebtorItem[] }>('/v1/accounts/debtors'),
    staleTime: 30_000,
    retry: 1,
  });
  const items = data?.items ?? [];

  return (
    <div className="card">
      <div className="dash__chart-header">        {!isLoading && items.length > 0 && (
          <span style={{ fontWeight: 700, color: 'var(--color-danger)', fontSize: 'var(--text-sm)' }}>
            {fmtMoney(items.reduce((s, d) => s + d.total_debt, 0))}
          </span>
        )}
      </div>
      {isLoading ? (
        <div className="spinner" />
      ) : items.length === 0 ? (
        <p style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-sm)' }}>Sin deudas pendientes</p>
      ) : (
        items.slice(0, 6).map((d) => (
          <div key={d.party_id} className="dash__performer">
            <div className="dash__performer-info">
              <div className="dash__performer-name">{d.party_name}</div>
              {d.oldest_date && (
                <div className="dash__performer-role">
                  Desde {new Date(d.oldest_date).toLocaleDateString('es-AR', { day: '2-digit', month: 'short' })}
                </div>
              )}
            </div>
            <div style={{ fontWeight: 700, color: 'var(--color-danger)', fontSize: 'var(--text-sm)' }}>
              {fmtMoney(d.total_debt)}
            </div>
          </div>
        ))
      )}
    </div>
  );
}

// ─── Página ───

export function DashboardVisualPage() {
  return (
    <div className="dash">
      <StatCards />

      <div className="dash__grid--3">
        <CashflowChart />
        <QuotesPipeline />
        <TodayAppointments />
      </div>

      <div className="dash__grid">
        <RecentSales />
        <Debtors />
      </div>

      <div className="dash__grid--3">
        <TopProducts />
        <LowStockAlerts />
        <AuditActivity />
      </div>
    </div>
  );
}

export default DashboardVisualPage;
