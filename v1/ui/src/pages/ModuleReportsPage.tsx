import { useMemo, useState } from 'react';
import { useQueries } from '@tanstack/react-query';
import { PageLayout } from '../components/PageLayout';
import { ReportsResultView } from '../components/ReportsResultView';
import { apiRequest } from '../lib/api';
import { useOptionalBranchSelection } from '../lib/useBranchSelection';
import { readActiveBranchId } from '../lib/branchSelectionStorage';
import { moduleCatalog, type ModuleDataset } from '../lib/moduleCatalog';
import { useI18n } from '../lib/i18n';
import { formatKpiValue } from '../lib/reportsResultPresentation';
import { appendBranchIdToReportPath, buildInitialValues, buildPath, currentRuntimeContext } from './modulePageUtils';

const EMPTY_DATASETS: ModuleDataset[] = [];

type ReportQueryResult = {
  dataset: ModuleDataset;
  data: unknown;
  error: unknown;
  isLoading: boolean;
};

function readReportMetric(data: unknown, key: string): unknown {
  if (!data || typeof data !== 'object') {
    return undefined;
  }
  if ('data' in data) {
    const inner = (data as { data: unknown }).data;
    if (inner && typeof inner === 'object' && !Array.isArray(inner)) {
      return (inner as Record<string, unknown>)[key];
    }
  }
  return (data as Record<string, unknown>)[key];
}

function reportSectionKey(path: string): 'sales' | 'inventory' | 'finance' {
  if (path.includes('inventory') || path.includes('low-stock')) {
    return 'inventory';
  }
  if (path.includes('cashflow') || path.includes('profit-margin')) {
    return 'finance';
  }
  return 'sales';
}

function reportSectionLabel(section: 'sales' | 'inventory' | 'finance'): string {
  switch (section) {
    case 'sales':
      return 'Ventas';
    case 'inventory':
      return 'Inventario';
    case 'finance':
      return 'Finanzas';
  }
}

function reportCardTone(section: 'sales' | 'inventory' | 'finance'): string {
  switch (section) {
    case 'sales':
      return 'reports-summary-card reports-summary-card--sales';
    case 'inventory':
      return 'reports-summary-card reports-summary-card--inventory';
    case 'finance':
      return 'reports-summary-card reports-summary-card--finance';
  }
}

