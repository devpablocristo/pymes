import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  listInAppNotifications,
  markInAppNotificationRead,
  type InAppNotificationItem,
} from '../lib/api';
import { labelForApprovalAction } from '../lib/approvalActionLabels';
import { formatFetchErrorForUser } from '../lib/formatFetchError';
import {
  NOTIFICATION_CHAT_HANDOFF_KEY,
  type NotificationChatHandoff,
} from '../lib/notificationChatHandoff';
import {
  approveRequest,
  listPendingApprovals,
  rejectRequest,
  type ApprovalResponse,
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

type FeedEntry =
  | { kind: 'in_app'; createdAt: string; notification: InAppNotificationItem }
  | { kind: 'approval'; createdAt: string; approval: ApprovalResponse };

function relativeTime(isoDate: string): string {
  const now = Date.now();
  const then = new Date(isoDate).getTime();
  const diffMs = now - then;
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return 'hace un momento';
  if (diffMin < 60) return `hace ${diffMin} minuto${diffMin === 1 ? '' : 's'}`;
  const diffHrs = Math.floor(diffMin / 60);
  if (diffHrs < 24) return `hace ${diffHrs} hora${diffHrs === 1 ? '' : 's'}`;
  const diffDays = Math.floor(diffHrs / 24);
  return `hace ${diffDays} día${diffDays === 1 ? '' : 's'}`;
}

function mergeFeed(notifications: InAppNotificationItem[], approvals: ApprovalResponse[]): FeedEntry[] {
  const rows: FeedEntry[] = [
    ...notifications.map((n) => ({
      kind: 'in_app' as const,
      createdAt: n.created_at,
      notification: n,
    })),
    ...approvals.map((a) => ({
      kind: 'approval' as const,
      createdAt: a.created_at,
      approval: a,
    })),
  ];
  rows.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
  return rows;
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

export function NotificationsCenterPage({ embedded = false }: NotificationsCenterPageProps) {
  const navigate = useNavigate();
  const [feed, setFeed] = useState<FeedEntry[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [pendingApprovalsCount, setPendingApprovalsCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [inAppError, setInAppError] = useState('');
  const [approvalNotes, setApprovalNotes] = useState<Record<string, string>>({});
  const [approvalProcessing, setApprovalProcessing] = useState<Record<string, boolean>>({});

  const load = useCallback(async () => {
    setLoading(true);
    setInAppError('');

    let notifications: InAppNotificationItem[] = [];
    let unread = 0;
    try {
      const res = await listInAppNotifications();
      notifications = res.items;
      unread = res.unread_count;
    } catch (err) {
      setInAppError(formatFetchErrorForUser(err, 'No se pudieron cargar los avisos in-app.'));
    }

    let approvals: ApprovalResponse[] = [];
    try {
      const res = await listPendingApprovals();
      approvals = res.approvals ?? [];
    } catch {
      approvals = [];
    }

    setUnreadCount(unread);
    setPendingApprovalsCount(approvals.length);
    setFeed(mergeFeed(notifications, approvals));
    setLoading(false);
  }, []);

  useEffect(() => {
    void load();
    const interval = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(interval);
  }, [load]);

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
    const handoff: NotificationChatHandoff = {
      notificationId: n.id,
      title: n.title,
      body: n.body,
      chatContext: n.chat_context && typeof n.chat_context === 'object' ? n.chat_context : {},
      scope: scope ?? undefined,
      routedAgent: routedAgent ?? undefined,
    };
    try {
      if (!n.read_at) {
        await markInAppNotificationRead(n.id);
      }
    } catch {
      // Seguimos al chat aunque falle marcar leída.
    }
    sessionStorage.setItem(NOTIFICATION_CHAT_HANDOFF_KEY, JSON.stringify(handoff));
    navigate('/chat');
    void load();
  }

  async function handleApprove(id: string): Promise<void> {
    setApprovalProcessing((prev) => ({ ...prev, [id]: true }));
    try {
      await approveRequest(id, approvalNotes[id] ?? '');
      await load();
    } finally {
      setApprovalProcessing((prev) => ({ ...prev, [id]: false }));
    }
  }

  async function handleReject(id: string): Promise<void> {
    setApprovalProcessing((prev) => ({ ...prev, [id]: true }));
    try {
      await rejectRequest(id, approvalNotes[id] ?? '');
      await load();
    } finally {
      setApprovalProcessing((prev) => ({ ...prev, [id]: false }));
    }
  }

  return (
    <>
      {inAppError ? <p className="form-error">{inAppError}</p> : null}
      {loading ? (
        <div className="card">
          <p>Cargando…</p>
        </div>
      ) : feed.length === 0 ? (
        <div className="card">
          <p className="text-secondary">No hay avisos ni solicitudes pendientes.</p>
        </div>
      ) : (
        <ul className="list-unstyled" style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {feed.map((entry) => {
            if (entry.kind === 'in_app') {
              const n = entry.notification;
              const scope = getNotificationScope(n.chat_context);
              const routedAgent = getNotificationRoutedAgent(n.chat_context);
              return (
                <li key={`n-${n.id}`} className="card" style={{ margin: 0, padding: '1rem' }}>
                  <div className="text-secondary" style={{ fontSize: '0.72rem', marginBottom: '0.35rem' }}>
                    Aviso
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: '1rem', flexWrap: 'wrap' }}>
                    <div style={{ flex: '1 1 12rem' }}>
                      <div style={{ fontWeight: 600 }}>{n.title}</div>
                      <p style={{ margin: '0.35rem 0 0', whiteSpace: 'pre-wrap' }}>{n.body}</p>
                      <div className="text-secondary" style={{ fontSize: '0.8rem', marginTop: '0.5rem' }}>
                        {new Date(n.created_at).toLocaleString()}
                        {n.read_at ? ' · Leída' : ''}
                      </div>
                      {scope || routedAgent ? (
                        <div className="text-secondary" style={{ fontSize: '0.78rem', marginTop: '0.25rem' }}>
                          {routedAgent ? `Agente: ${routedAgent}` : null}
                          {routedAgent && scope ? ' · ' : null}
                          {scope ? `Scope: ${scope}` : null}
                        </div>
                      ) : null}
                    </div>
                    <div
                      style={{
                        alignSelf: 'flex-end',
                        display: 'flex',
                        flexWrap: 'wrap',
                        gap: '0.5rem',
                        justifyContent: 'flex-end',
                      }}
                    >
                      <button
                        type="button"
                        className="btn-secondary btn-sm"
                        onClick={() =>
                          openWhatsAppPrefilledShare(buildInAppNotificationShareText(n.title, n.body))
                        }
                      >
                        Compartir
                      </button>
                      <button type="button" className="btn-primary btn-sm" onClick={() => void openInChat(n)}>
                        Más información
                      </button>
                    </div>
                  </div>
                </li>
              );
            }

            const approval = entry.approval;
            const isProcessing = approvalProcessing[approval.id] ?? false;
            const displayAction = labelForApprovalAction(approval.action_type);
            return (
              <li key={`a-${approval.id}`} className="approval-card" style={{ margin: 0, listStyle: 'none' }}>
                <div className="text-secondary" style={{ fontSize: '0.72rem', marginBottom: '0.35rem' }}>
                  Requiere decisión
                </div>
                <div className="approval-header">
                  <span className="approval-title">
                    {displayAction}
                    {approval.target_resource ? ` — ${approval.target_resource}` : ''}
                  </span>
                  <span className={`risk-badge ${approval.risk_level}`}>{approval.risk_level}</span>
                </div>
                <div className="approval-reason">{approval.reason}</div>
                <div className="approval-time">{relativeTime(approval.created_at)}</div>
                {approval.ai_summary ? <div className="approval-summary">{approval.ai_summary}</div> : null}
                <div style={{ marginBottom: '8px' }}>
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
                    Compartir
                  </button>
                </div>
                <div className="approval-actions">
                  <input
                    className="note-input"
                    placeholder="Nota (opcional)"
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
                    Aprobar
                  </button>
                  <button
                    type="button"
                    className="btn-reject"
                    disabled={isProcessing}
                    onClick={() => void handleReject(approval.id)}
                  >
                    Rechazar
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </>
  );
}

export default NotificationsCenterPage;
