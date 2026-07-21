import type { PymesChatHandoff } from './aiApi';

/** Clave sessionStorage para abrir el chat con contexto desde notificaciones. */
export const NOTIFICATION_CHAT_HANDOFF_KEY = 'pymes.notificationChatHandoff';

export type NotificationChatContext = {
  suggested_user_message?: string;
  scope?: PymesChatHandoff['insight_scope'];
  routed_agent?: string;
  content_language?: 'es' | 'en';
  period?: PymesChatHandoff['period'];
  compare?: PymesChatHandoff['compare'];
  top_limit?: PymesChatHandoff['top_limit'];
} & Record<string, unknown>;

export type NotificationChatHandoff = {
  notificationId: string;
  title: string;
  body: string;
  chatContext: NotificationChatContext;
  source: PymesChatHandoff['source'];
  notification_id: PymesChatHandoff['notification_id'];
  insight_scope?: PymesChatHandoff['insight_scope'];
  period?: PymesChatHandoff['period'];
  compare?: PymesChatHandoff['compare'];
  top_limit?: PymesChatHandoff['top_limit'];
  scope?: PymesChatHandoff['insight_scope'];
  routedAgent?: string;
  contentLanguage?: string;
};

/** Construye el payload estructurado que viaja al backend; no depende del texto visible del usuario. */
export function buildChatRequestHandoff(h: NotificationChatHandoff): PymesChatHandoff {
  return {
    source: h.source,
    notification_id: h.notification_id,
    insight_scope: h.insight_scope,
    period: h.period,
    compare: h.compare,
    top_limit: h.top_limit,
  };
}

/** Arma el primer mensaje al Asistente Pymes a partir del aviso y el JSON `chat_context`. */
export function buildHandoffUserMessage(h: NotificationChatHandoff): string {
  const suggested = h.chatContext.suggested_user_message;
  if (typeof suggested === 'string' && suggested.trim() !== '') {
    return suggested.trim();
  }
  return `Necesito más información sobre: ${h.title}\n\n${h.body}`;
}
