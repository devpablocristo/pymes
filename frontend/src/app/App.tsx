import { StrictMode, Suspense, lazy, type ReactNode } from 'react';
import { Route, Routes, Navigate } from 'react-router-dom';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ClerkSessionOrgSync } from '../components/ClerkSessionOrgSync';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { clerkEnabled } from '../lib/auth';
import { hasCompletedOnboarding } from '../lib/tenantProfile';

const Shell = lazy(() => import('../components/Shell').then((mod) => ({ default: mod.Shell })));
const AdminPage = lazy(() => import('../pages/AdminPage').then((mod) => ({ default: mod.AdminPage })));
const AutoRepairServicesPage = lazy(() => import('../pages/AutoRepairServicesPage').then((mod) => ({ default: mod.AutoRepairServicesPage })));
const AutoRepairVehiclesPage = lazy(() => import('../pages/AutoRepairVehiclesPage').then((mod) => ({ default: mod.AutoRepairVehiclesPage })));
const AutoRepairWorkOrdersPage = lazy(() => import('../pages/AutoRepairWorkOrdersPage').then((mod) => ({ default: mod.AutoRepairWorkOrdersPage })));
const BikeShopBicyclesPage = lazy(() => import('../pages/BikeShopBicyclesPage').then((mod) => ({ default: mod.BikeShopBicyclesPage })));
const BikeShopServicesPage = lazy(() => import('../pages/BikeShopServicesPage').then((mod) => ({ default: mod.BikeShopServicesPage })));
const BikeShopWorkOrdersPage = lazy(() => import('../pages/BikeShopWorkOrdersPage').then((mod) => ({ default: mod.BikeShopWorkOrdersPage })));
const WorkOrdersModuleSection = lazy(() =>
  import('../pages/WorkOrdersModuleSection').then((mod) => ({ default: mod.WorkOrdersModuleSection })),
);
const WorkOrdersKanbanPanel = lazy(() =>
  import('../pages/WorkOrdersKanbanPanel').then((mod) => ({ default: mod.WorkOrdersKanbanPanel })),
);
const WorkOrdersEditorPage = lazy(() =>
  import('../pages/WorkOrdersEditorPage').then((mod) => ({ default: mod.WorkOrdersEditorPage })),
);
const BeautySalonServicesPage = lazy(() => import('../pages/BeautySalonServicesPage').then((mod) => ({ default: mod.BeautySalonServicesPage })));
const BeautyStaffPage = lazy(() => import('../pages/BeautyStaffPage').then((mod) => ({ default: mod.BeautyStaffPage })));
const UnifiedChatPage = lazy(() =>
  import('../pages/UnifiedChatPage').then((mod) => ({ default: mod.UnifiedChatPage })),
);
const NotificationsCenterPage = lazy(() =>
  import('../pages/NotificationsCenterPage').then((mod) => ({ default: mod.NotificationsCenterPage })),
);
const CustomersPage = lazy(() => import('../pages/CustomersPage').then((mod) => ({ default: mod.CustomersPage })));
const IntakesPage = lazy(() => import('../pages/IntakesPage').then((mod) => ({ default: mod.IntakesPage })));
const LoginPage = lazy(() => import('../pages/LoginPage').then((mod) => ({ default: mod.LoginPage })));
const ModulePage = lazy(() => import('../pages/ModulePage').then((mod) => ({ default: mod.ModulePage })));
const OnboardingPage = lazy(() => import('../pages/OnboardingPage').then((mod) => ({ default: mod.OnboardingPage })));
const PublicPreviewPage = lazy(() => import('../pages/PublicPreviewPage').then((mod) => ({ default: mod.PublicPreviewPage })));
const PurchasesPage = lazy(() => import('../pages/PurchasesPage').then((mod) => ({ default: mod.PurchasesPage })));
const RestaurantDiningAreasPage = lazy(() => import('../pages/RestaurantDiningAreasPage').then((mod) => ({ default: mod.RestaurantDiningAreasPage })));
const RestaurantDiningTablesPage = lazy(() => import('../pages/RestaurantDiningTablesPage').then((mod) => ({ default: mod.RestaurantDiningTablesPage })));
const RestaurantTableSessionsPage = lazy(() => import('../pages/RestaurantTableSessionsPage').then((mod) => ({ default: mod.RestaurantTableSessionsPage })));
const SessionsPage = lazy(() => import('../pages/SessionsPage').then((mod) => ({ default: mod.SessionsPage })));
const SignupPage = lazy(() => import('../pages/SignupPage').then((mod) => ({ default: mod.SignupPage })));
const SpecialtiesPage = lazy(() => import('../pages/SpecialtiesPage').then((mod) => ({ default: mod.SpecialtiesPage })));
const TeachersPage = lazy(() => import('../pages/TeachersPage').then((mod) => ({ default: mod.TeachersPage })));
const AutomationRulesPage = lazy(() => import('../pages/AutomationRulesPage'));
const WhatsAppCampaignsPage = lazy(() => import('../pages/WhatsAppCampaignsPage').then((mod) => ({ default: mod.WhatsAppCampaignsPage })));
const WhatsAppInboxPage = lazy(() => import('../pages/WhatsAppInboxPage').then((mod) => ({ default: mod.WhatsAppInboxPage })));
const WatcherConfigPage = lazy(() => import('../pages/WatcherConfigPage'));
const CalendarPage = lazy(() =>
  import('../pages/CalendarPage').then((mod) => ({ default: mod.CalendarPage })),
);
const DashboardVisualPage = lazy(() =>
  import('../pages/DashboardVisualPage').then((mod) => ({ default: mod.DashboardVisualPage })),
);
const DashboardPage = lazy(() =>
  import('../pages/DashboardPage').then((mod) => ({ default: mod.DashboardPage })),
);
const InvoicesPage = lazy(() =>
  import('../pages/InvoicesPage').then((mod) => ({ default: mod.InvoicesPage })),
);
const SettingsHubPage = lazy(() =>
  import('../pages/SettingsHubPage').then((mod) => ({ default: mod.SettingsHubPage })),
);
const UIComponentsPage = lazy(() =>
  import('../pages/UIComponentsPage').then((mod) => ({ default: mod.UIComponentsPage })),
);
const CryptoPage = lazy(() => import('../pages/CryptoPage').then((m) => ({ default: m.CryptoPage })));

