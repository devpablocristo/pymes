import { request, type RequestOptions } from '@devpablocristo/core-authn/http/fetch';

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

export type CommercialChatRequest = {
  conversation_id?: string | null;
  message: string;
  confirmed_actions?: string[];
};

export type CommercialChatResponse = {
  conversation_id: string;
  reply: string;
  tokens_used: number;
  tool_calls: string[];
  pending_confirmations: string[];
};

export async function commercialChatSales(payload: CommercialChatRequest): Promise<CommercialChatResponse> {
  return request('/v1/chat/commercial/sales', aiOptions({ method: 'POST', body: payload }));
}

export async function commercialChatProcurement(payload: CommercialChatRequest): Promise<CommercialChatResponse> {
  return request('/v1/chat/commercial/procurement', aiOptions({ method: 'POST', body: payload }));
}

export type PymesAssistantChatResponse = CommercialChatResponse & {
  routed_agent: string;
  routed_mode: string;
};

/** Asistente Pymes — un solo chat interno con router LLM y sub-agentes especializados. */
export async function pymesAssistantChat(payload: CommercialChatRequest): Promise<PymesAssistantChatResponse> {
  return request('/v1/chat/pymes/', aiOptions({ method: 'POST', body: payload }));
}
