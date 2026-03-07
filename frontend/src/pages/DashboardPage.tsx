import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getDashboard,
  getMe,
  resetDashboard,
  saveDashboard,
} from '../lib/api';
import { DashboardBoard } from '../dashboard/components/DashboardBoard';
import { WidgetCatalog } from '../dashboard/components/WidgetCatalog';
import {
  dashboardContexts,
  type DashboardContext,
  type DashboardLayoutItem,
  type DashboardWidgetDefinition,
} from '../dashboard/types';
import {
  hiddenItems,
  moveLayoutItem,
  packLayoutItems,
  resizeLayoutItem,
  serializeLayout,
  toDashboardSavePayload,
  toggleLayoutItemVisibility,
  upsertWidgetInstance,
  visibleItems,
} from '../dashboard/utils/layout';

export function DashboardPage() {
  const queryClient = useQueryClient();
  const [selectedContext, setSelectedContext] = useState<DashboardContext>('home');
  const [editing, setEditing] = useState(false);
  const [catalogOpen, setCatalogOpen] = useState(false);
  const [draftItems, setDraftItems] = useState<DashboardLayoutItem[]>([]);

  const meQuery = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    staleTime: 60_000,
  });

  const dashboardQuery = useQuery({
    queryKey: ['dashboard', selectedContext],
    queryFn: () => getDashboard(String(selectedContext)),
  });

  useEffect(() => {
    setEditing(false);
    setCatalogOpen(false);
  }, [selectedContext]);

  useEffect(() => {
    if (dashboardQuery.data && !editing) {
      setDraftItems(
        packLayoutItems(
          dashboardQuery.data.layout.items,
          dashboardQuery.data.available_widgets,
        ),
      );
    }
  }, [dashboardQuery.data, editing]);

  const availableWidgets = dashboardQuery.data?.available_widgets ?? [];
  const normalizedDraft = useMemo(
    () => packLayoutItems(draftItems, availableWidgets),
    [draftItems, availableWidgets],
  );
  const hidden = hiddenItems(normalizedDraft);
  const visible = visibleItems(normalizedDraft);
  const dirty = dashboardQuery.data
    ? serializeLayout(normalizedDraft, availableWidgets) !==
      serializeLayout(dashboardQuery.data.layout.items, availableWidgets)
    : false;

  const saveMutation = useMutation({
    mutationFn: async () =>
      saveDashboard(
        toDashboardSavePayload(String(selectedContext), normalizedDraft, availableWidgets),
      ),
    onSuccess: (data) => {
      queryClient.setQueryData(['dashboard', selectedContext], data);
      setDraftItems(packLayoutItems(data.layout.items, data.available_widgets));
      setEditing(false);
      setCatalogOpen(false);
    },
  });

  const resetMutation = useMutation({
    mutationFn: async () => resetDashboard(String(selectedContext)),
    onSuccess: (data) => {
      queryClient.setQueryData(['dashboard', selectedContext], data);
      setDraftItems(packLayoutItems(data.layout.items, data.available_widgets));
      setEditing(false);
      setCatalogOpen(false);
    },
  });

  const contextMeta =
    dashboardContexts.find((context) => context.id === selectedContext) ?? dashboardContexts[0];

  const primaryError =
    (dashboardQuery.error as Error | null)?.message || (meQuery.error as Error | null)?.message || '';

  const busy = dashboardQuery.isLoading || meQuery.isLoading;
  const saving = saveMutation.isPending || resetMutation.isPending;

  function ensureWidgets(): DashboardWidgetDefinition[] {
    return dashboardQuery.data?.available_widgets ?? [];
  }

  function handleMove(instanceId: string, delta: -1 | 1) {
    setDraftItems((current) => moveLayoutItem(current, instanceId, delta, ensureWidgets()));
  }

  function handleResize(instanceId: string, deltaW: number, deltaH: number) {
    setDraftItems((current) => resizeLayoutItem(current, instanceId, deltaW, deltaH, ensureWidgets()));
  }

  function handleToggleVisibility(instanceId: string) {
    setDraftItems((current) =>
      toggleLayoutItemVisibility(current, instanceId, ensureWidgets()),
    );
  }

  function handleAddWidget(widget: DashboardWidgetDefinition) {
    setDraftItems((current) => upsertWidgetInstance(current, widget, ensureWidgets()));
  }

  function handleCancelEditing() {
    if (!dashboardQuery.data) {
      return;
    }
    setDraftItems(
      packLayoutItems(dashboardQuery.data.layout.items, dashboardQuery.data.available_widgets),
    );
    setEditing(false);
    setCatalogOpen(false);
  }

  return (
    <>
      <div className="dashboard-shell-header">
        <div className="dashboard-hero card">
          <div className="dashboard-hero-copy">
            <span className="dashboard-hero-kicker">{contextMeta.kicker}</span>
            <h1>{contextMeta.label}</h1>
            <p>{contextMeta.description}</p>
          </div>
          <div className="dashboard-hero-meta">
            <div>
              <small>Usuario</small>
              <strong>
                {String(
                  meQuery.data?.name ?? meQuery.data?.email ?? meQuery.data?.id ?? 'Sin identificar',
                )}
              </strong>
            </div>
            <div>
              <small>Layout</small>
              <strong>{dashboardQuery.data?.layout.source ?? 'cargando'}</strong>
            </div>
            <div>
              <small>Widgets visibles</small>
              <strong>{visible.length}</strong>
            </div>
            <div>
              <small>Catálogo</small>
              <strong>{availableWidgets.length}</strong>
            </div>
          </div>
        </div>

        <div className="dashboard-toolbar-row">
          <div className="dashboard-context-tabs">
            {dashboardContexts.map((context) => (
              <button
                key={context.id}
                type="button"
                className={`dashboard-context-tab${selectedContext === context.id ? ' active' : ''}`}
                onClick={() => setSelectedContext(context.id)}
              >
                <span>{context.label}</span>
                <small>{context.kicker}</small>
              </button>
            ))}
          </div>

          <div className="actions-row dashboard-actions">
            {editing ? (
              <>
                <button type="button" className="btn-secondary" onClick={() => setCatalogOpen(true)}>
                  Catálogo
                </button>
                <button
                  type="button"
                  className="btn-secondary"
                  onClick={handleCancelEditing}
                  disabled={saving}
                >
                  Cancelar
                </button>
                <button
                  type="button"
                  className="btn-danger"
                  onClick={() => resetMutation.mutate()}
                  disabled={saving}
                >
                  Resetear
                </button>
                <button
                  type="button"
                  className="btn-primary"
                  onClick={() => saveMutation.mutate()}
                  disabled={saving || !dirty}
                >
                  {saving ? 'Guardando...' : 'Guardar layout'}
                </button>
              </>
            ) : (
              <button
                type="button"
                className="btn-primary"
                onClick={() => setEditing(true)}
                disabled={busy}
              >
                Personalizar dashboard
              </button>
            )}
          </div>
        </div>
      </div>

      {primaryError ? <div className="alert alert-error">{primaryError}</div> : null}
      {dashboardQuery.data?.layout.source === 'default' && !editing ? (
        <div className="alert alert-warning">
          Estas viendo el layout base del sistema. Entra en modo edicion para guardar tu propia version.
        </div>
      ) : null}
      {editing && dirty ? (
        <div className="alert alert-success">
          Tienes cambios locales sin persistir en este contexto.
        </div>
      ) : null}

      {busy ? (
        <div className="spinner" />
      ) : dashboardQuery.data ? (
        <>
          <DashboardBoard
            context={selectedContext}
            items={normalizedDraft}
            widgets={availableWidgets}
            editing={editing}
            onMoveBackward={(instanceId) => handleMove(instanceId, -1)}
            onMoveForward={(instanceId) => handleMove(instanceId, 1)}
            onGrow={(instanceId) => handleResize(instanceId, 1, 1)}
            onShrink={(instanceId) => handleResize(instanceId, -1, -1)}
            onToggleVisibility={handleToggleVisibility}
          />

          <div className="dashboard-meta-grid compact-grid">
            <div className="dashboard-meta-card">
              <small>Contexto activo</small>
              <strong>{selectedContext}</strong>
              <p>{contextMeta.description}</p>
            </div>
            <div className="dashboard-meta-card">
              <small>Fuente del layout</small>
              <strong>{dashboardQuery.data.layout.source}</strong>
              <p>Version {dashboardQuery.data.layout.version}</p>
            </div>
            <div className="dashboard-meta-card">
              <small>Widgets ocultos</small>
              <strong>{hidden.length}</strong>
              <p>{hidden.length > 0 ? 'Listos para reactivar desde el catálogo.' : 'Sin widgets ocultos.'}</p>
            </div>
          </div>
        </>
      ) : null}

      <WidgetCatalog
        open={catalogOpen}
        widgets={availableWidgets}
        layoutItems={normalizedDraft}
        onAdd={handleAddWidget}
        onClose={() => setCatalogOpen(false)}
      />
    </>
  );
}
