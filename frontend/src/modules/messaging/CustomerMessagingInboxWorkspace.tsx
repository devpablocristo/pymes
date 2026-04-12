import { useMemo } from 'react';
import { useSearch } from '@devpablocristo/modules-search';
import {
  ConversationInbox,
  type ConversationInboxItem,
} from '@devpablocristo/modules-ui-conversation-inbox';
import '@devpablocristo/modules-ui-notification-feed/styles.css';
import '@devpablocristo/modules-ui-conversation-inbox/styles.css';
import { PageLayout } from '../../components/PageLayout';
import { usePageSearch } from '../../components/PageSearch';
import { formatFetchErrorForUser } from '../../lib/formatFetchError';
import { openCrudFormDialog } from '../crud';
import type { CustomerMessagingConversation } from '../../lib/api';
import {
  buildMessagingInboxSummary,
  formatMessagingConversationTimestamp,
  renderMessagingConversationStatusBadge,
} from './messagingHelpers';
import { useCustomerMessagingConversations } from './useCustomerMessagingConversations';

export function CustomerMessagingInboxWorkspace() {
  const search = usePageSearch();
  const { conversationsQuery, assignMutation, markReadMutation, resolveMutation } = useCustomerMessagingConversations();

  const conversations: CustomerMessagingConversation[] = conversationsQuery.data?.items ?? [];
  const filtered = useSearch(
    conversations,
    (row) => [row.party_name, row.phone, row.assigned_to, row.last_message_preview, row.status].filter(Boolean).join(' '),
    search,
  );

  const summary = useMemo(
    () => buildMessagingInboxSummary(conversations, filtered.length),
    [conversations, filtered.length],
  );

  const error = conversationsQuery.error
    ? formatFetchErrorForUser(conversationsQuery.error, 'No se pudo cargar la bandeja de conversaciones.')
    : '';

  async function handleAssign(row: CustomerMessagingConversation): Promise<void> {
    const values = await openCrudFormDialog({
      title: 'Asignar conversación',
      subtitle: row.party_name || row.phone || row.id,
      submitLabel: 'Asignar',
      fields: [
        {
          id: 'assigned_to',
          label: 'ID del operador (user_id)',
          required: true,
          defaultValue: row.assigned_to ?? '',
        },
      ],
    });
    const assignedTo = String(values?.assigned_to ?? '').trim();
    if (!assignedTo) return;
    await assignMutation.mutateAsync({ id: row.id, assignedTo });
  }

  const items: ConversationInboxItem[] = filtered.map((row) => ({
    id: row.id,
    contactName: <strong>{row.party_name || row.phone}</strong>,
    contactDetail: row.phone,
    preview: row.last_message_preview?.trim() || 'Sin preview todavía.',
    assignee: row.assigned_to ? `Operador: ${row.assigned_to}` : 'Sin asignar',
    status: renderMessagingConversationStatusBadge(row.status),
    timestamp: formatMessagingConversationTimestamp(row.last_message_at),
    badge: row.unread_count > 0 ? <span className="badge badge-warning">{row.unread_count}</span> : undefined,
    unread: row.unread_count > 0,
    tone: row.status === 'resolved' ? 'success' : row.unread_count > 0 ? 'attention' : 'default',
    actions: (
      <>
        <button
          type="button"
          className="btn-secondary btn-sm"
          onClick={() => void handleAssign(row)}
          disabled={assignMutation.isPending || markReadMutation.isPending || resolveMutation.isPending}
        >
          Asignar
        </button>
        {row.unread_count > 0 ? (
          <button
            type="button"
            className="btn-secondary btn-sm"
            onClick={() => void markReadMutation.mutateAsync(row.id)}
            disabled={assignMutation.isPending || markReadMutation.isPending || resolveMutation.isPending}
          >
            Marcar leído
          </button>
        ) : null}
        {row.status === 'open' ? (
          <button
            type="button"
            className="btn-primary btn-sm"
            onClick={() => void resolveMutation.mutateAsync(row.id)}
            disabled={assignMutation.isPending || markReadMutation.isPending || resolveMutation.isPending}
          >
            Resolver
          </button>
        ) : null}
      </>
    ),
  }));

  return (
    <PageLayout
      title="Bandeja de Mensajería"
      lead="Conversaciones entrantes, asignación operativa y seguimiento básico por contacto."
      actions={
        <button
          type="button"
          className="btn-secondary btn-sm"
          onClick={() => void conversationsQuery.refetch()}
          disabled={conversationsQuery.isFetching}
        >
          Recargar
        </button>
      }
    >
      <ConversationInbox
        items={items}
        loading={conversationsQuery.isLoading}
        emptyMessage="No hay conversaciones para mostrar."
        error={error}
        summary={summary}
      />
    </PageLayout>
  );
}
