import { useCallback, useEffect, useMemo, useState } from 'react';
import { confirmAction } from '@devpablocristo/core-browser';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import {
  archiveWorkOrder,
  getWorkOrder,
  restoreWorkOrder,
  updateWorkOrder,
  type WorkOrder as UnifiedWorkOrder,
} from '../lib/workOrdersApi';
import {
  CrudEntityEditorModal,
  type CrudEntityEditorModalField,
  type CrudEntityEditorModalStat,
} from '../modules/crud';
import { parseWorkOrderItemsJson, stringifyWorkOrderItems } from '../lib/workOrderItemsJson';
import './WorkOrderEditor.css';

type AutoRepairWorkOrder = UnifiedWorkOrder;

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
  items: string;
};

export type WorkOrderEditorProps = {
  orderId: string;
  variant: 'modal' | 'page';
  onClose: () => void;
  onSaved: (wo: AutoRepairWorkOrder) => void;
  onRecordRemoved?: (id: string) => void;
};

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

function textValue(value: CrudFieldValue | undefined): string {
  return typeof value === 'string' ? value : String(value ?? '');
}

function valuesToDraft(values: Record<string, CrudFieldValue>): Draft {
  return {
    status: textValue(values.status),
    vehicle_id: textValue(values.vehicle_id),
    vehicle_plate: textValue(values.vehicle_plate),
    customer_id: textValue(values.customer_id),
    customer_name: textValue(values.customer_name),
    booking_id: textValue(values.booking_id),
    requested_work: textValue(values.requested_work),
    diagnosis: textValue(values.diagnosis),
    notes: textValue(values.notes),
    internal_notes: textValue(values.internal_notes),
    currency: textValue(values.currency),
    promised_at_local: textValue(values.promised_at_local),
    ready_at_local: textValue(values.ready_at_local),
    delivered_at_local: textValue(values.delivered_at_local),
    items: textValue(values.items),
  };
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
    items: stringifyWorkOrderItems(wo.items),
  };
}

function buildFields(): CrudEntityEditorModalField[] {
  return [
    {
      id: 'status',
      label: 'Estado',
      type: 'select',
      options: STATUS_OPTIONS,
    },
    {
      id: 'currency',
      label: 'Moneda',
    },
    {
      id: 'vehicle_id',
      label: 'Vehículo (UUID)',
      fullWidth: true,
    },
    {
      id: 'vehicle_plate',
      label: 'Patente',
    },
    {
      id: 'customer_name',
      label: 'Cliente',
    },
    {
      id: 'customer_id',
      label: 'Cliente / Party (UUID)',
      fullWidth: true,
    },
    {
      id: 'booking_id',
      label: 'Turno (Appointment UUID)',
      fullWidth: true,
    },
    {
      id: 'promised_at_local',
      label: 'Prometida para',
      type: 'datetime-local',
    },
    {
      id: 'ready_at_local',
      label: 'Listo en',
      type: 'datetime-local',
    },
    {
      id: 'delivered_at_local',
      label: 'Entregado en',
      type: 'datetime-local',
    },
    {
      id: 'requested_work',
      label: 'Trabajo solicitado',
      type: 'textarea',
      fullWidth: true,
      rows: 3,
    },
    {
      id: 'diagnosis',
      label: 'Diagnóstico',
      type: 'textarea',
      fullWidth: true,
      rows: 3,
    },
    {
      id: 'notes',
      label: 'Notas',
      type: 'textarea',
      fullWidth: true,
      rows: 2,
    },
    {
      id: 'internal_notes',
      label: 'Notas internas',
      type: 'textarea',
      fullWidth: true,
      rows: 3,
    },
    {
      id: 'items',
      label: 'Ítems',
      fullWidth: true,
      editControl: ({ value, setValue }) => (
        <textarea
          className="wo-editor__items-textarea"
          value={textValue(value)}
          onChange={(event) => setValue(event.target.value)}
          spellCheck={false}
          rows={8}
        />
      ),
      readValue: '—',
    },
  ];
}

