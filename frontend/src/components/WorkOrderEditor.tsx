import { useCallback, useEffect, useMemo, useState } from 'react';
import { confirmAction } from '@devpablocristo/core-browser';
import { createPortal } from 'react-dom';
import {
  archiveAutoRepairWorkOrder,
  getAutoRepairWorkOrder,
  restoreAutoRepairWorkOrder,
  updateAutoRepairWorkOrder,
} from '../lib/autoRepairApi';
import type { AutoRepairWorkOrder } from '../lib/autoRepairTypes';
import { parseWorkOrderItemsJson, stringifyWorkOrderItems } from '../lib/workOrderItemsJson';
import './WorkOrderKanbanDetailModal.css';
import './WorkOrderEditor.css';

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
  vehicle_id: string;
  vehicle_plate: string;
  customer_id: string;
  customer_name: string;
  booking_id: string;
  requested_work: string;
  diagnosis: string;
  notes: string;
  internal_notes: string;
  currency: string;
  promised_at_local: string;
  ready_at_local: string;
  delivered_at_local: string;
  items_json: string;
};

function itemsJsonDirty(json: string, wo: AutoRepairWorkOrder): boolean {
  try {
    const parsed = parseWorkOrderItemsJson(json);
    return JSON.stringify(parsed) !== JSON.stringify(wo.items ?? []);
  } catch {
    return true;
  }
}

function woToDraft(wo: AutoRepairWorkOrder): Draft {
  return {
    status: wo.status,
    vehicle_id: wo.vehicle_id ?? '',
    vehicle_plate: wo.vehicle_plate ?? '',
    customer_id: wo.customer_id ?? '',
    customer_name: wo.customer_name ?? '',
    booking_id: wo.booking_id ?? '',
    requested_work: wo.requested_work ?? '',
    diagnosis: wo.diagnosis ?? '',
    notes: wo.notes ?? '',
    internal_notes: wo.internal_notes ?? '',
    currency: wo.currency ?? 'ARS',
    promised_at_local: toDatetimeLocalValue(wo.promised_at),
    ready_at_local: toDatetimeLocalValue(wo.ready_at),
    delivered_at_local: toDatetimeLocalValue(wo.delivered_at),
    items_json: stringifyWorkOrderItems(wo.items),
  };
}

export type WorkOrderEditorProps = {
  orderId: string;
  /** modal: portal oscuro; page: tarjeta embebida (misma UI de formulario). */
  variant: 'modal' | 'page';
  onClose: () => void;
  onSaved: (wo: AutoRepairWorkOrder) => void;
  /** Tras archivar (p. ej. quitar tarjeta del Kanban). */
  onRecordRemoved?: (id: string) => void;
};

/**
 * Único editor de OT (estado, vínculos, textos, fechas, ítems JSON).
 */
