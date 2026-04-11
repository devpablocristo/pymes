import { Route, Routes, Navigate } from 'react-router-dom';
import {
  AutoRepairWorkOrdersPage,
  BikeShopWorkOrdersBoard,
  BikeShopWorkOrdersPage,
  BikeShopWorkOrdersSection,
  CalendarPage,
  ConfiguredCrudIndexRedirect,
  DashboardVisualPage,
  InvoicesPage,
  ModulePage,
  NotificationsCenterPage,
  ProductsGalleryPage,
  ProductsListPage,
  ProductsModuleSection,
  RestaurantTableSessionsPage,
  SettingsHubPage,
  ConfiguredCrudModePage,
  StockCrudUiConfigurePage,
  StockListPage,
  StockModuleSection,
  UnifiedChatPage,
  CustomerMessagingCampaignsPage,
  CustomerMessagingInboxPage,
  WorkOrdersEditorPage,
  WorkOrdersKanbanPanel,
  WorkOrdersModuleSection,
  AutomationRulesPage,
  WatcherConfigPage,
} from './lazyRoutes';

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
      <Route path="/invoices" element={<InvoicesPage />} />
      <Route path="/modules/carWorkOrders" element={<WorkOrdersModuleSection />}>
        <Route index element={<ConfiguredCrudIndexRedirect resourceId="carWorkOrders" baseRoute="/modules/carWorkOrders" />} />
        <Route path="board" element={<WorkOrdersKanbanPanel />} />
        <Route path="list" element={<AutoRepairWorkOrdersPage />} />
        <Route path="edit/:orderId" element={<WorkOrdersEditorPage />} />
      </Route>
      <Route path="/modules/products" element={<ProductsModuleSection />}>
        <Route index element={<ConfiguredCrudIndexRedirect resourceId="products" baseRoute="/modules/products" />} />
        <Route path="list" element={<ProductsListPage />} />
        <Route path="gallery" element={<ProductsGalleryPage />} />
      </Route>
      <Route path="/modules/stock" element={<StockModuleSection />}>
        <Route index element={<ConfiguredCrudIndexRedirect resourceId="stock" baseRoute="/modules/stock" />} />
        <Route path="list" element={<StockListPage />} />
        <Route path="configure" element={<StockCrudUiConfigurePage />} />
        <Route path="explorer" element={<Navigate to="/modules/stock/list" replace />} />
        <Route path="gallery" element={<ConfiguredCrudModePage resourceId="stock" modeId="gallery" />} />
        <Route path="board" element={<ConfiguredCrudModePage resourceId="stock" modeId="kanban" />} />
      </Route>
      <Route path="/modules/:moduleId" element={<ModulePage />} />
      <Route path="/settings" element={<SettingsHubPage />} />
      <Route path="/settings/keys" element={<Navigate to="/settings" replace />} />
      <Route path="/settings/notifications" element={<Navigate to="/settings?section=notifications" replace />} />
      <Route path="/workshops/auto-repair/orders/*" element={<Navigate to="/modules/carWorkOrders" replace />} />
      <Route path="/workshops/bike-shop/orders" element={<BikeShopWorkOrdersSection />}>
        <Route index element={<ConfiguredCrudIndexRedirect resourceId="bikeWorkOrders" baseRoute="/workshops/bike-shop/orders" />} />
        <Route path="board" element={<BikeShopWorkOrdersBoard />} />
        <Route path="list" element={<BikeShopWorkOrdersPage />} />
      </Route>
      <Route path="/restaurants/dining/sessions" element={<RestaurantTableSessionsPage />} />
      <Route path="/automation-rules" element={<AutomationRulesPage />} />
      <Route path="/customer-messaging/campaigns" element={<CustomerMessagingCampaignsPage />} />
      <Route path="/customer-messaging/inbox" element={<CustomerMessagingInboxPage />} />
      <Route path="/watcher-config" element={<WatcherConfigPage />} />
      <Route path="/audit" element={<Navigate to="/settings?section=audit" replace />} />
      <Route path="/roles" element={<Navigate to="/settings?section=rbac" replace />} />
      <Route path="/dashboard" element={<DashboardVisualPage />} />
      <Route path="/stock" element={<Navigate to="/modules/stock/list" replace />} />
      <Route path="/modules/inventory" element={<Navigate to="/modules/stock/list" replace />} />
      <Route path="/modules/inventoryMovements" element={<Navigate to="/modules/stock/list" replace />} />
      <Route path="/agenda" element={<CalendarPage />} />
      <Route path="/calendar" element={<Navigate to="/agenda" replace />} />
    </Routes>
  );
}
