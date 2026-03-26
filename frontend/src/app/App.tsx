import { StrictMode, Suspense, lazy, type ReactNode } from 'react';
import { Route, Routes, Navigate, useLocation } from 'react-router-dom';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ClerkSessionOrgSync } from '../components/ClerkSessionOrgSync';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { clerkEnabled } from '../lib/auth';
import { hasCompletedOnboarding } from '../lib/tenantProfile';
import { wowdashOpenInDev } from '../lib/wowdashDevAccess';

const Shell = lazy(() => import('../components/Shell').then((mod) => ({ default: mod.Shell })));
const AdminPage = lazy(() => import('../pages/AdminPage').then((mod) => ({ default: mod.AdminPage })));
const AutoRepairServicesPage = lazy(() => import('../pages/AutoRepairServicesPage').then((mod) => ({ default: mod.AutoRepairServicesPage })));
const AutoRepairVehiclesPage = lazy(() => import('../pages/AutoRepairVehiclesPage').then((mod) => ({ default: mod.AutoRepairVehiclesPage })));
const AutoRepairWorkOrdersPage = lazy(() => import('../pages/AutoRepairWorkOrdersPage').then((mod) => ({ default: mod.AutoRepairWorkOrdersPage })));
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
const CommercialAssistantPage = lazy(() => import('../pages/CommercialAssistantPage').then((mod) => ({ default: mod.CommercialAssistantPage })));
const CustomersPage = lazy(() => import('../pages/CustomersPage').then((mod) => ({ default: mod.CustomersPage })));
const DashboardPage = lazy(() => import('../pages/DashboardPage').then((mod) => ({ default: mod.DashboardPage })));
const IntakesPage = lazy(() => import('../pages/IntakesPage').then((mod) => ({ default: mod.IntakesPage })));
const LoginPage = lazy(() => import('../pages/LoginPage').then((mod) => ({ default: mod.LoginPage })));
const ModulePage = lazy(() => import('../pages/ModulePage').then((mod) => ({ default: mod.ModulePage })));
const NotificationPreferencesPage = lazy(() => import('../pages/NotificationPreferencesPage').then((mod) => ({ default: mod.NotificationPreferencesPage })));
const OnboardingPage = lazy(() => import('../pages/OnboardingPage').then((mod) => ({ default: mod.OnboardingPage })));
const PublicPreviewPage = lazy(() => import('../pages/PublicPreviewPage').then((mod) => ({ default: mod.PublicPreviewPage })));
const PurchasesPage = lazy(() => import('../pages/PurchasesPage').then((mod) => ({ default: mod.PurchasesPage })));
const RestaurantDiningAreasPage = lazy(() => import('../pages/RestaurantDiningAreasPage').then((mod) => ({ default: mod.RestaurantDiningAreasPage })));
const RestaurantDiningTablesPage = lazy(() => import('../pages/RestaurantDiningTablesPage').then((mod) => ({ default: mod.RestaurantDiningTablesPage })));
const RestaurantTableSessionsPage = lazy(() => import('../pages/RestaurantTableSessionsPage').then((mod) => ({ default: mod.RestaurantTableSessionsPage })));
const SessionsPage = lazy(() => import('../pages/SessionsPage').then((mod) => ({ default: mod.SessionsPage })));
const SettingsPage = lazy(() => import('../pages/SettingsPage').then((mod) => ({ default: mod.SettingsPage })));
const SignupPage = lazy(() => import('../pages/SignupPage').then((mod) => ({ default: mod.SignupPage })));
const SpecialtiesPage = lazy(() => import('../pages/SpecialtiesPage').then((mod) => ({ default: mod.SpecialtiesPage })));
const TeachersPage = lazy(() => import('../pages/TeachersPage').then((mod) => ({ default: mod.TeachersPage })));
const AutomationRulesPage = lazy(() => import('../pages/AutomationRulesPage'));
const ApprovalInboxPage = lazy(() => import('../pages/ApprovalInboxPage'));
const WatcherConfigPage = lazy(() => import('../pages/WatcherConfigPage'));
const WowdashTemplateLabsPage = lazy(() =>
  import('../pages/WowdashTemplateLabsPage').then((mod) => ({ default: mod.WowdashTemplateLabsPage })),
);

function Suspended({ children }: { children: ReactNode }) {
  return <Suspense fallback={<div className="card"><p>Cargando…</p></div>}>{children}</Suspense>;
}

function RequireOnboarding({ children }: { children: React.ReactNode }) {
  if (!hasCompletedOnboarding()) {
    return <Navigate to="/onboarding" replace />;
  }
  return <>{children}</>;
}

/** Fuera de StrictMode el laboratorio Wowdash (ApexCharts no tolera el doble efecto de dev). */
function StrictDevShell({ children }: { children: ReactNode }) {
  return <StrictMode>{children}</StrictMode>;
}

/** Ruta canónica del template: `/console/wowdash/*` (misma sesión que el panel). */
function LabsWowdashRedirect() {
  const { pathname, search, hash } = useLocation();
  const to = `${pathname.replace(/^\/labs\/wowdash/, '/console/wowdash')}${search}${hash}`;
  return <Navigate to={to} replace />;
}

function WowdashConsoleEntry() {
  return (
    <Suspended>
      <WowdashTemplateLabsPage />
    </Suspended>
  );
}

export function App() {
  const wowdashDev = wowdashOpenInDev();
  return (
    <>
      <AuthTokenBridge />
      {clerkEnabled && <ClerkSessionOrgSync />}
      <Routes>
        {/* Clerk (path routing) usa subrutas: /login/tasks/choose-organization, etc. */}
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
          path="/labs/wowdash/*"
          element={
            wowdashDev ? (
              <LabsWowdashRedirect />
            ) : (
              <ProtectedRoute>
                <RequireOnboarding>
                  <LabsWowdashRedirect />
                </RequireOnboarding>
              </ProtectedRoute>
            )
          }
        />
        <Route
          path="/console/wowdash/*"
          element={
            wowdashDev ? (
              <WowdashConsoleEntry />
            ) : (
              <ProtectedRoute>
                <RequireOnboarding>
                  <WowdashConsoleEntry />
                </RequireOnboarding>
              </ProtectedRoute>
            )
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
                      <Route path="/" element={<DashboardPage />} />
                      <Route path="/assistant/commercial" element={<CommercialAssistantPage />} />
                      <Route path="/admin" element={<AdminPage />} />
                      <Route path="/billing" element={<Navigate to="/settings#facturacion" replace />} />
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
                      <Route path="/settings" element={<SettingsPage />} />
                      <Route path="/settings/keys" element={<Navigate to="/settings" replace />} />
                      <Route
                        path="/settings/notifications"
                        element={<NotificationPreferencesPage />}
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
                      <Route path="/beauty/salon/staff" element={<BeautyStaffPage />} />
                      <Route path="/beauty/salon/services" element={<BeautySalonServicesPage />} />
                      <Route path="/restaurants/dining/areas" element={<RestaurantDiningAreasPage />} />
                      <Route path="/restaurants/dining/tables" element={<RestaurantDiningTablesPage />} />
                      <Route path="/restaurants/dining/sessions" element={<RestaurantTableSessionsPage />} />
                      <Route path="/automation-rules" element={<AutomationRulesPage />} />
                      <Route path="/approvals" element={<ApprovalInboxPage />} />
                      <Route path="/watcher-config" element={<WatcherConfigPage />} />
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
