import { formatDate } from '../../crud/resourceConfigs.shared';
import type {
  CustomerMessagingCampaign,
  CustomerMessagingConversation,
} from '../../lib/api';

export function renderMessagingConversationStatusBadge(status: string) {
  const cls = status === 'open' ? 'badge-success' : status === 'resolved' ? 'badge-neutral' : 'badge-warning';
  const label = status === 'open' ? 'Abierta' : status === 'resolved' ? 'Resuelta' : status;
  return <span className={`badge ${cls}`}>{label}</span>;
}

export function renderMessagingCampaignStatusBadge(status: string) {
  const success = status === 'completed';
  const sending = status === 'sending';
  const cls = success ? 'badge-success' : sending ? 'badge-warning' : 'badge-neutral';
  return <span className={`badge ${cls}`}>{status}</span>;
}

export function buildMessagingInboxSummary(
  conversations: CustomerMessagingConversation[],
  visibleCount: number,
): string {
  const unread = conversations.reduce((acc, row) => acc + Number(row.unread_count ?? 0), 0);
  const open = conversations.filter((row) => row.status === 'open').length;
  const assigned = conversations.filter((row) => row.assigned_to).length;
  return `${visibleCount} visibles · ${open} abiertas · ${unread} sin leer · ${assigned} asignadas`;
}

export function buildMessagingCampaignsSummary(
  campaigns: CustomerMessagingCampaign[],
  visibleCount: number,
): string {
  return `${visibleCount} visibles · ${campaigns.filter((row) => row.status === 'draft').length} draft · ${
    campaigns.filter((row) => row.status === 'completed').length
  } completadas`;
}

export function formatMessagingConversationTimestamp(value?: string): string {
  return value ? formatDate(value) : 'Sin actividad todavía';
}
