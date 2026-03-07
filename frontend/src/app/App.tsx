import { Route, Routes } from 'react-router-dom';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { Shell } from '../components/Shell';
import { AdminPage } from '../pages/AdminPage';
import { APIKeysPage } from '../pages/APIKeysPage';
import { BillingPage } from '../pages/BillingPage';
import { DashboardPage } from '../pages/DashboardPage';
import { IntakesPage } from '../pages/IntakesPage';
import { LoginPage } from '../pages/LoginPage';
import { ModulePage } from '../pages/ModulePage';
import { NotificationPreferencesPage } from '../pages/NotificationPreferencesPage';
import { ProfessionalsPage } from '../pages/ProfessionalsPage';
import { PublicPreviewPage } from '../pages/PublicPreviewPage';
import { SessionsPage } from '../pages/SessionsPage';
import { SettingsPage } from '../pages/SettingsPage';
import { SignupPage } from '../pages/SignupPage';
import { SpecialtiesPage } from '../pages/SpecialtiesPage';

export function App() {
  return (
    <>
      <AuthTokenBridge />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/signup" element={<SignupPage />} />
        <Route
          path="*"
          element={
            <ProtectedRoute>
              <Shell>
                <Routes>
                  <Route path="/" element={<DashboardPage />} />
                  <Route path="/admin" element={<AdminPage />} />
                  <Route path="/billing" element={<BillingPage />} />
                  <Route path="/modules/:moduleId" element={<ModulePage />} />
                  <Route path="/settings" element={<SettingsPage />} />
                  <Route path="/settings/keys" element={<APIKeysPage />} />
                  <Route
                    path="/settings/notifications"
                    element={<NotificationPreferencesPage />}
                  />
                  <Route path="/professionals" element={<ProfessionalsPage />} />
                  <Route path="/specialties" element={<SpecialtiesPage />} />
                  <Route path="/intakes" element={<IntakesPage />} />
                  <Route path="/sessions" element={<SessionsPage />} />
                  <Route path="/public" element={<PublicPreviewPage />} />
                </Routes>
              </Shell>
            </ProtectedRoute>
          }
        />
      </Routes>
    </>
  );
}
