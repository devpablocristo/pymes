import { useCallback, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { usePageSearch } from '../components/PageSearch';
import { useSearch } from '@devpablocristo/modules-search';
import { useNavigate } from 'react-router-dom';
import {
  NotificationFeed,
  type NotificationFeedItem,
  type NotificationFeedTone,
} from '@devpablocristo/modules-ui-notification-feed';
import '@devpablocristo/modules-ui-notification-feed/styles.css';
import {
  listInAppNotifications,
  markInAppNotificationRead,
  type InAppNotificationItem,
} from '../lib/api';
import { labelForApprovalAction } from '../lib/approvalActionLabels';
import { humanInsightScopeLabel, humanRoutedLabel } from '../lib/aiLabels';
import { PageLayout } from '../components/PageLayout';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import { useI18n, type LanguageCode } from '../lib/i18n';
import { queryKeys } from '../lib/queryKeys';
import {
  NOTIFICATION_CHAT_HANDOFF_KEY,
  type NotificationChatHandoff,
} from '../lib/notificationChatHandoff';
import {
  approveRequest,
  rejectRequest,
} from '../lib/reviewApi';
import {
  buildApprovalShareText,
  buildInAppNotificationShareText,
  openWhatsAppPrefilledShare,
} from '../lib/whatsappPrefillShare';
import './ApprovalInboxPage.css';

type NotificationsCenterPageProps = {
  /** Dentro de Ajustes: sin cabecera de página completa. */
  embedded?: boolean;
};

type ApprovalNotification = {
  id: string;
  request_id: string;
  action_type: string;
  target_resource: string;
  reason: string;
  risk_level: string;
  status: string;
  ai_summary?: string;
  created_at: string;
  expires_at?: string;
};

function localeForLanguage(language: LanguageCode): string {
  return language === 'en' ? 'en-US' : 'es-AR';
}

function relativeTime(isoDate: string, language: LanguageCode, t: (key: string, variables?: Record<string, string | number>) => string): string {
  const now = Date.now();
  const then = new Date(isoDate).getTime();
  const diffMs = now - then;
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return t('ai.notifications.relative.now');
  if (diffMin < 60) {
    return diffMin === 1
      ? t('ai.notifications.relative.minute', { count: diffMin })
      : t('ai.notifications.relative.minutes', { count: diffMin });
  }
  const diffHrs = Math.floor(diffMin / 60);
  if (diffHrs < 24) {
    return diffHrs === 1
      ? t('ai.notifications.relative.hour', { count: diffHrs })
      : t('ai.notifications.relative.hours', { count: diffHrs });
  }
  const diffDays = Math.floor(diffHrs / 24);
  if (language === 'en') {
    return diffDays === 1
      ? t('ai.notifications.relative.day', { count: diffDays })
      : t('ai.notifications.relative.days', { count: diffDays });
  }
  return diffDays === 1
    ? t('ai.notifications.relative.day', { count: diffDays })
    : t('ai.notifications.relative.days', { count: diffDays });
}

function getApprovalNotification(data: unknown): ApprovalNotification | null {
  if (!data || typeof data !== 'object') return null;
  const record = data as Record<string, unknown>;
  if (record.source !== 'review_approval') return null;
  const approval = record.approval;
  if (!approval || typeof approval !== 'object') return null;
  const raw = approval as Record<string, unknown>;
  const id = typeof raw.id === 'string' ? raw.id.trim() : '';
  if (!id) return null;
  return {
    id,
    request_id: typeof raw.request_id === 'string' ? raw.request_id : '',
    action_type: typeof raw.action_type === 'string' ? raw.action_type : '',
    target_resource: typeof raw.target_resource === 'string' ? raw.target_resource : '',
    reason: typeof raw.reason === 'string' ? raw.reason : '',
    risk_level: typeof raw.risk_level === 'string' ? raw.risk_level : 'low',
    status: typeof raw.status === 'string' ? raw.status : 'pending',
    ai_summary: typeof raw.ai_summary === 'string' ? raw.ai_summary : undefined,
    created_at: typeof raw.created_at === 'string' ? raw.created_at : '',
    expires_at: typeof raw.expires_at === 'string' ? raw.expires_at : undefined,
  };
}

function getNotificationScope(chatContext: unknown): string | null {
  if (!chatContext || typeof chatContext !== 'object') return null;
  const scope = (chatContext as Record<string, unknown>).scope;
  return typeof scope === 'string' && scope.trim() !== '' ? scope : null;
}

function getNotificationRoutedAgent(chatContext: unknown): string | null {
  if (!chatContext || typeof chatContext !== 'object') return null;
  const routedAgent = (chatContext as Record<string, unknown>).routed_agent;
  return typeof routedAgent === 'string' && routedAgent.trim() !== '' ? routedAgent : null;
}

function getNotificationContentLanguage(chatContext: unknown): string | null {
  if (!chatContext || typeof chatContext !== 'object') return null;
  const contentLanguage = (chatContext as Record<string, unknown>).content_language;
  return typeof contentLanguage === 'string' && contentLanguage.trim() !== '' ? contentLanguage : null;
}

function toneForApproval(riskLevel: string): NotificationFeedTone {
  switch (riskLevel) {
    case 'high':
      return 'critical';
    case 'medium':
      return 'attention';
    case 'low':
      return 'default';
    default:
      return 'default';
  }
}

export function NotificationsCenterPage({ embedded = false }: NotificationsCenterPageProps) {
  const { language, t } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [approvalNotes, setApprovalNotes] = useState<Record<string, string>>({});
  const [approvalProcessing, setApprovalProcessing] = useState<Record<string, boolean>>({});
  const notificationsQuery = useQuery({
    queryKey: queryKeys.notifications.inApp,
    queryFn: listInAppNotifications,
    refetchInterval: 30_000,
  });
  const markReadMutation = useMutation({
    mutationFn: markInAppNotificationRead,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.notifications.inApp });
    },
  });
  const approveMutation = useMutation({
    mutationFn: ({ id, note }: { id: string; note: string }) => approveRequest(id, note),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.notifications.inApp });
    },
  });
  const rejectMutation = useMutation({
    mutationFn: ({ id, note }: { id: string; note: string }) => rejectRequest(id, note),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.notifications.inApp });
    },
  });
  const notifications = notificationsQuery.data?.items ?? [];
  const unreadCount = notificationsQuery.data?.unread_count ?? 0;
  const pendingApprovalsCount = notifications.filter((item) => getApprovalNotification(item.chat_context)).length;
  const inAppError = notificationsQuery.error
    ? formatFetchErrorForUser(notificationsQuery.error, t('ai.notifications.error.loadInApp'))
    : '';

  const summaryBadge = useMemo(() => {
    const parts: string[] = [];
    if (unreadCount > 0) parts.push(`${unreadCount} sin leer`);
    if (pendingApprovalsCount > 0) parts.push(`${pendingApprovalsCount} decisión${pendingApprovalsCount === 1 ? '' : 'es'}`);
    if (parts.length === 0) return 'Al día';
    return parts.join(' · ');
  }, [pendingApprovalsCount, unreadCount]);

  async function openInChat(n: InAppNotificationItem): Promise<void> {
    const scope = getNotificationScope(n.chat_context);
    const routedAgent = getNotificationRoutedAgent(n.chat_context);
    const contentLanguage = getNotificationContentLanguage(n.chat_context);
    const handoff: NotificationChatHandoff = {
      notificationId: n.id,
      title: n.title,
      body: n.body,
      chatContext: n.chat_context && typeof n.chat_context === 'object' ? n.chat_context : {},
      scope: scope ?? undefined,
      routedAgent: routedAgent ?? undefined,
      contentLanguage: contentLanguage ?? undefined,
    };
    try {
      if (!n.read_at) {
        await markReadMutation.mutateAsync(n.id);
      }
    } catch {
      // Seguimos al chat aunque falle marcar leída.
    }
    sessionStorage.setItem(NOTIFICATION_CHAT_HANDOFF_KEY, JSON.stringify(handoff));
    navigate('/chat');
  }

  async function handleApprove(id: string): Promise<void> {
    setApprovalProcessing((prev) => ({ ...prev, [id]: true }));
    try {
      await approveMutation.mutateAsync({ id, note: approvalNotes[id] ?? '' });
    } finally {
      setApprovalProcessing((prev) => ({ ...prev, [id]: false }));
    }
  }

  async function handleReject(id: string): Promise<void> {
    setApprovalProcessing((prev) => ({ ...prev, [id]: true }));
    try {
      await rejectMutation.mutateAsync({ id, note: approvalNotes[id] ?? '' });
    } finally {
      setApprovalProcessing((prev) => ({ ...prev, [id]: false }));
    }
  }

  const pageSearch = usePageSearch();
  const notifTextFn = useCallback((n: InAppNotificationItem) => `${n.title ?? ''} ${n.body ?? ''}`, []);
  const filteredNotifications = useSearch(notifications, notifTextFn, pageSearch);

  const items: NotificationFeedItem[] = filteredNotifications.map((n) => {
    const approval = getApprovalNotification(n.chat_context);
    if (approval) {
      const isProcessing = approvalProcessing[approval.id] ?? false;
      const displayAction = labelForApprovalAction(approval.action_type);

      return {
        id: `a-${n.id}`,
        eyebrow: t('ai.notifications.approval.requiresDecision'),
        title: (
          <>
            {displayAction}
            {approval.target_resource ? ` — ${approval.target_resource}` : ''}
          </>
        ),
        badge: <span className={`risk-badge ${approval.risk_level}`}>{approval.risk_level}</span>,
        body: (
          <>
            <div className="approval-reason">{approval.reason}</div>
            {approval.ai_summary ? <div className="approval-summary">{approval.ai_summary}</div> : null}
          </>
        ),
        timestamp: relativeTime(approval.created_at || n.created_at, language, t),
        extra: (
          <button
            type="button"
            className="btn-secondary btn-sm"
            onClick={() =>
              openWhatsAppPrefilledShare(
                buildApprovalShareText({
                  actionLabel: displayAction,
                  targetResource: approval.target_resource,
                  reason: approval.reason,
                  aiSummary: approval.ai_summary,
                }),
              )
            }
          >
            {t('ai.notifications.item.share')}
          </button>
        ),
        actions: (
          <div className="approval-actions">
            <input
              className="note-input"
              aria-label={`Nota para ${displayAction}`}
              placeholder={t('ai.notifications.approval.notePlaceholder')}
              value={approvalNotes[approval.id] ?? ''}
              onChange={(e) =>
                setApprovalNotes((prev) => ({ ...prev, [approval.id]: e.target.value }))
              }
            />
            <button
              type="button"
              className="btn-approve"
              disabled={isProcessing}
              onClick={() => void handleApprove(approval.id)}
            >
              {t('ai.notifications.approval.approve')}
            </button>
            <button
              type="button"
              className="btn-reject"
              disabled={isProcessing}
              onClick={() => void handleReject(approval.id)}
            >
              {t('ai.notifications.approval.reject')}
            </button>
          </div>
        ),
        unread: !n.read_at,
        tone: toneForApproval(approval.risk_level),
      };
    }

    const scope = getNotificationScope(n.chat_context);
    const routedAgent = getNotificationRoutedAgent(n.chat_context);

    return {
      id: `n-${n.id}`,
      eyebrow: t('ai.notifications.item.notice'),
      title: n.title,
      body: <p className="u-m-0 u-pre-wrap">{n.body}</p>,
      timestamp: (
        <>
          {new Date(n.created_at).toLocaleString(localeForLanguage(language))}
          {n.read_at ? ` · ${t('ai.notifications.item.read')}` : ''}
        </>
      ),
      meta:
        scope || routedAgent ? (
          <>
            {routedAgent ? `${t('ai.chat.meta.agent')}: ${humanRoutedLabel(routedAgent, language)}` : null}
            {routedAgent && scope ? ' · ' : null}
            {scope ? `${t('ai.chat.meta.context')}: ${humanInsightScopeLabel(scope, language)}` : null}
          </>
        ) : undefined,
      actions: (
        <>
          <button
            type="button"
            className="btn-secondary btn-sm"
            onClick={() => openWhatsAppPrefilledShare(buildInAppNotificationShareText(n.title, n.body))}
          >
            {t('ai.notifications.item.share')}
          </button>
          <button type="button" className="btn-primary btn-sm" onClick={() => void openInChat(n)}>
            {t('ai.notifications.item.explainInChat')}
          </button>
        </>
      ),
      unread: !n.read_at,
      tone: 'default',
    };
  });

  const feed = (
    <NotificationFeed
      error={inAppError ? <p role="alert" className="form-error">{inAppError}</p> : undefined}
      loading={notificationsQuery.isLoading}
      loadingMessage={t('ai.notifications.loading')}
      emptyMessage={<p className="text-secondary">{t('ai.notifications.empty')}</p>}
      items={items}
      summary={summaryBadge}
    />
  );

  if (embedded) {
    return (
      <div data-embedded="true">
        {feed}
      </div>
    );
  }

  return (
    <PageLayout title={t('ai.notifications.pageTitle')} lead={t('ai.notifications.pageLead')}>
      {feed}
    </PageLayout>
  );
}

export default NotificationsCenterPage;
