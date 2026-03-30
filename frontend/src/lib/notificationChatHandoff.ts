/** Clave sessionStorage para abrir el chat con contexto desde notificaciones. */
export const NOTIFICATION_CHAT_HANDOFF_KEY = 'pymes.notificationChatHandoff';

export type NotificationChatHandoff = {
  notificationId: string;
  title: string;
  body: string;
  chatContext: Record<string, unknown>;
  scope?: string;
  routedAgent?: string;
};

/** Arma el primer mensaje al Asistente Pymes a partir del aviso y el JSON `chat_context`. */
export function buildHandoffUserMessage(h: NotificationChatHandoff): string {
  const suggested = h.chatContext.suggested_user_message;
  if (typeof suggested === 'string' && suggested.trim() !== '') {
    return suggested.trim();
  }
  return `Necesito más información sobre: ${h.title}\n\n${h.body}`;
}
