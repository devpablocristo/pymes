import { FormEvent, useEffect, useMemo, useState } from 'react';
import { Link, Navigate, useParams } from 'react-router-dom';
import { apiRequest, downloadAPIFile, getSession } from '../lib/api';
import { LazyConfiguredCrudPage, hasLazyCrudResource } from '../crud/lazyCrudPage';
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
import { vocab } from '../lib/vocabulary';

function currentRuntimeContext(): ModuleRuntimeContext {
  const now = new Date();
  const today = now.toISOString().slice(0, 10);
  const monthStart = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1))
    .toISOString()
    .slice(0, 10);
  return { orgId: '', today, monthStart };
}

function buildInitialValues(fields: ModuleField[] | undefined, ctx: ModuleRuntimeContext): Record<string, string> {
  return Object.fromEntries(
    (fields ?? []).map((field) => [field.name, resolveModuleDefault(field.defaultValue, ctx)]),
  );
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

function groupedModuleActions(module: ModuleDefinition): Array<{ key: string; title: string; actions: ModuleAction[] }> {
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
    return value
      .map((item) => (typeof item === 'object' ? JSON.stringify(item) : String(item)))
      .join(', ');
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
  return null;
}

function ResultView({ data }: { data: unknown }) {
  const { t } = useI18n();

  if (data === null || data === undefined) {
    return null;
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
            {rows.slice(0, 12).map((row, index) => (
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
  }, [defId, kind, runtimeKey]);

  function onSubmit(event: FormEvent): void {
    event.preventDefault();
    void execute(values);
  }

  const httpMethod = kind === 'dataset' ? 'GET' : (definition as ModuleAction).method;

  return (
    <section className="card module-card">
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
                    onChange={(event) =>
                      setValues((current) => ({ ...current, [field.name]: event.target.value }))
                    }
                  />
                ) : field.type === 'select' ? (
                  <select
                    value={values[field.name] ?? ''}
                    onChange={(event) =>
                      setValues((current) => ({ ...current, [field.name]: event.target.value }))
                    }
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
                    onChange={(event) =>
                      setValues((current) => ({ ...current, [field.name]: event.target.value }))
                    }
                  />
                )}
              </div>
            ))}
          </div>
        )}

        <div className="actions-row module-actions-row">
          <button type="submit" className="btn-primary" disabled={loading}>
            {loading ? t('common.status.processing') : kind === 'dataset' ? t('common.actions.update') : sentenceCase(localizeText((definition as ModuleAction).submitLabel ?? t('common.actions.run')))}
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
      {result !== null && <ResultView data={result} />}
    </section>
  );
}

function NotFoundState() {
  const { t, localizeText, localizeUiText, sentenceCase } = useI18n();
  return (
    <>
      <div className="page-header">
        <h1>{sentenceCase(t('module.notFound.title'))}</h1>
        <p>{t('module.notFound.description')}</p>
      </div>
      <div className="card">
        <div className="card-header">
          <h2>{sentenceCase(t('module.notFound.available'))}</h2>
        </div>
        <div className="module-link-grid">
          {moduleList.map((module) => (
            <Link key={module.id} to={`/modules/${module.id}`} className="module-link-card">
              <div>
                <strong>{localizeUiText(module.title)}</strong>
                <p>{localizeText(module.summary)}</p>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </>
  );
}

function ModuleHeader({ module, runtime }: { module: ModuleDefinition; runtime: ModuleRuntimeContext }) {
  const { t, localizeText, localizeUiText, sentenceCase } = useI18n();
  const showExplorerChrome =
    (module.datasets?.length ?? 0) > 0 || (module.actions?.length ?? 0) > 0;
  const summaryText = localizeText(vocab(module.summary)).trim();
  return (
    <>
      <div className="page-header module-page-header">
        <div>
          <h1>{localizeUiText(vocab(module.title))}</h1>
          {summaryText.length > 0 && <p>{summaryText}</p>}
          {module.helpIntro && <p className="module-help-intro">{localizeText(module.helpIntro)}</p>}
        </div>
        {showExplorerChrome && (
          <div className="module-runtime-card">
            <span>{t('module.runtime.activeOrg')}</span>
            <strong>{runtime.orgId || t('module.runtime.resolving')}</strong>
            <small>{t('module.runtime.surfaces', { count: (module.datasets?.length ?? 0) + (module.actions?.length ?? 0) })}</small>
          </div>
        )}
      </div>

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

function ModuleExplorerPage({ moduleId }: { moduleId: string }) {
  const { t, sentenceCase } = useI18n();
  const module = useMemo(() => moduleCatalog[moduleId], [moduleId]);
  const [runtime, setRuntime] = useState<ModuleRuntimeContext>(() => currentRuntimeContext());
  const [bootstrapError, setBootstrapError] = useState('');
  const [showAllOperations, setShowAllOperations] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const session = await getSession();
        if (!cancelled) {
          setRuntime((current) => ({ ...current, orgId: session.auth.org_id }));
          setBootstrapError('');
        }
      } catch (err) {
        if (!cancelled) {
          setBootstrapError(String(err));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const configGroupKeys = module?.explorerConfigGroupKeys;
  const allActionGroups = useMemo(
    () => (module ? groupedModuleActions(module) : []),
    [module],
  );
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

  return (
    <>
      <ModuleHeader module={module} runtime={runtime} />
      {bootstrapError && <div className="alert alert-warning">{t('module.bootstrap.error', { error: bootstrapError })}</div>}

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
            <h2>
              {sentenceCase(
                configFocusActive ? t('module.sections.config') : t('module.sections.actions'),
              )}
            </h2>
            <span className="badge badge-neutral">
              {configFocusActive ? visibleActionCount : module.actions.length}
            </span>
            {configGroupKeys && configGroupKeys.length > 0 && (
              <button
                type="button"
                className="btn btn-secondary btn-sm module-explorer-toggle"
                onClick={() => setShowAllOperations((v) => !v)}
              >
                {showAllOperations
                  ? t('module.explorer.backToConfigOnly')
                  : t('module.explorer.showAllOperations')}
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
    </>
  );
}

export function ModulePage() {
  const { moduleId = '' } = useParams();
  const [isCrudModule, setIsCrudModule] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    void hasLazyCrudResource(moduleId).then((result) => {
      if (!cancelled) {
        setIsCrudModule(result);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [moduleId]);

  if (moduleId === 'workOrders') {
    return <Navigate to="/modules/workOrders" replace />;
  }

  if (isCrudModule == null) {
    return <div className="card"><p>Cargando modulo…</p></div>;
  }
  if (isCrudModule) {
    return <LazyConfiguredCrudPage resourceId={moduleId} />;
  }
  return <ModuleExplorerPage moduleId={moduleId} />;
}
