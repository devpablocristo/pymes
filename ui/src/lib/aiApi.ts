import { apiRequest } from './api';
import type {
  CommercialChatRequest,
  InsightNotificationsResponse,
  PymesAssistantChatBlock,
  PymesAssistantChatResponse,
} from '../types/aiChat';

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

/** Asistente Pymes: browser -> Pymes backend -> Axis Companion. */
export async function pymesAssistantChat(payload: CommercialChatRequest): Promise<PymesAssistantChatResponse> {
  return apiRequest<PymesAssistantChatResponse>('/v1/ai/chat', {
    method: 'POST',
    body: payload,
  });
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

/** Lista conversaciones del usuario autenticado vía Pymes backend. */
export async function listConversations(limit = 50): Promise<{ items: ConversationSummary[] }> {
  return apiRequest(`/v1/ai/chat/conversations?limit=${limit}`);
}

/** Carga una conversación con su historial de mensajes vía Pymes backend. */
export async function getConversation(conversationId: string): Promise<ConversationDetail> {
  return apiRequest(`/v1/ai/chat/conversations/${conversationId}`);
}

export async function createInsightNotifications(payload?: {
  kind?: 'insight';
  period?: 'today' | 'week' | 'month';
  compare?: boolean;
  top_limit?: number;
  preferred_language?: 'es' | 'en';
}): Promise<InsightNotificationsResponse> {
  return apiRequest('/v1/ai/notifications', { method: 'POST', body: payload ?? {} });
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
  return apiRequest(`/v1/ai/watchers?${params.toString()}`);
}

export async function createWatcher(payload: CreateWatcherRequest): Promise<WatcherResponse> {
  return apiRequest('/v1/ai/watchers', { method: 'POST', body: payload });
}

export async function updateWatcher(
  id: string,
  config: Record<string, unknown>,
  enabled: boolean,
): Promise<WatcherResponse> {
  return apiRequest(`/v1/ai/watchers/${id}`, { method: 'PATCH', body: { config, enabled } });
}
