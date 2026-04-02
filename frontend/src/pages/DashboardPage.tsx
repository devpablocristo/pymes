import { useUser } from '@clerk/react';
import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { PageLayout } from '../components/PageLayout';
import {
  getDashboard,
  getMe,
  resetDashboard,
  saveDashboard,
} from '../lib/api';
import { DashboardBoard } from '../dashboard/components/DashboardBoard';
import { WidgetCatalog } from '../dashboard/components/WidgetCatalog';
import { getVisibleWidgetKeys } from '../lib/profileFilters';
import { clerkEnabled } from '../lib/auth';
import { useI18n } from '../lib/i18n';
import { greetingDisplayName } from '../lib/profileDisplay';
import { getTenantProfile } from '../lib/tenantProfile';
import type { MeProfileResponse } from '../lib/types';
import {
  type DashboardContext,
  type DashboardLayoutItem,
  type DashboardWidgetDefinition,
} from '../dashboard/types';
import {
  moveLayoutItem,
  packLayoutItems,
  resizeLayoutItem,
  serializeLayout,
  toDashboardSavePayload,
  toggleLayoutItemVisibility,
  upsertWidgetInstance,
} from '../dashboard/utils/layout';

function DashboardWelcomeText({ me }: { me: MeProfileResponse | undefined }) {
  const { t } = useI18n();
  if (!clerkEnabled) {
    const name = greetingDisplayName(me, undefined);
    return name ? t('dashboard.welcome', { name }) : t('dashboard.heading');
  }
  return <ClerkDashboardWelcomeText me={me} />;
}

function ClerkDashboardWelcomeText({ me }: { me: MeProfileResponse | undefined }) {
  const { t } = useI18n();
  const { user, isLoaded } = useUser();
  const name = greetingDisplayName(me, user ?? undefined);
  if (!isLoaded) {
    return t('dashboard.heading');
  }
  return name ? t('dashboard.welcome', { name }) : t('dashboard.heading');
}

export function DashboardPage() {
  const queryClient = useQueryClient();
  const selectedContext: DashboardContext = 'home';
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
    if (dashboardQuery.data && !editing) {
      setDraftItems(
        packLayoutItems(
          dashboardQuery.data.layout.items,
          dashboardQuery.data.available_widgets,
        ),
      );
    }
  }, [dashboardQuery.data, editing]);

  const profileWidgetKeys = useMemo(() => getVisibleWidgetKeys(), []);
  const availableWidgets = useMemo(
    () => (dashboardQuery.data?.available_widgets ?? []).filter((w) => profileWidgetKeys.has(w.widget_key)),
    [dashboardQuery.data?.available_widgets, profileWidgetKeys],
  );
  const normalizedDraft = useMemo(
    () => packLayoutItems(draftItems, availableWidgets),
    [draftItems, availableWidgets],
  );
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

  const profile = getTenantProfile();
  const { t } = useI18n();
  const headerActions = (
    <div className="actions-row dashboard-actions">
      <Link to="/dashboard" className="btn-secondary btn-sm">
        {t('shell.dashboard.backToSummary')}
      </Link>
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
            {saving ? 'Guardando...' : 'Guardar'}
          </button>
        </>
      ) : (
        <button
          type="button"
          className="btn-secondary"
          onClick={() => setEditing(true)}
          disabled={busy}
        >
          Personalizar
        </button>
      )}
    </div>
  );

  return (
    <PageLayout
      className="dashboard-page"
      title={<DashboardWelcomeText me={meQuery.data} />}
      lead={profile?.businessName || undefined}
      actions={headerActions}
    >
      {primaryError ? <div className="alert alert-error">{primaryError}</div> : null}
      {editing && dirty ? (
        <div className="alert alert-success">
          Tenés cambios sin guardar.
        </div>
      ) : null}

      {busy ? (
        <div className="spinner" />
      ) : dashboardQuery.data ? (
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
      ) : null}

      <WidgetCatalog
        open={catalogOpen}
        widgets={availableWidgets}
        layoutItems={normalizedDraft}
        onAdd={handleAddWidget}
        onClose={() => setCatalogOpen(false)}
      />
    </PageLayout>
  );
}
