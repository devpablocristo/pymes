import { StrictMode, useEffect, type ReactNode } from 'react';
import { Route, Routes, Navigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ClerkSessionOrgSync } from '../components/ClerkSessionOrgSync';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { clerkEnabled } from '../lib/auth';
import { getTenantSettings } from '../lib/api';
import { queryKeys } from '../lib/queryKeys';
import { syncTenantProfileFromSettings } from '../lib/tenantProfile';
import { LoginPage, OnboardingPage, Shell, SignupPage } from './lazyRoutes';
import { ShellRoutes } from './ShellRoutes';
import { Suspended } from './suspended';

function RequireOnboarding({ children }: { children: ReactNode }) {
  const tenantSettingsQuery = useQuery({
    queryKey: queryKeys.tenant.settings,
    queryFn: getTenantSettings,
    staleTime: 60_000,
    retry: false,
  });
  const tenantSettings = tenantSettingsQuery.data;

  useEffect(() => {
    if (!tenantSettings?.onboarding_completed_at) {
      return;
    }
    syncTenantProfileFromSettings(tenantSettings);
  }, [tenantSettings]);

  if (tenantSettingsQuery.isPending) {
    return <div className="spinner" aria-label="Cargando" />;
  }

  if (tenantSettingsQuery.isError || !tenantSettings) {
    return <Navigate to="/onboarding" replace />;
  }

  if (!tenantSettings.onboarding_completed_at) {
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
                      <ShellRoutes />
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
