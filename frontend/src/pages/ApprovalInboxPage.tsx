import { useCallback, useEffect, useState } from 'react';
import {
  listPendingApprovals,
  approveRequest,
  rejectRequest,
  type ApprovalResponse,
} from '../lib/reviewApi';
import './ApprovalInboxPage.css';

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
  return `hace ${diffDays} dia${diffDays === 1 ? '' : 's'}`;
}

const ACTION_TYPE_DISPLAY: Record<string, string> = {
  'appointment.book': 'Agendar turno',
  'appointment.reschedule': 'Reagendar turno',
  'appointment.cancel': 'Cancelar turno',
  'discount.apply': 'Aplicar descuento',
  'payment_link.generate': 'Link de pago',
  'refund.create': 'Reembolso',
  'sale.create': 'Crear venta',
  'quote.create': 'Crear presupuesto',
  'notification.bulk_send': 'Envio masivo',
};

export default function ApprovalInboxPage() {
  const [approvals, setApprovals] = useState<ApprovalResponse[]>([]);
  const [loading, setLoading] = useState(true);
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [processing, setProcessing] = useState<Record<string, boolean>>({});

  const loadApprovals = useCallback(async () => {
    try {
      const resp = await listPendingApprovals();
      setApprovals(resp.approvals || []);
    } catch {
      setApprovals([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadApprovals();
    const interval = setInterval(loadApprovals, 30000);
    return () => clearInterval(interval);
  }, [loadApprovals]);

  const handleApprove = async (id: string) => {
    setProcessing((prev) => ({ ...prev, [id]: true }));
    try {
      await approveRequest(id, notes[id] || '');
      setApprovals((prev) => prev.filter((a) => a.id !== id));
    } catch {
      // silenciar — el card permanece
    } finally {
      setProcessing((prev) => ({ ...prev, [id]: false }));
    }
  };

  const handleReject = async (id: string) => {
    setProcessing((prev) => ({ ...prev, [id]: true }));
    try {
      await rejectRequest(id, notes[id] || '');
      setApprovals((prev) => prev.filter((a) => a.id !== id));
    } catch {
      // silenciar
    } finally {
      setProcessing((prev) => ({ ...prev, [id]: false }));
    }
  };

  if (loading) {
    return (
      <div className="approval-inbox-page">
        <div className="loading">Cargando aprobaciones...</div>
      </div>
    );
  }

  return (
    <div className="approval-inbox-page">
      <h1>
        Aprobaciones pendientes
        {approvals.length > 0 && (
          <span className="count-badge">{approvals.length}</span>
        )}
      </h1>
      <p className="subtitle">Solicitudes que requieren tu decision</p>

      {approvals.length === 0 && (
        <div className="empty-state">No hay aprobaciones pendientes</div>
      )}

      {approvals.map((approval) => {
        const isProcessing = processing[approval.id] || false;
        const displayAction =
          ACTION_TYPE_DISPLAY[approval.action_type] || approval.action_type;
        return (
          <div key={approval.id} className="approval-card">
            <div className="approval-header">
              <span className="approval-title">
                {displayAction}
                {approval.target_resource
                  ? ` — ${approval.target_resource}`
                  : ''}
              </span>
              <span className={`risk-badge ${approval.risk_level}`}>
                {approval.risk_level}
              </span>
            </div>
            <div className="approval-reason">{approval.reason}</div>
            <div className="approval-time">
              {relativeTime(approval.created_at)}
            </div>
            {approval.ai_summary && (
              <div className="approval-summary">{approval.ai_summary}</div>
            )}
            <div className="approval-actions">
              <input
                className="note-input"
                placeholder="Nota (opcional)"
                value={notes[approval.id] || ''}
                onChange={(e) =>
                  setNotes((prev) => ({ ...prev, [approval.id]: e.target.value }))
                }
              />
              <button
                className="btn-approve"
                disabled={isProcessing}
                onClick={() => handleApprove(approval.id)}
              >
                Aprobar
              </button>
              <button
                className="btn-reject"
                disabled={isProcessing}
                onClick={() => handleReject(approval.id)}
              >
                Rechazar
              </button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
