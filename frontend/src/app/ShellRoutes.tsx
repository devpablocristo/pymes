import type { ReactNode } from 'react';
import { Route, Routes, Navigate, useLocation, useParams } from 'react-router-dom';
import type { CrudViewModeId } from '../components/CrudPage';
import { PageLayout } from '../components/PageLayout';
import { toCrudResourceSlug } from '../crud/crudResourceSlug';
import { useBranchSelection } from '../lib/useBranchSelection';
import { useTenantSlug } from '../lib/tenantSlug';
import { getTenantProfile } from '../lib/tenantProfile';
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

type WorkOrdersResourceId = 'carWorkOrders' | 'bikeWorkOrders';

function BranchSelectionLoading() {
  return (
    <PageLayout title="Sucursal activa" lead="Cargando sucursal seleccionada.">
      <div className="card">
        <p>Cargando sucursal…</p>
      </div>
    </PageLayout>
  );
}

/** Resuelve qué variante de work-orders mostrar según el subvertical del profile. */
function resolveWorkOrdersResourceId(): WorkOrdersResourceId {
  const profile = getTenantProfile();
  return profile?.subVertical === 'bike_shop' ? 'bikeWorkOrders' : 'carWorkOrders';
}

function normalizeLegacyWorkOrdersRemainder(remainder: string): string {
  if (!remainder || remainder === '/') {
    return '/list';
  }
  if (remainder === '/board' || remainder.startsWith('/board/')) {
    return '/list';
  }
  if (remainder.startsWith('/edit/')) {
    return '/list';
  }
  return remainder;
}

