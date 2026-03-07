let tokenProvider: (() => Promise<string | null>) | null = null;

export function registerTokenProvider(provider: () => Promise<string | null>): void {
  tokenProvider = provider;
}

export type RequestOptions = {
  method?: string;
  body?: unknown;
  rawBody?: BodyInit | null;
  headers?: Record<string, string>;
  orgId?: string;
  skipJSONContentType?: boolean;
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

function resolveBaseURL(): string {
  const configured = import.meta.env.VITE_API_URL?.trim();
  if (configured) {
    return configured;
  }

  if (typeof window === 'undefined') {
    return 'http://localhost:8100';
  }

  const protocol = window.location.protocol || 'http:';
  const hostname = window.location.hostname || 'localhost';
  return `${protocol}//${hostname}:8100`;
}

function resolveLocalAPIKeyFallback(): string | null {
  if (!isLocalhost()) {
    return null;
  }

  return 'psk_local_admin';
}

async function buildHeaders(options: RequestOptions): Promise<Record<string, string>> {
  const headers: Record<string, string> = {
    ...(options.headers ?? {}),
  };
  if (
    !options.skipJSONContentType &&
    !('Content-Type' in headers) &&
    !(typeof FormData !== 'undefined' && options.rawBody instanceof FormData)
  ) {
    headers['Content-Type'] = 'application/json';
  }

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
  return headers;
}

export async function requestResponse(path: string, options: RequestOptions = {}): Promise<Response> {
  const headers = await buildHeaders(options);
  const baseURL = resolveBaseURL();
  const response = await fetch(`${baseURL}${path}`, {
    method: options.method ?? 'GET',
    headers,
    body:
      options.rawBody ??
      (options.body !== undefined ? JSON.stringify(options.body) : undefined),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new HttpError(text || `HTTP ${response.status}`);
  }
  return response;
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const response = await requestResponse(path, options);
  const contentType = response.headers.get('content-type') ?? '';
  if (contentType.includes('application/json')) {
    return (await response.json()) as T;
  }
  return (await response.text()) as T;
}
