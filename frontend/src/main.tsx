import React, { Component, type ErrorInfo, type ReactNode } from 'react';
import ReactDOM from 'react-dom/client';
import { ClerkProvider } from '@clerk/react';
import { esMX } from '@clerk/localizations';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import { clerkEnabled, clerkPublishableKey } from './lib/auth';
import { clerkAppearance } from './lib/clerkAppearance';
import { App } from './app/App';
import { LanguageProvider } from './lib/i18n';
import { applyAdminSkin } from './lib/adminSkin';
import { applyTheme } from './lib/theme';
import './styles.css';

applyTheme();
applyAdminSkin();

class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled render error:', error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="error-boundary-fallback" role="alert">
          <h1>Algo salió mal</h1>
          <p className="text-secondary u-mb-md">Recargá la página. Si el problema continúa, contactá soporte.</p>
          {import.meta.env.DEV && (
            <pre className="error-boundary-fallback__dev-pre">
              {this.state.error.message}
            </pre>
          )}
          <button type="button" className="btn-primary" onClick={() => window.location.reload()}>
            Recargar
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}

const queryClient = new QueryClient();

const app = (
  <ErrorBoundary>
    <QueryClientProvider client={queryClient}>
      <LanguageProvider>
        <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
          <App />
        </BrowserRouter>
      </LanguageProvider>
    </QueryClientProvider>
  </ErrorBoundary>
);

ReactDOM.createRoot(document.getElementById('root')!).render(
  clerkEnabled ? (
    <ClerkProvider
      publishableKey={clerkPublishableKey}
      localization={esMX}
      appearance={clerkAppearance}
    >
      {app}
    </ClerkProvider>
  ) : (
    app
  ),
);
