import { FormEvent, useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { apiRequest, downloadAPIFile, getAdminBootstrap } from '../lib/api';
import {
  moduleCatalog,
  moduleGroups,
  moduleList,
  resolveModuleDefault,
  type ModuleAction,
  type ModuleDataset,
  type ModuleDefinition,
  type ModuleField,
  type ModuleRuntimeContext,
} from '../lib/moduleCatalog';

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
    body[field.name] = field.type === 'number' ? Number(raw) : raw;
  }
  return body;
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

function extractRows(data: unknown): Array<Record<string, unknown>> | null {
  if (Array.isArray(data) && data.every((item) => item && typeof item === 'object')) {
    return data as Array<Record<string, unknown>>;
  }
  if (data && typeof data === 'object' && Array.isArray((data as { items?: unknown[] }).items)) {
    const items = (data as { items: unknown[] }).items;
    if (items.every((item) => item && typeof item === 'object')) {
      return items as Array<Record<string, unknown>>;
    }
  }
  return null;
}

function ResultView({ data }: { data: unknown }) {
  if (data === null || data === undefined) {
    return null;
  }

  const rows = extractRows(data);
  if (rows && rows.length > 0) {
    const columns = Array.from(
      new Set(rows.flatMap((row) => Object.keys(row))),
    ).slice(0, 8);
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
      setError(`Completa: ${missing.join(', ')}`);
      return;
    }

    const path = buildPath(definition.path, fields, currentValues, runtime);
    const action = definition as ModuleAction;
    const isAction = kind === 'action';
    const body = buildBody(fields, currentValues);
    const hasBody = Object.keys(body).length > 0;

    setLoading(true);
    setError('');
    setSuccess('');
    try {
      if (isAction && action.response === 'download') {
        const filename = await downloadAPIFile(path, { method: action.method });
        setSuccess(`Archivo descargado: ${filename}`);
        setResult({ filename, path });
      } else {
        const payload = await apiRequest(path, {
          method: isAction ? action.method : 'GET',
          body: hasBody ? body : action.sendEmptyBody ? {} : undefined,
        });
        setResult(payload);
        setSuccess(isAction && action.method !== 'GET' ? 'Operación completada.' : 'Datos actualizados.');
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

  return (
    <section className="card module-card">
      <div className="card-header">
        <div>
          <h2>{definition.title}</h2>
          <p className="text-secondary">{definition.description}</p>
        </div>
        <span className="badge badge-neutral mono">{definition.path}</span>
      </div>

      <form onSubmit={onSubmit} className="module-form-stack">
        {fields.length > 0 && (
          <div className="module-fields-grid">
            {fields.map((field) => (
              <div key={field.name} className={`form-group ${field.type === 'textarea' ? 'full-width' : ''}`}>
                <label>{field.label}</label>
                {field.type === 'textarea' ? (
                  <textarea
                    rows={3}
                    placeholder={field.placeholder}
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
                    <option value="">Seleccionar...</option>
                    {field.options?.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    type={field.type === 'number' ? 'number' : field.type === 'date' ? 'date' : 'text'}
                    placeholder={field.placeholder}
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
            {loading ? 'Procesando...' : kind === 'dataset' ? 'Actualizar' : ((definition as ModuleAction).submitLabel ?? 'Ejecutar')}
          </button>
          {kind === 'dataset' && (
            <button type="button" className="btn-secondary" onClick={() => void execute(values)} disabled={loading}>
              Refrescar
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
  return (
    <>
      <div className="page-header">
        <h1>Módulo no encontrado</h1>
        <p>La ruta solicitada no coincide con ningún módulo registrado en el frontend.</p>
      </div>
      <div className="card">
        <div className="card-header">
          <h2>Módulos disponibles</h2>
        </div>
        <div className="module-link-grid">
          {moduleList.map((module) => (
            <Link key={module.id} to={`/modules/${module.id}`} className="module-link-card">
              <span className="sidebar-token">{module.icon}</span>
              <div>
                <strong>{module.title}</strong>
                <p>{module.summary}</p>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </>
  );
}

function ModuleHeader({ module, runtime }: { module: ModuleDefinition; runtime: ModuleRuntimeContext }) {
  const groupLabel =
    moduleGroups.find((group) => group.id === module.group)?.label ?? module.group;
  return (
    <>
      <div className="page-header module-page-header">
        <div>
          <div className="module-kicker">
            <span className="sidebar-token">{module.icon}</span>
            <span className="badge badge-neutral">{module.badge ?? groupLabel}</span>
          </div>
          <h1>{module.title}</h1>
          <p>{module.summary}</p>
        </div>
        <div className="module-runtime-card">
          <span>Org activa</span>
          <strong>{runtime.orgId || 'resolviendo...'}</strong>
          <small>{(module.datasets?.length ?? 0) + (module.actions?.length ?? 0)} superficies conectadas</small>
        </div>
      </div>

      <div className="stats-grid compact-grid">
        <div className="stat-card">
          <div className="stat-label">Datasets</div>
          <div className="stat-value">{module.datasets?.length ?? 0}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Acciones</div>
          <div className="stat-value">{module.actions?.length ?? 0}</div>
        </div>
        <div className="stat-card">
          <div className="stat-label">Ruta base</div>
          <div className="stat-value stat-value-sm mono">/modules/{module.id}</div>
        </div>
      </div>

      {module.notes && module.notes.length > 0 && (
        <div className="card module-notes-card">
          <div className="card-header">
            <h2>Notas operativas</h2>
          </div>
          <div className="notes-list">
            {module.notes.map((note) => (
              <p key={note}>{note}</p>
            ))}
          </div>
        </div>
      )}
    </>
  );
}

export function ModulePage() {
  const { moduleId = '' } = useParams();
  const module = useMemo(() => moduleCatalog[moduleId], [moduleId]);
  const [runtime, setRuntime] = useState<ModuleRuntimeContext>(() => currentRuntimeContext());
  const [bootstrapError, setBootstrapError] = useState('');

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const bootstrap = await getAdminBootstrap();
        if (!cancelled) {
          setRuntime((current) => ({ ...current, orgId: bootstrap.settings.org_id }));
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

  if (!module) {
    return <NotFoundState />;
  }

  return (
    <>
      <ModuleHeader module={module} runtime={runtime} />
      {bootstrapError && <div className="alert alert-warning">No se pudo resolver bootstrap: {bootstrapError}</div>}

      {module.datasets && module.datasets.length > 0 && (
        <div className="module-section">
          <div className="section-title-row">
            <h2>Lecturas</h2>
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
            <h2>Acciones</h2>
            <span className="badge badge-neutral">{module.actions.length}</span>
          </div>
          <div className="module-grid">
            {module.actions.map((action) => (
              <EndpointCard key={action.id} definition={action} runtime={runtime} kind="action" />
            ))}
          </div>
        </div>
      )}
    </>
  );
}
