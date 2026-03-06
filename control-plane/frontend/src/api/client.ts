let tokenProvider: (() => Promise<string | null>) | null = null;

export function registerTokenProvider(provider: () => Promise<string | null>): void {
  tokenProvider = provider;
}

type RequestOptions = {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  orgId?: string;
};

class HttpError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'HttpError';
  }
}

function isLocalhost(): boolean {
  if (typeof window === 'undefined') {
    return true;
  }

  return ['localhost', '127.0.0.1'].includes(window.location.hostname);
}

function resolveBaseURLs(): string[] {
  const candidates: string[] = [];
  const configured = import.meta.env.VITE_API_URL?.trim();
  if (configured) {
    candidates.push(configured);
  }

  if (typeof window === 'undefined') {
    candidates.push('http://localhost:8100', 'http://localhost:8080');
  } else {
    const protocol = window.location.protocol || 'http:';
    const hostname = window.location.hostname || 'localhost';
    candidates.push(`${protocol}//${hostname}:8100`, `${protocol}//${hostname}:8080`);
  }

  return [...new Set(candidates)];
}

function resolveLocalAPIKeyFallback(): string | null {
  if (!isLocalhost()) {
    return null;
  }

  return 'psk_local_admin';
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers ?? {}),
  };

  const token = tokenProvider ? await tokenProvider() : null;
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  } else {
    const apiKey = import.meta.env.VITE_API_KEY?.trim() || resolveLocalAPIKeyFallback();
    if (apiKey) {
      headers['X-API-KEY'] = apiKey;
      headers['X-Actor'] = import.meta.env.VITE_API_ACTOR?.trim() || 'local-admin';
      headers['X-Role'] = import.meta.env.VITE_API_ROLE?.trim() || 'admin';
      headers['X-Scopes'] =
        import.meta.env.VITE_API_SCOPES?.trim() || 'admin:console:read,admin:console:write';
    }
  }

  if (options.orgId) {
    headers['X-Org-ID'] = options.orgId;
  }

  let lastError: unknown = null;
  for (const baseURL of resolveBaseURLs()) {
    try {
      const response = await fetch(`${baseURL}${path}`, {
        method: options.method ?? 'GET',
        headers,
        body: options.body ? JSON.stringify(options.body) : undefined,
      });

      if (!response.ok) {
        const text = await response.text();
        throw new HttpError(text || `HTTP ${response.status}`);
      }

      const contentType = response.headers.get('content-type') ?? '';
      if (contentType.includes('application/json')) {
        return (await response.json()) as T;
      }
      return (await response.text()) as T;
    } catch (error) {
      lastError = error;
      if (error instanceof Error && error.name === 'HttpError') {
        throw error;
      }
    }
  }

  if (lastError instanceof Error) {
    throw lastError;
  }

  throw new Error('No se pudo conectar con el backend');
}