function Suspended({ children }: { children: ReactNode }) {
  return <Suspense fallback={<div className="card"><p>Cargando…</p></div>}>{children}</Suspense>;
}

function RequireOnboarding({ children }: { children: React.ReactNode }) {
  if (!hasCompletedOnboarding()) {
    return <Navigate to="/onboarding" replace />;
  }
  return <>{children}</>;
}

/** En desarrollo mantenemos StrictMode sobre toda la consola protegida. */
function StrictDevShell({ children }: { children: ReactNode }) {
  return <StrictMode>{children}</StrictMode>;
}


export function App() {
  return (
    <>
      <AuthTokenBridge />
      {clerkEnabled && <ClerkSessionOrgSync />}
      <Routes>
        <Route
          path="/login/*"
          element={
            <StrictDevShell>
              <Suspended>
                <LoginPage />
              </Suspended>
            </StrictDevShell>
          }
        />
        <Route
          path="/signup/*"
          element={
            <StrictDevShell>
              <Suspended>
                <SignupPage />
              </Suspended>
            </StrictDevShell>
          }
        />
        <Route
          path="/onboarding"
          element={
            <StrictDevShell>
              <ProtectedRoute>
                <Suspended>
                  <OnboardingPage />
                </Suspended>
              </ProtectedRoute>
            </StrictDevShell>
          }
        />
        <Route
          path="*"
          element={
            <StrictDevShell>
              <ProtectedRoute>
                <RequireOnboarding>
                  <Suspended>
                    <Shell>
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
                      <Route
                        path="/settings/notifications"
                        element={<Navigate to="/settings?section=notifications" replace />}
                      />
                      <Route path="/professionals/teachers" element={<TeachersPage />} />
                      <Route path="/professionals/teachers/specialties" element={<SpecialtiesPage />} />
                      <Route path="/professionals/teachers/intakes" element={<IntakesPage />} />
                      <Route path="/professionals/teachers/sessions" element={<SessionsPage />} />
                      <Route path="/professionals/teachers/public" element={<PublicPreviewPage />} />
                      <Route path="/workshops/auto-repair/vehicles" element={<AutoRepairVehiclesPage />} />
                      <Route path="/workshops/auto-repair/services" element={<AutoRepairServicesPage />} />
                      <Route
                        path="/workshops/auto-repair/orders/*"
                        element={<Navigate to="/modules/workOrders" replace />}
                      />
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
                      <Route path="/audit" element={<Suspended><AdminPage section="audit" /></Suspended>} />
                      <Route path="/roles" element={<Suspended><AdminPage section="rbac" /></Suspended>} />
                      <Route path="/dashboard" element={<DashboardVisualPage />} />
                      <Route path="/dashboard/widgets" element={<DashboardPage />} />
                      <Route path="/calendar" element={<CalendarPage />} />
                      </Routes>
                    </Shell>
                  </Suspended>
                </RequireOnboarding>
              </ProtectedRoute>
            </StrictDevShell>
          }
        />
      </Routes>
    </>
  );
}
