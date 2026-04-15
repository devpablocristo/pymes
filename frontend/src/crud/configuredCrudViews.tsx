import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import type { CrudPageConfig, CrudViewModeConfig, CrudViewModeId } from '../components/CrudPage';
import { PageLayout } from '../components/PageLayout';
import { CrudModuleSection } from '../modules/crud';
import { applyCrudUiOverride, CRUD_UI_CHANGE_EVENT, CRUD_UI_STORAGE_KEY } from '../lib/crudUiConfig';
import { Navigate } from 'react-router-dom';
import { loadLazyCrudPageConfig } from './lazyCrudPage';
import { PymesSimpleCrudListModeContent } from './PymesSimpleCrudListModeContent';
import { crudModuleCatalog } from './crudModuleCatalog';

function fallbackViewModes(resourceId: string): CrudViewModeConfig[] {
  return [{ id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vista lista', isDefault: true }];
}

function NoEnabledViews({ resourceId }: { resourceId: string }) {
  return (
    <PageLayout title="Módulo" lead="No hay vistas activas para este recurso.">
      <div className="empty-state">
        <p>{resourceId} no tiene vistas habilitadas en la configuración actual.</p>
      </div>
    </PageLayout>
  );
}

function resolveViewModes<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T> | null,
): CrudViewModeConfig[] {
  const resolved = config ? applyCrudUiOverride(resourceId, config) : config;
  const modes =
    resolved == null
      ? fallbackViewModes(resourceId)
      : resolved.viewModes
        ? resolved.viewModes
        : fallbackViewModes(resourceId);
  return [...modes].sort((a, b) => Number(Boolean(b.isDefault)) - Number(Boolean(a.isDefault)));
}

function useCrudUiConfigVersion() {
  const [version, setVersion] = useState(0);

  useEffect(() => {
    function refreshOnCrudUiConfigChange() {
      setVersion((current) => current + 1);
    }

    function refreshOnStorage(event: StorageEvent) {
      if (event.key == null || event.key === CRUD_UI_STORAGE_KEY) {
        setVersion((current) => current + 1);
      }
    }

    window.addEventListener(CRUD_UI_CHANGE_EVENT, refreshOnCrudUiConfigChange);
    window.addEventListener('storage', refreshOnStorage);
    return () => {
      window.removeEventListener(CRUD_UI_CHANGE_EVENT, refreshOnCrudUiConfigChange);
      window.removeEventListener('storage', refreshOnStorage);
    };
  }, []);

  return version;
}

function useCrudConfig(resourceId: string) {
  const [config, setConfig] = useState<CrudPageConfig<{ id: string }> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    setLoading(true);
    void loadLazyCrudPageConfig(resourceId)
      .then((nextConfig) => {
        if (!cancelled) {
          setConfig(nextConfig);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [resourceId]);

  return { config, error, loading };
}

function resolveCrudConfig<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T> | null,
): CrudPageConfig<T> | null {
  return config ? applyCrudUiOverride(resourceId, config) : config;
}

function modeActionLink<T extends { id: string }>(resourceId: string, config: CrudPageConfig<T> | null) {
  const title = crudModuleCatalog[resourceId]?.title?.trim() || resourceId;
  return {
    to: `/modules/${resourceId}/configure`,
    label: 'Configurar',
    hideWhenActivePattern: `/modules/${resourceId}/configure`,
    activeReplacement: {
      to: `/modules/${resourceId}`,
      label: `Volver a ${title.toLowerCase()}`,
    },
  };
}

export function ConfiguredCrudSection({
  resourceId,
  baseRoute,
  contextPatternByModeId,
  actionLink,
  includeCanonicalMissing = false,
}: {
  resourceId: string;
  baseRoute: string;
  contextPatternByModeId?: Partial<Record<CrudViewModeId, string>>;
  actionLink?: {
    to: string;
    label: string;
    hideWhenActivePattern?: string;
    activeReplacement?: {
      to: string;
      label: string;
    };
  };
  includeCanonicalMissing?: boolean;
}) {
  const { config, loading } = useCrudConfig(resourceId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const resolvedConfig = useMemo(() => resolveCrudConfig(resourceId, config), [config, resourceId, uiConfigVersion]);
  const viewModes = useMemo(
    () => resolveViewModes(resourceId, config),
    [config, includeCanonicalMissing, resourceId, uiConfigVersion],
  );

  if (loading && config == null) {
    return (
      <CrudModuleSection
        modes={[{ path: `${baseRoute}/list`, label: '...' }]}
        groupAriaLabel="Cargando vistas"
        actionLink={actionLink ?? modeActionLink(resourceId, resolvedConfig)}
      />
    );
  }

  if (viewModes.length === 0) {
    return <NoEnabledViews resourceId={resourceId} />;
  }

  return (
    <CrudModuleSection
      modes={viewModes.map((mode) => ({
        path: `${baseRoute}/${mode.path}`,
        label: mode.label,
        contextPattern: contextPatternByModeId?.[mode.id],
      }))}
      groupAriaLabel={viewModes[0]?.ariaLabel ?? 'Cambiar vista'}
      actionLink={actionLink ?? modeActionLink(resourceId, resolvedConfig)}
    />
  );
}

export function ConfiguredCrudModePage({
  resourceId,
  modeId,
  mergeConfig,
  allowGenericModeFallback = false,
}: {
  resourceId: string;
  modeId: CrudViewModeId;
  mergeConfig?: Record<string, unknown>;
  allowGenericModeFallback?: boolean;
}) {
  const { config, error, loading } = useCrudConfig(resourceId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const viewModes = useMemo(
    () => resolveViewModes(resourceId, config),
    [allowGenericModeFallback, config, resourceId, uiConfigVersion],
  );
  const activeMode = viewModes.find((mode) => mode.id === modeId) ?? null;

  if (error) {
    return (
      <PageLayout title="Módulo" lead="No se pudo cargar la configuración de vistas.">
        <div className="alert alert-error">{error}</div>
      </PageLayout>
    );
  }

  if (loading && config == null) {
    return (
      <PageLayout title="Módulo" lead="Cargando vista configurada.">
        <div className="card">
          <p>Cargando módulo…</p>
        </div>
      </PageLayout>
    );
  }

  if (viewModes.length === 0) {
    return <NoEnabledViews resourceId={resourceId} />;
  }

  if (!activeMode) {
    return (
      <PageLayout title="Módulo" lead="La vista pedida no está habilitada para este recurso.">
        <div className="empty-state">
          <p>{resourceId} no expone el modo {modeId}.</p>
        </div>
      </PageLayout>
    );
  }

  const custom = activeMode.render?.();
  if (custom) {
    return custom;
  }

  if (modeId === 'list') {
    return <PymesSimpleCrudListModeContent resourceId={resourceId} />;
  }

  if (allowGenericModeFallback) {
    return <PymesSimpleCrudListModeContent resourceId={resourceId} mode={modeId} />;
  }

  return (
    <PageLayout title="Módulo" lead="No existe render para la vista configurada.">
      <div className="empty-state">
        <p>
          El recurso {resourceId} no define <code>viewModes[].render</code> para el modo {modeId}.
        </p>
      </div>
    </PageLayout>
  );
}

export function ConfiguredCrudIndexRedirect({
  resourceId,
  baseRoute,
}: {
  resourceId: string;
  baseRoute: string;
}) {
  const { config, loading } = useCrudConfig(resourceId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const viewModes = useMemo(() => resolveViewModes(resourceId, config), [config, resourceId, uiConfigVersion]);
  const target = viewModes[0]?.path || 'list';

  if (loading && config == null) {
    return (
      <PageLayout title="Módulo" lead="Cargando vista inicial.">
        <div className="card">
          <p>Cargando módulo…</p>
        </div>
      </PageLayout>
    );
  }

  if (viewModes.length === 0) {
    return <NoEnabledViews resourceId={resourceId} />;
  }

  return <Navigate to={`${baseRoute}/${target}`} replace />;
}

export function ConfiguredCrudStandalonePage({
  resourceId,
  baseRoute,
}: {
  resourceId: string;
  baseRoute: string;
}) {
  const { config, error, loading } = useCrudConfig(resourceId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const resolvedConfig = useMemo(() => resolveCrudConfig(resourceId, config), [config, resourceId, uiConfigVersion]);
  const viewModes = useMemo(
    () => resolveViewModes(resourceId, config),
    [config, resourceId, uiConfigVersion],
  );
  const activeMode = viewModes[0]?.id ?? 'list';

  if (error) {
    return (
      <PageLayout title="Módulo" lead="No se pudo cargar la configuración de vistas.">
        <div className="alert alert-error">{error}</div>
      </PageLayout>
    );
  }

  if (loading && config == null) {
    return (
      <PageLayout title="Módulo" lead="Cargando vista configurada.">
        <div className="card">
          <p>Cargando módulo…</p>
        </div>
      </PageLayout>
    );
  }

  if (viewModes.length === 0) {
    return <NoEnabledViews resourceId={resourceId} />;
  }

  return (
    <CrudModuleSection
      modes={viewModes.map((mode) => ({
        path: `${baseRoute}/${mode.path}`,
        label: mode.label,
      }))}
      groupAriaLabel={viewModes[0]?.ariaLabel ?? 'Cambiar vista'}
      actionLink={modeActionLink(resourceId, resolvedConfig)}
    >
      <ConfiguredCrudModePage resourceId={resourceId} modeId={activeMode} allowGenericModeFallback />
    </CrudModuleSection>
  );
}

export function ConfiguredCrudRouteModePage() {
  const { moduleId = '', modePath = '' } = useParams();
  const { config, error, loading } = useCrudConfig(moduleId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const resolvedConfig = useMemo(() => resolveCrudConfig(moduleId, config), [config, moduleId, uiConfigVersion]);
  const viewModes = useMemo(
    () => resolveViewModes(moduleId, config),
    [config, moduleId, uiConfigVersion],
  );
  const mode = viewModes.find((entry) => entry.path === modePath) ?? null;

  if (error) {
    return (
      <PageLayout title="Módulo" lead="No se pudo cargar la configuración de vistas.">
        <div className="alert alert-error">{error}</div>
      </PageLayout>
    );
  }

  if (loading && config == null) {
    return (
      <PageLayout title="Módulo" lead="Cargando vista configurada.">
        <div className="card">
          <p>Cargando módulo…</p>
        </div>
      </PageLayout>
    );
  }

  if (viewModes.length === 0) {
    return <NoEnabledViews resourceId={moduleId} />;
  }

  if (!mode) {
    return (
      <PageLayout title="Módulo" lead="La vista pedida no está habilitada para este recurso.">
        <div className="empty-state">
          <p>{moduleId} no expone la ruta {modePath}.</p>
        </div>
      </PageLayout>
    );
  }

  return (
    <CrudModuleSection
      modes={viewModes.map((entry) => ({
        path: `/modules/${moduleId}/${entry.path}`,
        label: entry.label,
      }))}
      groupAriaLabel={viewModes[0]?.ariaLabel ?? 'Cambiar vista'}
      actionLink={modeActionLink(moduleId, resolvedConfig)}
    >
      <ConfiguredCrudModePage resourceId={moduleId} modeId={mode.id} allowGenericModeFallback />
    </CrudModuleSection>
  );
}

export function ConfiguredCrudNestedRouteModePage({ resourceId, baseRoute }: { resourceId: string; baseRoute: string }) {
  const { modePath = '' } = useParams();
  const { config, error, loading } = useCrudConfig(resourceId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const resolvedConfig = useMemo(() => resolveCrudConfig(resourceId, config), [config, resourceId, uiConfigVersion]);
  const viewModes = useMemo(
    () => resolveViewModes(resourceId, config),
    [config, resourceId, uiConfigVersion],
  );
  const mode = viewModes.find((entry) => entry.path === modePath) ?? null;

  if (error) {
    return (
      <PageLayout title="Módulo" lead="No se pudo cargar la configuración de vistas.">
        <div className="alert alert-error">{error}</div>
      </PageLayout>
    );
  }

  if (loading && config == null) {
    return (
      <PageLayout title="Módulo" lead="Cargando vista configurada.">
        <div className="card">
          <p>Cargando módulo…</p>
        </div>
      </PageLayout>
    );
  }

  if (viewModes.length === 0) {
    return <NoEnabledViews resourceId={resourceId} />;
  }

  if (!mode) {
    return (
      <PageLayout title="Módulo" lead="La vista pedida no está habilitada para este recurso.">
        <div className="empty-state">
          <p>{resourceId} no expone la ruta {modePath}.</p>
        </div>
      </PageLayout>
    );
  }

  return (
    <CrudModuleSection
      modes={viewModes.map((entry) => ({
        path: `${baseRoute}/${entry.path}`,
        label: entry.label,
      }))}
      groupAriaLabel={viewModes[0]?.ariaLabel ?? 'Cambiar vista'}
      actionLink={modeActionLink(resourceId, resolvedConfig)}
    >
      <ConfiguredCrudModePage resourceId={resourceId} modeId={mode.id} allowGenericModeFallback />
    </CrudModuleSection>
  );
}
