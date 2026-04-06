import { Route, Routes, Navigate } from 'react-router-dom';
import {
  AutoRepairWorkOrdersPage,
  BikeShopWorkOrdersBoard,
  BikeShopWorkOrdersPage,
  BikeShopWorkOrdersSection,
  CalendarPage,
  CryptoPage,
  DashboardVisualPage,
  InvoicesPage,
  ModulePage,
  NotificationsCenterPage,
  PublicPreviewPage,
  RestaurantTableSessionsPage,
  SettingsHubPage,
  StockPage,
  UIComponentsPage,
  UnifiedChatPage,
  WhatsAppCampaignsPage,
  WhatsAppInboxPage,
  WorkOrdersEditorPage,
  WorkOrdersKanbanPanel,
  WorkOrdersModuleSection,
  AutomationRulesPage,
  WatcherConfigPage,
} from './lazyRoutes';

/**
 * Rutas bajo el Shell autenticado (producto + demos internas).
 * Demos / showcase: `/crypto`, `/ui` (no confundir con módulos de negocio).
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
      <Route path="/crypto" element={<CryptoPage />} />
      <Route path="/ui" element={<UIComponentsPage />} />
      <Route path="/modules/workOrders" element={<WorkOrdersModuleSection />}>
        <Route index element={<Navigate to="board" replace />} />
        <Route path="board" element={<WorkOrdersKanbanPanel />} />
        <Route path="list" element={<AutoRepairWorkOrdersPage />} />
        <Route path="edit/:orderId" element={<WorkOrdersEditorPage />} />
      </Route>
      <Route path="/modules/:moduleId" element={<ModulePage />} />
      <Route path="/settings" element={<SettingsHubPage />} />
      <Route path="/settings/keys" element={<Navigate to="/settings" replace />} />
      <Route path="/settings/notifications" element={<Navigate to="/settings?section=notifications" replace />} />
      <Route path="/scheduling/public-preview" element={<PublicPreviewPage />} />
      <Route path="/workshops/auto-repair/orders/*" element={<Navigate to="/modules/workOrders" replace />} />
      <Route path="/workshops/bike-shop/orders" element={<BikeShopWorkOrdersSection />}>
        <Route index element={<Navigate to="board" replace />} />
        <Route path="board" element={<BikeShopWorkOrdersBoard />} />
        <Route path="list" element={<BikeShopWorkOrdersPage />} />
      </Route>
      <Route path="/restaurants/dining/sessions" element={<RestaurantTableSessionsPage />} />
      <Route path="/automation-rules" element={<AutomationRulesPage />} />
      <Route path="/whatsapp/campaigns" element={<WhatsAppCampaignsPage />} />
      <Route path="/whatsapp/inbox" element={<WhatsAppInboxPage />} />
      <Route path="/approvals" element={<Navigate to="/notifications" replace />} />
      <Route path="/watcher-config" element={<WatcherConfigPage />} />
      <Route path="/audit" element={<Navigate to="/settings?section=audit" replace />} />
      <Route path="/roles" element={<Navigate to="/settings?section=rbac" replace />} />
      <Route path="/dashboard" element={<DashboardVisualPage />} />
      <Route path="/stock" element={<StockPage />} />
      <Route path="/modules/inventory" element={<Navigate to="/stock" replace />} />
      <Route path="/modules/inventoryMovements" element={<Navigate to="/stock" replace />} />
      <Route path="/calendar" element={<CalendarPage />} />
    </Routes>
  );
}
