import { request as rawRequest, type RequestOptions as RawRequestOptions } from '@devpablocristo/platform-http/fetch';
import { apiRequest, type TenantAwareRequestOptions } from './api';
import type {
  CommercialChatRequest,
  InsightNotificationsResponse,
  PymesAssistantChatBlock,
  PymesAssistantChatResponse,
} from '../types/aiChat';

// Companion es el backend del chat. La base URL se inyecta vía
// `VITE_COMPANION_BASE_URL`; `VITE_COMPANION_API_KEY` queda solo para dev local
// sin sesión Clerk.
function resolveCompanionBaseURLs(): string[] {
  const env = import.meta.env as Record<string, string | undefined>;
  const configured = env.VITE_COMPANION_BASE_URL?.trim();
  const candidates: string[] = [];
  if (configured) {
    candidates.push(configured);
  }
  if (typeof window !== 'undefined') {
    const protocol = window.location.protocol || 'http:';
    const hostname = window.location.hostname || 'localhost';
    // Default dev: Axis Companion local en el puerto publicado por ../axis.
    candidates.push(`${protocol}//${hostname}:18085`);
  }
  return [...new Set(candidates)];
}

const companionBaseURLs = resolveCompanionBaseURLs();
const companionAPIKey = (import.meta.env.VITE_COMPANION_API_KEY ?? '').trim();

function companionOptions(options: TenantAwareRequestOptions = {}): TenantAwareRequestOptions {
  return { ...options, baseURLs: companionBaseURLs };
}

async function companionRequest<T = unknown>(path: string, options: TenantAwareRequestOptions = {}): Promise<T> {
  const resolved = companionOptions(options);
  if (!companionAPIKey) {
    return apiRequest<T>(path, resolved);
  }

  const { tenantSlug: _tenantSlug, skipTenantSlug: _skipTenantSlug, orgId: _orgId, ...rawOptions } = resolved;
  return rawRequest<T>(path, {
    ...(rawOptions as RawRequestOptions),
    headers: {
      ...(rawOptions.headers ?? {}),
      'X-API-Key': companionAPIKey,
    },
  });
}

export type {
  CommercialChatRequest,
  InsightNotificationItem,
  InsightNotificationsResponse,
  PymesAssistantAction,
  PymesAssistantChatBaseResponse,
  PymesAssistantChatBlock,
  PymesChatOutputKind,
  PymesAssistantChatResponse,
  PymesChatHandoff,
  PymesChatHandoffSource,
  PymesInsightOutputKind,
  PymesInsightServiceKind,
  PymesRoutedAgent,
  PymesRoutingSource,
} from '../types/aiChat';

/** Asistente Pymes — chat interno contra Companion (POST /v1/chat). */
export async function pymesAssistantChat(payload: CommercialChatRequest): Promise<PymesAssistantChatResponse> {
  return companionRequest('/v1/chat', { method: 'POST', body: payload });
}

// ── Conversaciones persistidas (Companion agent_conversations) ──

export type ConversationSummary = {
  id: string;
  title?: string;
  created_at: string;
  updated_at: string;
  message_count: number;
  product_surface?: string;
};

export type ConversationMessage = {
  role: string;
  content: string;
  /** Timestamp en formato ISO. `timestamp` es el campo canónico (Companion).
   *  `ts` es el alias legacy heredado de pymes-ai; lo aceptamos como sinónimo
   *  hasta que todos los consumidores migren a `timestamp`. */
  timestamp?: string | null;
  ts?: string | null;
  tool_calls?: string[];
  blocks?: PymesAssistantChatBlock[];
};

export type ConversationDetail = {
  id: string;
  title?: string;
  messages: ConversationMessage[];
  created_at: string;
  updated_at: string;
};

/** Lista conversaciones del usuario autenticado (Companion GET /v1/chat/conversations). */
export async function listConversations(limit = 50): Promise<{ items: ConversationSummary[] }> {
  return companionRequest(`/v1/chat/conversations?limit=${limit}`);
}

/** Carga una conversación con su historial de mensajes (Companion GET /v1/chat/conversations/{id}). */
export async function getConversation(conversationId: string): Promise<ConversationDetail> {
  return companionRequest(`/v1/chat/conversations/${conversationId}`);
}

export async function createInsightNotifications(payload?: {
  kind?: 'insight';
  period?: 'today' | 'week' | 'month';
  compare?: boolean;
  top_limit?: number;
  preferred_language?: 'es' | 'en';
}): Promise<InsightNotificationsResponse> {
  return companionRequest('/v1/notifications', { method: 'POST', body: payload ?? {} });
}

export interface WatcherResponse {
  id: string;
  org_id: string;
  name: string;
  watcher_type: string;
  config: Record<string, unknown>;
  enabled: boolean;
  last_run_at?: string | null;
  last_result?: { found: number; proposed: number; executed: number } | null;
  created_at: string;
  updated_at: string;
}

export interface CreateWatcherRequest {
  org_id: string;
  name: string;
  watcher_type: string;
  config: Record<string, unknown>;
  enabled: boolean;
}

export async function listWatchers(orgID: string): Promise<{ watchers: WatcherResponse[] }> {
  const params = new URLSearchParams({ org_id: orgID });
  return companionRequest(`/v1/watchers?${params.toString()}`);
}

export async function createWatcher(payload: CreateWatcherRequest): Promise<WatcherResponse> {
  return companionRequest('/v1/watchers', { method: 'POST', body: payload });
}

export async function updateWatcher(
  id: string,
  config: Record<string, unknown>,
  enabled: boolean,
): Promise<WatcherResponse> {
  return companionRequest(`/v1/watchers/${id}`, { method: 'PATCH', body: { config, enabled } });
}
