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

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const baseURL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers ?? {}),
  };

  const token = tokenProvider ? await tokenProvider() : null;
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  } else {
    const apiKey = import.meta.env.VITE_API_KEY;
    if (apiKey) {
      headers['X-API-KEY'] = apiKey;
      headers['X-Actor'] = import.meta.env.VITE_API_ACTOR ?? 'frontend-api-key';
      headers['X-Role'] = import.meta.env.VITE_API_ROLE ?? 'admin';
      headers['X-Scopes'] = import.meta.env.VITE_API_SCOPES ?? 'admin:console:read,admin:console:write';
    }
  }

  if (options.orgId) {
    headers['X-Org-ID'] = options.orgId;
  }

  const response = await fetch(`${baseURL}${path}`, {
    method: options.method ?? 'GET',
    headers,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `HTTP ${response.status}`);
  }

  const contentType = response.headers.get('content-type') ?? '';
  if (contentType.includes('application/json')) {
    return (await response.json()) as T;
  }
  return (await response.text()) as T;
}
