import { useI18n } from '../lib/i18n';
import {
  extractTabularRows,
  formatKpiValue,
  formatReportCell,
  isKpiEnvelopePath,
  numericMetricMax,
  orderedReportColumns,
  readReportPeriod,
  reportBarMetricKey,
  reportColumnLabel,
  tableScalarColumnsForRows,
} from '../lib/reportsResultPresentation';

type Props = {
  data: unknown;
  datasetPath: string;
};

function kpiFieldOrder(datasetPath: string): string[] | null {
  if (datasetPath.includes('sales-summary')) {
    return ['total_sales', 'count_sales', 'average_ticket'];
  }
  if (datasetPath.includes('cashflow-summary')) {
    return ['total_income', 'total_expense', 'balance'];
  }
  if (datasetPath.includes('profit-margin')) {
    return ['revenue', 'cost', 'gross_profit', 'margin_pct'];
  }
  return null;
}

export function ReportsResultView({ data, datasetPath }: Props) {
  const { t, language, sentenceCase } = useI18n();
  const { from, to } = readReportPeriod(data);

  const kpiOrder = isKpiEnvelopePath(datasetPath) ? kpiFieldOrder(datasetPath) : null;
  if (kpiOrder && data && typeof data === 'object' && 'data' in data) {
    const inner = (data as { data: unknown }).data;
    if (inner && typeof inner === 'object' && !Array.isArray(inner)) {
      const record = inner as Record<string, unknown>;
      const entries = kpiOrder
        .filter((key) => key in record)
        .map((key) => [key, record[key]] as const);
      const rest = Object.entries(record).filter(([k]) => !kpiOrder.includes(k));
      const allEntries = [...entries, ...rest];

      return (
        <div className="report-result">
          {from && to ? (
            <p className="report-result-period text-secondary">
              {sentenceCase(t('module.reports.period'))}: <strong>{from}</strong> → <strong>{to}</strong>
            </p>
          ) : null}
          <div className="stats-grid compact-grid report-kpi-grid">
            {allEntries.map(([key, value]) => (
              <div key={key} className="stat-card report-kpi-card">
                <div className="stat-label">{reportColumnLabel(key, language)}</div>
                <div className="stat-value report-kpi-value">{formatKpiValue(key, value, language)}</div>
              </div>
            ))}
          </div>
          <RawJsonDetails data={data} />
        </div>
      );
    }
  }

  if (datasetPath.includes('inventory-valuation') && data && typeof data === 'object') {
    const env = data as { total?: unknown; items?: unknown };
    if (typeof env.total === 'number') {
      const totalVal = env.total;
      const rows = Array.isArray(env.items)
        ? (env.items as unknown[]).filter((r): r is Record<string, unknown> => r !== null && typeof r === 'object')
        : [];
      return (
        <div className="report-result">
          <div className="report-inventory-total card report-highlight-card">
            <span className="text-secondary">{sentenceCase(t('module.reports.inventoryTotal'))}</span>
            <strong className="report-inventory-total-value">{formatReportCell('valuation', totalVal, language)}</strong>
          </div>
          {rows.length === 0 ? (
            <div className="empty-state module-result-empty">
              <p>{t('module.result.emptyList')}</p>
            </div>
          ) : (
            <ReportItemsTable rows={rows} datasetPath={datasetPath} />
          )}
          <RawJsonDetails data={data} />
        </div>
      );
    }
  }

  const rows = extractTabularRows(data);
  if (rows !== null) {
    if (rows.length === 0) {
      return (
        <div className="empty-state module-result-empty">
          <p>{t('module.result.emptyList')}</p>
        </div>
      );
    }
    return (
      <div className="report-result">
        {from && to ? (
          <p className="report-result-period text-secondary">
            {sentenceCase(t('module.reports.period'))}: <strong>{from}</strong> → <strong>{to}</strong>
          </p>
        ) : null}
        <ReportItemsTable rows={rows} datasetPath={datasetPath} />
        <RawJsonDetails data={data} />
      </div>
    );
  }

  return <GenericObjectFallback data={data} />;
}

