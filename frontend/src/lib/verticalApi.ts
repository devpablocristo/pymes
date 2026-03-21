import { request, type RequestOptions } from '@devpablocristo/core-authn/http/fetch';

type VerticalRequestConfig = {
  envVar: string;
  fallbackPorts: number[];
  translateError?: (message: string) => string;
};

function resolveVerticalBaseURLs(envVar: string, fallbackPorts: number[]): string[] {
  const candidates: string[] = [];
  const env = import.meta.env as Record<string, string | undefined>;
  const configured = env[envVar]?.trim();
  if (configured) {
    candidates.push(configured);
  }

  // Solo usar fallbacks basados en el hostname actual del navegador.
  // No hardcodear localhost para evitar requests a localhost en produccion.
  if (typeof window !== 'undefined') {
    const protocol = window.location.protocol || 'http:';
    const hostname = window.location.hostname || 'localhost';
    fallbackPorts.forEach((port) => {
      candidates.push(`${protocol}//${hostname}:${port}`);
    });
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
  const baseURLs = resolveVerticalBaseURLs(config.envVar, config.fallbackPorts);

  return async function verticalRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
    try {
      return await request<T>(path, { ...options, baseURLs });
    } catch (error) {
      if (error instanceof Error) {
        throw new Error(normalizeErrorMessage(error.message, config.translateError));
      }
      throw error;
    }
  };
}
