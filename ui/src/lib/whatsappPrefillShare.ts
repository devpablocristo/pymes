/** Longitud conservadora para no romper URLs de wa.me en navegadores antiguos. */
const MAX_SHARE_CHARS = 1800;

function truncateMessage(text: string): string {
  if (text.length <= MAX_SHARE_CHARS) {
    return text;
  }
  return `${text.slice(0, MAX_SHARE_CHARS - 1)}…`;
}

/**
 * Abre WhatsApp (web o app) con el mensaje prellenado; el usuario elige el contacto o grupo.
 * No envía nada por la API de WhatsApp Business del tenant.
 */
export function openWhatsAppPrefilledShare(message: string): void {
  const body = truncateMessage(message.trim());
  if (!body) {
    return;
  }
  const url = `https://wa.me/?text=${encodeURIComponent(body)}`;
  window.open(url, '_blank', 'noopener,noreferrer');
}

export function buildInAppNotificationShareText(title: string, body: string): string {
  return truncateMessage(`*Pymes — Aviso*\n${title}\n\n${body}`);
}

export function buildApprovalShareText(params: {
  actionLabel: string;
  targetResource: string;
  reason: string;
  aiSummary?: string;
}): string {
  const resource = params.targetResource?.trim() ?? '';
  const head = resource
    ? `*Pymes — Aprobación pendiente*\n${params.actionLabel} — ${resource}`
    : `*Pymes — Aprobación pendiente*\n${params.actionLabel}`;
  const mid = `\n\n${params.reason}`;
  const tail = params.aiSummary?.trim() ? `\n\n_${params.aiSummary.trim()}_` : '';
  return truncateMessage(`${head}${mid}${tail}`);
}