export function ReportsBusinessPage() {
  const { language } = useI18n();
  const module = moduleCatalog.reports;
  const branchSelection = useOptionalBranchSelection();
  const selectedBranchId = branchSelection?.selectedBranchId ?? readActiveBranchId();
  const runtime = useMemo(() => currentRuntimeContext(), []);
  const [draftRange, setDraftRange] = useState(() => ({ from: runtime.monthStart, to: runtime.today }));
  const [range, setRange] = useState(() => ({ from: runtime.monthStart, to: runtime.today }));

  const datasets = module.datasets ?? EMPTY_DATASETS;
  const queries = useQueries({
    queries: datasets.map((dataset) => ({
      queryKey: ['reports', dataset.id ?? dataset.path, range.from, range.to, selectedBranchId],
      queryFn: async () => {
        const values = buildInitialValues(dataset.fields, { ...runtime, monthStart: range.from, today: range.to });
        if (dataset.fields?.some((field) => field.name === 'from')) {
          values.from = range.from;
        }
        if (dataset.fields?.some((field) => field.name === 'to')) {
          values.to = range.to;
        }
        const path = appendBranchIdToReportPath(
          buildPath(dataset.path, dataset.fields, values, { ...runtime, monthStart: range.from, today: range.to }),
          selectedBranchId,
        );
        return apiRequest(path);
      },
      retry: false,
    })),
  });

  const results = useMemo<ReportQueryResult[]>(
    () =>
      datasets.map((dataset, index) => ({
        dataset,
        data: queries[index]?.data,
        error: queries[index]?.error,
        isLoading: Boolean(queries[index]?.isLoading),
      })),
    [datasets, queries],
  );

  const summaryCards = [
    {
      label: 'Ventas',
      value: readReportMetric(
        results.find((entry) => entry.dataset.path.includes('sales-summary'))?.data,
        'total_sales',
      ),
      tone: reportCardTone('sales'),
    },
    {
      label: 'Ticket promedio',
      value: readReportMetric(
        results.find((entry) => entry.dataset.path.includes('sales-summary'))?.data,
        'average_ticket',
      ),
      tone: reportCardTone('sales'),
    },
    {
      label: 'Balance',
      value: readReportMetric(
        results.find((entry) => entry.dataset.path.includes('cashflow-summary'))?.data,
        'balance',
      ),
      tone: reportCardTone('finance'),
    },
    {
      label: 'Inventario',
      value: readReportMetric(
        results.find((entry) => entry.dataset.path.includes('inventory-valuation'))?.data,
        'total',
      ),
      tone: reportCardTone('inventory'),
    },
  ];

  const groupedResults = (['sales', 'inventory', 'finance'] as const).map((section) => ({
    section,
    title: reportSectionLabel(section),
    items: results.filter((entry) => reportSectionKey(entry.dataset.path) === section),
  }));

  return (
    <PageLayout
      className="module-page reports-page"
      title={module.title}
      lead={module.summary}
      actions={
        <form
          className="reports-filters"
          onSubmit={(event) => {
            event.preventDefault();
            setRange(draftRange);
          }}
        >
          <label className="reports-filter-field">
            <span>Desde</span>
            <input
              type="date"
              value={draftRange.from}
              onChange={(event) => setDraftRange((current) => ({ ...current, from: event.target.value }))}
            />
          </label>
          <label className="reports-filter-field">
            <span>Hasta</span>
            <input
              type="date"
              value={draftRange.to}
              onChange={(event) => setDraftRange((current) => ({ ...current, to: event.target.value }))}
            />
          </label>
          <button type="submit" className="btn-primary">
            Actualizar
          </button>
        </form>
      }
    >
      <div className="stats-grid compact-grid reports-summary-grid">
        {summaryCards.map((card) => (
          <div key={card.label} className={`stat-card ${card.tone}`}>
            <div className="stat-label">{card.label}</div>
            <div className="stat-value report-kpi-value">
              {formatKpiValue(
                card.label === 'Ticket promedio'
                  ? 'average_ticket'
                  : card.label === 'Balance'
                    ? 'balance'
                    : card.label === 'Inventario'
                      ? 'valuation'
                      : 'total_sales',
                card.value,
                language,
              )}
            </div>
          </div>
        ))}
      </div>

      {groupedResults.map((group) => (
        <section key={group.section} className="reports-section">
          <div className="section-title-row">
            <h2>{group.title}</h2>
          </div>
          <div className="module-grid reports-grid">
            {group.items.map((entry) => (
              <article key={entry.dataset.id ?? entry.dataset.path} className="card reports-card">
                <div className="card-header module-card-header-inner">
                  <div>
                    <h3>{entry.dataset.title}</h3>
                    <p className="text-secondary">{entry.dataset.description}</p>
                  </div>
                </div>
                {entry.isLoading ? (
                  <div className="empty-state module-result-empty">
                    <p>Cargando reporte…</p>
                  </div>
                ) : entry.error ? (
                  <div className="alert alert-error">
                    {entry.error instanceof Error ? entry.error.message : String(entry.error)}
                  </div>
                ) : entry.data != null ? (
                  <ReportsResultView data={entry.data} datasetPath={entry.dataset.path} showRawJson={false} />
                ) : (
                  <div className="empty-state module-result-empty">
                    <p>Sin datos para mostrar.</p>
                  </div>
                )}
              </article>
            ))}
          </div>
        </section>
      ))}
    </PageLayout>
  );
}
