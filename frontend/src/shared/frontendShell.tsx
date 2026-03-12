import { useEffect, type PropsWithChildren, type ReactNode } from 'react';
import { Link, Navigate, NavLink, useLocation } from 'react-router-dom';
import { SignIn, SignUp, UserButton, useAuth } from '@clerk/clerk-react';
import { clerkEnabled } from '@pymes/ts-pkg/auth';
import { registerTokenProvider } from '@pymes/ts-pkg/http';
import { useI18n } from '../lib/i18n';

export type TokenProvider = () => Promise<string | null>;
export type TokenProviderRegistrar = (provider: TokenProvider) => void;

export function SharedAuthTokenBridge({
  registerProviders = [registerTokenProvider],
}: {
  registerProviders?: TokenProviderRegistrar[];
}) {
  if (!clerkEnabled) {
    return <LocalAuthTokenBridge registerProviders={registerProviders} />;
  }

  return <ClerkAuthTokenBridge registerProviders={registerProviders} />;
}

function LocalAuthTokenBridge({
  registerProviders,
}: {
  registerProviders: TokenProviderRegistrar[];
}) {
  useEffect(() => {
    const provider = async () => null;
    registerProviders.forEach((registerProvider) => registerProvider(provider));
  }, [registerProviders]);

  return null;
}

function ClerkAuthTokenBridge({
  registerProviders,
}: {
  registerProviders: TokenProviderRegistrar[];
}) {
  const { getToken } = useAuth();

  useEffect(() => {
    const provider = async () => (await getToken()) ?? null;
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

function ClerkProtectedRoute({ children }: PropsWithChildren) {
  const { isLoaded, isSignedIn } = useAuth();
  const location = useLocation();
  if (!isLoaded) {
    return (
      <div className="app-layout">
        <div className="main-content">
          <div className="spinner" />
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
  if (clerkEnabled) {
    return (
      <div className="auth-layout">
        <SignIn routing="path" path="/login" signUpUrl="/signup" />
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
  if (clerkEnabled) {
    return (
      <div className="auth-layout">
        <SignUp routing="path" path="/signup" signInUrl="/login" />
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

export type AppShellNavItem = {
  to: string;
  label: string;
  icon: ReactNode;
  end?: boolean;
};

export type AppShellNavSection = {
  label: string;
  items: AppShellNavItem[];
};

export function AppShell({
  children,
  brandTitle,
  brandSubtitle,
  sections,
  footerContent,
}: PropsWithChildren<{
  brandTitle: string;
  brandSubtitle: string;
  sections: AppShellNavSection[];
  footerContent?: ReactNode;
}>) {
  const location = useLocation();
  const { t, sentenceCase } = useI18n();

  useEffect(() => {
    const main = document.querySelector<HTMLElement>('.main-content');
    main?.scrollTo({ top: 0, left: 0, behavior: 'auto' });
  }, [location.pathname]);

  return (
    <div className="app-layout">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <h1>{brandTitle}</h1>
          <small>{sentenceCase(brandSubtitle)}</small>
        </div>

        <nav className="sidebar-nav">
          {sections.map((section) => (
            <NavSection key={section.label} label={section.label} items={section.items} />
          ))}
        </nav>

        <div className="sidebar-footer">
          {footerContent ?? (clerkEnabled ? <UserButton /> : <span style={{ fontSize: '0.78rem' }}>{t('shell.footer.localDev')}</span>)}
        </div>
      </aside>

      <main className="main-content">{children}</main>
    </div>
  );
}

function NavSection({ label, items }: AppShellNavSection) {
  const { sentenceCase } = useI18n();

  return (
    <>
      <div className="sidebar-section-label">{sentenceCase(label)}</div>
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          className={({ isActive }) => `sidebar-link${isActive ? ' active' : ''}`}
        >
          {item.icon}
          <span>{sentenceCase(item.label)}</span>
        </NavLink>
      ))}
    </>
  );
}