export function WorkOrderEditor({ orderId, variant, onClose, onSaved, onRecordRemoved }: WorkOrderEditorProps) {
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [wo, setWo] = useState<AutoRepairWorkOrder | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [archiveBusy, setArchiveBusy] = useState(false);
  const [restoreBusy, setRestoreBusy] = useState(false);

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
    void load(orderId);
  }, [orderId, load]);

  const isDirty = useMemo(() => {
    if (!wo || !draft) return false;
    const nextPromised = fromDatetimeLocal(draft.promised_at_local);
    const prevPromised = wo.promised_at;
    const promisedDiffers = (nextPromised ?? '') !== (prevPromised ?? '');
    const onlyClearingPromised = !!prevPromised && !nextPromised;

    const nextReady = fromDatetimeLocal(draft.ready_at_local);
    const nextDelivered = fromDatetimeLocal(draft.delivered_at_local);

    return (
      draft.status !== wo.status ||
      draft.vehicle_id !== (wo.vehicle_id ?? '') ||
      draft.vehicle_plate !== (wo.vehicle_plate ?? '') ||
      draft.customer_id !== (wo.customer_id ?? '') ||
      draft.customer_name !== (wo.customer_name ?? '') ||
      draft.booking_id !== (wo.booking_id ?? '') ||
      draft.requested_work !== (wo.requested_work ?? '') ||
      draft.diagnosis !== (wo.diagnosis ?? '') ||
      draft.notes !== (wo.notes ?? '') ||
      draft.internal_notes !== (wo.internal_notes ?? '') ||
      draft.currency !== (wo.currency ?? '') ||
      (promisedDiffers && !onlyClearingPromised) ||
      (nextReady ?? '') !== (wo.ready_at ?? '') ||
      (nextDelivered ?? '') !== (wo.delivered_at ?? '') ||
      itemsJsonDirty(draft.items_json, wo)
    );
  }, [wo, draft]);

  const isArchived = Boolean(wo?.archived_at);
  const canSave = isDirty && !saving && !loading && wo != null && draft != null;
  const closeDisabled = saving || archiveBusy || restoreBusy;

  const requestClose = useCallback(() => {
    if (closeDisabled) {
      return;
    }

    void (async () => {
      if (!isDirty) {
        onClose();
        return;
      }

      const confirmed = await confirmAction({
        title: 'Cancelar edición',
        description: '¿Realmente querés cancelar? Se perderán los cambios no guardados.',
        confirmLabel: 'Sí, cancelar',
        cancelLabel: 'Seguir editando',
      });
      if (!confirmed) {
        return;
      }

      onClose();
    })();
  }, [closeDisabled, isDirty, onClose]);

  useEffect(() => {
    if (variant !== 'modal') return;
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key !== 'Escape') {
        return;
      }
      ev.preventDefault();
      requestClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [requestClose, variant]);

  const handleArchive = async () => {
    if (!wo) return;
    if (isDirty) {
      const discardChanges = await confirmAction({
        title: 'Descartar cambios',
        description: 'Hay cambios sin guardar. ¿Archivar sin guardar?',
        confirmLabel: 'Sí, archivar',
        cancelLabel: 'Seguir editando',
        tone: 'danger',
      });
      if (!discardChanges) return;
    }
    const confirmed = await confirmAction({
      title: 'Archivar orden de trabajo',
      description: '¿Archivar esta orden de trabajo? Va a salir del listado activo.',
      confirmLabel: 'Archivar',
      cancelLabel: 'Cancelar',
      tone: 'danger',
    });
    if (!confirmed) return;
    setArchiveBusy(true);
    setError(null);
    try {
      await archiveAutoRepairWorkOrder(wo.id);
      onRecordRemoved?.(wo.id);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo archivar');
    } finally {
      setArchiveBusy(false);
    }
  };

  const handleRestore = async () => {
    if (!wo) return;
    const confirmed = await confirmAction({
      title: 'Restaurar orden de trabajo',
      description: '¿Restaurar esta orden al listado activo?',
      confirmLabel: 'Restaurar',
      cancelLabel: 'Cancelar',
    });
    if (!confirmed) return;
    setRestoreBusy(true);
    setError(null);
    try {
      await restoreAutoRepairWorkOrder(wo.id);
      const data = await getAutoRepairWorkOrder(wo.id);
      setWo(data);
      setDraft(woToDraft(data));
      onSaved(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo restaurar');
    } finally {
      setRestoreBusy(false);
    }
  };

  const handleSave = async () => {
    if (!wo || !draft) return;
    setSaving(true);
    setError(null);
    try {
      const body: Parameters<typeof updateAutoRepairWorkOrder>[1] = {};
      if (draft.status !== wo.status) body.status = draft.status;
      if (draft.vehicle_id.trim() !== (wo.vehicle_id ?? '').trim()) {
        body.vehicle_id = draft.vehicle_id.trim();
      }
      if (draft.vehicle_plate !== (wo.vehicle_plate ?? '')) body.vehicle_plate = draft.vehicle_plate;
      if (draft.customer_id.trim() !== (wo.customer_id ?? '').trim()) {
        const c = draft.customer_id.trim();
        body.customer_id = c.length > 0 ? c : undefined;
      }
      if (draft.customer_name !== (wo.customer_name ?? '')) body.customer_name = draft.customer_name;
      if (draft.booking_id.trim() !== (wo.booking_id ?? '').trim()) {
        const a = draft.booking_id.trim();
        body.booking_id = a.length > 0 ? a : undefined;
      }
      if (draft.requested_work !== (wo.requested_work ?? '')) body.requested_work = draft.requested_work;
      if (draft.diagnosis !== (wo.diagnosis ?? '')) body.diagnosis = draft.diagnosis;
      if (draft.notes !== (wo.notes ?? '')) body.notes = draft.notes;
      if (draft.internal_notes !== (wo.internal_notes ?? '')) body.internal_notes = draft.internal_notes;
      if (draft.currency !== (wo.currency ?? '')) body.currency = draft.currency;

      const nextPromised = fromDatetimeLocal(draft.promised_at_local);
      if ((nextPromised ?? '') !== (wo.promised_at ?? '')) {
        if (nextPromised) body.promised_at = nextPromised;
      }

      const nextReady = fromDatetimeLocal(draft.ready_at_local);
      if ((nextReady ?? '') !== (wo.ready_at ?? '')) {
        if (nextReady) body.ready_at = nextReady;
      }

      const nextDelivered = fromDatetimeLocal(draft.delivered_at_local);
      if ((nextDelivered ?? '') !== (wo.delivered_at ?? '')) {
        if (nextDelivered) body.delivered_at = nextDelivered;
      }

      const parsedItems = parseWorkOrderItemsJson(draft.items_json);
      if (JSON.stringify(parsedItems) !== JSON.stringify(wo.items ?? [])) {
        body.items = parsedItems;
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

  const formInner = (
    <>
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
                onChange={(ev) =>
                  setDraft((d) => (d ? { ...d, currency: ev.target.value.toUpperCase().slice(0, 8) } : d))
                }
                maxLength={8}
              />
            </div>
            <div className="wo-modal__field wo-modal__field--full">
              <label className="wo-modal__label" htmlFor="wo-vehicle-id">
                Vehículo (UUID)
              </label>
              <input
                id="wo-vehicle-id"
                className="wo-modal__input"
                value={draft.vehicle_id}
                onChange={(ev) => setDraft((d) => (d ? { ...d, vehicle_id: ev.target.value } : d))}
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
              <label className="wo-modal__label" htmlFor="wo-customer-id">
                Cliente / Party (UUID)
              </label>
              <input
                id="wo-customer-id"
                className="wo-modal__input"
                value={draft.customer_id}
                onChange={(ev) => setDraft((d) => (d ? { ...d, customer_id: ev.target.value } : d))}
              />
            </div>
            <div className="wo-modal__field wo-modal__field--full">
              <label className="wo-modal__label" htmlFor="wo-booking-id">
                Turno (Appointment UUID)
              </label>
              <input
                id="wo-booking-id"
                className="wo-modal__input"
                value={draft.booking_id}
                onChange={(ev) => setDraft((d) => (d ? { ...d, booking_id: ev.target.value } : d))}
              />
            </div>
            <div className="wo-modal__field">
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
            <div className="wo-modal__field">
              <label className="wo-modal__label" htmlFor="wo-ready">
                Listo en
              </label>
              <input
                id="wo-ready"
                type="datetime-local"
                className="wo-modal__input"
                value={draft.ready_at_local}
                onChange={(ev) => setDraft((d) => (d ? { ...d, ready_at_local: ev.target.value } : d))}
              />
            </div>
            <div className="wo-modal__field">
              <label className="wo-modal__label" htmlFor="wo-delivered">
                Entregado en
              </label>
              <input
                id="wo-delivered"
                type="datetime-local"
                className="wo-modal__input"
                value={draft.delivered_at_local}
                onChange={(ev) => setDraft((d) => (d ? { ...d, delivered_at_local: ev.target.value } : d))}
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
            <div className="wo-modal__field wo-modal__field--full">
              <label className="wo-modal__label" htmlFor="wo-items-json">
                Ítems (JSON)
              </label>
              <textarea
                id="wo-items-json"
                className="wo-modal__textarea wo-modal__textarea--items"
                value={draft.items_json}
                onChange={(ev) => setDraft((d) => (d ? { ...d, items_json: ev.target.value } : d))}
                spellCheck={false}
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
        </>
      )}
    </>
  );

  const footerActions =
    wo && draft && !loading ? (
      <div className="wo-modal__footer app-modal__footer wo-modal__footer--split">
        <div className="wo-modal__footer-start">
          {!isArchived ? (
            <button
              type="button"
              className="wo-modal__btn wo-modal__btn--danger"
              disabled={archiveBusy || saving}
              onClick={() => void handleArchive()}
            >
              {archiveBusy ? 'Archivando…' : 'Archivar'}
            </button>
          ) : (
            <button
              type="button"
              className="wo-modal__btn wo-modal__btn--restore"
              disabled={restoreBusy || saving}
              onClick={() => void handleRestore()}
            >
              {restoreBusy ? 'Restaurando…' : 'Restaurar'}
            </button>
          )}
        </div>
        <div className="wo-modal__footer-end">
          <button
            type="button"
            className="wo-modal__btn wo-modal__btn--ghost app-modal__action"
            onClick={requestClose}
            disabled={closeDisabled}
          >
            Cancelar
          </button>
          <button
            type="button"
            className="wo-modal__btn wo-modal__btn--primary app-modal__action"
            disabled={!canSave}
            onClick={() => void handleSave()}
          >
            {saving ? 'Guardando…' : 'Guardar'}
          </button>
        </div>
      </div>
    ) : null;

  const header = (
    <div className="wo-modal__header app-modal__header">
      <div className="wo-modal__title-block app-modal__title-block">
        <div className="wo-modal__eyebrow app-modal__eyebrow">Orden de trabajo</div>
        <h2 id="wo-modal-title" className="wo-modal__title app-modal__title">
          {loading ? 'Cargando…' : (wo?.number ?? '—')}
        </h2>
      </div>
      <div className="wo-modal__header-trailing">
        {variant === 'modal' ? (
          <button
            type="button"
            className="wo-modal__close app-modal__close"
            onClick={requestClose}
            aria-label="Cerrar"
            disabled={closeDisabled}
          >
            ×
          </button>
        ) : null}
      </div>
    </div>
  );

  const body = <div className="wo-modal__body app-modal__body">{formInner}</div>;

  if (variant === 'page') {
    return (
      <div className="wo-editor-page">
        <div className="wo-editor-page__toolbar">
          <button type="button" className="btn btn-secondary btn-sm" onClick={requestClose} disabled={closeDisabled}>
            ← Volver a la lista
          </button>
        </div>
        <div className="wo-modal wo-modal--embedded app-modal" role="dialog" aria-modal="false" aria-labelledby="wo-modal-title">
          {header}
          {body}
          {footerActions}
        </div>
      </div>
    );
  }

  const modal = (
    <div
      className="wo-modal-backdrop app-modal-backdrop"
      role="presentation"
      onMouseDown={(ev) => {
        if (ev.target === ev.currentTarget) requestClose();
      }}
    >
      <div className="wo-modal app-modal" role="dialog" aria-modal="true" aria-labelledby="wo-modal-title">
        {header}
        {body}
        {footerActions}
      </div>
    </div>
  );

  return createPortal(modal, document.body);
}
