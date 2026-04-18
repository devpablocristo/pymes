import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useQueries, useQuery } from '@tanstack/react-query';
import { Link, Navigate, useParams } from 'react-router-dom';
import { fromCrudResourceSlug, toCrudResourceSlug } from '../crud/crudResourceSlug';
import { tenantLink, useActiveTenantSlug } from '../lib/tenantSlug';
import { PageLayout } from '../components/PageLayout';
import { ReportsResultView } from '../components/ReportsResultView';
import { apiRequest, downloadAPIFile, getSession } from '../lib/api';
import { useOptionalBranchSelection } from '../lib/useBranchSelection';
import { readActiveBranchId } from '../lib/branchSelectionStorage';
import { hasLazyCrudResource } from '../crud/lazyCrudPage';
import { ConfiguredCrudStandalonePage } from '../crud/configuredCrudViews';
import {
  moduleCatalog,
  moduleList,
  resolveModuleDefault,
  type ModuleAction,
  type ModuleDataset,
  type ModuleDefinition,
  type ModuleField,
  type ModuleRuntimeContext,
} from '../lib/moduleCatalog';
import { useI18n } from '../lib/i18n';
import { queryKeys } from '../lib/queryKeys';
import { formatKpiValue, isReportDatasetPath } from '../lib/reportsResultPresentation';
import { vocab } from '../lib/vocabulary';

const EMPTY_DATASETS: ModuleDataset[] = [];

function currentRuntimeContext(): ModuleRuntimeContext {
  const now = new Date();
  const today = now.toISOString().slice(0, 10);
  const monthStart = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1)).toISOString().slice(0, 10);
  return { orgId: '', today, monthStart };
}

function buildInitialValues(fields: ModuleField[] | undefined, ctx: ModuleRuntimeContext): Record<string, string> {
  return Object.fromEntries((fields ?? []).map((field) => [field.name, resolveModuleDefault(field.defaultValue, ctx)]));
}

function buildPath(
  template: string,
  fields: ModuleField[] | undefined,
  values: Record<string, string>,
  ctx: ModuleRuntimeContext,
): string {
  let path = template.replace(/\{\{(\w+)\}\}/g, (_match, key: string) =>
    encodeURIComponent((ctx as Record<string, string>)[key] ?? ''),
  );
  const params = new URLSearchParams();

  for (const field of fields ?? []) {
    const value = (values[field.name] ?? '').trim();
    if (field.location === 'path') {
      path = path.replace(`:${field.name}`, encodeURIComponent(value));
      continue;
    }
    if (field.location === 'query' && value !== '') {
      params.set(field.name, value);
    }
  }

  if (params.size > 0) {
    path += `${path.includes('?') ? '&' : '?'}${params.toString()}`;
  }
  return path;
}

function appendBranchIdToReportPath(path: string, branchId: string | null | undefined): string {
  const normalizedBranchId = branchId?.trim();
  if (!normalizedBranchId || !isReportDatasetPath(path)) {
    return path;
  }
  const separator = path.includes('?') ? '&' : '?';
  return `${path}${separator}branch_id=${encodeURIComponent(normalizedBranchId)}`;
}

function buildBody(fields: ModuleField[] | undefined, values: Record<string, string>): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  for (const field of fields ?? []) {
    if (field.location !== 'body') {
      continue;
    }
    const raw = (values[field.name] ?? '').trim();
    if (raw === '') {
      continue;
    }
    if (field.type === 'json') {
      try {
        body[field.name] = JSON.parse(raw) as unknown;
      } catch {
        throw new Error(`JSON inválido en «${field.label}»`);
      }
    } else if (field.type === 'number') {
      body[field.name] = Number(raw);
    } else {
      body[field.name] = raw;
    }
  }
  return body;
}

function groupedModuleActions(
  module: ModuleDefinition,
): Array<{ key: string; title: string; actions: ModuleAction[] }> {
  const actions = module.actions ?? [];
  const order = module.actionGroupOrder;
  const labels = module.actionGroupLabels;
  const map = new Map<string, ModuleAction[]>();
  for (const action of actions) {
    const key = action.group ?? '_ungrouped';
    if (!map.has(key)) {
      map.set(key, []);
    }
    map.get(key)!.push(action);
  }
  const keys: string[] = [];
  if (order) {
    for (const k of order) {
      if (map.has(k)) {
        keys.push(k);
      }
    }
  }
  for (const k of map.keys()) {
    if (!keys.includes(k)) {
      keys.push(k);
    }
  }
  return keys.map((key) => ({
    key,
    title: labels?.[key] ?? (key === '_ungrouped' ? 'Acciones' : key),
    actions: map.get(key) ?? [],
  }));
}

