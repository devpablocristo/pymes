import { Route, Routes, Navigate } from 'react-router-dom';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ClerkSessionOrgSync } from '../components/ClerkSessionOrgSync';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { Shell } from '../components/Shell';
import { AdminPage } from '../pages/AdminPage';
import { CommercialAssistantPage } from '../pages/CommercialAssistantPage';
import { DashboardPage } from '../pages/DashboardPage';
import { AutoRepairServicesPage } from '../pages/AutoRepairServicesPage';
import { AutoRepairVehiclesPage } from '../pages/AutoRepairVehiclesPage';
import { AutoRepairWorkOrdersPage } from '../pages/AutoRepairWorkOrdersPage';
import { BeautySalonServicesPage } from '../pages/BeautySalonServicesPage';
import { BeautyStaffPage } from '../pages/BeautyStaffPage';
import { RestaurantDiningAreasPage } from '../pages/RestaurantDiningAreasPage';
import { RestaurantDiningTablesPage } from '../pages/RestaurantDiningTablesPage';
import { RestaurantTableSessionsPage } from '../pages/RestaurantTableSessionsPage';
import { IntakesPage } from '../pages/IntakesPage';
import { LoginPage } from '../pages/LoginPage';
import { CustomersPage } from '../pages/CustomersPage';
import { PurchasesPage } from '../pages/PurchasesPage';
import { ModulePage } from '../pages/ModulePage';
import { NotificationPreferencesPage } from '../pages/NotificationPreferencesPage';
import { OnboardingPage } from '../pages/OnboardingPage';
import { PublicPreviewPage } from '../pages/PublicPreviewPage';
import { SessionsPage } from '../pages/SessionsPage';
import { SettingsPage } from '../pages/SettingsPage';
import { SignupPage } from '../pages/SignupPage';
import { SpecialtiesPage } from '../pages/SpecialtiesPage';
import { TeachersPage } from '../pages/TeachersPage';
import { clerkEnabled } from '../lib/auth';
import { hasCompletedOnboarding } from '../lib/tenantProfile';

function RequireOnboarding({ children }: { children: React.ReactNode }) {
  if (!hasCompletedOnboarding()) {
    return <Navigate to="/onboarding" replace />;
  }
  return <>{children}</>;
}

export function App() {
  return (
    <>
      <AuthTokenBridge />
      {clerkEnabled && <ClerkSessionOrgSync />}
      <Routes>
        {/* Clerk (path routing) usa subrutas: /login/tasks/choose-organization, etc. */}
        <Route path="/login/*" element={<LoginPage />} />
        <Route path="/signup/*" element={<SignupPage />} />
        <Route
          path="/onboarding"
          element={
            <ProtectedRoute>
              <OnboardingPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="*"
          element={
            <ProtectedRoute>
              <RequireOnboarding>
                <Shell>
                  <Routes>
                    <Route path="/" element={<DashboardPage />} />
                    <Route path="/assistant/commercial" element={<CommercialAssistantPage />} />
                    <Route path="/admin" element={<AdminPage />} />
                    <Route path="/billing" element={<Navigate to="/settings#facturacion" replace />} />
                    <Route path="/modules/customers" element={<CustomersPage />} />
                    <Route path="/modules/purchases" element={<PurchasesPage />} />
                    <Route path="/compras" element={<PurchasesPage />} />
                    <Route path="/purchases" element={<Navigate to="/compras" replace />} />
                    <Route path="/modules/:moduleId" element={<ModulePage />} />
                    <Route path="/settings" element={<SettingsPage />} />
                    <Route path="/settings/keys" element={<Navigate to="/settings" replace />} />
                    <Route
                      path="/settings/notifications"
                      element={<NotificationPreferencesPage />}
                    />
                    <Route path="/professionals" element={<Navigate to="/professionals/teachers" replace />} />
                    <Route path="/specialties" element={<Navigate to="/professionals/teachers/specialties" replace />} />
                    <Route path="/intakes" element={<Navigate to="/professionals/teachers/intakes" replace />} />
                    <Route path="/sessions" element={<Navigate to="/professionals/teachers/sessions" replace />} />
                    <Route path="/public" element={<Navigate to="/professionals/teachers/public" replace />} />
                    <Route path="/professionals/teachers" element={<TeachersPage />} />
                    <Route path="/professionals/teachers/specialties" element={<SpecialtiesPage />} />
                    <Route path="/professionals/teachers/intakes" element={<IntakesPage />} />
                    <Route path="/professionals/teachers/sessions" element={<SessionsPage />} />
                    <Route path="/professionals/teachers/public" element={<PublicPreviewPage />} />
                    <Route path="/workshops" element={<Navigate to="/workshops/auto-repair/vehicles" replace />} />
                    <Route path="/workshops/vehicles" element={<Navigate to="/workshops/auto-repair/vehicles" replace />} />
                    <Route path="/workshops/services" element={<Navigate to="/workshops/auto-repair/services" replace />} />
                    <Route path="/workshops/orders" element={<Navigate to="/workshops/auto-repair/orders" replace />} />
                    <Route path="/workshops/auto-repair/vehicles" element={<AutoRepairVehiclesPage />} />
                    <Route path="/workshops/auto-repair/services" element={<AutoRepairServicesPage />} />
                    <Route path="/workshops/auto-repair/orders" element={<AutoRepairWorkOrdersPage />} />
                    <Route path="/beauty" element={<Navigate to="/beauty/salon/staff" replace />} />
                    <Route path="/beauty/salon/staff" element={<BeautyStaffPage />} />
                    <Route path="/beauty/salon/services" element={<BeautySalonServicesPage />} />
                    <Route path="/restaurants" element={<Navigate to="/restaurants/dining/areas" replace />} />
                    <Route path="/restaurants/dining/areas" element={<RestaurantDiningAreasPage />} />
                    <Route path="/restaurants/dining/tables" element={<RestaurantDiningTablesPage />} />
                    <Route path="/restaurants/dining/sessions" element={<RestaurantTableSessionsPage />} />
                  </Routes>
                </Shell>
              </RequireOnboarding>
            </ProtectedRoute>
          }
        />
      </Routes>
    </>
  );
}
