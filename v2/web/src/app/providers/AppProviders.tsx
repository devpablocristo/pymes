import type { PropsWithChildren } from "react";
import { ErrorBoundary } from "./ErrorBoundary";
import { I18nProvider } from "./I18nProvider";
import { ThemeProvider } from "./ThemeProvider";

export function AppProviders({ children }: PropsWithChildren) {
  return (
    <ErrorBoundary>
      <ThemeProvider>
        <I18nProvider>{children}</I18nProvider>
      </ThemeProvider>
    </ErrorBoundary>
  );
}