function missingRequiredFields(fields: ModuleField[] | undefined, values: Record<string, string>): string[] {
  return (fields ?? [])
    .filter((field) => field.required && (values[field.name] ?? '').trim() === '')
    .map((field) => field.label);
}

function stringifyValue(value: unknown): string {
  if (value === null || value === undefined) {
    return '---';
  }
  if (typeof value === 'string') {
    return value || '---';
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  if (Array.isArray(value)) {
    return value.map((item) => (typeof item === 'object' ? JSON.stringify(item) : String(item))).join(', ');
  }
  return JSON.stringify(value);
}

function isScalarCell(value: unknown): boolean {
  if (value === null || value === undefined) {
    return true;
  }
  const t = typeof value;
  return t === 'string' || t === 'number' || t === 'boolean';
}

/** Columnas legibles: solo escalares por fila (evita columna `items` en devoluciones, etc.). */
function tableColumnsForRows(rows: Array<Record<string, unknown>>, maxCols: number): string[] {
  const allKeys = Array.from(new Set(rows.flatMap((row) => Object.keys(row))));
  const scalarKeys = allKeys.filter((key) => rows.every((row) => isScalarCell(row[key])));
  const ordered = scalarKeys.length > 0 ? scalarKeys : allKeys;
  const priority = (k: string): number => {
    if (k === 'id') {
      return 0;
    }
    if (k.endsWith('_id') || k.endsWith('Id')) {
      return 1;
    }
    return 2;
  };
  ordered.sort((a, b) => {
    const pa = priority(a);
    const pb = priority(b);
    if (pa !== pb) {
      return pa - pb;
    }
    return a.localeCompare(b);
  });
  return ordered.slice(0, maxCols);
}

function extractRows(data: unknown): Array<Record<string, unknown>> | null {
  if (Array.isArray(data) && data.every((item) => item && typeof item === 'object')) {
    return data as Array<Record<string, unknown>>;
  }
  if (data && typeof data === 'object' && 'items' in data) {
    const raw = (data as { items: unknown }).items;
    if (raw == null) {
      return [];
    }
    if (!Array.isArray(raw)) {
      return null;
    }
    if (raw.length === 0) {
      return [];
    }
    if (raw.every((item) => item && typeof item === 'object')) {
      return raw as Array<Record<string, unknown>>;
    }
  }
  if (data && typeof data === 'object' && 'data' in data) {
    const inner = (data as { data: unknown }).data;
    if (Array.isArray(inner)) {
      if (inner.length === 0) {
        return [];
      }
      if (inner.every((item) => item && typeof item === 'object')) {
        return inner as Array<Record<string, unknown>>;
      }
      return null;
    }
    if (inner && typeof inner === 'object' && !Array.isArray(inner)) {
      const row = inner as Record<string, unknown>;
      const scalarRow = Object.fromEntries(Object.entries(row).filter(([, v]) => isScalarCell(v)));
      if (Object.keys(scalarRow).length > 0) {
        return [scalarRow];
      }
    }
  }
  return null;
}

function ResultView({ data, datasetPath }: { data: unknown; datasetPath?: string }) {
  const { t } = useI18n();

  if (data === null || data === undefined) {
    return null;
  }

  if (datasetPath && isReportDatasetPath(datasetPath)) {
    return <ReportsResultView data={data} datasetPath={datasetPath} />;
  }

  const rows = extractRows(data);
  if (rows !== null) {
    if (rows.length === 0) {
      return (
        <div className="empty-state module-result-empty">
          <p>{t('module.result.emptyList')}</p>
        </div>
      );
    }
    const columns = tableColumnsForRows(rows, 8);
    return (
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              {columns.map((column) => (
                <th key={column}>{column}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.slice(0, 100).map((row, index) => (
              <tr key={String(row.id ?? row.code ?? row.name ?? index)}>
                {columns.map((column) => (
                  <td key={column}>{stringifyValue(row[column])}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

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
                <strong>{stringifyValue(value)}</strong>
              </div>
            ))}
          </div>
        )}
        <pre>{JSON.stringify(data, null, 2)}</pre>
      </>
    );
  }

  return <pre>{String(data)}</pre>;
}

type EndpointCardProps = {
  definition: ModuleDataset | ModuleAction;
  runtime: ModuleRuntimeContext;
  kind: 'dataset' | 'action';
};

function EndpointCard({ definition, runtime, kind }: EndpointCardProps) {
  const { t, localizeText, localizeUiText, sentenceCase } = useI18n();
  const fields = definition.fields ?? [];
  const runtimeKey = `${runtime.orgId}:${runtime.today}:${runtime.monthStart}`;
  const defId = definition.id ?? definition.path;
  const [values, setValues] = useState<Record<string, string>>(() => buildInitialValues(fields, runtime));
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [result, setResult] = useState<unknown>(null);

  useEffect(() => {
    setValues(buildInitialValues(fields, runtime));
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reiniciar solo cuando cambia la definición o el runtime key
  }, [defId, runtimeKey]);

  async function execute(currentValues: Record<string, string>): Promise<void> {
    const missing = missingRequiredFields(fields, currentValues);
    if (missing.length > 0) {
      setError(t('module.validation.complete', { fields: missing.map((field) => localizeText(field)).join(', ') }));
      return;
    }

    const path = buildPath(definition.path, fields, currentValues, runtime);
    const action = definition as ModuleAction;
    const isAction = kind === 'action';
    let body: Record<string, unknown>;
    try {
      body = buildBody(fields, currentValues);
    } catch (parseErr) {
      setError(String(parseErr));
      return;
    }
    const hasBody = Object.keys(body).length > 0;

    setLoading(true);
    setError('');
    setSuccess('');
    try {
      if (isAction && action.response === 'download') {
        const filename = await downloadAPIFile(path, { method: action.method });
        setSuccess(t('module.success.download', { filename }));
        setResult({ filename, path });
      } else {
        const payload = await apiRequest(path, {
          method: isAction ? action.method : 'GET',
          body: hasBody ? body : action.sendEmptyBody ? {} : undefined,
        });
        setResult(payload);
        setSuccess(isAction && action.method !== 'GET' ? t('module.success.completed') : t('module.success.updated'));
      }
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (kind === 'dataset' && (definition as ModuleDataset).autoLoad) {
      const initial = buildInitialValues(fields, runtime);
      if (missingRequiredFields(fields, initial).length === 0) {
        void execute(initial);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- auto-ejecutar solo al cambiar de definición/runtime
  }, [defId, kind, runtimeKey]);

  function onSubmit(event: FormEvent): void {
    event.preventDefault();
    void execute(values);
  }

  const httpMethod = kind === 'dataset' ? 'GET' : (definition as ModuleAction).method;
  const anchorId =
    kind === 'dataset'
      ? `module-dataset-${(definition as ModuleDataset).id ?? definition.path}`
      : `module-action-${(definition as ModuleAction).id}`;

  return (
    <section id={anchorId} className="card module-card module-endpoint-anchor">
      <div className="card-header module-card-header-inner">
        <div>
          <h2>{localizeUiText(vocab(definition.title))}</h2>
          <p className="text-secondary">{localizeText(vocab(definition.description))}</p>
        </div>
      </div>
      <div className="module-endpoint-meta">
        <span className={`http-method http-method-${httpMethod.toLowerCase()}`}>{httpMethod}</span>
        <code className="endpoint-path" title={definition.path}>
          {definition.path}
        </code>
      </div>

      <form onSubmit={onSubmit} className="module-form-stack">
        {fields.length > 0 && (
          <div className="module-fields-grid">
            {fields.map((field) => (
              <div
                key={field.name}
                className={`form-group ${field.type === 'textarea' || field.type === 'json' ? 'full-width' : ''}`}
              >
                <label>{localizeText(field.label)}</label>
                {field.type === 'textarea' || field.type === 'json' ? (
                  <textarea
                    rows={field.type === 'json' ? 4 : 3}
                    className={field.type === 'json' ? 'mono' : undefined}
                    placeholder={field.placeholder ? localizeText(field.placeholder) : undefined}
                    value={values[field.name] ?? ''}
                    onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))}
                  />
                ) : field.type === 'select' ? (
                  <select
                    value={values[field.name] ?? ''}
                    onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))}
                  >
                    <option value="">{t('module.select.placeholder')}</option>
                    {field.options?.map((option) => (
                      <option key={option.value} value={option.value}>
                        {localizeText(option.label)}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    type={field.type === 'number' ? 'number' : field.type === 'date' ? 'date' : 'text'}
                    placeholder={field.placeholder ? localizeText(field.placeholder) : undefined}
                    value={values[field.name] ?? ''}
                    onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))}
                  />
                )}
              </div>
            ))}
          </div>
        )}

        <div className="actions-row module-actions-row">
          <button type="submit" className="btn-primary" disabled={loading}>
            {loading
              ? t('common.status.processing')
              : kind === 'dataset'
                ? t('common.actions.update')
                : sentenceCase(localizeText((definition as ModuleAction).submitLabel ?? t('common.actions.run')))}
          </button>
          {kind === 'dataset' && (
            <button type="button" className="btn-secondary" onClick={() => void execute(values)} disabled={loading}>
              {t('common.actions.refresh')}
            </button>
          )}
        </div>
      </form>

      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}
      {result !== null && (
        <ResultView data={result} datasetPath={kind === 'dataset' ? definition.path : undefined} />
      )}
    </section>
  );
}

