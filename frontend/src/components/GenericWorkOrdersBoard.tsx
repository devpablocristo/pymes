/**
 * Tablero Kanban genérico para órdenes de trabajo.
 * Reutilizable por cualquier vertical (auto-repair, bike-shop, etc.).
 */
import { useUser } from '@clerk/react';
import { StatusKanbanBoard, type KanbanColumnDef, type SuppressCardOpen } from '@devpablocristo/modules-kanban-board';
import { normalize } from '@devpablocristo/core-browser/search';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState, type ReactElement, type ReactNode, type RefObject } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { CreatedByPillsBar } from './CreatedByPillsBar';
import { clerkEnabled } from '../lib/auth';
import { applyWorkOrderCreatorFilter, type CreatorFilterState } from '../lib/workOrderCreatorFilter';
import {
  canonicalWorkOrderStatus,
  defaultCanonStatusForKanbanPhase,
  isWorkOrderKanbanTerminalStatus,
  workOrderKanbanPhaseFromStatus,
  workOrderStatusBadgeLabel,
  type WorkOrderKanbanPhase,
} from '../lib/workOrderKanban';
import { useI18n } from '../lib/i18n';
import '../pages/WorkOrdersKanbanPanel.css';

/** Tipo mínimo que una OT debe cumplir para funcionar en el tablero. */
export type GenericWorkOrder = {
  id: string;
  number: string;
  status: string;
  asset_label: string;
  customer_name: string;
  requested_work: string;
  promised_at?: string;
  opened_at: string;
  created_by: string;
  archived_at?: string | null;
};

export type GenericWorkOrdersBoardProps<T extends GenericWorkOrder> = {
  /** Funciones API */
  listAll: () => Promise<T[]>;
  listArchived: () => Promise<T[]>;
  patchStatus: (id: string, status: string) => Promise<T>;
  /** Query keys para react-query */
  queryKey: readonly unknown[];
  /** Label del asset (ej. "Vehículo", "Bicicleta") */
  title: string;
  /** Slot superior con switch board/list */
  headerLeadSlot?: ReactNode;
  /** Path a la vista lista */
  listPath: string;
  /** Botones extra (nuevo, exportar, etc.) */
  extraToolbar?: ReactNode;
  /** Modal de detalle — recibe orderId y callbacks */
  renderDetailModal?: (props: {
    orderId: string | null;
    onClose: () => void;
    onSaved: (wo: T) => void;
    onRecordRemoved: (id: string) => void;
  }) => ReactNode;
};

const COLUMN_ORDER: KanbanColumnDef[] = [
  { id: 'wo_intake', label: 'Ingreso' },
  { id: 'wo_quote', label: 'Presupuesto / repuestos' },
  { id: 'wo_shop', label: 'Taller' },
  { id: 'wo_exit', label: 'Salida' },
  { id: 'wo_closed', label: 'Cerradas' },
];

const COLUMN_IDS = new Set(COLUMN_ORDER.map((c) => c.id));

function WorkOrderStatusBadge({ status }: { status: string }) {
  const canon = canonicalWorkOrderStatus(status);
  const label = workOrderStatusBadgeLabel(status);
  const mods = ['wo-kanban__badge'];
  if (canon === 'invoiced') mods.push('wo-kanban__badge--terminal-success');
  if (canon === 'cancelled') mods.push('wo-kanban__badge--terminal-danger');
  if (canon === 'on_hold') mods.push('wo-kanban__badge--hold');
  return <span className={mods.join(' ')}>{label}</span>;
}

function isPromiseOverdue(promisedAt?: string): boolean {
  if (!promisedAt) return false;
  const t = new Date(promisedAt).getTime();
  if (Number.isNaN(t)) return false;
  return t < Date.now();
}

function isPromiseSoon(promisedAt?: string): boolean {
  if (!promisedAt) return false;
  const t = new Date(promisedAt).getTime();
  if (Number.isNaN(t)) return false;
  const day = 24 * 60 * 60 * 1000;
  return t >= Date.now() && t <= Date.now() + day;
}

function resolveDropColumnId<T extends GenericWorkOrder>(overId: string | undefined, items: T[]): string | null {
  if (!overId) return null;
  if (overId.startsWith('col-')) {
    const s = overId.slice(4);
    return COLUMN_IDS.has(s) ? s : null;
  }
  const overCard = items.find((x) => x.id === overId);
  if (overCard) {
    const c = workOrderKanbanPhaseFromStatus(overCard.status);
    return COLUMN_IDS.has(c) ? c : 'wo_intake';
  }
  return null;
}

