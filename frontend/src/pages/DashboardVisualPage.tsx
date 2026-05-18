/**
 * Dashboard visual — Spike template. Conectado a la API real.
 * ApexCharts vía CDN global (window.ApexCharts).
 */
import { useEffect, useRef, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  createSchedulingClient,
  type DashboardStats,
  type DayAgendaItem,
  type Service,
  type Resource,
} from '@devpablocristo/modules-scheduling';
import { HttpError } from '@devpablocristo/core-authn/http/fetch';
import { useDashboardDataEndpoint } from '../dashboard/hooks/useDashboardDataEndpoint';
import { HOME_DASHBOARD_CONTEXT } from '../dashboard/types';
import {
  formatDashboardMoney,
  formatDashboardShortDate,
  localeForLanguage,
} from '../dashboard/utils/format';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { useI18n } from '../lib/i18n';
import { apiRequest } from '../lib/api';
import { useOptionalBranchSelection } from '../lib/useBranchSelection';
import { readActiveBranchId } from '../lib/branchSelectionStorage';
import type {
  SalesSummaryData,
  CashflowSummaryData,
  QuotesPipelineData,
  RecentSalesData,
  TopServicesData,
} from '../dashboard/types';
import './DashboardVisualPage.css';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
declare const ApexCharts: any;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type DashItem = Record<string, any>;

const schedulingClient = createSchedulingClient(apiRequest);

// ─── Theme-aware label colors para ApexCharts ───
// ApexCharts pinta texto SVG con `fill` desde la opción `colors` del eje;
// en dark mode los hardcodes #8898aa / #5a6a85 / #2a3547 quedan ilegibles.
// Esta helper devuelve los tokens del tema activo en el momento del render.
// Light mode preserva exactamente los hardcodes previos y omite chart.foreColor
// para no alterar el rendering existente.
function getChartLabelColors(): {
  axisMuted: string;        // antes #8898aa hardcoded (light)
  axisSecondary: string;    // antes #5a6a85 hardcoded (light)
  textStrong: string;       // antes #2a3547 hardcoded (light)
  foreColor: string | undefined; // chart.foreColor: undefined en light, claro en dark
} {
  const isDark = typeof document !== 'undefined'
    && document.documentElement.getAttribute('data-theme') === 'dark';
  return isDark
    ? { axisMuted: '#94a3b8', axisSecondary: '#cbd5e1', textStrong: '#e2e8f0', foreColor: '#94a3b8' }
    : { axisMuted: '#8898aa', axisSecondary: '#5a6a85', textStrong: '#2a3547', foreColor: undefined };
}

// ─── ApexCharts hook ───

function useApexChart(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  buildOptions: () => any,
  deps: React.DependencyList,
) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!ref.current || typeof ApexCharts === 'undefined') return;
    const chart = new ApexCharts(ref.current, buildOptions());
    void chart.render();
    return () => { chart.destroy(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
  return ref;
}

// Etiquetas de los últimos N meses para el eje X del tooltip
const MONTH_SHORT = ['Ene','Feb','Mar','Abr','May','Jun','Jul','Ago','Sep','Oct','Nov','Dic'];

function lastNMonthLabels(n: number): string[] {
  const now = new Date();
  const labels: string[] = [];
  for (let i = n - 1; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1);
    labels.push(MONTH_SHORT[d.getMonth()]);
  }
  return labels;
}

