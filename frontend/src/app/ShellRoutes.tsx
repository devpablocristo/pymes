import { Route, Routes, Navigate } from 'react-router-dom';
import type { CrudViewModeId } from '../components/CrudPage';
import { PageLayout } from '../components/PageLayout';
import { useBranchSelection } from '../lib/branchContext';
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
  ConfiguredCrudNestedRouteModePage,
  ConfiguredCrudRouteModePage,
  UnifiedChatPage,
  CustomerMessagingCampaignsPage,
  CustomerMessagingInboxPage,
  WorkOrdersEditorPage,
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

function BranchAwareWorkOrdersModePage({
  resourceId,
  modeId,
}: {
  resourceId: 'carWorkOrders' | 'bikeWorkOrders';
  modeId: CrudViewModeId;
}) {
  const { isLoading, selectedBranchId } = useBranchSelection();
  if (isLoading) {
    return <BranchSelectionLoading />;
  }
  return (
    <ConfiguredCrudModePage
      key={`${resourceId}:${modeId}:${selectedBranchId ?? 'all'}`}
      resourceId={resourceId}
      modeId={modeId}
    />
  );
}

function BranchAwareWorkOrdersNestedRouteModePage({
  resourceId,
  baseRoute,
}: {
  resourceId: 'carWorkOrders' | 'bikeWorkOrders';
  baseRoute: string;
}) {
  const { isLoading, selectedBranchId } = useBranchSelection();
  if (isLoading) {
    return <BranchSelectionLoading />;
  }
  return (
    <ConfiguredCrudNestedRouteModePage
      key={`${resourceId}:nested:${selectedBranchId ?? 'all'}`}
      resourceId={resourceId}
      baseRoute={baseRoute}
    />
  );
}

function BranchAwareInventoryModePage({ modeId }: { modeId: CrudViewModeId }) {
  const { isLoading, selectedBranchId } = useBranchSelection();
  if (isLoading) {
    return <BranchSelectionLoading />;
  }
  return <ConfiguredCrudModePage key={`inventory:${modeId}:${selectedBranchId ?? 'all'}`} resourceId="inventory" modeId={modeId} />;
}

/**
 * Rutas bajo el Shell autenticado (producto).
 */