function CardPreview<T extends GenericWorkOrder>({ row }: { row: T }) {
  const overdue = isPromiseOverdue(row.promised_at);
  const soon = !overdue && isPromiseSoon(row.promised_at);
  const cls = ['m-kanban__card', 'm-kanban__card--overlay'];
  if (overdue) cls.push('m-kanban__card--overdue');
  else if (soon) cls.push('m-kanban__card--soon');
  return (
    <div className={cls.join(' ')} aria-hidden="true">
      <strong>{row.number}</strong>
      <div className="wo-kanban__badges" aria-hidden="true">
        <WorkOrderStatusBadge status={row.status} />
      </div>
      <div className="m-kanban__card-meta">
        {row.asset_label || '—'} · {row.customer_name || 'Sin cliente'}
      </div>
    </div>
  );
}

function KanbanCardBody<T extends GenericWorkOrder>({
  row,
  onOpen,
  suppressOpenRef,
}: {
  row: T;
  onOpen: () => void;
  suppressOpenRef: RefObject<SuppressCardOpen>;
}) {
  const overdue = isPromiseOverdue(row.promised_at);
  const soon = !overdue && isPromiseSoon(row.promised_at);
  const cls = ['m-kanban__card'];
  if (overdue) cls.push('m-kanban__card--overdue');
  else if (soon) cls.push('m-kanban__card--soon');

  const handleClick = () => {
    const s = suppressOpenRef.current;
    if (s != null && s.id === row.id && Date.now() < s.until) return;
    onOpen();
  };

  return (
    <div
      className={cls.join(' ')}
      title="Clic para editar · arrastrar para mover de fase"
      aria-label={`Orden ${row.number}. ${row.customer_name || 'Sin cliente'}. ${workOrderStatusBadgeLabel(row.status)}.`}
      draggable={false}
      onClick={handleClick}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleClick(); } }}
      role="button"
      tabIndex={0}
    >
      <strong>{row.number}</strong>
      <div className="wo-kanban__badges">
        <WorkOrderStatusBadge status={row.status} />
      </div>
      <div className="m-kanban__card-meta">
        {row.asset_label || '—'} · {row.customer_name || 'Sin cliente'}
      </div>
      {row.promised_at ? (
        <div className="m-kanban__card-meta">
          Promesa: {new Date(row.promised_at).toLocaleString()}
          {overdue ? ' · vencida' : null}
          {soon ? ' · próxima' : null}
        </div>
      ) : null}
    </div>
  );
}

