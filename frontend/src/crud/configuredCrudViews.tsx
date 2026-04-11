import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import type { CrudPageConfig, CrudViewModeConfig, CrudViewModeId } from '../components/CrudPage';
import { PageLayout } from '../components/PageLayout';
import { CrudExplorerPage, CrudModuleSection } from '../modules/crud';
import { apiRequest } from '../lib/api';
import { applyCrudUiOverride, CRUD_UI_CHANGE_EVENT, CRUD_UI_STORAGE_KEY } from '../lib/crudUiConfig';
import { Navigate } from 'react-router-dom';
import { loadLazyCrudPageConfig, LazyConfiguredCrudPage } from './lazyCrudPage';

function fallbackViewModes(resourceId: string): CrudViewModeConfig[] {
  return [{ id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vista lista', isDefault: true }];
}

function resolveViewModes<T extends { id: string }>(resourceId: string, config: CrudPageConfig<T> | null): CrudViewModeConfig[] {
  const resolved = config ? applyCrudUiOverride(resourceId, config) : config;
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

export function ConfiguredCrudSection({
  resourceId,
  baseRoute,
  contextPatternByModeId,
  actionLink,
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
}) {
  const { config, loading } = useCrudConfig(resourceId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const viewModes = useMemo(() => resolveViewModes(resourceId, config), [config, resourceId, uiConfigVersion]);

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
}: {
  resourceId: string;
  modeId: CrudViewModeId;
  mergeConfig?: Record<string, unknown>;
}) {
  const [searchParams] = useSearchParams();
  const explorerSelectedId = searchParams.get('selected')?.trim() || undefined;
  const { config, error, loading } = useCrudConfig(resourceId);
  const uiConfigVersion = useCrudUiConfigVersion();
  const viewModes = useMemo(() => resolveViewModes(resourceId, config), [config, resourceId, uiConfigVersion]);
  const activeMode = viewModes.find((mode) => mode.id === modeId) ?? null;

  if (error) {
    return (
      <PageLayout title="Módulo" lead="No se pudo cargar la configuración de vistas.">
        <div className="alert alert-error">{error}</div>
      </PageLayout>
    );
  }

  if (modeId === 'list') {
    return <LazyConfiguredCrudPage resourceId={resourceId} mergeConfig={mergeConfig} />;
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

  if (modeId === 'table-detail' && config?.explorerDetail) {
    return (
      <ConfiguredCrudTableDetailPage
        resourceId={resourceId}
        config={config}
        initialSelectedId={explorerSelectedId}
      />
    );
  }

  const custom = activeMode.render?.();
  if (custom) {
    return custom;
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

function ConfiguredCrudTableDetailPage<T extends { id: string }>({
  resourceId,
  config,
  initialSelectedId,
}: {
  resourceId: string;
  config: CrudPageConfig<T>;
  initialSelectedId?: string;
}) {
  const [items, setItems] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reload = async () => {
    setLoading(true);
    setError(null);
    try {
      if (config.dataSource?.list) {
        setItems(await config.dataSource.list({ archived: false }));
      } else if (config.basePath) {
        const data = await apiRequest<{ items?: T[] | null }>(`${config.basePath}?limit=500`);
        setItems(data.items ?? []);
      } else {
        setItems([]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setItems([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void reload();
  }, [resourceId]);

  const detail = config.explorerDetail;
  const metrics = (detail?.metrics ?? []).map((metric) => ({
    id: metric.id,
    label: metric.label,
    value: metric.value(items),
    tone: metric.tone,
    helper: typeof metric.helper === 'function' ? metric.helper(items) : metric.helper,
  }));

  return (
    <CrudExplorerPage<T>
      key={initialSelectedId ?? '__stock_explorer__'}
      title={config.labelPluralCap}
      singularLabel={config.label}
      pluralLabel={config.labelPlural}
      items={items}
      loading={loading}
      error={error ? <div className="alert alert-error">{error}</div> : undefined}
      searchText={config.searchText}
      searchPlaceholder={config.searchPlaceholder ?? 'Buscar...'}
      emptyState={config.emptyState ?? `No hay ${config.labelPlural} registrados.`}
      metrics={metrics}
      filters={detail?.filters}
      columns={(config.columns ?? []).map((column) => ({
        id: column.key,
        header: column.header,
        className: column.className,
        render: (row: T) => (column.render ? column.render(row[column.key], row) : String(row[column.key] ?? '') || '---'),
      }))}
      rowActions={(config.rowActions ?? []).map((action) => ({
        id: action.id,
        label: action.label,
        kind: action.kind,
        isVisible: (row: T) => action.isVisible?.(row, { archived: false }) ?? true,
        onClick: async (row: T) => {
          await action.onClick(row, {
            items,
            reload,
            setError,
          });
        },
      }))}
      toolbarActions={[
        ...(config.toolbarActions ?? [])
          .filter((action) => action.isVisible?.({ archived: false, items }) ?? true)
          .map((action) => ({
            id: action.id,
            label: action.label,
            kind: action.kind,
            isVisible: ({ items: ctxItems }: { items: T[]; selectedItem: T | null }) =>
              action.isVisible?.({ archived: false, items: ctxItems }) ?? true,
            onClick: async (_ctx: { items: T[]; selectedItem: T | null }) => {
              await action.onClick({
                items,
                reload,
                setError: (msg: string) => {
                  setError(msg);
                },
              });
            },
          })),
        {
          id: 'reload',
          label: 'Recargar',
          kind: 'secondary',
          onClick: async (_ctx: { items: T[]; selectedItem: T | null }) => {
            await reload();
          },
        },
      ]}
      viewModes={[
        { id: 'table-detail', label: detail?.title ?? 'Detalle' },
        { id: 'list', label: 'Lista' },
      ]}
      initialViewMode="table-detail"
      detailTitle={detail?.title ?? 'Detalle'}
      detailEmptyState={detail?.emptyState}
      renderDetail={detail ? (row) => detail.renderDetail(row, { items, reload }) : undefined}
      initialSelectedId={initialSelectedId}
    />
  );
}
