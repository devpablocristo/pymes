import * as Sentry from '@sentry/react';

const dsn = import.meta.env.VITE_SENTRY_DSN as string | undefined;

/**
 * Inicializa Sentry solo si VITE_SENTRY_DSN está configurado.
 * En desarrollo local no hace nada.
 */
export function initSentry(): void {
  if (!dsn) {
    return;
  }
  Sentry.init({
    dsn,
    environment: import.meta.env.MODE,
    // Solo capturar errores, no performance traces
    tracesSampleRate: 0,
    // Filtrar errores de red/auth que no aportan
    beforeSend(event) {
      const message = event.exception?.values?.[0]?.value ?? '';
      if (message.includes('401') || message.includes('NetworkError')) {
        return null;
      }
      return event;
    },
  });
}

/**
 * Captura un error manualmente (para catch blocks que no son render errors).
 */
export function captureError(error: unknown, context?: Record<string, string>): void {
  if (!dsn) {
    return;
  }
  Sentry.captureException(error, context ? { tags: context } : undefined);
}

export { Sentry };
