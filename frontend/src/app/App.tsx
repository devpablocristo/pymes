import { StrictMode, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useAuth, useOrganizationList, useSession } from '@clerk/react';
import { Route, Routes, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { clerkEnabled } from '../lib/auth';
import { BranchProvider } from '../lib/branchContext';
import { clearTenantProfile } from '../lib/tenantProfile';
import { InviteAcceptPage, LoginPage, OnboardingPage, Shell, SignupPage } from './lazyRoutes';
import { ShellRoutes } from './ShellRoutes';
import { Suspended } from './suspended';
import { TenantAccessBoundary } from './TenantAccessBoundary';

const tenantActivationTimeoutMs = 15_000;

export function searchWithoutActivateOrg(search: string): string {
  const params = new URLSearchParams(search);
  params.delete('activate_org');
  const query = params.toString();
  return query ? `?${query}` : '';
}

function withTimeout<T>(promise: Promise<T>, timeoutMs: number, message: string): Promise<T> {
  return new Promise((resolve, reject) => {
    const timeout = window.setTimeout(() => reject(new Error(message)), timeoutMs);
    promise.then(
      (value) => {
        window.clearTimeout(timeout);
        resolve(value);
      },
      (error) => {
        window.clearTimeout(timeout);
        reject(error);
      },
    );
  });
}

function messageFromError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  if (typeof error === 'string' && error.trim()) {
    return error;
  }
  return fallback;
}

function RequireActiveTenant({ children }: { children: ReactNode }) {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
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
  const [activationError, setActivationError] = useState('');
  const activateFromURLInFlightRef = useRef(false);
  const memberships = useMemo(() => userMemberships.data ?? [], [userMemberships.data]);
  const pendingActivateOrgID = useMemo(() => {
    return new URLSearchParams(location.search).get('activate_org')?.trim() ?? '';
  }, [location.search]);
  const hasTenantSlugInPath = useMemo(() => {
    const firstSegment = location.pathname.split('/').find((part) => part.trim() !== '')?.trim() ?? '';
    return Boolean(firstSegment && !['login', 'signup', 'invite', 'onboarding'].includes(firstSegment));
  }, [location.pathname]);

  const activateTenant = useCallback(
    async (tenantID: string) => {
      if (!tenantID || !setActive) {
        return;
      }
      setSwitchingTenantID(tenantID);
      try {
        clearTenantProfile();
        queryClient.clear();
        await withTimeout(
          setActive({ organization: tenantID }),
          tenantActivationTimeoutMs,
          'No se pudo activar el tenant en Clerk.',
        );
        await withTimeout(
          (session?.reload() ?? Promise.resolve()).then(() => undefined),
          tenantActivationTimeoutMs,
          'No se pudo refrescar la sesión del tenant.',
        );
      } finally {
        setSwitchingTenantID('');
      }
    },
    [queryClient, session, setActive],
  );

  useEffect(() => {
    if (!authLoaded || !isSignedIn || !pendingActivateOrgID || activateFromURLInFlightRef.current) {
      return;
    }
    activateFromURLInFlightRef.current = true;
    setActivationError('');
    void activateTenant(pendingActivateOrgID)
      .then(() => {
        navigate(
          {
            pathname: location.pathname,
            search: searchWithoutActivateOrg(location.search),
            hash: location.hash,
          },
          { replace: true },
        );
      })
      .catch((error) => {
        setActivationError(messageFromError(error, 'No se pudo activar el tenant.'));
      })
      .finally(() => {
        activateFromURLInFlightRef.current = false;
      });
  }, [
    activateTenant,
    authLoaded,
    isSignedIn,
    location.hash,
    location.pathname,
    location.search,
    navigate,
    pendingActivateOrgID,
  ]);

  useEffect(() => {
    if (pendingActivateOrgID) {
      return;
    }
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
    pendingActivateOrgID,
    switchingTenantID,
    userMemberships.isLoading,
  ]);

  if (activationError && pendingActivateOrgID) {
    return (
      <main className="auth-page">
        <section className="auth-card" role="alert">
          <h1>No se pudo activar el tenant</h1>
          <p className="text-muted">{activationError}</p>
        </section>
      </main>
    );
  }

  if (!authLoaded || switchingTenantID || pendingActivateOrgID) {
    return <div className="spinner" aria-label="Cargando" />;
  }

  if (hasTenantSlugInPath) {
    return <>{children}</>;
  }

  if (!listLoaded || userMemberships.isLoading) {
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
              <Suspended>
                <InviteAcceptPage />
              </Suspended>
            </StrictDevShell>
          }
        />
        <Route
          path="/onboarding"
          element={
            <StrictDevShell>
              <Suspended>
                <OnboardingPage />
              </Suspended>
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
                    <TenantAccessBoundary>
                      <BranchProvider>
                        <Suspended>
                          <Shell>
                            <ShellRoutes />
                          </Shell>
                        </Suspended>
                      </BranchProvider>
                    </TenantAccessBoundary>
                  </RequireActiveTenant>
                ) : (
                  <TenantAccessBoundary>
                    <BranchProvider>
                      <Suspended>
                        <Shell>
                          <ShellRoutes />
                        </Shell>
                      </Suspended>
                    </BranchProvider>
                  </TenantAccessBoundary>
                )}
              </ProtectedRoute>
            </StrictDevShell>
          }
        />
      </Routes>
    </>
  );
}
