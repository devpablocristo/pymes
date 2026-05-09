import { StrictMode, useCallback, useEffect, useMemo, useRef, useState, type ReactNode, type FormEvent } from 'react';
import { useAuth, useClerk, useOrganizationList, useSession, useUser } from '@clerk/react';
import { Route, Routes, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { AuthTokenBridge } from '../components/AuthTokenBridge';
import { ProtectedRoute } from '../components/ProtectedRoute';
import { setInitialPassword } from '../lib/api';
import { clerkEnabled } from '../lib/auth';
import { BranchProvider } from '../lib/branchContext';
import { clearTenantProfile } from '../lib/tenantProfile';
import { InviteAcceptPage, LoginPage, OnboardingPage, Shell, SignupPage } from './lazyRoutes';
import { ShellRoutes } from './ShellRoutes';
import { Suspended } from './suspended';
import { TenantAccessBoundary } from './TenantAccessBoundary';

const tenantActivationTimeoutMs = 15_000;

export function searchWithoutActivateOrg(search: string): string {
  return searchWithoutKeys(search, ['activate_org']);
}

function searchWithoutKeys(search: string, keys: string[]): string {
  const params = new URLSearchParams(search);
  for (const key of keys) {
    params.delete(key);
  }
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
  const clerk = useClerk();
  const { isLoaded: authLoaded, isSignedIn, orgId } = useAuth();
  const { user, isLoaded: userLoaded } = useUser();
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
  const [wrongAccountForActivate, setWrongAccountForActivate] = useState(false);
  const activateFromURLInFlightRef = useRef(false);
  const memberships = useMemo(() => userMemberships.data ?? [], [userMemberships.data]);
  const pendingActivateOrgID = useMemo(() => {
    return new URLSearchParams(location.search).get('activate_org')?.trim() ?? '';
  }, [location.search]);
  const requirePasswordSetup = useMemo(() => {
    // Detectamos por DOS vías:
    // 1. Query `require_password=1` que setea el backend en /v1/tenant-invites/exchange.
    //    Este path se PIERDE si Clerk task `choose-organization` redirige al user
    //    por /onboarding antes de llegar al dashboard (OnboardingPage no preserva
    //    el query al hacer window.location.assign).
    // 2. user.passwordEnabled === false desde el SDK Clerk (lectura directa, robusta).
    //    Esto cubre el caso donde el query se perdió Y cualquier otro flow donde
    //    el invitado entró sin setear password.
    if (new URLSearchParams(location.search).get('require_password') === '1') {
      return true;
    }
    if (userLoaded && user && user.passwordEnabled === false) {
      return true;
    }
    return false;
  }, [location.search, userLoaded, user]);
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
    setWrongAccountForActivate(false);
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
    const isMemberOfPending = listLoaded
      ? memberships.some((m) => (m.organization?.id ?? '') === pendingActivateOrgID)
      : true;
    const looksLikeWrongAccount = listLoaded && !userMemberships.isLoading && !isMemberOfPending;
    const handleSignOutAndRetry = (): void => {
      const returnTo = location.pathname + location.search + location.hash;
      void clerk.signOut({ redirectUrl: returnTo }).catch(() => {
        window.location.assign(returnTo);
      });
    };
    if (looksLikeWrongAccount || wrongAccountForActivate) {
      return (
        <main className="auth-page">
          <section className="auth-card" role="alert">
            <h1>Esta invitación es para otra cuenta</h1>
            <p className="text-muted">
              La sesión actual no pertenece al tenant solicitado. Cerrá sesión y
              volvé a abrir el link de la invitación con la cuenta correcta.
            </p>
            <button type="button" className="btn-primary" onClick={handleSignOutAndRetry}>
              Cerrar sesión y reintentar
            </button>
          </section>
        </main>
      );
    }
    return (
      <main className="auth-page">
        <section className="auth-card" role="alert">
          <h1>No se pudo activar el tenant</h1>
          <p className="text-muted">{activationError}</p>
          <button type="button" className="btn-secondary" onClick={handleSignOutAndRetry}>
            Cerrar sesión
          </button>
        </section>
      </main>
    );
  }

  if (!authLoaded || switchingTenantID || pendingActivateOrgID) {
    return <div className="spinner" aria-label="Cargando" />;
  }

  if (requirePasswordSetup && isSignedIn) {
    console.info('[RequireActiveTenant] mostrando RequirePasswordView', {
      pathname: location.pathname,
      search: location.search,
    });
    return (
      <RequirePasswordView
        onDone={() => {
          navigate(
            {
              pathname: location.pathname,
              search: searchWithoutKeys(location.search, ['require_password']),
              hash: location.hash,
            },
            { replace: true },
          );
        }}
      />
    );
  }
  if (requirePasswordSetup) {
    // Diagnóstico: el query está pero algo bloquea el render (típico:
    // !isSignedIn por race en Clerk). Lo logueamos y dejamos que la app
    // siga al render normal — sin pantalla de password el invite quedaría
    // funcionalmente roto, pero al menos no se rompe la UI.
    console.warn('[RequireActiveTenant] require_password=1 pero render bloqueado', {
      isSignedIn,
      authLoaded,
      pendingActivateOrgID,
      switchingTenantID,
    });
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

function RequirePasswordView({ onDone }: { onDone: () => void }) {
  const { isLoaded, user } = useUser();
  const { session } = useSession();
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');

  // Si Clerk reporta que ya tiene password (caso edge: el query quedó pero
  // el user ya lo seteó en otra pestaña), saltamos la pantalla.
  useEffect(() => {
    if (isLoaded && user?.passwordEnabled) {
      onDone();
    }
  }, [isLoaded, user, onDone]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setErrorMsg('');
    if (password.length < 8) {
      setErrorMsg('La contraseña debe tener al menos 8 caracteres.');
      return;
    }
    if (password !== confirm) {
      setErrorMsg('Las contraseñas no coinciden.');
      return;
    }
    setSubmitting(true);
    try {
      // Delegamos al backend porque el SDK Clerk frontend rechaza cambios
      // sin "elevated auth" para users que llegaron por ticket. El backend
      // usa el secret key y puede setear password de un user que aún no
      // tiene una.
      await setInitialPassword(password);
      await session?.reload().catch(() => undefined);
      // Forzamos también un reload del user object del SDK para que
      // `user.passwordEnabled` refleje el nuevo estado en otros componentes.
      await user?.reload().catch(() => undefined);
      onDone();
    } catch (err) {
      const message = err instanceof Error && err.message ? err.message : 'No se pudo configurar la contraseña.';
      setErrorMsg(message);
    } finally {
      setSubmitting(false);
    }
  };

  if (!isLoaded) {
    return <div className="spinner" aria-label="Cargando" />;
  }

  const inputType = showPassword ? 'text' : 'password';

  return (
    <main className="auth-page">
      <section className="auth-card">
        <h1>Configurá tu contraseña</h1>
        <p className="text-muted">
          Antes de continuar, definí una contraseña para poder iniciar sesión a futuro.
        </p>
        <form onSubmit={handleSubmit} className="auth-form">
          <label>
            Contraseña nueva
            <input
              type={inputType}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
              minLength={8}
              required
              autoFocus
            />
          </label>
          <label>
            Confirmá contraseña
            <input
              type={inputType}
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              autoComplete="new-password"
              minLength={8}
              required
            />
          </label>
          <label className="checkbox-row" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <input
              type="checkbox"
              checked={showPassword}
              onChange={(e) => setShowPassword(e.target.checked)}
            />
            <span>Mostrar contraseña</span>
          </label>
          {errorMsg && <p role="alert" className="text-error">{errorMsg}</p>}
          <button type="submit" className="btn-primary" disabled={submitting}>
            {submitting ? 'Guardando…' : 'Guardar contraseña'}
          </button>
        </form>
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
