import { StrictMode, useEffect, type ReactNode } from 'react';
import { Route, Routes, Navigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ClerkSessionOrgSync } from '../components/ClerkSessionOrgSync';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { clerkEnabled } from '../lib/auth';
import { getTenantSettings } from '../lib/api';
import { BranchProvider } from '../lib/branchContext';
import { queryKeys } from '../lib/queryKeys';
import { hasCompletedOnboarding, syncTenantProfileFromSettings } from '../lib/tenantProfile';
import { LoginPage, OnboardingPage, Shell, SignupPage } from './lazyRoutes';
import { ShellRoutes } from './ShellRoutes';
import { Suspended } from './suspended';

function RequireOnboarding({ children }: { children: ReactNode }) {
  const localProfileExists = hasCompletedOnboarding();

  const tenantSettingsQuery = useQuery({
    queryKey: queryKeys.tenant.settings,
    queryFn: getTenantSettings,
    staleTime: 60_000,
    retry: 2,
    retryDelay: 1_000,
  });
  const tenantSettings = tenantSettingsQuery.data;

  useEffect(() => {
    if (!tenantSettings) {
      return;
    }
    syncTenantProfileFromSettings(tenantSettings);
  }, [tenantSettings]);

  // Si localStorage confirma onboarding completado, no bloquear la UI
  // mientras la API carga o si falla transitoriamente (hard refresh, token no listo).
  if (localProfileExists) {
    if (tenantSettingsQuery.isPending) {
      return <>{children}</>;
    }
  }

  // Sin perfil local, o con perfil local potencialmente viejo: depender de la API.
  if (tenantSettingsQuery.isPending) {
    return <div className="spinner" aria-label="Cargando" />;
  }

  if (tenantSettingsQuery.isError || !tenantSettings) {
    if (localProfileExists) {
      return <>{children}</>;
    }
    return (
      <div style={{ padding: '2rem', textAlign: 'center' }}>
        <p>No se pudo cargar la configuración del tenant.</p>
        <button type="button" onClick={() => tenantSettingsQuery.refetch()} style={{ marginTop: '1rem' }}>
          Reintentar
        </button>
      </div>
    );
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
                  <BranchProvider>
                    <Suspended>
                      <Shell>
                        <ShellRoutes />
                      </Shell>
                    </Suspended>
                  </BranchProvider>
                </RequireOnboarding>
              </ProtectedRoute>
            </StrictDevShell>
          }
        />
      </Routes>
    </>
  );
}
