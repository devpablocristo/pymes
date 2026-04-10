import { request, type RequestOptions } from '@devpablocristo/core-authn/http/fetch';
import type {
  CommercialChatRequest,
  InsightNotificationsResponse,
  PymesAssistantChatBlock,
  PymesAssistantChatResponse,
} from '../types/aiChat';

function resolveAiBaseURLs(): string[] {
  const env = import.meta.env as Record<string, string | undefined>;
  const configured = env.VITE_AI_API_URL?.trim();
  const candidates: string[] = [];
  if (configured) {
    candidates.push(configured);
  }
  if (typeof window !== 'undefined') {
    const protocol = window.location.protocol || 'http:';
    const hostname = window.location.hostname || 'localhost';
    candidates.push(`${protocol}//${hostname}:8200`);
  }
  return [...new Set(candidates)];
}

const aiBaseURLs = resolveAiBaseURLs();

function aiOptions(options: RequestOptions = {}): RequestOptions {
  return { ...options, baseURLs: aiBaseURLs };
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

/** Asistente Pymes — un solo chat interno con router LLM y sub-agentes especializados. */
export async function pymesAssistantChat(payload: CommercialChatRequest): Promise<PymesAssistantChatResponse> {
  return request('/v1/chat', aiOptions({ method: 'POST', body: payload }));
}

// ── Conversaciones persistidas ──

export type ConversationSummary = {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
  message_count: number;
};

export type ConversationMessage = {
  role: string;
  content: string;
  ts?: string | null;
  tool_calls?: string[];
  blocks?: PymesAssistantChatBlock[];
};

export type ConversationDetail = {
  id: string;
  title: string;
  messages: ConversationMessage[];
  created_at: string;
  updated_at: string;
};

/** Lista conversaciones del usuario autenticado. */
export async function listConversations(limit = 50): Promise<{ items: ConversationSummary[] }> {
  return request(`/v1/chat/conversations?limit=${limit}`, aiOptions());
}

/** Carga una conversación con su historial de mensajes. */
export async function getConversation(conversationId: string): Promise<ConversationDetail> {
  return request(`/v1/chat/conversations/${conversationId}`, aiOptions());
}

export async function createInsightNotifications(payload?: {
  kind?: 'insight';
  period?: 'today' | 'week' | 'month';
  compare?: boolean;
  top_limit?: number;
  preferred_language?: 'es' | 'en';
}): Promise<InsightNotificationsResponse> {
  return request('/v1/notifications', aiOptions({ method: 'POST', body: payload ?? {} }));
}
