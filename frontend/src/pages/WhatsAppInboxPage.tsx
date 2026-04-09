import { useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearch } from '@devpablocristo/modules-search';
import {
  ConversationInbox,
  type ConversationInboxItem,
} from '@devpablocristo/modules-ui-conversation-inbox';
import '@devpablocristo/modules-ui-notification-feed/styles.css';
import '@devpablocristo/modules-ui-conversation-inbox/styles.css';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { formatDate } from '../crud/resourceConfigs.shared';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import {
  assignWhatsAppConversation,
  listCustomerMessagingConversations,
  markWhatsAppConversationRead,
  resolveWhatsAppConversation,
  type CustomerMessagingConversation,
} from '../lib/api';
import { queryKeys } from '../lib/queryKeys';

function statusBadge(status: string) {
  const cls = status === 'open' ? 'badge-success' : status === 'resolved' ? 'badge-neutral' : 'badge-warning';
  const label = status === 'open' ? 'Abierta' : status === 'resolved' ? 'Resuelta' : status;
  return <span className={`badge ${cls}`}>{label}</span>;
}

export function CustomerMessagingInboxPage() {
  const queryClient = useQueryClient();
  const search = usePageSearch();
  const conversationsQuery = useQuery({
    queryKey: queryKeys.customerMessaging.conversations,
    queryFn: () => listCustomerMessagingConversations(),
    refetchInterval: 30_000,
  });
  const assignMutation = useMutation({
    mutationFn: ({ id, assignedTo }: { id: string; assignedTo: string }) => assignWhatsAppConversation(id, assignedTo),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.customerMessaging.conversations });
    },
  });
  const markReadMutation = useMutation({
    mutationFn: markWhatsAppConversationRead,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.customerMessaging.conversations });
    },
  });
  const resolveMutation = useMutation({
    mutationFn: resolveWhatsAppConversation,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.customerMessaging.conversations });
    },
  });

  const conversations: CustomerMessagingConversation[] = conversationsQuery.data?.items ?? [];
  const filtered = useSearch(
    conversations,
    (row) => [row.party_name, row.phone, row.assigned_to, row.last_message_preview, row.status].filter(Boolean).join(' '),
    search,
  );

  const summary = useMemo(() => {
    const unread = conversations.reduce((acc, row) => acc + Number(row.unread_count ?? 0), 0);
    const open = conversations.filter((row) => row.status === 'open').length;
    const assigned = conversations.filter((row) => row.assigned_to).length;
    return `${filtered.length} visibles · ${open} abiertas · ${unread} sin leer · ${assigned} asignadas`;
  }, [conversations, filtered.length]);

  const error = conversationsQuery.error
    ? formatFetchErrorForUser(conversationsQuery.error, 'No se pudo cargar la bandeja de conversaciones.')
    : '';

  async function handleAssign(row: CustomerMessagingConversation): Promise<void> {
    const assignedTo = (window.prompt('ID del operador (user_id)', row.assigned_to) ?? '').trim();
    if (!assignedTo) return;
    await assignMutation.mutateAsync({ id: row.id, assignedTo });
  }

  const items: ConversationInboxItem[] = filtered.map((row) => ({
    id: row.id,
    contactName: <strong>{row.party_name || row.phone}</strong>,
    contactDetail: row.phone,
    preview: row.last_message_preview?.trim() || 'Sin preview todavía.',
    assignee: row.assigned_to ? `Operador: ${row.assigned_to}` : 'Sin asignar',
    status: statusBadge(row.status),
    timestamp: row.last_message_at ? formatDate(row.last_message_at) : 'Sin actividad todavía',
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
      title="Bandeja WhatsApp"
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

export const WhatsAppInboxPage = CustomerMessagingInboxPage
