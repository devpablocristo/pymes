import { useCallback, useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { getAutoRepairWorkOrder, updateAutoRepairWorkOrder } from '../lib/autoRepairApi';
import type { AutoRepairWorkOrder } from '../lib/autoRepairTypes';
import './WorkOrderKanbanDetailModal.css';

const STATUS_OPTIONS: { value: string; label: string }[] = [
  { value: 'received', label: 'Recibido' },
  { value: 'diagnosing', label: 'Diagnóstico' },
  { value: 'quote_pending', label: 'Presupuesto' },
  { value: 'awaiting_parts', label: 'Repuestos' },
  { value: 'in_progress', label: 'En taller' },
  { value: 'quality_check', label: 'Control' },
  { value: 'ready_for_pickup', label: 'Listo retiro' },
  { value: 'delivered', label: 'Entregado' },
  { value: 'invoiced', label: 'Facturado' },
  { value: 'on_hold', label: 'En pausa' },
  { value: 'cancelled', label: 'Cancelado' },
];

function toDatetimeLocalValue(iso: string | undefined): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function fromDatetimeLocal(s: string): string | undefined {
  const t = s.trim();
  if (!t) return undefined;
  const d = new Date(t);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toISOString();
}

type Draft = {
  status: string;
  vehicle_plate: string;
  customer_name: string;
  requested_work: string;
  diagnosis: string;
  notes: string;
  internal_notes: string;
  currency: string;
  promised_at_local: string;
};

function woToDraft(wo: AutoRepairWorkOrder): Draft {
  return {
    status: wo.status,
    vehicle_plate: wo.vehicle_plate ?? '',
    customer_name: wo.customer_name ?? '',
    requested_work: wo.requested_work ?? '',
    diagnosis: wo.diagnosis ?? '',
    notes: wo.notes ?? '',
    internal_notes: wo.internal_notes ?? '',
    currency: wo.currency ?? 'ARS',
    promised_at_local: toDatetimeLocalValue(wo.promised_at),
  };
}

export type WorkOrderKanbanDetailModalProps = {
  orderId: string | null;
  onClose: () => void;
  onSaved: (wo: AutoRepairWorkOrder) => void;
};

export function WorkOrderKanbanDetailModal({ orderId, onClose, onSaved }: WorkOrderKanbanDetailModalProps) {
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [wo, setWo] = useState<AutoRepairWorkOrder | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      const data = await getAutoRepairWorkOrder(id);
      setWo(data);
      setDraft(woToDraft(data));
    } catch (e) {
      setWo(null);
      setDraft(null);
      setError(e instanceof Error ? e.message : 'No se pudo cargar la orden');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!orderId) {
      setWo(null);
      setDraft(null);
      setError(null);
      return;
    }
    void load(orderId);
  }, [orderId, load]);

  useEffect(() => {
    if (!orderId) return;
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [orderId, onClose]);

  const isDirty = useMemo(() => {
    if (!wo || !draft) return false;
    const nextPromised = fromDatetimeLocal(draft.promised_at_local);
    const prevPromised = wo.promised_at;
    const promisedDiffers = (nextPromised ?? '') !== (prevPromised ?? '');
    const onlyClearingPromised = !!prevPromised && !nextPromised;
    return (
      draft.status !== wo.status ||
      draft.vehicle_plate !== (wo.vehicle_plate ?? '') ||
      draft.customer_name !== (wo.customer_name ?? '') ||
      draft.requested_work !== (wo.requested_work ?? '') ||
      draft.diagnosis !== (wo.diagnosis ?? '') ||
      draft.notes !== (wo.notes ?? '') ||
      draft.internal_notes !== (wo.internal_notes ?? '') ||
      draft.currency !== (wo.currency ?? '') ||
      (promisedDiffers && !onlyClearingPromised)
    );
  }, [wo, draft]);

  const canSave = isDirty && !saving && !loading && wo != null && draft != null;

  const handleSave = async () => {
    if (!orderId || !wo || !draft) return;
    setSaving(true);
    setError(null);
    try {
      const body: Parameters<typeof updateAutoRepairWorkOrder>[1] = {};
      if (draft.status !== wo.status) body.status = draft.status;
      if (draft.vehicle_plate !== (wo.vehicle_plate ?? '')) body.vehicle_plate = draft.vehicle_plate;
      if (draft.customer_name !== (wo.customer_name ?? '')) body.customer_name = draft.customer_name;
      if (draft.requested_work !== (wo.requested_work ?? '')) body.requested_work = draft.requested_work;
      if (draft.diagnosis !== (wo.diagnosis ?? '')) body.diagnosis = draft.diagnosis;
      if (draft.notes !== (wo.notes ?? '')) body.notes = draft.notes;
      if (draft.internal_notes !== (wo.internal_notes ?? '')) body.internal_notes = draft.internal_notes;
      if (draft.currency !== (wo.currency ?? '')) body.currency = draft.currency;

      const nextPromised = fromDatetimeLocal(draft.promised_at_local);
      if ((nextPromised ?? '') !== (wo.promised_at ?? '') && nextPromised) {
        body.promised_at = nextPromised;
      }

      if (Object.keys(body).length === 0) {
        onClose();
        return;
      }

      const updated = await updateAutoRepairWorkOrder(orderId, body);
      setWo(updated);
      setDraft(woToDraft(updated));
      onSaved(updated);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo guardar');
    } finally {
      setSaving(false);
    }
  };

  if (!orderId) return null;

  const modal = (
    <div
      className="wo-modal-backdrop"
      role="presentation"
      onMouseDown={(ev) => {
        if (ev.target === ev.currentTarget) onClose();
      }}
    >
      <div className="wo-modal" role="dialog" aria-modal="true" aria-labelledby="wo-modal-title">
        <div className="wo-modal__header">
          <div className="wo-modal__title-block">
            <div className="wo-modal__eyebrow">Orden de trabajo</div>
            <h2 id="wo-modal-title" className="wo-modal__title">
              {loading ? 'Cargando…' : wo?.number ?? '—'}
            </h2>
          </div>
          <button type="button" className="wo-modal__close" onClick={onClose} aria-label="Cerrar">
            ×
          </button>
        </div>

        <div className="wo-modal__body">
          {error ? (
            <p className="wo-modal__error" role="alert">
              {error}
            </p>
          ) : null}
          {loading || !wo || !draft ? (
            <p className="wo-modal__loading">{loading ? 'Cargando datos…' : null}</p>
          ) : (
            <>
              <div className="wo-modal__grid">
                <div className="wo-modal__field">
                  <label className="wo-modal__label" htmlFor="wo-status">
                    Estado
                  </label>
                  <select
                    id="wo-status"
                    className="wo-modal__select"
                    value={draft.status}
                    onChange={(ev) => setDraft((d) => (d ? { ...d, status: ev.target.value } : d))}
                  >
                    {STATUS_OPTIONS.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="wo-modal__field">
                  <label className="wo-modal__label" htmlFor="wo-currency">
                    Moneda
                  </label>
                  <input
                    id="wo-currency"
                    className="wo-modal__input"
                    value={draft.currency}
                    onChange={(ev) => setDraft((d) => (d ? { ...d, currency: ev.target.value.toUpperCase().slice(0, 8) } : d))}
                    maxLength={8}
                  />
                </div>
                <div className="wo-modal__field">
                  <label className="wo-modal__label" htmlFor="wo-plate">
                    Patente
                  </label>
                  <input
                    id="wo-plate"
                    className="wo-modal__input"
                    value={draft.vehicle_plate}
                    onChange={(ev) => setDraft((d) => (d ? { ...d, vehicle_plate: ev.target.value } : d))}
                  />
                </div>
                <div className="wo-modal__field">
                  <label className="wo-modal__label" htmlFor="wo-customer">
                    Cliente
                  </label>
                  <input
                    id="wo-customer"
                    className="wo-modal__input"
                    value={draft.customer_name}
                    onChange={(ev) => setDraft((d) => (d ? { ...d, customer_name: ev.target.value } : d))}
                  />
                </div>
                <div className="wo-modal__field wo-modal__field--full">
                  <label className="wo-modal__label" htmlFor="wo-promised">
                    Prometida para
                  </label>
                  <input
                    id="wo-promised"
                    type="datetime-local"
                    className="wo-modal__input"
                    value={draft.promised_at_local}
                    onChange={(ev) => setDraft((d) => (d ? { ...d, promised_at_local: ev.target.value } : d))}
                  />
                </div>
                <div className="wo-modal__field wo-modal__field--full">
                  <label className="wo-modal__label" htmlFor="wo-requested">
                    Trabajo solicitado
                  </label>
                  <textarea
                    id="wo-requested"
                    className="wo-modal__textarea"
                    value={draft.requested_work}
                    onChange={(ev) => setDraft((d) => (d ? { ...d, requested_work: ev.target.value } : d))}
                  />
                </div>
                <div className="wo-modal__field wo-modal__field--full">
                  <label className="wo-modal__label" htmlFor="wo-diagnosis">
                    Diagnóstico
                  </label>
                  <textarea
                    id="wo-diagnosis"
                    className="wo-modal__textarea"
                    value={draft.diagnosis}
                    onChange={(ev) => setDraft((d) => (d ? { ...d, diagnosis: ev.target.value } : d))}
                  />
                </div>
                <div className="wo-modal__field wo-modal__field--full">
                  <label className="wo-modal__label" htmlFor="wo-notes">
                    Notas
                  </label>
                  <textarea
                    id="wo-notes"
                    className="wo-modal__textarea"
                    value={draft.notes}
                    onChange={(ev) => setDraft((d) => (d ? { ...d, notes: ev.target.value } : d))}
                  />
                </div>
                <div className="wo-modal__field wo-modal__field--full">
                  <label className="wo-modal__label" htmlFor="wo-internal">
                    Notas internas
                  </label>
                  <textarea
                    id="wo-internal"
                    className="wo-modal__textarea"
                    value={draft.internal_notes}
                    onChange={(ev) => setDraft((d) => (d ? { ...d, internal_notes: ev.target.value } : d))}
                  />
                </div>
              </div>

              <div className="wo-modal__field wo-modal__field--full">
                <span className="wo-modal__label">Totales (solo lectura)</span>
                <p className="wo-modal__readonly">
                  Servicios {wo.subtotal_services.toLocaleString()} · Repuestos {wo.subtotal_parts.toLocaleString()} · IVA{' '}
                  {wo.tax_total.toLocaleString()} · Total {wo.total.toLocaleString()} {wo.currency}
                </p>
              </div>

              {wo.items.length > 0 ? (
                <div className="wo-modal__field wo-modal__field--full">
                  <span className="wo-modal__label">Ítems (editar en lista detalle)</span>
                  <ul className="wo-modal__items">
                    {wo.items.map((it) => (
                      <li key={it.id ?? `${it.description}-${it.sort_order}`}>
                        {it.item_type}: {it.description} × {it.quantity}
                      </li>
                    ))}
                  </ul>
                </div>
              ) : null}

              <div className="wo-modal__footer">
                <button type="button" className="wo-modal__btn wo-modal__btn--ghost" onClick={onClose}>
                  Cancelar
                </button>
                <button type="button" className="wo-modal__btn wo-modal__btn--primary" disabled={!canSave} onClick={() => void handleSave()}>
                  {saving ? 'Guardando…' : 'Guardar'}
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );

  return createPortal(modal, document.body);
}