export function GenericWorkOrdersBoard<T extends GenericWorkOrder>({
  listAll,
  listArchived,
  patchStatus,
  queryKey,
  title,
  headerLeadSlot,
  listPath,
  extraToolbar,
  renderDetailModal,
}: GenericWorkOrdersBoardProps<T>) {
  const { user, isLoaded: clerkUserLoaded } = useUser();
  const selfId = user?.id;
  const { t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();
  const showArchived = searchParams.get('archived') === '1';

  const [items, setItems] = useState<T[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [detailOrderId, setDetailOrderId] = useState<string | null>(null);
  const [creatorFilter, setCreatorFilter] = useState<CreatorFilterState>(() =>
    clerkEnabled ? { mode: 'pick', actors: new Set() } : { mode: 'all' },
  );

  const queryClient = useQueryClient();
  const woQuery = useQuery({
    queryKey: [...queryKey, showArchived ? 'archived' : 'active'],
    queryFn: () => (showArchived ? listArchived() : listAll()),
  });

  const patchMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) => patchStatus(id, status),
  });

  useEffect(() => {
    if (woQuery.data) { setItems(woQuery.data); setError(null); }
    if (woQuery.error) { setError(woQuery.error instanceof Error ? woQuery.error.message : 'Error al cargar órdenes'); }
  }, [woQuery.data, woQuery.error]);

  const reload = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey });
  }, [queryClient, queryKey]);

  const boardItems = useMemo(
    () => applyWorkOrderCreatorFilter(items, { authEnabled: clerkEnabled, authUserLoaded: clerkUserLoaded, selfId, creatorFilter }),
    [items, creatorFilter, clerkUserLoaded, selfId],
  );

  const handleMoveCard = useCallback(
    (id: string, targetPhase: string) => {
      const next = defaultCanonStatusForKanbanPhase(targetPhase as WorkOrderKanbanPhase);
      if (next == null) return;
      setItems((prev) => prev.map((x) => (x.id === id ? { ...x, status: next } as T : x)));
      void (async () => {
        try {
          const updated = await patchMutation.mutateAsync({ id, status: next });
          setItems((prev) => prev.map((x) => (x.id === id ? updated : x)));
          setError(null);
        } catch (e) {
          await reload();
          setError(e instanceof Error ? e.message : 'Error al guardar');
        }
      })();
    },
    [reload, patchMutation],
  );

  const handleModalSaved = useCallback((wo: T) => {
    setItems((prev) => prev.map((x) => (x.id === wo.id ? wo : x)));
  }, []);

  const handleOrderRemoved = useCallback((id: string) => {
    setItems((prev) => prev.filter((x) => x.id !== id));
    setDetailOrderId(null);
  }, []);

  const filterRow = useCallback((row: T, q: string) => {
    const hay = normalize(
      [row.number, row.asset_label, row.customer_name, row.requested_work, row.created_by,
       canonicalWorkOrderStatus(row.status), workOrderStatusBadgeLabel(row.status)].join(' '),
    );
    return hay.includes(normalize(q));
  }, []);

  const statsLine = useCallback(
    (visible: number) => visible === 1
      ? `${visible} orden de trabajo${showArchived ? ' archivada' : ''}`
      : `${visible} órdenes de trabajo${showArchived ? ' archivadas' : ''}`,
    [showArchived],
  );

  const showCreatorBar = clerkEnabled && clerkUserLoaded && user != null;

  return (
    <>
      <StatusKanbanBoard<T>
        columns={COLUMN_ORDER}
        columnIdSet={COLUMN_IDS}
        getRowColumnId={(row) => workOrderKanbanPhaseFromStatus(row.status)}
        fallbackColumnId="wo_intake"
        items={boardItems}
        loading={woQuery.isLoading}
        error={error}
        onMoveCard={handleMoveCard}
        resolveDropColumnId={(overId) => resolveDropColumnId(overId, items)}
        sortInColumn={(a, b) => (b.opened_at || '').localeCompare(a.opened_at || '')}
        filterRow={filterRow}
        isRowDraggable={(row) => !showArchived && !isWorkOrderKanbanTerminalStatus(row.status)}
        isColumnDroppable={(columnId) => !showArchived && columnId !== 'wo_closed'}
        onCardOpen={(row) => setDetailOrderId(row.id)}
        renderCard={({ row, onOpen, suppressOpenRef }) => (
          <KanbanCardBody row={row} onOpen={onOpen} suppressOpenRef={suppressOpenRef} />
        )}
        renderOverlayCard={(row) => <CardPreview row={row} />}
        title={title}
        subtitle={showArchived ? 'Archivadas' : undefined}
        headerLeadSlot={headerLeadSlot}
        searchPlaceholder="Buscar..."
        afterStats={showCreatorBar ? (
          <CreatedByPillsBar items={items} creatorFilter={creatorFilter} onFilterChange={setCreatorFilter} selfId={selfId} />
        ) : null}
        toolbarButtonRow={
          <>
            {extraToolbar}
            <button
              type="button"
              className="btn-secondary btn-sm"
              onClick={() => {
                setSearchParams((prev) => {
                  const p = new URLSearchParams(prev);
                  if (p.get('archived') === '1') p.delete('archived');
                  else p.set('archived', '1');
                  return p;
                });
              }}
            >
              {showArchived ? 'Ver activas' : 'Ver archivadas'}
            </button>
          </>
        }
        statsLine={statsLine}
        columnFooter={() => (
          <Link to={listPath} className="m-kanban__column-add" draggable={false}>
            <span className="m-kanban__column-add-icon" aria-hidden="true">+</span>
            Añadir orden
          </Link>
        )}
      />
      {renderDetailModal?.({
        orderId: detailOrderId,
        onClose: () => setDetailOrderId(null),
        onSaved: handleModalSaved,
        onRecordRemoved: handleOrderRemoved,
      })}
    </>
  );
}
