import { FormEvent, useEffect, useState } from 'react';
import { ReportsResultView } from '../components/ReportsResultView';
import { apiRequest, downloadAPIFile } from '../lib/api';
import { useI18n } from '../lib/i18n';
import { isReportDatasetPath } from '../lib/reportsResultPresentation';
import { vocab } from '../lib/vocabulary';
import type { ModuleAction, ModuleDataset, ModuleRuntimeContext } from '../lib/moduleCatalog';
import {
  buildBody,
  buildInitialValues,
  buildPath,
  extractRows,
  missingRequiredFields,
  stringifyValue,
  tableColumnsForRows,
} from './modulePageUtils';

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

export function EndpointCard({ definition, runtime, kind }: EndpointCardProps) {
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
      {result !== null && <ResultView data={result} datasetPath={kind === 'dataset' ? definition.path : undefined} />}
    </section>
  );
}
