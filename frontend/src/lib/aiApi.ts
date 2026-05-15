import { apiRequest, type TenantAwareRequestOptions } from './api';
import type {
  CommercialChatRequest,
  InsightNotificationsResponse,
  PymesAssistantChatBlock,
  PymesAssistantChatResponse,
} from '../types/aiChat';

// Companion reemplaza a pymes-ai como backend del chat. La base URL del
// servicio Companion se inyecta vía `VITE_COMPANION_BASE_URL` en build.
// Mantenemos `VITE_AI_API_URL` como fallback de compatibilidad mientras los
// pipelines de deploy se actualizan.
function resolveCompanionBaseURLs(): string[] {
  const env = import.meta.env as Record<string, string | undefined>;
  const configured = (env.VITE_COMPANION_BASE_URL ?? env.VITE_AI_API_URL)?.trim();
  const candidates: string[] = [];
  if (configured) {
    candidates.push(configured);
  }
  if (typeof window !== 'undefined') {
    const protocol = window.location.protocol || 'http:';
    const hostname = window.location.hostname || 'localhost';
    // Default dev: companion-dev en GCP detrás del mismo proxy same-origin.
    candidates.push(`${protocol}//${hostname}:18085`);
  }
  return [...new Set(candidates)];
}

const companionBaseURLs = resolveCompanionBaseURLs();

function companionOptions(options: TenantAwareRequestOptions = {}): TenantAwareRequestOptions {
  return { ...options, baseURLs: companionBaseURLs };
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
  return apiRequest('/v1/chat', companionOptions({ method: 'POST', body: payload }));
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
  return apiRequest(`/v1/chat/conversations?limit=${limit}`, companionOptions());
}

/** Carga una conversación con su historial de mensajes (Companion GET /v1/chat/conversations/{id}). */
export async function getConversation(conversationId: string): Promise<ConversationDetail> {
  return apiRequest(`/v1/chat/conversations/${conversationId}`, companionOptions());
}

export async function createInsightNotifications(payload?: {
  kind?: 'insight';
  period?: 'today' | 'week' | 'month';
  compare?: boolean;
  top_limit?: number;
  preferred_language?: 'es' | 'en';
}): Promise<InsightNotificationsResponse> {
  return apiRequest('/v1/notifications', companionOptions({ method: 'POST', body: payload ?? {} }));
}
