import type { PropsWithChildren } from "react";
import { BrowserRouter } from "react-router-dom";
import type { HttpClient } from "@devpablocristo/platform-http";
import { ProductApiProvider } from "../../api/ProductApiContext";
import {
  RuntimeAuthProvider,
} from "../../auth/RuntimeAuthProvider";
import { AuthValueProvider, type AuthContextValue } from "../../auth/AuthContext";
import type { components } from "../../api/schema.generated";
import { ErrorBoundary } from "./ErrorBoundary";
import { I18nProvider } from "./I18nProvider";
import { ThemeProvider } from "./ThemeProvider";

type RuntimeConfig = components["schemas"]["RuntimeConfig"];

type AppProvidersProps = PropsWithChildren<{
  authValue?: AuthContextValue;
  apiClient?: HttpClient;
  runtimeConfig?: RuntimeConfig;
}>;

export function AppProviders({
  children,
  authValue,
  apiClient,
  runtimeConfig,
}: AppProvidersProps) {
  const routedChildren = (
    <ProductApiProvider client={apiClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </ProductApiProvider>
  );

  return (
    <ErrorBoundary>
      <ThemeProvider>
        <I18nProvider>
          {authValue ? (
            <AuthValueProvider value={authValue}>{routedChildren}</AuthValueProvider>
          ) : (
            <RuntimeAuthProvider config={runtimeConfig}>{routedChildren}</RuntimeAuthProvider>
          )}
        </I18nProvider>
      </ThemeProvider>
    </ErrorBoundary>
  );
}
