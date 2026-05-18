import { useEffect, useState, type PropsWithChildren } from 'react';
import { Link, Navigate, useLocation } from 'react-router-dom';
import { SignIn, SignUp, useAuth } from '@clerk/react';
import { registerTokenProvider } from '@devpablocristo/platform-authn/http/fetch';
import { createClerkTokenProvider } from '@devpablocristo/platform-authn/providers/clerk';
import { clerkEnabled } from '../lib/auth';
import { useI18n } from '../lib/i18n';

export type TokenProvider = () => Promise<string | null>;
export type TokenProviderRegistrar = (provider: TokenProvider) => void;

const defaultRegisterProviders: TokenProviderRegistrar[] = [registerTokenProvider];

export function SharedAuthTokenBridge({
  registerProviders = defaultRegisterProviders,
}: {
  registerProviders?: TokenProviderRegistrar[];
}) {
  if (!clerkEnabled) {
    return <LocalAuthTokenBridge registerProviders={registerProviders} />;
  }

  return <ClerkAuthTokenBridge registerProviders={registerProviders} />;
}

function LocalAuthTokenBridge({ registerProviders }: { registerProviders: TokenProviderRegistrar[] }) {
  useEffect(() => {
    const provider = async () => null;
    registerProviders.forEach((registerProvider) => registerProvider(provider));
  }, [registerProviders]);

  return null;
}

function ClerkAuthTokenBridge({ registerProviders }: { registerProviders: TokenProviderRegistrar[] }) {
  const { getToken } = useAuth();

  useEffect(() => {
    const provider = createClerkTokenProvider(() => getToken({ skipCache: true }));
    registerProviders.forEach((registerProvider) => registerProvider(provider));
  }, [getToken, registerProviders]);

  return null;
}

export function SharedProtectedRoute({ children }: PropsWithChildren) {
  if (!clerkEnabled) {
    return <>{children}</>;
  }

  return <ClerkProtectedRoute>{children}</ClerkProtectedRoute>;
}

const clerkLoadTimeoutMs = 15_000;

function ClerkProtectedRoute({ children }: PropsWithChildren) {
  const { isLoaded, isSignedIn } = useAuth();
  const location = useLocation();
  const { t } = useI18n();
  const [loadTimedOut, setLoadTimedOut] = useState(false);

  useEffect(() => {
    if (isLoaded) {
      return;
    }
    const id = window.setTimeout(() => setLoadTimedOut(true), clerkLoadTimeoutMs);
    return () => window.clearTimeout(id);
  }, [isLoaded]);

  if (!isLoaded) {
    return (
      <div className="app-layout">
        <div className="main-content">
          {loadTimedOut ? (
            <div className="auth-card auth-state-card">
              <h1 className="auth-state-title">{t('auth.clerk.loadTimeout.title')}</h1>
              <p className="auth-state-body">{t('auth.clerk.loadTimeout.hint')}</p>
            </div>
          ) : (
            <div className="spinner" />
          )}
        </div>
      </div>
    );
  }
  if (!isSignedIn) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }
  return <>{children}</>;
}

export function SharedLoginPage() {
  const { t } = useI18n();
  const location = useLocation();
  const { isLoaded, isSignedIn } = useAuth();
  const redirectUrl = authRedirectURLFromLocation(location);
  if (clerkEnabled) {
    if (isClerkOrganizationTaskPath(location.pathname)) {
      if (!isLoaded) {
        return (
          <div className="auth-layout">
            <div className="spinner" aria-label="Cargando" />
          </div>
        );
      }
      if (isSignedIn) {
        return <Navigate to="/onboarding" replace />;
      }
    }
    return (
      <div className="auth-layout">
        <SignIn
          routing="path"
          path="/login"
          signUpUrl={redirectUrl ? `/signup?redirect_url=${encodeURIComponent(redirectUrl)}` : '/signup'}
          fallbackRedirectUrl={redirectUrl || undefined}
          forceRedirectUrl={redirectUrl || undefined}
          signUpFallbackRedirectUrl={redirectUrl || undefined}
          signUpForceRedirectUrl={redirectUrl || undefined}
        />
      </div>
    );
  }
  return (
    <div className="auth-layout">
      <div className="auth-card">
        <h1>{t('auth.login.localTitle')}</h1>
        <p>{t('auth.login.localDescription')}</p>
        <Link to="/">{t('auth.goPanel')}</Link>
      </div>
    </div>
  );
}

export function SharedSignupPage() {
  const { t } = useI18n();
  const location = useLocation();
  const { isLoaded, isSignedIn } = useAuth();
  const redirectUrl = authRedirectURLFromLocation(location);
  if (clerkEnabled) {
    if (isClerkOrganizationTaskPath(location.pathname)) {
      if (!isLoaded) {
        return (
          <div className="auth-layout">
            <div className="spinner" aria-label="Cargando" />
          </div>
        );
      }
      if (isSignedIn) {
        return <Navigate to="/onboarding" replace />;
      }
    }
    return (
      <div className="auth-layout">
        <SignUp
          routing="path"
          path="/signup"
          signInUrl={redirectUrl ? `/login?redirect_url=${encodeURIComponent(redirectUrl)}` : '/login'}
          fallbackRedirectUrl={redirectUrl || undefined}
          forceRedirectUrl={redirectUrl || undefined}
          signInFallbackRedirectUrl={redirectUrl || undefined}
          signInForceRedirectUrl={redirectUrl || undefined}
        />
      </div>
    );
  }
  return (
    <div className="auth-layout">
      <div className="auth-card">
        <h1>{t('auth.signup.localTitle')}</h1>
        <p>{t('auth.signup.localDescription')}</p>
        <Link to="/">{t('auth.goPanel')}</Link>
      </div>
    </div>
  );
}

function isClerkOrganizationTaskPath(pathname: string): boolean {
  const path = pathname.toLowerCase();
  return path.includes('/tasks/choose-organization') || path.includes('/tasks/create-organization');
}

function authRedirectURLFromLocation(location: ReturnType<typeof useLocation>): string {
  const params = new URLSearchParams(location.search);
  const fromQuery = params.get('redirect_url')?.trim() ?? '';
  if (fromQuery.startsWith('/')) {
    return fromQuery;
  }
  const state = location.state as { from?: { pathname?: string; search?: string } } | null;
  const pathname = state?.from?.pathname?.trim() ?? '';
  if (!pathname.startsWith('/')) {
    return '';
  }
  return `${pathname}${state?.from?.search ?? ''}`;
}
