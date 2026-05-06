import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Navigate, useParams } from 'react-router-dom';
import { fromCrudResourceSlug, toCrudResourceSlug } from '../crud/crudResourceSlug';
import { tenantLink, useActiveTenantSlug } from '../lib/tenantSlug';
import { getTenantProfile } from '../lib/tenantProfile';
import { PageLayout } from '../components/PageLayout';
import { getSession } from '../lib/api';
import { hasLazyCrudResource } from '../crud/lazyCrudPage';
import { ConfiguredCrudStandalonePage } from '../crud/configuredCrudViews';
import { moduleCatalog, type ModuleRuntimeContext } from '../lib/moduleCatalog';
import { useI18n } from '../lib/i18n';
import { queryKeys } from '../lib/queryKeys';
import { vocab } from '../lib/vocabulary';
import { currentRuntimeContext, groupedModuleActions } from './modulePageUtils';
import { EndpointCard } from './ModuleEndpointCards';
import { ModuleDatasetsAndActionsIndex, ModuleOverviewCards, NotFoundState } from './ModuleExplorerSections';
import { ReportsBusinessPage } from './ModuleReportsPage';

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
    const profile = getTenantProfile();
    const workOrdersSlug = toCrudResourceSlug(
      profile?.subVertical === 'bike_shop' ? 'bikeWorkOrders' : 'carWorkOrders',
    );
    return <Navigate to={tenantLink(`/${workOrdersSlug}`, tenantSlug)} replace />;
  }
  if (moduleId === 'reports') {
    return <ReportsBusinessPage />;
  }

  if (crudModuleQuery.isError) {
    return (
      <PageLayout title="Módulo" lead="No se pudo resolver la configuración del módulo.">
        <div className="alert alert-error">
          <p>No se pudo resolver la configuración del módulo.</p>
          <p>{crudModuleQuery.error instanceof Error ? crudModuleQuery.error.message : 'Error al cargar el módulo.'}</p>
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