function buildStats(wo: AutoRepairWorkOrder | null): CrudEntityEditorModalStat[] {
  if (!wo) return [];
  return [
    {
      id: 'services',
      label: 'Servicios',
      value: `${wo.subtotal_services.toLocaleString()} ${wo.currency ?? 'ARS'}`,
    },
    {
      id: 'parts',
      label: 'Repuestos',
      value: `${wo.subtotal_parts.toLocaleString()} ${wo.currency ?? 'ARS'}`,
    },
    {
      id: 'tax',
      label: 'IVA',
      value: `${wo.tax_total.toLocaleString()} ${wo.currency ?? 'ARS'}`,
    },
    {
      id: 'total',
      label: 'Total',
      value: `${wo.total.toLocaleString()} ${wo.currency ?? 'ARS'}`,
      tone: 'info',
    },
  ];
}

export function WorkOrderEditor({ orderId, variant, onClose, onSaved, onRecordRemoved }: WorkOrderEditorProps) {
  const fields = useMemo(() => buildFields(), []);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [wo, setWo] = useState<AutoRepairWorkOrder | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [archiveBusy, setArchiveBusy] = useState(false);
  const [restoreBusy, setRestoreBusy] = useState(false);

  const load = useCallback(async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      const data = await getWorkOrder(id);
      setWo(data);
    } catch (e) {
      setWo(null);
      setError(e instanceof Error ? e.message : 'No se pudo cargar la orden');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(orderId);
  }, [orderId, load]);

  const isArchived = Boolean(wo?.archived_at);
  const closeDisabled = saving || archiveBusy || restoreBusy;
  const stats = useMemo(() => buildStats(wo), [wo]);

  const handleArchive = useCallback(
    async (dirty: boolean) => {
      if (!wo) return;
      if (dirty) {
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
        await archiveWorkOrder(wo.id);
        onRecordRemoved?.(wo.id);
        onClose();
      } catch (e) {
        setError(e instanceof Error ? e.message : 'No se pudo archivar');
      } finally {
        setArchiveBusy(false);
      }
    },
    [onClose, onRecordRemoved, wo],
  );

  const handleRestore = useCallback(async () => {
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
      await restoreWorkOrder(wo.id);
      const restored = await getWorkOrder(wo.id);
      setWo(restored);
      onSaved(restored);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo restaurar');
    } finally {
      setRestoreBusy(false);
    }
  }, [onSaved, wo]);

  const handleSave = useCallback(
    async (values: Record<string, CrudFieldValue>) => {
      if (!wo) return;
      const draft = valuesToDraft(values);
      setSaving(true);
      setError(null);
      try {
        const body: Parameters<typeof updateWorkOrder>[1] = {};
        if (draft.status !== wo.status) body.status = draft.status;
        if (draft.vehicle_id.trim() !== (wo.vehicle_id ?? '').trim()) {
          body.vehicle_id = draft.vehicle_id.trim();
        }
        if (draft.vehicle_plate !== (wo.vehicle_plate ?? '')) body.vehicle_plate = draft.vehicle_plate;
        if (draft.customer_id.trim() !== (wo.customer_id ?? '').trim()) {
          const customerId = draft.customer_id.trim();
          body.customer_id = customerId.length > 0 ? customerId : undefined;
        }
        if (draft.customer_name !== (wo.customer_name ?? '')) body.customer_name = draft.customer_name;
        if (draft.booking_id.trim() !== (wo.booking_id ?? '').trim()) {
          const bookingId = draft.booking_id.trim();
          body.booking_id = bookingId.length > 0 ? bookingId : undefined;
        }
        if (draft.requested_work !== (wo.requested_work ?? '')) body.requested_work = draft.requested_work;
        if (draft.diagnosis !== (wo.diagnosis ?? '')) body.diagnosis = draft.diagnosis;
        if (draft.notes !== (wo.notes ?? '')) body.notes = draft.notes;
        if (draft.internal_notes !== (wo.internal_notes ?? '')) body.internal_notes = draft.internal_notes;
        if (draft.currency !== (wo.currency ?? '')) body.currency = draft.currency;

        const nextPromised = fromDatetimeLocal(draft.promised_at_local);
        if ((nextPromised ?? '') !== (wo.promised_at ?? '') && nextPromised) {
          body.promised_at = nextPromised;
        }

        const nextReady = fromDatetimeLocal(draft.ready_at_local);
        if ((nextReady ?? '') !== (wo.ready_at ?? '') && nextReady) {
          body.ready_at = nextReady;
        }

        const nextDelivered = fromDatetimeLocal(draft.delivered_at_local);
        if ((nextDelivered ?? '') !== (wo.delivered_at ?? '') && nextDelivered) {
          body.delivered_at = nextDelivered;
        }

        const parsedItems = parseWorkOrderItemsJson(draft.items);
        if (JSON.stringify(parsedItems) !== JSON.stringify(wo.items ?? [])) {
          body.items = parsedItems;
        }

        if (Object.keys(body).length === 0) {
          onClose();
          return;
        }

        const updated = await updateWorkOrder(orderId, body);
        setWo(updated);
        onSaved(updated);
        onClose();
      } catch (e) {
        setError(e instanceof Error ? e.message : 'No se pudo guardar');
      } finally {
        setSaving(false);
      }
    },
    [onClose, onSaved, orderId, wo],
  );

  return (
    <CrudEntityEditorModal
      open
      variant={variant}
      editBehavior="edit-only"
      mode="update"
      title={loading ? 'Cargando…' : wo?.number ?? '—'}
      eyebrow="Orden de trabajo"
      fields={fields}
      stats={stats}
      initialValues={wo ? woToDraft(wo) : undefined}
      loading={loading}
      loadingLabel="Cargando datos…"
      error={error}
      disableSubmit={!wo || saving || archiveBusy || restoreBusy}
      disableSubmitWhenPristine
      submitLabel={saving ? 'Guardando…' : 'Guardar'}
      cancelLabel="Cancelar"
      onCancel={onClose}
      onSubmit={(values) => void handleSave(values)}
      confirmDiscard={{
        title: 'Cancelar edición',
        description: '¿Realmente querés cancelar? Se perderán los cambios no guardados.',
        confirmLabel: 'Sí, cancelar',
        cancelLabel: 'Seguir editando',
      }}
      headerActions={
        variant === 'modal'
          ? ({ requestCancel }) => (
              <button
                type="button"
                className="wo-editor__close"
                onClick={requestCancel}
                aria-label="Cerrar"
                disabled={closeDisabled}
              >
                ×
              </button>
            )
          : null
      }
      editingStartActions={({ dirty }) =>
        !wo ? null : !isArchived ? (
          <button
            type="button"
            className="btn btn-danger"
            disabled={archiveBusy || saving || restoreBusy}
            onClick={() => void handleArchive(dirty)}
          >
            {archiveBusy ? 'Archivando…' : 'Archivar'}
          </button>
        ) : (
          <button
            type="button"
            className="btn btn-secondary"
            disabled={restoreBusy || saving || archiveBusy}
            onClick={() => void handleRestore()}
          >
            {restoreBusy ? 'Restaurando…' : 'Restaurar'}
          </button>
        )
      }
      rootClassName={variant === 'page' ? 'wo-editor-page' : 'wo-editor-root'}
      panelClassName={variant === 'page' ? 'wo-editor-panel wo-editor-panel--page' : 'wo-editor-panel'}
      pageToolbarClassName="wo-editor-page__toolbar"
      pageToolbar={
        variant === 'page' ? (
          <button type="button" className="btn btn-secondary btn-sm" onClick={onClose} disabled={closeDisabled}>
            ← Volver a la lista
          </button>
        ) : undefined
      }
    />
  );
}