function SparklineChart({ color, data, seriesName }: { color: string; data: number[]; seriesName?: string }) {
  const categories = useMemo(() => lastNMonthLabels(data.length), [data.length]);
  const ref = useApexChart(
    () => ({
      chart: {
        type: 'area',
        height: 60,
        sparkline: { enabled: true },
        animations: { enabled: false },
        toolbar: { show: false },
      },
      series: [{ name: seriesName ?? 'Valor', data }],
      xaxis: { categories },
      stroke: { curve: 'smooth', width: 2 },
      fill: { type: 'gradient', gradient: { shadeIntensity: 1, opacityFrom: 0.3, opacityTo: 0, stops: [0, 90, 100] } },
      colors: [color],
      markers: {
        size: 0,
        hover: { size: 5, sizeOffset: 0 },
        strokeWidth: 2,
        strokeColors: [color],
        fillColor: '#fff',
      },
      tooltip: {
        enabled: true,
        theme: 'dark',
        fixed: { enabled: false },
        x: { show: true },
        y: {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          title: { formatter: () => seriesName ?? '' },
        },
        marker: { show: true },
      },
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [color, JSON.stringify(data), seriesName, JSON.stringify(categories)],
  );
  return <div ref={ref} className="sparkline-chart-wrap" style={{ marginTop: 8 }} />;
}

// ─── KPI stat cards ───

const KPI_SPARKLINES: Record<string, number[]> = {
  sales:   [8, 12, 7, 15, 18, 22, 20, 28, 25, 32, 28, 35],
  ticket:  [12, 10, 13, 14, 11, 15, 13, 16, 14, 15, 13, 14],
  income:  [20, 35, 28, 40, 38, 45, 42, 50, 48, 55, 52, 69],
  expense: [90, 110, 95, 130, 120, 140, 135, 150, 145, 160, 155, 150],
};

type KpiTone = 'blue' | 'teal' | 'yellow' | 'red';

const KPI_TONE: Record<KpiTone, { iconBg: string; iconColor: string; sparkColor: string; trendBg: string }> = {
  blue:   { iconBg: '#0085db', iconColor: '#fff', sparkColor: '#0085db', trendBg: '#e5f3fb' },
  teal:   { iconBg: '#4bd08b', iconColor: '#fff', sparkColor: '#4bd08b', trendBg: '#dffff3' },
  yellow: { iconBg: '#f8c076', iconColor: '#fff', sparkColor: '#f8c076', trendBg: '#fff6ea' },
  red:    { iconBg: '#fb977d', iconColor: '#fff', sparkColor: '#fb977d', trendBg: '#ffede9' },
};

interface KpiCardProps {
  label: string;
  value: string;
  sub: string;
  trend?: string;
  trendUp?: boolean;
  tone: KpiTone;
  icon: string;
  sparkKey: keyof typeof KPI_SPARKLINES;
  loading: boolean;
}

function KpiCard({ label, value, sub, trend, trendUp, tone, icon, sparkKey, loading }: KpiCardProps) {
  const t = KPI_TONE[tone];
  return (
    <div className="card spike-kpi-card">
      <div className="spike-kpi-body">
        <div className="spike-kpi-left">
          <div className="spike-kpi-label">{label}</div>
          <div className="spike-kpi-value">{loading ? <span className="spinner" /> : value}</div>
          <div className="spike-kpi-sub">{sub}</div>
          {trend && (
            <div className="spike-kpi-trend">
              <span className="spike-kpi-trend-icon" style={{ background: t.trendBg, color: trendUp ? '#4bd08b' : '#fb977d' }}>
                <i className={`ti ti-arrow-${trendUp ? 'up' : 'down'}`} />
              </span>
              <span style={{ color: trendUp ? '#4bd08b' : '#fb977d', fontWeight: 600, fontSize: 12 }}>{trend}</span>
              <span style={{ color: '#adb5bd', fontSize: 11 }}>vs mes anterior</span>
            </div>
          )}
        </div>
        <div className="spike-kpi-icon" style={{ background: t.iconBg, color: t.iconColor }}>
          <i className={`ti ti-${icon}`} />
        </div>
      </div>
      <SparklineChart color={t.sparkColor} data={KPI_SPARKLINES[sparkKey]} seriesName={label} />
    </div>
  );
}

function StatCards() {
  const { t, language } = useI18n();
  const sales    = useDashboardDataEndpoint<SalesSummaryData>('/v1/dashboard-data/sales-summary', HOME_DASHBOARD_CONTEXT);
  const cashflow = useDashboardDataEndpoint<CashflowSummaryData>('/v1/dashboard-data/cashflow-summary', HOME_DASHBOARD_CONTEXT);

  return (
    <div className="spike-kpi-grid">
      <KpiCard
        label="Ventas del mes"
        value={sales.data ? formatDashboardMoney(sales.data.total_sales, language) : '—'}
        sub={sales.data && typeof sales.data.count_sales === 'number' ? `${sales.data.count_sales} operaciones` : ''}
        trend="+23%" trendUp
        tone="blue" icon="currency-dollar" sparkKey="sales"
        loading={sales.isLoading}
      />
      <KpiCard
        label="Ticket promedio"
        value={sales.data ? formatDashboardMoney(sales.data.average_ticket, language) : '—'}
        sub="Por operación"
        trend="+8%" trendUp
        tone="teal" icon="receipt" sparkKey="ticket"
        loading={sales.isLoading}
      />
      <KpiCard
        label="Ingresos"
        value={cashflow.data ? formatDashboardMoney(cashflow.data.total_income, language) : '—'}
        sub="Cobros del período"
        trend="+15%" trendUp
        tone="yellow" icon="trending-up" sparkKey="income"
        loading={cashflow.isLoading}
      />
      <KpiCard
        label="Egresos"
        value={cashflow.data ? formatDashboardMoney(cashflow.data.total_expense, language) : '—'}
        sub="Gastos del período"
        trend="+12%" trendUp={false}
        tone="red" icon="trending-down" sparkKey="expense"
        loading={cashflow.isLoading}
      />
    </div>
  );
}

// ─── Balance mensual ───

function BalanceChart() {
  const { t, language } = useI18n();
  const { data, isLoading } = useDashboardDataEndpoint<CashflowSummaryData>('/v1/dashboard-data/cashflow-summary', HOME_DASHBOARD_CONTEXT);

  const income  = data?.total_income  ?? 0;
  const expense = data?.total_expense ?? 0;
  const toK     = (v: number) => Math.round(v / 1000);

  // Usa valores demo visuales para la tendencia de 6 meses (los valores reales están en los KPI cards)
  // Se calcula la escala relativa entre income y expense del período actual
  const incomeK  = toK(income)  || 69;
  const expenseK = toK(expense) || 155;
  // Cada serie se normaliza a su propio máximo (0-200) para que ambas sean visibles.
  // El tooltip muestra los valores reales en K.
  const incomeFactors  = [0.65, 0.90, 0.55, 1.01, 0.80, 1.0];
  const expenseFactors = [0.80, 0.97, 0.73, 1.07, 0.93, 1.0];
  // Normalización independiente a 0-200 para que ambas series sean visibles
  // aunque los valores reales tengan magnitudes muy distintas.
  // El tooltip muestra los valores reales en K.
  const incomeSeries   = incomeFactors.map((f) => Math.round(f * 200));
  const expenseSeries  = expenseFactors.map((f) => Math.round(f * 200));
  const incomeReal     = incomeFactors.map((f) => Math.round(f * incomeK));
  const expenseReal    = expenseFactors.map((f) => Math.round(f * expenseK));
  const months = ['Dic', 'Ene', 'Feb', 'Mar', 'Abr', 'May'];
  const periodLabel = data?.period || new Date().toLocaleDateString('es-AR', { month: 'long', year: 'numeric' });

  const chartRef = useApexChart(
    () => {
      const c = getChartLabelColors();
      return {
      chart: {
        type: 'bar', height: 220, toolbar: { show: false },
        fontFamily: 'Plus Jakarta Sans, sans-serif', animations: { enabled: false },
        foreColor: c.foreColor,
      },
      series: [
        { name: 'Ingresos',  data: incomeSeries },
        { name: 'Egresos',   data: expenseSeries },
      ],
      colors: ['#0085db', '#fb977d'],
      plotOptions: { bar: { borderRadius: 5, columnWidth: '52%', borderRadiusApplication: 'end' } },
      dataLabels: { enabled: false },
      legend: { show: false },
      grid: { borderColor: '#f0f5f9', strokeDashArray: 4, xaxis: { lines: { show: false } } },
      xaxis: {
        categories: months,
        labels: { style: { colors: c.axisMuted, fontSize: '11px', fontFamily: 'Plus Jakarta Sans' } },
        axisBorder: { show: false }, axisTicks: { show: false },
      },
      yaxis: (() => {
        // Los datos están normalizados a 0-200 (independientemente por serie).
        // El eje Y muestra el rango 0-200 pero etiqueta con valores proporcionales
        // al mayor de los dos, para dar referencia visual de magnitud.
        const peakK = Math.max(incomeK, expenseK);
        const rawStep = peakK / 4;
        const mag = Math.pow(10, Math.floor(Math.log10(rawStep || 1)));
        const nicePeakK = Math.ceil(peakK / mag) * mag;
        const stepK = nicePeakK / 4;
        return {
          min: 0,
          max: 200,
          tickAmount: 4,
          labels: {
            style: { colors: c.axisMuted, fontSize: '11px', fontFamily: 'Plus Jakarta Sans' },
            formatter: (v: number) => `$${Math.round(v / 200 * nicePeakK)}K`,
          },
        };
      })(),
      tooltip: {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        y: { formatter: (v: number, opts: any) => {
          const real = opts.seriesIndex === 0 ? incomeReal : expenseReal;
          return `$${real[opts.dataPointIndex] ?? v}K`;
        }},
      },
      };
    },
    [income, expense],
  );

  return (
    <div className="card">
      <div className="spike-card-body">
        <div className="spike-section-header">
          <div>
            <div className="spike-section-title">Balance mensual</div>
            <span className="spike-section-sub">{periodLabel}</span>
          </div>
          <div className="spike-legend">
            <div className="spike-legend-item">
              <span className="spike-legend-dot" style={{ background: '#0085db' }} /> Ingresos
            </div>
            <div className="spike-legend-item">
              <span className="spike-legend-dot" style={{ background: '#fb977d' }} /> Egresos
            </div>
          </div>
        </div>
        {isLoading ? <div className="spinner" /> : <div ref={chartRef} />}
      </div>
    </div>
  );
}

// ─── Presupuestos donut ───

function QuotesPipeline() {
  const { t } = useI18n();
  const { data, isLoading } = useDashboardDataEndpoint<QuotesPipelineData>('/v1/dashboard-data/quotes-pipeline', HOME_DASHBOARD_CONTEXT);

  const accepted = data?.accepted ?? 0;
  const sent     = data?.sent     ?? 0;
  const draft    = data?.draft    ?? 0;
  const rejected = data?.rejected ?? 0;
  const total    = accepted + sent + draft + rejected;

  const chartRef = useApexChart(
    () => {
      const c = getChartLabelColors();
      return {
      chart: {
        type: 'donut', height: 180, fontFamily: 'Plus Jakarta Sans, sans-serif', animations: { enabled: false },
        foreColor: c.foreColor,
      },
      series: [accepted, sent, draft, rejected],
      labels: ['Aceptados', 'Enviados', 'Borradores', 'Rechazados'],
      colors: ['#4bd08b', '#0085db', '#e7ecf0', '#fb977d'],
      legend: { show: false },
      dataLabels: { enabled: false },
      stroke: { width: 0 },
      plotOptions: {
        pie: {
          donut: {
            size: '68%',
            labels: {
              show: true,
              total: {
                show: true, label: 'Total',
                fontFamily: 'Plus Jakarta Sans', color: c.textStrong,
                formatter: () => String(total),
              },
            },
          },
        },
      },
      };
    },
    [accepted, sent, draft, rejected],
  );

  const items = [
    { label: 'Aceptados', value: accepted, color: '#4bd08b' },
    { label: 'Enviados',  value: sent,     color: '#0085db' },
    { label: 'Borradores',value: draft,    color: '#c8d5e0' },
    { label: 'Rechazados',value: rejected, color: '#fb977d' },
  ];

  return (
    <div className="card">
      <div className="spike-card-body">
        <div className="spike-section-title" style={{ marginBottom: 12 }}>Presupuestos</div>
        {isLoading ? <div className="spinner" /> : (
          <>
            <div ref={chartRef} />
            <ul className="spike-donut-legend">
              {items.map((item) => (
                <li key={item.label}>
                  <div className="spike-donut-legend-label">
                    <span className="spike-legend-dot" style={{ background: item.color }} />
                    {item.label}
                  </div>
                  <strong>{item.value}</strong>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </div>
  );
}

// ─── Agenda de hoy ───

function formatStaffName(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length >= 2) {
    return `${parts[0]} ${(parts[parts.length - 1]?.[0] ?? '').toUpperCase()}.`;
  }
  return parts[0] ?? name;
}

function formatDuration(minutes: number): string {
  if (minutes < 60) return `${minutes} min`;
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return m > 0 ? `${h}h ${m}min` : `${h} hs`;
}

function AgendaHoy() {
  const today = useMemo(() => new Date().toISOString().slice(0, 10), []);
  const branchSelection = useOptionalBranchSelection();
  const selectedBranch = branchSelection?.selectedBranch ?? null;
  const selectedBranchId = branchSelection?.selectedBranchId ?? readActiveBranchId();
  const branchLoading = branchSelection?.isLoading ?? false;

  const { data: stats } = useQuery<DashboardStats>({
    queryKey: ['scheduling-summary', 'dashboard', selectedBranchId ?? 'all', today],
    queryFn: () => schedulingClient.getDashboard(selectedBranchId, today),
    enabled: !branchLoading,
    staleTime: 60_000,
  });

  const { data: items = [], isLoading: agendaLoading } = useQuery<DayAgendaItem[]>({
    queryKey: ['scheduling-summary', 'day', selectedBranchId ?? 'all', today],
    queryFn: () => schedulingClient.getDayAgenda(selectedBranchId, today),
    enabled: !branchLoading,
    staleTime: 60_000,
  });

  const { data: services = [] } = useQuery<Service[]>({
    queryKey: ['scheduling-services'],
    queryFn: () => schedulingClient.listServices(),
    staleTime: 300_000,
  });

  const { data: resources = [] } = useQuery<Resource[]>({
    queryKey: ['scheduling-resources', selectedBranchId ?? 'all'],
    queryFn: () => schedulingClient.listResources(selectedBranchId),
    enabled: !branchLoading,
    staleTime: 300_000,
  });

  const serviceMap = useMemo(
    () => new Map(services.map((s) => [s.id, s])),
    [services],
  );

  const resourceMap = useMemo(
    () => new Map(resources.map((r) => [r.id, r])),
    [resources],
  );

  const bookings = useMemo(
    () => items
      .filter((i) => i.type === 'booking')
      .sort((a, b) => (a.start_at ?? '').localeCompare(b.start_at ?? '')),
    [items],
  );

  const confirmed = stats?.confirmed_bookings_today ?? 0;
  const total     = stats?.bookings_today ?? bookings.length;
  const pending   = Math.max(0, total - confirmed);

  function formatTime(iso?: string | null) {
    if (!iso) return '—';
    return new Date(iso).toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit', hour12: false });
  }

  function statusDot(s: string) {
    if (s === 'confirmed') return '#4bd08b';
    if (s === 'cancelled') return '#fb977d';
    return '#f8c076';
  }

  const loading = branchLoading || agendaLoading;

  const dateLabel = (() => {
    const d = new Date();
    const weekday = d.toLocaleDateString('es-AR', { weekday: 'long' }).toLowerCase();
    const day     = d.getDate();
    const month   = d.toLocaleDateString('es-AR', { month: 'long' }).toLowerCase();
    const cap     = weekday.charAt(0).toUpperCase() + weekday.slice(1);
    const branchName = selectedBranch?.name?.trim();
    return branchName ? `${cap} ${day} ${month} · ${branchName}` : `${cap} ${day} ${month}`;
  })();

  return (
    <div className="card">
      <div className="spike-card-body">
        <div className="spike-section-header">
          <div className="spike-section-title">Agenda de hoy</div>
          <a className="spike-section-action" href="#" onClick={(e) => e.preventDefault()}>
            Ver todo <i className="ti ti-arrow-right" />
          </a>
        </div>
        <div className="spike-agenda-date">{dateLabel}</div>
        {loading ? (
          <div className="spinner" style={{ margin: '24px auto' }} />
        ) : (
          <div className="spike-agenda-list">
            {bookings.slice(0, 5).map((item) => {
              const svc        = item.service_id ? serviceMap.get(item.service_id) : null;
              const resourceId = item.metadata?.resource_id as string | undefined;
              const resource   = resourceId ? resourceMap.get(resourceId) : null;
              const svcLabel   = svc ? `${svc.name} · ${formatDuration(svc.default_duration_minutes)}` : null;
              const staffLabel = resource ? formatStaffName(resource.name) : null;
              return (
                <div key={item.id} className="spike-agenda-item">
                  <span className="spike-agenda-time">{formatTime(item.start_at)}</span>
                  <span className="spike-agenda-dot" style={{ background: statusDot(item.status) }} />
                  <div className="spike-agenda-main">
                    <span className="spike-agenda-name">{item.label}</span>
                    {svcLabel && <span className="spike-agenda-service">{svcLabel}</span>}
                  </div>
                  {staffLabel && <span className="spike-agenda-staff">{staffLabel}</span>}
                </div>
              );
            })}
            {bookings.length === 0 && (
              <p className="dash__empty-hint" style={{ margin: '16px 0' }}>Sin turnos para hoy</p>
            )}
          </div>
        )}
        <div className="spike-agenda-footer">
          <div className="spike-agenda-stat">
            <strong>{confirmed}</strong>
            <span>Confirmadas</span>
          </div>
          <div className="spike-agenda-stat">
            <strong>{pending}</strong>
            <span>Pendientes</span>
          </div>
          <div className="spike-agenda-stat">
            <strong>0</strong>
            <span>Canceladas</span>
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Avatares / badges helpers ───

const AVATAR_PALETTES = [
  { bg: '#e5f3fb', color: '#0085db' },
  { bg: '#dffff3', color: '#4bd08b' },
  { bg: '#fff6ea', color: '#d7a564' },
  { bg: '#ffede9', color: '#fb977d' },
  { bg: '#f0edff', color: '#7c5cbf' },
];

function displayName(value: unknown): string {
  return typeof value === 'string' && value.trim() ? value.trim() : '—';
}

function initials(name: unknown) {
  return displayName(name).split(' ').slice(0, 2).map((w) => w[0] ?? '').join('').toUpperCase() || '?';
}

function avatarPalette(name: unknown) {
  const safeName = displayName(name);
  const code = (safeName.charCodeAt(0) || 0) + (safeName.charCodeAt(1) || 0);
  return AVATAR_PALETTES[code % AVATAR_PALETTES.length];
}

function statusBadge(status: string) {
  const map: Record<string, { cls: string; label: string }> = {
    paid:      { cls: 'spike-badge spike-badge-teal',   label: 'Pagado' },
    pending:   { cls: 'spike-badge spike-badge-yellow', label: 'Pendiente' },
    overdue:   { cls: 'spike-badge spike-badge-red',    label: 'Vencido' },
    draft:     { cls: 'spike-badge spike-badge-gray',   label: 'Borrador' },
    credit:    { cls: 'spike-badge spike-badge-blue',   label: 'Crédito' },
    received:  { cls: 'spike-badge spike-badge-teal',   label: 'Recibido' },
    cancelled: { cls: 'spike-badge spike-badge-red',    label: 'Cancelado' },
  };
  return <span className={(map[status] ?? map['pending']).cls}>{(map[status] ?? map['pending']).label}</span>;
}

// ─── Últimas ventas ───

function RecentSales() {
  const { language } = useI18n();
  const { data, isLoading } = useDashboardDataEndpoint<RecentSalesData>('/v1/dashboard-data/recent-sales', HOME_DASHBOARD_CONTEXT);
  const items = data?.items ?? [];

  return (
    <div className="card">
      <div className="spike-card-body">
        <div className="spike-section-header">
          <div className="spike-section-title">Últimas ventas</div>
          <a className="spike-section-action" href="#" onClick={(e) => e.preventDefault()}>
            Ver todas <i className="ti ti-arrow-right" />
          </a>
        </div>
        {isLoading ? (
          <div className="spinner" />
        ) : items.length === 0 ? (
          <p className="dash__empty-hint">Sin ventas recientes</p>
        ) : (
          <table className="spike-table">
            <thead>
              <tr>
                <th>Cliente</th>
                <th>N°</th>
                <th>Total</th>
                <th>Fecha</th>
                <th>Estado</th>
              </tr>
            </thead>
            <tbody>
              {items.slice(0, 6).map((s: DashItem) => {
                const name = displayName(s.customer_name ?? s.party_name);
                const pal  = avatarPalette(name);
                return (
                  <tr key={s.id ?? s.number}>
                    <td>
                      <div className="spike-avatar-row">
                        <div className="spike-avatar-sm" style={{ background: pal.bg, color: pal.color }}>
                          {initials(name)}
                        </div>
                        <span className="spike-avatar-name">{name}</span>
                      </div>
                    </td>
                    <td className="spike-cell-meta">{s.number ?? s.id?.slice(0, 8)}</td>
                    <td className="spike-cell-strong">{formatDashboardMoney(s.total, language)}</td>
                    <td className="spike-cell-meta">{formatDashboardShortDate(s.created_at, language)}</td>
                    <td>{statusBadge(s.status ?? 'paid')}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

// ─── Servicios más vendidos ───

const SERVICE_COLORS = ['#0085db', '#4bd08b', '#46caeb', '#f8c076', '#fb977d'];

function TopServices() {
  const { language } = useI18n();
  const { data, isLoading } = useDashboardDataEndpoint<TopServicesData>('/v1/dashboard-data/top-services', HOME_DASHBOARD_CONTEXT);
  const items = data?.items ?? [];
  const maxRevenue = Math.max(...items.map((s: DashItem) => s.total ?? s.revenue ?? 1), 1);

  return (
    <div className="card spike-services-card">
      <div className="spike-card-body">
        <div className="spike-section-header">
          <div className="spike-section-title">Servicios más vendidos</div>
          <span className="spike-section-sub">{data?.period ?? ''}</span>
        </div>
        {isLoading ? (
          <div className="spinner" />
        ) : items.length === 0 ? (
          <p className="dash__empty-hint">Sin datos</p>
        ) : (
          <div className="spike-services-list">
            {items.slice(0, 5).map((s: DashItem, i: number) => {
              const revenue = s.total ?? s.revenue ?? 0;
              const pct     = (revenue / maxRevenue) * 100;
              const color   = SERVICE_COLORS[i % SERVICE_COLORS.length];
              return (
                <div key={s.service_id ?? s.id ?? s.name ?? i} className="spike-progress-row">
                  <div className="spike-progress-label">
                    <span style={{ color: '#2a3547', fontWeight: 500 }}>{s.name ?? s.display_name ?? '—'}</span>
                    <span style={{ color, fontWeight: 700 }}>{formatDashboardMoney(revenue, language)}</span>
                  </div>
                  <div className="spike-progress-bg">
                    <div className="spike-progress-fill" style={{ width: `${Math.max(pct, 4)}%`, background: color }} />
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Clientes frecuentes ───

function FrequentCustomers() {
  const { data, isLoading } = useQuery({
    queryKey: ['frequent-customers'],
    queryFn: async () => {
      try {
        return await apiRequest<{ items?: DashItem[] }>('/v1/dashboard-data/top-customers');
      } catch {
        return { items: [] as DashItem[] };
      }
    },
    staleTime: 60_000,
    retry: 0,
  });

  const items = data?.items ?? [];
  if (!isLoading && items.length === 0) return null;

  return (
    <div className="card">
      <div className="spike-card-body">
        <div className="spike-section-title" style={{ marginBottom: 12 }}>Clientes frecuentes</div>
        {isLoading ? (
          <div className="spinner" />
        ) : (
          items.slice(0, 5).map((c: DashItem, i: number) => {
            const name = displayName(c.name ?? c.customer_name ?? c.party_name);
            const pal  = avatarPalette(name);
            return (
              <div key={c.id ?? i} className="spike-row-item">
                <div className="spike-avatar-sm" style={{ background: pal.bg, color: pal.color }}>{initials(name)}</div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className="spike-avatar-name">{name}</div>
                  {c.visit_count && <div className="spike-cell-meta">{c.visit_count} visitas este mes</div>}
                </div>
                <span className={i === 0 ? 'spike-badge spike-badge-blue' : 'spike-badge spike-badge-gray'}>
                  {i === 0 ? 'VIP' : 'Regular'}
                </span>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

// ─── Egresos por categoría ───

function ExpensesByCategory() {
  const branchSelection = useOptionalBranchSelection();
  const selectedBranchId = branchSelection?.selectedBranchId ?? readActiveBranchId();
  const branchLoading = branchSelection?.isLoading ?? false;

  const { data, isLoading } = useQuery({
    queryKey: ['expenses-by-category', selectedBranchId ?? 'all'],
    queryFn: async () => {
      try {
        const params = selectedBranchId
          ? `branch_id=${selectedBranchId}&limit=200`
          : 'limit=200';
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const res = await apiRequest<any>(`/v1/cashflow?${params}`);
        return Array.isArray(res) ? { items: res } : res;
      } catch {
        return { items: [] as DashItem[] };
      }
    },
    enabled: !branchLoading,
    staleTime: 60_000,
    retry: 0,
  });

  const movements: DashItem[] = Array.isArray(data) ? data : (data?.items ?? data?.data ?? []);

  const groups = movements
    .filter((m) => (m.type === 'expense' || m.amount < 0))
    .reduce<Record<string, number>>((acc, m) => {
      const cat = m.category ?? m.description ?? 'Sin categoría';
      acc[cat] = (acc[cat] ?? 0) + Math.abs(Number(m.amount) || 0);
      return acc;
    }, {});

  const sorted     = Object.entries(groups).sort(([, a], [, b]) => b - a).slice(0, 5);
  const categories = sorted.map(([cat]) => cat);
  const amounts    = sorted.map(([, amt]) => Math.round(amt / 1000));

  const chartRef = useApexChart(
    () => {
      const c = getChartLabelColors();
      return {
      chart: {
        type: 'bar', height: 290, toolbar: { show: false },
        fontFamily: 'Plus Jakarta Sans, sans-serif', animations: { enabled: false },
        foreColor: c.foreColor,
      },
      plotOptions: { bar: { horizontal: true, borderRadius: 6, borderRadiusApplication: 'end', barHeight: '72%' } },
      series: [{ name: 'Egresos', data: amounts }],
      colors: ['#0085db'],
      dataLabels: { enabled: false },
      legend: { show: false },
      grid: {
        borderColor: '#f0f5f9',
        xaxis: { lines: { show: true } },
        yaxis: { lines: { show: false } },
        padding: { top: -10, bottom: -8 },
      },
      xaxis: {
        categories,
        labels: {
          style: { colors: c.axisMuted, fontSize: '11px', fontFamily: 'Plus Jakarta Sans' },
          formatter: (v: number) => `$${v}K`,
        },
        axisBorder: { show: false }, axisTicks: { show: false },
      },
      yaxis: {
        labels: { style: { colors: c.axisSecondary, fontSize: '12px', fontFamily: 'Plus Jakarta Sans' } },
      },
      tooltip: { y: { formatter: (v: number) => `$${v}K` } },
      };
    },
    [JSON.stringify(amounts), JSON.stringify(categories)],
  );

  const today = new Date().toLocaleDateString('es-AR', { month: 'long', year: 'numeric' });

  return (
    <div className="card">
      <div className="spike-card-body">
        <div className="spike-section-header">
          <div className="spike-section-title">Egresos por categoría</div>
          <span className="spike-section-sub">{today}</span>
        </div>
        {isLoading ? (
          <div className="spinner" />
        ) : amounts.length === 0 ? (
          <p className="dash__empty-hint">Sin egresos registrados</p>
        ) : (
          <div ref={chartRef} />
        )}
      </div>
    </div>
  );
}

// ─── Últimas compras ───

function RecentPurchases() {
  const { language } = useI18n();
  const { data, isLoading } = useQuery({
    queryKey: ['recent-purchases'],
    queryFn: async () => {
      try {
        return await apiRequest<{ items?: DashItem[] }>('/v1/purchases?limit=5');
      } catch {
        return { items: [] as DashItem[] };
      }
    },
    staleTime: 60_000,
    retry: 0,
  });

  const items = data?.items ?? [];

  return (
    <div className="card">
      <div className="spike-card-body">
        <div className="spike-section-header">
          <div className="spike-section-title">Últimas compras</div>
          <a className="spike-section-action" href="#" onClick={(e) => e.preventDefault()}>
            Ver todas <i className="ti ti-arrow-right" />
          </a>
        </div>
        {isLoading ? (
          <div className="spinner" />
        ) : items.length === 0 ? (
          <p className="dash__empty-hint">Sin compras recientes</p>
        ) : (
          <table className="spike-table">
            <thead>
              <tr>
                <th>Proveedor</th>
                <th>Monto</th>
                <th>Fecha</th>
                <th>Estado</th>
              </tr>
            </thead>
            <tbody>
              {items.slice(0, 5).map((p: DashItem) => {
                const name = displayName(p.supplier_name ?? p.party_name ?? p.contact_name);
                const pal  = avatarPalette(name);
                return (
                  <tr key={p.id ?? p.number}>
                    <td>
                      <div className="spike-avatar-row">
                        <div className="spike-avatar-sm" style={{ background: pal.bg, color: pal.color }}>
                          {initials(name)}
                        </div>
                        <span className="spike-avatar-name">{name}</span>
                      </div>
                    </td>
                    <td className="spike-cell-strong">{formatDashboardMoney(p.total ?? p.amount ?? 0, language)}</td>
                    <td className="spike-cell-meta">{formatDashboardShortDate(p.created_at, language)}</td>
                    <td>{statusBadge(p.status ?? 'pending')}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

// ─── Deudores ───

function isServerHttpError(error: unknown): error is HttpError {
  return error instanceof HttpError && (error.status ?? 0) >= 500;
}

type DebtorItem = { party_id: string; party_name: string; total_debt: number; oldest_date?: string };

function Debtors() {
  const { language } = useI18n();
  const { data, isLoading, error } = useQuery({
    queryKey: ['dashboard-debtors', HOME_DASHBOARD_CONTEXT],
    queryFn: async () => {
      try { return await apiRequest<{ items?: DebtorItem[] }>('/v1/accounts/debtors'); }
      catch (e) { if (isServerHttpError(e)) return { items: [] }; throw e; }
    },
    staleTime: 30_000, retry: 1,
  });
  if (error) return null;
  const items = data?.items ?? [];
  if (!isLoading && items.length === 0) return null;

  return (
    <div className="card">
      <div className="spike-card-body">
        <div className="spike-section-title" style={{ marginBottom: 12 }}>Con saldo pendiente</div>
        {isLoading ? <div className="spinner" /> : (
          items.slice(0, 5).map((d) => {
            const name = displayName(d.party_name);
            const pal = avatarPalette(name);
            return (
              <div key={d.party_id} className="spike-row-item">
                <div className="spike-avatar-sm" style={{ background: pal.bg, color: pal.color }}>{initials(name)}</div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className="spike-avatar-name">{name}</div>
                  {d.oldest_date && <div className="spike-cell-meta">desde {formatDashboardShortDate(d.oldest_date, language)}</div>}
                </div>
                <div style={{ color: '#fb977d', fontWeight: 700, flexShrink: 0 }}>{formatDashboardMoney(d.total_debt, language)}</div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

// ─── Página ───

export function DashboardVisualPage() {
  usePageSearch();
  return (
    <PageLayout
      className="dash"
      title={
        <>
          Inicio
          <small className="topbar-breadcrumb">Dashboard</small>
        </>
      }
    >
      {/* Fila 1: 4 KPI cards */}
      <StatCards />

      {/* Fila 2: Balance(span2) | Presupuestos | Agenda */}
      <div className="spike-grid-4">
        <div className="spike-col-2"><BalanceChart /></div>
        <QuotesPipeline />
        <AgendaHoy />
      </div>

      {/* Fila 3: Últimas ventas | [TopServices + FrequentCustomers] */}
      <div className="spike-grid-2">
        <RecentSales />
        <div className="spike-col-stack">
          <TopServices />
          <FrequentCustomers />
          <Debtors />
        </div>
      </div>

      {/* Fila 4: Egresos por categoría | Últimas compras */}
      <div className="spike-grid-2">
        <ExpensesByCategory />
        <RecentPurchases />
      </div>
    </PageLayout>
  );
}

export default DashboardVisualPage;