function WorkOrdersLegacyAliasRedirect() {
  const { orgSlug = '' } = useParams();
  const profileSlug = useTenantSlug();
  const slug = profileSlug ?? orgSlug;
  const location = useLocation();
  const resourceSlug = toCrudResourceSlug(resolveWorkOrdersResourceId());
  const remainder = location.pathname.replace(/^\/[^/]+\/work-orders/, '');

  if (!slug) {
    return <Navigate to="/onboarding" replace />;
  }

  return (
    <Navigate
      to={`/${slug}/${resourceSlug}${normalizeLegacyWorkOrdersRemainder(remainder)}${location.search}${location.hash}`}
      replace
    />
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

/** Catch-all para URLs legacy sin slug: prepende el slug actual y preserva el resto del path. */
function LegacyPathRedirect() {
  const slug = useTenantSlug();
  const location = useLocation();
  if (!slug) return <Navigate to="/onboarding" replace />;
  return <Navigate to={`/${slug}${location.pathname}${location.search}${location.hash}`} replace />;
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
      <Route path="assistant/commercial" element={<Navigate to="../chat" replace />} />
      <Route path="notifications" element={<NotificationsCenterPage />} />
      <Route path="agenda" element={<CalendarPage />} />
      <Route path="calendar" element={<Navigate to="../agenda" replace />} />
      <Route path="settings" element={<SettingsHubPage />} />
      <Route path="settings/keys" element={<Navigate to="../settings" replace />} />
      <Route path="settings/notifications" element={<Navigate to="../settings?section=notifications" replace />} />
      <Route path="admin" element={<Navigate to="../settings" replace />} />
      <Route path="billing" element={<Navigate to="../settings?section=gateway" replace />} />
      <Route path="audit" element={<Navigate to="../settings?section=audit" replace />} />
      <Route path="roles" element={<Navigate to="../settings?section=rbac" replace />} />
      <Route path="automation-rules" element={<AutomationRulesPage />} />
      <Route path="customer-messaging/campaigns" element={<CustomerMessagingCampaignsPage />} />
      <Route path="customer-messaging/inbox" element={<CustomerMessagingInboxPage />} />
      <Route path="watcher-config" element={<WatcherConfigPage />} />
      <Route path="restaurants/dining/sessions" element={<RestaurantTableSessionsPage />} />

      {/* alias legado: /work-orders/* -> recurso CRUD canónico según subvertical */}
      <Route path="work-orders/*" element={<WorkOrdersLegacyAliasRedirect />} />

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

/** Legacy: /modules/:moduleId/... → /{slug}/:moduleId/... */
function LegacyModulesRedirect() {
  const slug = useTenantSlug();
  const { moduleId = '' } = useParams();
  const location = useLocation();
  if (!slug) return <Navigate to="/onboarding" replace />;
  const remainder = location.pathname.replace(/^\/modules\/[^/]+/, '');
  return <Navigate to={`/${slug}/${moduleId}${remainder}${location.search}${location.hash}`} replace />;
}

/** Legacy: /workshops/{sub}/{module}/* → recursos canónicos bajo /{slug}/... */
export function resolveLegacyWorkshopDestination(slug: string, pathname: string): string {
  const match = pathname.match(/^\/workshops\/([^/]+)\/([^/]+)(\/.*)?$/);
  const segment = match?.[1] ?? '';
  const moduleId = match?.[2] ?? '';
  const remainder = match?.[3] ?? '';

  if (segment === 'auto-repair' && moduleId === 'vehicles') {
    return `/${slug}/${toCrudResourceSlug('workshopVehicles')}${remainder}`;
  }
  if (segment === 'bike-shop') {
    return `/${slug}/${toCrudResourceSlug('bikeWorkOrders')}${normalizeLegacyWorkOrdersRemainder(remainder)}`;
  }
  return `/${slug}/${toCrudResourceSlug('carWorkOrders')}${normalizeLegacyWorkOrdersRemainder(remainder)}`;
}

function LegacyWorkshopsRedirect() {
  const slug = useTenantSlug();
  const location = useLocation();
  if (!slug) return <Navigate to="/onboarding" replace />;
  const destination = resolveLegacyWorkshopDestination(slug, location.pathname);
  return <Navigate to={`${destination}${location.search}${location.hash}`} replace />;
}

/**
 * Rutas bajo el Shell autenticado (producto).
 *
 * Política de URL: `/{tenant-slug}/{recurso}/{subpath}`. El tenant slug se
 * deriva de `tenantProfile.businessName` slugificado. URLs viejas sin slug
 * redirigen automáticamente al slug actual.
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

      {/* Legacy redirects: capturan URLs autenticadas viejas y prepende slug. */}
      <Route path="/dashboard/*" element={<LegacyPathRedirect />} />
      <Route path="/chat/*" element={<LegacyPathRedirect />} />
      <Route path="/notifications/*" element={<LegacyPathRedirect />} />
      <Route path="/agenda/*" element={<LegacyPathRedirect />} />
      <Route path="/calendar/*" element={<LegacyPathRedirect />} />
      <Route path="/settings/*" element={<LegacyPathRedirect />} />
      <Route path="/admin/*" element={<LegacyPathRedirect />} />
      <Route path="/billing/*" element={<LegacyPathRedirect />} />
      <Route path="/invoices/*" element={<LegacyPathRedirect />} />
      <Route path="/audit/*" element={<LegacyPathRedirect />} />
      <Route path="/roles/*" element={<LegacyPathRedirect />} />
      <Route path="/automation-rules/*" element={<LegacyPathRedirect />} />
      <Route path="/customer-messaging/*" element={<LegacyPathRedirect />} />
      <Route path="/watcher-config/*" element={<LegacyPathRedirect />} />
      <Route path="/assistant/*" element={<LegacyPathRedirect />} />
      <Route path="/restaurants/*" element={<LegacyPathRedirect />} />

      {/* /modules/* → /{slug}/* */}
      <Route path="/modules/:moduleId/*" element={<LegacyModulesRedirect />} />

      <Route path="/work-orders/*" element={<LegacyPathRedirect />} />

      {/* /workshops/{sub}/* → recursos CRUD canónicos */}
      <Route path="/workshops/*" element={<LegacyWorkshopsRedirect />} />
    </Routes>
  );
}