export function ShellRoutes() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/dashboard" replace />} />
      <Route path="/notifications" element={<NotificationsCenterPage />} />
      <Route path="/chat" element={<UnifiedChatPage />} />
      <Route path="/assistant/commercial" element={<Navigate to="/chat" replace />} />
      <Route path="/admin" element={<Navigate to="/settings" replace />} />
      <Route path="/billing" element={<Navigate to="/settings?section=gateway" replace />} />
      <Route path="/invoices" element={<Navigate to="/modules/invoices" replace />} />
      <Route
        path="/modules/carWorkOrders"
        element={
          <ConfiguredCrudSectionPage
            resourceId="carWorkOrders"
            baseRoute="/modules/carWorkOrders"
            contextPatternByModeId={{ list: '/modules/carWorkOrders/edit/:orderId' }}
            actionLink={{
              to: '/modules/carWorkOrders/configure',
              label: 'Configurar',
              hideWhenActivePattern: '/modules/carWorkOrders/configure',
              activeReplacement: {
                to: '/modules/carWorkOrders/list',
                label: 'Volver a órdenes de trabajo',
              },
            }}
            includeCanonicalMissing
          />
        }
      >
        <Route index element={<ConfiguredCrudIndexRedirect resourceId="carWorkOrders" baseRoute="/modules/carWorkOrders" />} />
        <Route path="board" element={<BranchAwareWorkOrdersModePage resourceId="carWorkOrders" modeId="kanban" />} />
        <Route path="list" element={<BranchAwareWorkOrdersModePage resourceId="carWorkOrders" modeId="list" />} />
        <Route path="edit/:orderId" element={<WorkOrdersEditorPage />} />
        <Route
          path=":modePath"
          element={<BranchAwareWorkOrdersNestedRouteModePage resourceId="carWorkOrders" baseRoute="/modules/carWorkOrders" />}
        />
      </Route>
      <Route
        path="/modules/bikeWorkOrders"
        element={
          <ConfiguredCrudSectionPage
            resourceId="bikeWorkOrders"
            baseRoute="/modules/bikeWorkOrders"
            actionLink={{
              to: '/modules/bikeWorkOrders/configure',
              label: 'Configurar',
              hideWhenActivePattern: '/modules/bikeWorkOrders/configure',
              activeReplacement: {
                to: '/modules/bikeWorkOrders/list',
                label: 'Volver a órdenes de trabajo',
              },
            }}
            includeCanonicalMissing
          />
        }
      >
        <Route index element={<ConfiguredCrudIndexRedirect resourceId="bikeWorkOrders" baseRoute="/modules/bikeWorkOrders" />} />
        <Route path="board" element={<BranchAwareWorkOrdersModePage resourceId="bikeWorkOrders" modeId="kanban" />} />
        <Route path="list" element={<BranchAwareWorkOrdersModePage resourceId="bikeWorkOrders" modeId="list" />} />
        <Route
          path=":modePath"
          element={<BranchAwareWorkOrdersNestedRouteModePage resourceId="bikeWorkOrders" baseRoute="/modules/bikeWorkOrders" />}
        />
      </Route>
      <Route
        path="/modules/inventory"
        element={
          <ConfiguredCrudSectionPage
            resourceId="inventory"
            baseRoute="/modules/inventory"
            actionLink={{
              to: '/modules/inventory/configure',
              label: 'Configurar',
              hideWhenActivePattern: '/modules/inventory/configure',
              activeReplacement: {
                to: '/modules/inventory/list',
                label: 'Volver al inventario',
              },
            }}
          />
        }
      >
        <Route index element={<ConfiguredCrudIndexRedirect resourceId="inventory" baseRoute="/modules/inventory" />} />
        <Route path="list" element={<BranchAwareInventoryModePage modeId="list" />} />
        <Route path="configure" element={<CrudUiConfigurePage />} />
        <Route path="explorer" element={<Navigate to="/modules/inventory/list" replace />} />
        <Route path="gallery" element={<BranchAwareInventoryModePage modeId="gallery" />} />
        <Route path="board" element={<BranchAwareInventoryModePage modeId="kanban" />} />
      </Route>
      <Route path="/modules/:moduleId/configure" element={<CrudUiConfigurePage />} />
      <Route path="/modules/:moduleId/:modePath" element={<ConfiguredCrudRouteModePage />} />
      <Route path="/modules/:moduleId" element={<ModulePage />} />
      <Route path="/settings" element={<SettingsHubPage />} />
      <Route path="/settings/keys" element={<Navigate to="/settings" replace />} />
      <Route path="/settings/notifications" element={<Navigate to="/settings?section=notifications" replace />} />
      <Route path="/workshops/auto-repair/orders/*" element={<Navigate to="/modules/carWorkOrders" replace />} />
      <Route
        path="/workshops/bike-shop/orders"
        element={
          <ConfiguredCrudSectionPage
            resourceId="bikeWorkOrders"
            baseRoute="/workshops/bike-shop/orders"
            actionLink={{
              to: '/modules/bikeWorkOrders/configure',
              label: 'Configurar',
              hideWhenActivePattern: '/modules/bikeWorkOrders/configure',
              activeReplacement: {
                to: '/workshops/bike-shop/orders/list',
                label: 'Volver a órdenes de trabajo',
              },
            }}
            includeCanonicalMissing
          />
        }
      >
        <Route index element={<ConfiguredCrudIndexRedirect resourceId="bikeWorkOrders" baseRoute="/workshops/bike-shop/orders" />} />
        <Route path="board" element={<BranchAwareWorkOrdersModePage resourceId="bikeWorkOrders" modeId="kanban" />} />
        <Route path="list" element={<BranchAwareWorkOrdersModePage resourceId="bikeWorkOrders" modeId="list" />} />
        <Route
          path=":modePath"
          element={<BranchAwareWorkOrdersNestedRouteModePage resourceId="bikeWorkOrders" baseRoute="/workshops/bike-shop/orders" />}
        />
      </Route>
      <Route path="/restaurants/dining/sessions" element={<RestaurantTableSessionsPage />} />
      <Route path="/automation-rules" element={<AutomationRulesPage />} />
      <Route path="/customer-messaging/campaigns" element={<CustomerMessagingCampaignsPage />} />
      <Route path="/customer-messaging/inbox" element={<CustomerMessagingInboxPage />} />
      <Route path="/watcher-config" element={<WatcherConfigPage />} />
      <Route path="/audit" element={<Navigate to="/settings?section=audit" replace />} />
      <Route path="/roles" element={<Navigate to="/settings?section=rbac" replace />} />
      <Route path="/dashboard" element={<DashboardVisualPage />} />
      <Route path="/modules/inventoryMovements" element={<Navigate to="/modules/inventory/list" replace />} />
      <Route path="/agenda" element={<CalendarPage />} />
      <Route path="/calendar" element={<Navigate to="/agenda" replace />} />
    </Routes>
  );
}
