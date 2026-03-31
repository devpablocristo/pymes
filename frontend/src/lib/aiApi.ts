import { request, type RequestOptions } from '@devpablocristo/core-authn/http/fetch';
import type {
  CommercialChatRequest,
  InsightNotificationItem,
  InsightNotificationsResponse,
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
  PymesInsightOutputKind,
  PymesInsightServiceKind,
  PymesRoutedAgent,
  PymesRoutingSource,
} from '../types/aiChat';

/** Asistente Pymes — un solo chat interno con router LLM y sub-agentes especializados. */
export async function pymesAssistantChat(payload: CommercialChatRequest): Promise<PymesAssistantChatResponse> {
  return request('/v1/chat', aiOptions({ method: 'POST', body: payload }));
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
