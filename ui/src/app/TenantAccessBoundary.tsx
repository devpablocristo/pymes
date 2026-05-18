import { useEffect, useMemo, useRef, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Navigate, useLocation } from 'react-router-dom';
import { getSession, getTenantSettings, registerTenantSlugProvider } from '../lib/api';
import { queryKeys, tenantSlugKey } from '../lib/queryKeys';
import { clearTenantProfile, syncTenantProfileFromSettings } from '../lib/tenantProfile';
import { TenantAccessProvider } from '../lib/TenantAccessProvider';
import type { TenantAccess } from '../lib/tenantAccessContext';

const RESERVED_TOP_LEVEL_PATHS = new Set(['login', 'signup', 'invite', 'onboarding']);

function pathTenantSlug(pathname: string): string | null {
  const segment = pathname.split('/').find((part) => part.trim() !== '')?.trim() ?? '';
  if (!segment || RESERVED_TOP_LEVEL_PATHS.has(segment)) {
    return null;
  }
  return segment;
}

function normalizeSlug(value: string | null | undefined): string {
  return (value ?? '').trim().toLowerCase();
}

function isAccessError(error: unknown): boolean {
  const status = typeof error === 'object' && error !== null && 'status' in error ? Number((error as { status?: unknown }).status) : 0;
  if (status === 401 || status === 403) {
    return true;
  }
  const message = error instanceof Error ? error.message : String(error);
  return /\b(401|403)\b|unauthorized|forbidden|tenant_mismatch|active tenant membership/i.test(message);
}

function isCurrentAccessQuery(queryKey: readonly unknown[], tenantSlug: string | null): boolean {
  if (!tenantSlug) {
    return false;
  }
  return queryKey[0] === 'tenant-slug' && queryKey[1] === tenantSlug;
}

function clearTenantScopedState(queryClient: ReturnType<typeof useQueryClient>, tenantSlug: string | null = null): void {
  clearTenantProfile();
  try {
    queryClient.removeQueries({
      predicate: (query) => !isCurrentAccessQuery(query.queryKey, tenantSlug),
    });
  } catch {
    // Query cache cleanup is best-effort state hygiene; access denial is still rendered.
  }
  if (typeof window === 'undefined') {
    return;
  }
  for (const storage of [window.sessionStorage, window.localStorage]) {
    for (const key of Object.keys(storage)) {
      if (/tenant|branch|pymes/i.test(key)) {
        storage.removeItem(key);
      }
    }
  }
}

function TenantAccessDenied({ tenantSlug }: { tenantSlug: string | null }) {
  return (
    <main className="auth-page">
      <section className="auth-card" role="alert">
        <h1>Acceso al tenant denegado</h1>
        <p className="text-muted">
          La sesión actual no tiene una membresía activa para {tenantSlug ? <strong>{tenantSlug}</strong> : 'este tenant'}.
        </p>
        <p className="text-muted">Cerrá sesión o cambiá al tenant correcto desde tu proveedor de identidad.</p>
      </section>
    </main>
  );
}

function TenantAccessCleanup({ tenantSlug, children }: { tenantSlug: string | null; children: ReactNode }) {
  const queryClient = useQueryClient();
  const clearedRef = useRef(false);
  useEffect(() => {
    if (clearedRef.current) {
      return;
    }
    clearedRef.current = true;
    clearTenantScopedState(queryClient, tenantSlug);
  }, [queryClient, tenantSlug]);
  return <>{children ?? <TenantAccessDenied tenantSlug={tenantSlug} />}</>;
}

export function TenantAccessBoundary({ children }: { children: ReactNode }) {
  const location = useLocation();
  const queryClient = useQueryClient();
  const requestedSlug = pathTenantSlug(location.pathname);
  const normalizedRequestedSlug = normalizeSlug(requestedSlug);
  const previousTenantIdRef = useRef<string | null>(null);

  const sessionQuery = useQuery({
    queryKey: tenantSlugKey(requestedSlug ?? 'none', queryKeys.session.current),
    queryFn: () => getSession({ tenantSlug: requestedSlug, skipTenantSlug: !requestedSlug }),
    enabled: Boolean(requestedSlug),
    staleTime: 60_000,
    retry: false,
  });

  const sessionTenantSlug = normalizeSlug(sessionQuery.data?.tenant?.slug ?? null);
  const sessionTenantId = sessionQuery.data?.tenant?.id ?? sessionQuery.data?.auth.org_id ?? '';
  const slugMatchesSession = Boolean(
    normalizedRequestedSlug && sessionTenantSlug && normalizedRequestedSlug === sessionTenantSlug,
  );

  const settingsQuery = useQuery({
    queryKey: sessionTenantId
      ? ['tenant', sessionTenantId, ...queryKeys.tenant.settings]
      : tenantSlugKey(requestedSlug ?? 'none', queryKeys.tenant.settings),
    queryFn: () => getTenantSettings({ tenantSlug: requestedSlug }),
    enabled: Boolean(requestedSlug && sessionQuery.data && slugMatchesSession),
    staleTime: 60_000,
    retry: false,
  });

  useEffect(() => {
    if (!settingsQuery.data) {
      return;
    }
    syncTenantProfileFromSettings(settingsQuery.data);
  }, [settingsQuery.data]);

  const access = useMemo<TenantAccess | null>(() => {
    if (!sessionQuery.data || !settingsQuery.data || !requestedSlug || !slugMatchesSession || !sessionTenantId) {
      return null;
    }
    return {
      tenantId: sessionTenantId,
      tenantSlug: sessionTenantSlug,
      tenantName: sessionQuery.data.tenant?.name ?? sessionQuery.data.auth.tenant_name ?? sessionTenantSlug,
      role: sessionQuery.data.membership?.role ?? sessionQuery.data.auth.role,
      session: sessionQuery.data,
      settings: settingsQuery.data,
    };
  }, [requestedSlug, sessionQuery.data, sessionTenantId, sessionTenantSlug, settingsQuery.data, slugMatchesSession]);

  useEffect(() => {
    if (!access) {
      return;
    }
    const previousTenantId = previousTenantIdRef.current;
    if (previousTenantId && previousTenantId !== access.tenantId) {
      clearTenantScopedState(queryClient, access.tenantSlug);
    }
    previousTenantIdRef.current = access.tenantId;
  }, [access, queryClient]);

  useEffect(() => {
    if (!access) {
      return undefined;
    }
    return registerTenantSlugProvider(() => access.tenantSlug);
  }, [access]);

  if (!requestedSlug) {
    return <Navigate to="/onboarding" replace />;
  }

  if (sessionQuery.isPending || (sessionQuery.data && slugMatchesSession && settingsQuery.isPending)) {
    return <div className="spinner" aria-label="Cargando" />;
  }

  if (
    sessionQuery.isError ||
    settingsQuery.isError ||
    !sessionQuery.data ||
    !sessionTenantSlug ||
    !slugMatchesSession
  ) {
    return (
      <TenantAccessCleanup tenantSlug={requestedSlug}>
        <TenantAccessDenied tenantSlug={requestedSlug} />
      </TenantAccessCleanup>
    );
  }

  if (!settingsQuery.data?.onboarding_completed_at) {
    clearTenantProfile();
    return <Navigate to="/onboarding" replace />;
  }

  if (!access || isAccessError(sessionQuery.error) || isAccessError(settingsQuery.error)) {
    return (
      <TenantAccessCleanup tenantSlug={requestedSlug}>
        <TenantAccessDenied tenantSlug={requestedSlug} />
      </TenantAccessCleanup>
    );
  }

  return <TenantAccessProvider value={access}>{children}</TenantAccessProvider>;
}
