import { Route, Routes, Navigate } from 'react-router-dom';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { Shell } from '../components/Shell';
import { AdminPage } from '../pages/AdminPage';
import { APIKeysPage } from '../pages/APIKeysPage';
import { BillingPage } from '../pages/BillingPage';
import { DashboardPage } from '../pages/DashboardPage';
import { AutoRepairServicesPage } from '../pages/AutoRepairServicesPage';
import { AutoRepairVehiclesPage } from '../pages/AutoRepairVehiclesPage';
import { AutoRepairWorkOrdersPage } from '../pages/AutoRepairWorkOrdersPage';
import { BeautySalonServicesPage } from '../pages/BeautySalonServicesPage';
import { BeautyStaffPage } from '../pages/BeautyStaffPage';
import { IntakesPage } from '../pages/IntakesPage';
import { LoginPage } from '../pages/LoginPage';
import { CustomersPage } from '../pages/CustomersPage';
import { ModulePage } from '../pages/ModulePage';
import { NotificationPreferencesPage } from '../pages/NotificationPreferencesPage';
import { OnboardingPage } from '../pages/OnboardingPage';
import { PublicPreviewPage } from '../pages/PublicPreviewPage';
import { SessionsPage } from '../pages/SessionsPage';
import { SettingsPage } from '../pages/SettingsPage';
import { SignupPage } from '../pages/SignupPage';
import { SpecialtiesPage } from '../pages/SpecialtiesPage';
import { TeachersPage } from '../pages/TeachersPage';
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
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/signup" element={<SignupPage />} />
        <Route path="/onboarding" element={<OnboardingPage />} />
        <Route
          path="*"
          element={
            <ProtectedRoute>
              <RequireOnboarding>
                <Shell>
                  <Routes>
                    <Route path="/" element={<DashboardPage />} />
                    <Route path="/admin" element={<AdminPage />} />
                    <Route path="/billing" element={<BillingPage />} />
                    <Route path="/modules/customers" element={<CustomersPage />} />
                    <Route path="/modules/:moduleId" element={<ModulePage />} />
                    <Route path="/settings" element={<SettingsPage />} />
                    <Route path="/settings/keys" element={<APIKeysPage />} />
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
