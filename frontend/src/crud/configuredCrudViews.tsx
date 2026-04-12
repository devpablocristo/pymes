import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import type { CrudPageConfig, CrudViewModeConfig, CrudViewModeId } from '../components/CrudPage';
import { PageLayout } from '../components/PageLayout';
import { CrudModuleSection } from '../modules/crud';
import { applyCrudUiOverride, CRUD_UI_CHANGE_EVENT, CRUD_UI_STORAGE_KEY } from '../lib/crudUiConfig';
import { Navigate } from 'react-router-dom';
import { loadLazyCrudPageConfig, LazyConfiguredCrudPage } from './lazyCrudPage';
import { PymesSimpleCrudListModeContent } from './PymesSimpleCrudListModeContent';
import { crudModuleCatalog } from './crudModuleCatalog';

const CANONICAL_VIEW_MODES: CrudViewModeConfig[] = [
  { id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Cambiar vista', isDefault: true },
  { id: 'gallery', label: 'Galería', path: 'gallery', ariaLabel: 'Cambiar vista' },
  { id: 'kanban', label: 'Tablero', path: 'board', ariaLabel: 'Cambiar vista' },
];

function fallbackViewModes(resourceId: string): CrudViewModeConfig[] {
  return [{ id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vista lista', isDefault: true }];
}

function mergeCanonicalViewModes<T extends { id: string }>(config: CrudPageConfig<T> | null): CrudViewModeConfig[] {
  const declared = config?.viewModes ?? [];
  const merged = CANONICAL_VIEW_MODES.map((canonical) => declared.find((mode) => mode.id === canonical.id) ?? canonical);
  return merged;
}

function resolveViewModes<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T> | null,
  options?: { includeCanonicalMissing?: boolean },
): CrudViewModeConfig[] {
  const configWithModes =
    config && options?.includeCanonicalMissing
      ? { ...config, viewModes: mergeCanonicalViewModes(config) }
      : config;
  const resolved = configWithModes ? applyCrudUiOverride(resourceId, configWithModes) : configWithModes;
  const modes = resolved?.viewModes?.length ? resolved.viewModes : fallbackViewModes(resourceId);
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

function modeActionLink(resourceId: string) {
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
  const viewModes = useMemo(
    () => resolveViewModes(resourceId, config, { includeCanonicalMissing }),
    [config, includeCanonicalMissing, resourceId, uiConfigVersion],
  );

  if (loading && config == null) {
    return (
      <CrudModuleSection
        modes={[{ path: `${baseRoute}/list`, label: '...' }]}
        groupAriaLabel="Cargando vistas"
        actionLink={actionLink}
      />
    );
  }

  return (
    <CrudModuleSection
      modes={viewModes.map((mode) => ({
        path: `${baseRoute}/${mode.path}`,
        label: mode.label,
        contextPattern: contextPatternByModeId?.[mode.id],
      }))}
      groupAriaLabel={viewModes[0]?.ariaLabel ?? 'Cambiar vista'}
      actionLink={actionLink}
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
    () => resolveViewModes(resourceId, config, { includeCanonicalMissing: allowGenericModeFallback }),
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
    return <LazyConfiguredCrudPage resourceId={resourceId} mergeConfig={mergeConfig} />;
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
  const viewModes = useMemo(
    () => resolveViewModes(resourceId, config, { includeCanonicalMissing: true }),
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

  return (
    <CrudModuleSection
      modes={viewModes.map((mode) => ({
        path: `${baseRoute}/${mode.path}`,
        label: mode.label,
      }))}
      groupAriaLabel={viewModes[0]?.ariaLabel ?? 'Cambiar vista'}
      actionLink={modeActionLink(resourceId)}
    >
      <ConfiguredCrudModePage resourceId={resourceId} modeId={activeMode} allowGenericModeFallback />
    </CrudModuleSection>
  );
}

export function ConfiguredCrudRouteModePage() {
  const { moduleId = '', modePath = '' } = useParams();
  const { config, error, loading } = useCrudConfig(moduleId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const viewModes = useMemo(
    () => resolveViewModes(moduleId, config, { includeCanonicalMissing: true }),
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
      actionLink={modeActionLink(moduleId)}
    >
      <ConfiguredCrudModePage resourceId={moduleId} modeId={mode.id} allowGenericModeFallback />
    </CrudModuleSection>
  );
}

export function ConfiguredCrudNestedRouteModePage({ resourceId, baseRoute }: { resourceId: string; baseRoute: string }) {
  const { modePath = '' } = useParams();
  const { config, error, loading } = useCrudConfig(resourceId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const viewModes = useMemo(
    () => resolveViewModes(resourceId, config, { includeCanonicalMissing: true }),
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
      actionLink={modeActionLink(resourceId)}
    >
      <ConfiguredCrudModePage resourceId={resourceId} modeId={mode.id} allowGenericModeFallback />
    </CrudModuleSection>
  );
}