function NotFoundState() {
  const { t, localizeText, localizeUiText, sentenceCase } = useI18n();
  return (
    <PageLayout title={sentenceCase(t('module.notFound.title'))} lead={t('module.notFound.description')}>
      <div className="card">
        <div className="card-header">
          <h2>{sentenceCase(t('module.notFound.available'))}</h2>
        </div>
        <div className="module-link-grid">
          {moduleList.map((module) => (
            <Link key={module.id} to={`/${toCrudResourceSlug(module.id)}`} relative="path" className="module-link-card">
              <div>
                <strong>{localizeUiText(module.title)}</strong>
                <p>{localizeText(module.summary)}</p>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </PageLayout>
  );
}

function ModuleDatasetsAndActionsIndex({ module }: { module: ModuleDefinition }) {
  const { t, localizeText, localizeUiText, sentenceCase } = useI18n();
  const rows: Array<{ key: string; anchor: string; title: string; method: string; path: string }> = [];
  for (const d of module.datasets ?? []) {
    const id = d.id ?? d.path;
    rows.push({
      key: `ds-${id}`,
      anchor: `module-dataset-${id}`,
      title: d.title,
      method: 'GET',
      path: d.path,
    });
  }
  for (const a of module.actions ?? []) {
    rows.push({
      key: `ac-${a.id}`,
      anchor: `module-action-${a.id}`,
      title: a.title,
      method: a.method,
      path: a.path,
    });
  }
  if (rows.length === 0) {
    return null;
  }
  return (
    <div className="card module-ops-index-card">
      <div className="card-header module-card-header-inner">
        <h2>{sentenceCase(t('module.index.title'))}</h2>
      </div>
      <div className="module-ops-index-body">
        <div className="module-ops-badge-row" role="navigation" aria-label={localizeText(t('module.index.title'))}>
          {rows.map((r) => (
            <a key={r.key} href={`#${r.anchor}`} className="badge badge-neutral module-ops-badge-link">
              {localizeUiText(vocab(r.title))}
            </a>
          ))}
        </div>
        <div className="module-ops-index-table-wrap table-wrap">
          <table>
            <thead>
              <tr>
                <th>{sentenceCase(t('module.index.columnOp'))}</th>
                <th>{sentenceCase(t('module.index.columnMethod'))}</th>
                <th>{sentenceCase(t('module.index.columnPath'))}</th>
                <th>{sentenceCase(t('module.index.jump'))}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.key}>
                  <td>{localizeUiText(vocab(r.title))}</td>
                  <td>
                    <span className={`http-method http-method-${r.method.toLowerCase()}`}>{r.method}</span>
                  </td>
                  <td>
                    <code className="endpoint-path">{r.path}</code>
                  </td>
                  <td>
                    <a href={`#${r.anchor}`} className="btn-secondary btn-sm">
                      {sentenceCase(t('module.index.jump'))}
                    </a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function ModuleOverviewCards({ module }: { module: ModuleDefinition }) {
  const { t, localizeText, localizeUiText, sentenceCase } = useI18n();
  const showExplorerChrome = (module.datasets?.length ?? 0) > 0 || (module.actions?.length ?? 0) > 0;
  return (
    <>
      {module.helpIntro && <p className="module-help-intro">{localizeText(module.helpIntro)}</p>}
      {showExplorerChrome && (
        <div className="stats-grid compact-grid">
          {(module.datasets?.length ?? 0) > 0 && (
            <div className="stat-card">
              <div className="stat-label">{sentenceCase(t('module.stats.datasets'))}</div>
              <div className="stat-value">{module.datasets?.length ?? 0}</div>
            </div>
          )}
          <div className="stat-card">
            <div className="stat-label">{sentenceCase(t('module.stats.actions'))}</div>
            <div className="stat-value">{module.actions?.length ?? 0}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">{sentenceCase(t('module.stats.consolePath'))}</div>
            <div className="stat-value stat-value-sm mono">/modules/{module.id}</div>
          </div>
        </div>
      )}

      {module.setupGuide && module.setupGuide.steps.length > 0 && (
        <div className="card module-setup-guide">
          <div className="card-header">
            <h2>
              {module.setupGuide.title
                ? localizeUiText(module.setupGuide.title)
                : sentenceCase(t('module.setupGuide.title'))}
            </h2>
          </div>
          <ol className="module-setup-steps">
            {module.setupGuide.steps.map((step, idx) => (
              <li key={idx}>{localizeText(step)}</li>
            ))}
          </ol>
        </div>
      )}

      {module.notes && module.notes.length > 0 && (
        <div className="card module-notes-card">
          <div className="card-header">
            <h2>{sentenceCase(t('module.notes.title'))}</h2>
          </div>
          <div className="notes-list">
            {module.notes.map((note) => (
              <p key={note}>{localizeText(note)}</p>
            ))}
          </div>
        </div>
      )}
    </>
  );
}

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

function ReportsBusinessPage() {
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
      value: readReportMetric(results.find((entry) => entry.dataset.path.includes('sales-summary'))?.data, 'total_sales'),
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
      value: readReportMetric(results.find((entry) => entry.dataset.path.includes('cashflow-summary'))?.data, 'balance'),
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

function ModuleExplorerPage({ moduleId }: { moduleId: string }) {
  const { t, localizeText, localizeUiText, sentenceCase } = useI18n();
  const module = useMemo(() => moduleCatalog[moduleId], [moduleId]);
  const [runtime, setRuntime] = useState<ModuleRuntimeContext>(() => currentRuntimeContext());
  const [showAllOperations, setShowAllOperations] = useState(false);
  const sessionQuery = useQuery({
    queryKey: queryKeys.session.current,
    queryFn: getSession,
    retry: false,
  });

  useEffect(() => {
    if (sessionQuery.data) {
      setRuntime((current) => ({ ...current, orgId: sessionQuery.data.auth.org_id }));
    }
  }, [sessionQuery.data]);

  const configGroupKeys = module?.explorerConfigGroupKeys;
  const allActionGroups = useMemo(() => (module ? groupedModuleActions(module) : []), [module]);
  const visibleActionGroups = useMemo(() => {
    if (!module || !configGroupKeys?.length || showAllOperations) {
      return allActionGroups;
    }
    const allow = new Set(configGroupKeys);
    return allActionGroups.filter((g) => allow.has(g.key));
  }, [module, allActionGroups, configGroupKeys, showAllOperations]);
  const configFocusActive = Boolean(module && configGroupKeys?.length && !showAllOperations);
  const visibleActionCount = visibleActionGroups.reduce((n, g) => n + g.actions.length, 0);

  if (!module) {
    return <NotFoundState />;
  }

  const showExplorerChrome = (module.datasets?.length ?? 0) > 0 || (module.actions?.length ?? 0) > 0;
  const summaryText = localizeText(vocab(module.summary)).trim();
  const headerActions = showExplorerChrome ? (
    <div className="module-runtime-card">
      <span>{t('module.runtime.activeOrg')}</span>
      <strong>{runtime.orgId || t('module.runtime.resolving')}</strong>
      <small>
        {t('module.runtime.surfaces', { count: (module.datasets?.length ?? 0) + (module.actions?.length ?? 0) })}
      </small>
    </div>
  ) : undefined;

  return (
    <PageLayout
      className="module-page"
      title={localizeUiText(vocab(module.title))}
      lead={summaryText.length > 0 ? summaryText : undefined}
      actions={headerActions}
    >
      <ModuleOverviewCards module={module} />
      <ModuleDatasetsAndActionsIndex module={module} />
      {sessionQuery.error && (
        <div className="alert alert-warning">
          {t('module.bootstrap.error', {
            error: sessionQuery.error instanceof Error ? sessionQuery.error.message : String(sessionQuery.error),
          })}
        </div>
      )}

      {module.datasets && module.datasets.length > 0 && (
        <div className="module-section">
          <div className="section-title-row">
            <h2>{sentenceCase(t('module.sections.reads'))}</h2>
            <span className="badge badge-neutral">{module.datasets.length}</span>
          </div>
          <div className="module-grid">
            {module.datasets.map((dataset) => (
              <EndpointCard key={dataset.id} definition={dataset} runtime={runtime} kind="dataset" />
            ))}
          </div>
        </div>
      )}

      {module.actions && module.actions.length > 0 && (
        <div className="module-section">
          <div className="section-title-row">
            <h2>{sentenceCase(configFocusActive ? t('module.sections.config') : t('module.sections.actions'))}</h2>
            <span className="badge badge-neutral">
              {configFocusActive ? visibleActionCount : module.actions.length}
            </span>
            {configGroupKeys && configGroupKeys.length > 0 && (
              <button
                type="button"
                className="btn btn-secondary btn-sm module-explorer-toggle"
                onClick={() => setShowAllOperations((v) => !v)}
              >
                {showAllOperations ? t('module.explorer.backToConfigOnly') : t('module.explorer.showAllOperations')}
              </button>
            )}
          </div>
          {configFocusActive && (
            <p className="text-muted module-config-focus-hint">{t('module.explorer.configHint')}</p>
          )}
          {(() => {
            const actionGroups = visibleActionGroups;
            const showGroupTitles =
              actionGroups.length > 1 || (actionGroups[0] !== undefined && actionGroups[0].key !== '_ungrouped');
            return actionGroups.map((section) => (
              <div key={section.key} className="module-action-group">
                {showGroupTitles && (
                  <div className="module-action-group-heading">
                    <h3>{section.title}</h3>
                    <span className="badge badge-neutral">{section.actions.length}</span>
                  </div>
                )}
                <div className="module-grid">
                  {section.actions.map((action) => (
                    <EndpointCard key={action.id} definition={action} runtime={runtime} kind="action" />
                  ))}
                </div>
              </div>
            ));
          })()}
        </div>
      )}
    </PageLayout>
  );
}

export function ModulePage() {
  const { moduleId: urlModuleId = '' } = useParams();
  const moduleId = fromCrudResourceSlug(urlModuleId);
  const urlSlug = toCrudResourceSlug(moduleId);
  const tenantSlug = useActiveTenantSlug();
  const crudModuleQuery = useQuery({
    queryKey: queryKeys.modules.isCrud(moduleId),
    queryFn: () => hasLazyCrudResource(moduleId),
  });

  if (moduleId === 'workOrders') {
    return <Navigate to={tenantLink('/work-orders', tenantSlug)} replace />;
  }
  if (moduleId === 'reports') {
    return <ReportsBusinessPage />;
  }

  if (crudModuleQuery.isError) {
    return (
      <PageLayout title="Módulo" lead="No se pudo resolver la configuración del módulo.">
        <div className="alert alert-error">
          {crudModuleQuery.error instanceof Error ? crudModuleQuery.error.message : 'Error al cargar el módulo.'}
        </div>
      </PageLayout>
    );
  }

  if (crudModuleQuery.data == null) {
    return (
      <PageLayout title="Módulo" lead="Cargando configuración y superficies disponibles.">
        <div className="card">
          <p>Cargando modulo…</p>
        </div>
      </PageLayout>
    );
  }
  if (crudModuleQuery.data) {
    const baseRoute = tenantLink(`/${urlSlug}`, tenantSlug);
    return <ConfiguredCrudStandalonePage resourceId={moduleId} baseRoute={baseRoute} />;
  }
  return <ModuleExplorerPage moduleId={moduleId} />;
}
