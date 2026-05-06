import type { ReactNode } from 'react';
import { Route, Routes, Navigate, useParams } from 'react-router-dom';
import type { CrudViewModeId } from '../components/CrudPage';
import { PageLayout } from '../components/PageLayout';
import { useBranchSelection } from '../lib/useBranchSelection';
import { useTenantSlug } from '../lib/tenantSlug';
import {
  CalendarPage,
  ConfiguredCrudSectionPage,
  ConfiguredCrudIndexRedirect,
  CrudUiConfigurePage,
  DashboardVisualPage,
  ModulePage,
  NotificationsCenterPage,
  RestaurantTableSessionsPage,
  SettingsHubPage,
  ConfiguredCrudModePage,
  ConfiguredCrudRouteModePage,
  UnifiedChatPage,
  CustomerMessagingCampaignsPage,
  CustomerMessagingInboxPage,
  AutomationRulesPage,
  WatcherConfigPage,
} from './lazyRoutes';

function BranchSelectionLoading() {
  return (
    <PageLayout title="Sucursal activa" lead="Cargando sucursal seleccionada.">
      <div className="card">
        <p>Cargando sucursal…</p>
      </div>
    </PageLayout>
  );
}

function InventoryModePage({ modeId }: { modeId: CrudViewModeId }) {
  const { isLoading, selectedBranchId } = useBranchSelection();
  if (isLoading) return <BranchSelectionLoading />;
  return <ConfiguredCrudModePage key={`inventory:${modeId}:${selectedBranchId ?? 'all'}`} resourceId="inventory" modeId={modeId} />;
}

function InventorySectionLayout({ slug }: { slug: string }) {
  const baseRoute = `/${slug}/inventory`;
  return (
    <ConfiguredCrudSectionPage
      resourceId="inventory"
      baseRoute={baseRoute}
      actionLink={{
        to: `${baseRoute}/configure`,
        label: 'Configurar',
        hideWhenActivePattern: `${baseRoute}/configure`,
        activeReplacement: {
          to: `${baseRoute}/list`,
          label: 'Volver al inventario',
        },
      }}
    />
  );
}

/** Raíz: resuelve el slug del tenant activo y redirige al dashboard; si no hay profile, a /onboarding. */
function RootTenantRedirect() {
  const slug = useTenantSlug();
  if (!slug) return <Navigate to="/onboarding" replace />;
  return <Navigate to={`/${slug}/dashboard`} replace />;
}

/** Valida que el :orgSlug de la URL coincida con el profile. Si no hay profile, manda a /onboarding. */
function TenantSlugGate({ children }: { children: ReactNode }) {
  const { orgSlug = '' } = useParams();
  const slug = useTenantSlug();
  if (!slug) return <Navigate to="/onboarding" replace />;
  // Dev: aceptamos cualquier slug en la URL. En prod con backend esto se valida contra orgs.slug.
  void orgSlug;
  return <>{children}</>;
}

function TenantScopedRoutes() {
  const { orgSlug = '' } = useParams();
  const profileSlug = useTenantSlug();
  const slug = profileSlug ?? orgSlug;
  return (
    <Routes>
      <Route index element={<Navigate to="dashboard" replace />} />
      <Route path="dashboard" element={<DashboardVisualPage />} />
      <Route path="chat" element={<UnifiedChatPage />} />
      <Route path="notifications" element={<NotificationsCenterPage />} />
      <Route path="agenda" element={<CalendarPage />} />
      <Route path="settings" element={<SettingsHubPage />} />
      <Route path="automation-rules" element={<AutomationRulesPage />} />
      <Route path="customer-messaging/campaigns" element={<CustomerMessagingCampaignsPage />} />
      <Route path="customer-messaging/inbox" element={<CustomerMessagingInboxPage />} />
      <Route path="watcher-config" element={<WatcherConfigPage />} />
      <Route path="restaurants/dining/sessions" element={<RestaurantTableSessionsPage />} />

      {/* inventory: sección con sus propios routes */}
      <Route path="inventory" element={<InventorySectionLayout slug={slug} />}>
        <Route index element={<ConfiguredCrudIndexRedirect resourceId="inventory" baseRoute={`/${slug}/inventory`} />} />
        <Route path="list" element={<InventoryModePage modeId="list" />} />
        <Route path="configure" element={<CrudUiConfigurePage />} />
        <Route path="explorer" element={<Navigate to="../list" replace />} />
        <Route path="gallery" element={<InventoryModePage modeId="gallery" />} />
        <Route path="board" element={<InventoryModePage modeId="kanban" />} />
      </Route>

      {/* CRUD genérico: /{slug}/{moduleId}[/modePath] */}
      <Route path=":moduleId/configure" element={<CrudUiConfigurePage />} />
      <Route path=":moduleId/:modePath" element={<ConfiguredCrudRouteModePage />} />
      <Route path=":moduleId" element={<ModulePage />} />
    </Routes>
  );
}

/**
 * Rutas bajo el Shell autenticado (producto).
 *
 * Política de URL: `/{tenant-slug}/{recurso}/{subpath}`. El tenant slug se
 * deriva de `tenantProfile.businessName` slugificado.
 */
export function ShellRoutes() {
  return (
    <Routes>
      <Route path="/" element={<RootTenantRedirect />} />

      {/* Rutas autenticadas bajo tenant slug */}
      <Route
        path="/:orgSlug/*"
        element={
          <TenantSlugGate>
            <TenantScopedRoutes />
          </TenantSlugGate>
        }
      />

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
