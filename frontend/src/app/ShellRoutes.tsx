import { Route, Routes, Navigate } from 'react-router-dom';
import { Suspended } from './suspended';
import {
  AdminPage,
  AutoRepairServicesPage,
  AutoRepairVehiclesPage,
  AutoRepairWorkOrdersPage,
  BeautySalonServicesPage,
  BeautyStaffPage,
  BikeShopBicyclesPage,
  BikeShopServicesPage,
  BikeShopWorkOrdersPage,
  CalendarPage,
  CryptoPage,
  CustomersPage,
  DashboardPage,
  DashboardVisualPage,
  IntakesPage,
  InvoicesPage,
  ModulePage,
  NotificationsCenterPage,
  PublicPreviewPage,
  PurchasesPage,
  RestaurantDiningAreasPage,
  RestaurantDiningTablesPage,
  RestaurantTableSessionsPage,
  SessionsPage,
  SettingsHubPage,
  SpecialtiesPage,
  TeachersPage,
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
      <Route path="/admin" element={<AdminPage />} />
      <Route path="/billing" element={<Navigate to="/settings#facturacion" replace />} />
      <Route path="/invoices" element={<InvoicesPage />} />
      <Route path="/crypto" element={<CryptoPage />} />
      <Route path="/ui" element={<UIComponentsPage />} />
      <Route path="/modules/customers" element={<CustomersPage />} />
      <Route path="/modules/purchases" element={<PurchasesPage />} />
      <Route path="/compras" element={<PurchasesPage />} />
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
      <Route path="/professionals/teachers" element={<TeachersPage />} />
      <Route path="/professionals/teachers/specialties" element={<SpecialtiesPage />} />
      <Route path="/professionals/teachers/intakes" element={<IntakesPage />} />
      <Route path="/professionals/teachers/sessions" element={<SessionsPage />} />
      <Route path="/scheduling/public-preview" element={<PublicPreviewPage />} />
      <Route path="/workshops/auto-repair/vehicles" element={<AutoRepairVehiclesPage />} />
      <Route path="/workshops/auto-repair/services" element={<AutoRepairServicesPage />} />
      <Route path="/workshops/auto-repair/orders/*" element={<Navigate to="/modules/workOrders" replace />} />
      <Route path="/workshops/bike-shop/bicycles" element={<BikeShopBicyclesPage />} />
      <Route path="/workshops/bike-shop/services" element={<BikeShopServicesPage />} />
      <Route path="/workshops/bike-shop/orders" element={<BikeShopWorkOrdersPage />} />
      <Route path="/beauty/salon/staff" element={<BeautyStaffPage />} />
      <Route path="/beauty/salon/services" element={<BeautySalonServicesPage />} />
      <Route path="/restaurants/dining/areas" element={<RestaurantDiningAreasPage />} />
      <Route path="/restaurants/dining/tables" element={<RestaurantDiningTablesPage />} />
      <Route path="/restaurants/dining/sessions" element={<RestaurantTableSessionsPage />} />
      <Route path="/automation-rules" element={<AutomationRulesPage />} />
      <Route path="/whatsapp/campaigns" element={<WhatsAppCampaignsPage />} />
      <Route path="/whatsapp/inbox" element={<WhatsAppInboxPage />} />
      <Route path="/approvals" element={<Navigate to="/notifications" replace />} />
      <Route path="/watcher-config" element={<WatcherConfigPage />} />
      <Route
        path="/audit"
        element={
          <Suspended>
            <AdminPage section="audit" />
          </Suspended>
        }
      />
      <Route
        path="/roles"
        element={
          <Suspended>
            <AdminPage section="rbac" />
          </Suspended>
        }
      />
      <Route path="/dashboard" element={<DashboardVisualPage />} />
      <Route path="/dashboard/widgets" element={<DashboardPage />} />
      <Route path="/calendar" element={<CalendarPage />} />
    </Routes>
  );
}