function ReportItemsTable({
  rows,
  datasetPath,
}: {
  rows: Array<Record<string, unknown>>;
  datasetPath: string;
}) {
  const { language, t, sentenceCase } = useI18n();
  const baseCols = tableScalarColumnsForRows(rows, 12);
  const columns = orderedReportColumns(datasetPath, baseCols);
  const barKey = reportBarMetricKey(datasetPath);
  const maxBar = barKey ? numericMetricMax(rows.slice(0, 50), barKey) : 0;

  return (
    <div className="table-wrap report-table-wrap">
      <table className="report-data-table">
        <thead>
          <tr>
            {columns.map((column) => (
              <th
                key={column}
                className={
                  column === 'revenue' ||
                  column === 'total' ||
                  column === 'valuation' ||
                  column === 'quantity' ||
                  column === 'count'
                    ? 'report-col-num'
                    : undefined
                }
              >
                {reportColumnLabel(column, language)}
              </th>
            ))}
            {barKey && maxBar > 0 ? (
              <th className="report-col-bar" title={t('module.reports.barHint')}>
                {sentenceCase(t('module.reports.barHint'))}
              </th>
            ) : null}
          </tr>
        </thead>
        <tbody>
          {rows.slice(0, 100).map((row, index) => (
            <tr key={String(row.id ?? row.product_id ?? row.sku ?? index)}>
              {columns.map((column) => (
                <td
                  key={column}
                  className={
                    column === 'revenue' ||
                    column === 'total' ||
                    column === 'valuation' ||
                    column === 'cost_price'
                      ? 'report-col-num report-col-strong'
                      : column === 'quantity' || column === 'count'
                        ? 'report-col-num'
                        : undefined
                  }
                >
                  {formatReportCell(column, row[column], language)}
                </td>
              ))}
              {barKey && maxBar > 0 ? (
                <td className="report-col-bar">
                  <ReportBar value={row[barKey]} max={maxBar} />
                </td>
              ) : null}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ReportBar({ value, max }: { value: unknown; max: number }) {
  const n = typeof value === 'number' && !Number.isNaN(value) ? value : 0;
  const pct = max > 0 ? Math.min(100, Math.round((n / max) * 100)) : 0;
  return (
    <div className="report-bar-track" role="presentation">
      <div className="report-bar-fill" style={{ width: `${pct}%` }} />
    </div>
  );
}

function RawJsonDetails({ data }: { data: unknown }) {
  const { t, sentenceCase } = useI18n();
  return (
    <details className="report-raw-json">
      <summary>{sentenceCase(t('module.reports.rawJson'))}</summary>
      <pre className="mono">{JSON.stringify(data, null, 2)}</pre>
    </details>
  );
}

function GenericObjectFallback({ data }: { data: unknown }) {
  const { t, sentenceCase } = useI18n();
  if (data && typeof data === 'object') {
    const entries = Object.entries(data as Record<string, unknown>);
    const scalarEntries = entries.filter(([, value]) => {
      const type = typeof value;
      return value == null || type === 'string' || type === 'number' || type === 'boolean';
    });
    return (
      <>
        {scalarEntries.length > 0 && (
          <div className="kv-grid">
            {scalarEntries.map(([key, value]) => (
              <div key={key} className="kv-item">
                <span>{key}</span>
                <strong>{value === null || value === undefined ? '---' : String(value)}</strong>
              </div>
            ))}
          </div>
        )}
        <details className="report-raw-json">
          <summary>{sentenceCase(t('module.reports.rawJson'))}</summary>
          <pre className="mono">{JSON.stringify(data, null, 2)}</pre>
        </details>
      </>
    );
  }
  return <pre>{String(data)}</pre>;
}
