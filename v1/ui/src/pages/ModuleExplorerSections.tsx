import { Link } from 'react-router-dom';
import { PageLayout } from '../components/PageLayout';
import { toCrudResourceSlug } from '../crud/crudResourceSlug';
import { moduleList, type ModuleDefinition } from '../lib/moduleCatalog';
import { useI18n } from '../lib/i18n';
import { vocab } from '../lib/vocabulary';

export function NotFoundState() {
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

export function ModuleDatasetsAndActionsIndex({ module }: { module: ModuleDefinition }) {
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

export function ModuleOverviewCards({ module }: { module: ModuleDefinition }) {
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
