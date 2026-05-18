import * as Sentry from '@sentry/react';
import { createSentryReporter, registerErrorReporter, captureError } from '@devpablocristo/core-browser/observability';

const dsn = import.meta.env.VITE_SENTRY_DSN as string | undefined;

/**
 * Inicializa Sentry solo si VITE_SENTRY_DSN está configurado.
 * En desarrollo local no hace nada.
 */
export function initSentry(): void {
  const reporter = createSentryReporter(dsn, Sentry, import.meta.env.MODE);
  if (reporter) {
    registerErrorReporter(reporter);
  }
}

export { captureError };
