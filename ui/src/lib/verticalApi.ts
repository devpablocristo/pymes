import { apiRequest, type TenantAwareRequestOptions } from './api';

type VerticalRequestConfig = {
  envVar: string;
  devPorts: number[];
  translateError?: (message: string) => string;
  /** Límite de espera (ms). 0 = sin límite. Evita spinners infinitos si el vertical no responde. */
  timeoutMs?: number;
  timeoutMessage?: string;
};

function withTimeout<T>(promise: Promise<T>, ms: number, message: string): Promise<T> {
  if (ms <= 0) {
    return promise;
  }
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => reject(new Error(message)), ms);
    promise.then(
      (value) => {
        window.clearTimeout(timer);
        resolve(value);
      },
      (err) => {
        window.clearTimeout(timer);
        reject(err);
      },
    );
  });
}

function resolveVerticalBaseURLs(envVar: string, devPorts: number[]): string[] {
  const candidates: string[] = [];
  const env = import.meta.env as Record<string, string | undefined>;
  const configured = env[envVar]?.trim();
  if (configured) {
    candidates.push(configured);
  }

  // Solo usar puertos de desarrollo cuando el navegador corre localmente.
  // En hosting real, un puerto como web.app:8282 es un falso destino y rompe DEV.
  if (typeof window !== 'undefined') {
    const protocol = window.location.protocol || 'http:';
    const hostname = window.location.hostname || 'localhost';
    if (hostname === 'localhost' || hostname === '127.0.0.1') {
      devPorts.forEach((port) => {
        candidates.push(`${protocol}//${hostname}:${port}`);
      });
    }
  }

  return [...new Set(candidates)];
}

function normalizeErrorMessage(raw: string, translateError?: (message: string) => string): string {
  const trimmed = raw.trim();
  const withoutPrefix = trimmed.replace(/^HttpError:\s*/i, '');

  let parsedMessage = withoutPrefix;
  try {
    const parsed = JSON.parse(withoutPrefix) as { error?: string; message?: string };
    parsedMessage = parsed.error ?? parsed.message ?? withoutPrefix;
  } catch {
    parsedMessage = withoutPrefix;
  }

  const normalized = parsedMessage.trim();
  if (translateError) {
    return translateError(normalized);
  }
  return normalized;
}

export function createVerticalRequest(config: VerticalRequestConfig) {
  const baseURLs = resolveVerticalBaseURLs(config.envVar, config.devPorts);
  const timeoutMs = config.timeoutMs ?? 0;
  const timeoutMessage =
    config.timeoutMessage ??
    'El servidor tardó demasiado en responder. Comprobá que el backend vertical esté en marcha y el puerto en VITE_*_API_URL.';

  return async function verticalRequest<T>(path: string, options: TenantAwareRequestOptions = {}): Promise<T> {
    const run = async (): Promise<T> => {
      try {
        return await apiRequest<T>(path, { ...options, baseURLs });
      } catch (error) {
        if (error instanceof Error) {
          throw new Error(normalizeErrorMessage(error.message, config.translateError));
        }
        throw error;
      }
    };
    try {
      return await withTimeout(run(), timeoutMs, timeoutMessage);
    } catch (error) {
      if (error instanceof Error) {
        throw new Error(normalizeErrorMessage(error.message, config.translateError));
      }
      throw error;
    }
  };
}
