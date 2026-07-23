import type { PropsWithChildren } from "react";
import { BrowserRouter } from "react-router-dom";
import { ErrorBoundary } from "./ErrorBoundary";
import { I18nProvider } from "./I18nProvider";
import { ThemeProvider } from "./ThemeProvider";

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <ErrorBoundary>
      <ThemeProvider>
        <I18nProvider>
          <BrowserRouter>{children}</BrowserRouter>
        </I18nProvider>
      </ThemeProvider>
    </ErrorBoundary>
  );
}
