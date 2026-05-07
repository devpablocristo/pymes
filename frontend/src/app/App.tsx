import { StrictMode, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useAuth, useOrganizationList, useSession } from '@clerk/react';
import { Route, Routes, Navigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { clerkEnabled } from '../lib/auth';
import { getTenantSettings } from '../lib/api';
import { BranchProvider } from '../lib/branchContext';
import { queryKeys } from '../lib/queryKeys';
import { hasCompletedOnboarding, syncTenantProfileFromSettings } from '../lib/tenantProfile';
import { InviteAcceptPage, LoginPage, OnboardingPage, Shell, SignupPage } from './lazyRoutes';
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

function RequireActiveTenant({ children }: { children: ReactNode }) {
  const { isLoaded: authLoaded, isSignedIn, orgId } = useAuth();
  const { session } = useSession();
  const {
    isLoaded: listLoaded,
    setActive,
    userMemberships,
  } = useOrganizationList({
    userMemberships: { pageSize: 50 },
  });
  const [switchingTenantID, setSwitchingTenantID] = useState('');
  const memberships = useMemo(() => userMemberships.data ?? [], [userMemberships.data]);

  const activateTenant = useCallback(
    async (tenantID: string) => {
      if (!tenantID || !setActive) {
        return;
      }
      setSwitchingTenantID(tenantID);
      try {
        await setActive({ organization: tenantID });
        await session?.reload();
      } finally {
        setSwitchingTenantID('');
      }
    },
    [session, setActive],
  );

  useEffect(() => {
    if (!authLoaded || !listLoaded || !isSignedIn || orgId || userMemberships.isLoading) {
      return;
    }
    const onlyTenantID = memberships.length === 1 ? (memberships[0]?.organization?.id ?? '') : '';
    if (!onlyTenantID || switchingTenantID) {
      return;
    }
    void activateTenant(onlyTenantID);
  }, [
    activateTenant,
    authLoaded,
    isSignedIn,
    listLoaded,
    memberships,
    orgId,
    switchingTenantID,
    userMemberships.isLoading,
  ]);

  if (!authLoaded || !listLoaded || userMemberships.isLoading || switchingTenantID) {
    return <div className="spinner" aria-label="Cargando" />;
  }

  if (!isSignedIn || orgId) {
    return <>{children}</>;
  }

  if (memberships.length === 0) {
    return <Navigate to="/onboarding" replace />;
  }

  return (
    <main className="auth-page">
      <section className="auth-card">
        <h1>Elegí un tenant</h1>
        <p className="text-muted">Tu cuenta pertenece a varios tenants. Elegí con cuál querés trabajar.</p>
        <div className="profile-org-switcher__list">
          {memberships.map((membership) => {
            const tenantID = membership.organization?.id ?? '';
            const tenantName = membership.organization?.name?.trim() || tenantID || 'Tenant';
            return (
              <button
                key={membership.id}
                type="button"
                className="btn-secondary"
                disabled={!tenantID || switchingTenantID !== ''}
                onClick={() => {
                  void activateTenant(tenantID);
                }}
              >
                {tenantName}
              </button>
            );
          })}
        </div>
      </section>
    </main>
  );
}

/** En desarrollo mantenemos StrictMode sobre toda la consola protegida. */
function StrictDevShell({ children }: { children: ReactNode }) {
  return <StrictMode>{children}</StrictMode>;
}

export function App() {
  return (
    <>
      <AuthTokenBridge />
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
          path="/invite/accept"
          element={
            <StrictDevShell>
              <ProtectedRoute>
                <Suspended>
                  <InviteAcceptPage />
                </Suspended>
              </ProtectedRoute>
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
                {clerkEnabled ? (
                  <RequireActiveTenant>
                    <RequireOnboarding>
                      <BranchProvider>
                        <Suspended>
                          <Shell>
                            <ShellRoutes />
                          </Shell>
                        </Suspended>
                      </BranchProvider>
                    </RequireOnboarding>
                  </RequireActiveTenant>
                ) : (
                  <RequireOnboarding>
                    <BranchProvider>
                      <Suspended>
                        <Shell>
                          <ShellRoutes />
                        </Shell>
                      </Suspended>
                    </BranchProvider>
                  </RequireOnboarding>
                )}
              </ProtectedRoute>
            </StrictDevShell>
          }
        />
      </Routes>
    </>
  );
}
