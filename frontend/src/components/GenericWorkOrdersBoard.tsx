/**
 * Tablero Kanban genérico para órdenes de trabajo.
 * Reutilizable por cualquier vertical (auto-repair, bike-shop, etc.).
 */
import { useUser } from '@clerk/react';
import { type KanbanColumnDef, type SuppressCardOpen } from '@devpablocristo/modules-kanban-board';
import { normalize } from '@devpablocristo/core-browser/search';
import { useMutation } from '@tanstack/react-query';
import { useCallback, useMemo, useState, type ReactNode, type RefObject } from 'react';
import { Link } from 'react-router-dom';
import {
  createCrudKanbanArchiveTerminalDragPolicy,
  CrudArchivedSearchParamToggle,
  CrudKanbanSurface,
  useCrudKanbanMove,
  useCrudRemoteArchivedListState,
} from '../modules/crud';
import { CreatedByPillsBar } from './CreatedByPillsBar';
import { clerkEnabled } from '../lib/auth';
import { applyWorkOrderCreatorFilter, type CreatorFilterState } from '../lib/workOrderCreatorFilter';
import {
  canonicalWorkOrderStatus,
  workOrderStatusBadgeLabel,
  workOrderKanbanTransitionModel,
  type WorkOrderKanbanPhase,
} from '../lib/workOrderKanban';
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
  renderExtraToolbar?: (props: {
    items: T[];
    reload: () => Promise<void>;
    setError: (message: string | null) => void;
    showArchived: boolean;
  }) => ReactNode;
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
    const c = workOrderKanbanTransitionModel.getColumnIdForStatus(overCard.status);
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
  renderExtraToolbar,
  renderDetailModal,
}: GenericWorkOrdersBoardProps<T>) {
  const { user, isLoaded: clerkUserLoaded } = useUser();
  const selfId = user?.id;
  const [error, setError] = useState<string | null>(null);
  const [detailOrderId, setDetailOrderId] = useState<string | null>(null);
  const [creatorFilter, setCreatorFilter] = useState<CreatorFilterState>(() => ({ mode: 'all' }));

  const {
    showArchived,
    items,
    setItems,
    error: queryError,
    setError: setQueryError,
    loading,
    reload,
    upsertInListCache,
    removeFromListCache,
  } = useCrudRemoteArchivedListState<T>({
    queryKey,
    listActive: listAll,
    listArchived,
    loadErrorMessage: 'Error al cargar órdenes',
  });

  const patchMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) => patchStatus(id, status),
  });

  const setBoardError = useCallback((message: string | null) => {
    setError(message);
    setQueryError(message);
  }, [setQueryError]);

  const boardItems = useMemo(
    () => applyWorkOrderCreatorFilter(items, { authEnabled: clerkEnabled, authUserLoaded: clerkUserLoaded, selfId, creatorFilter }),
    [items, creatorFilter, clerkUserLoaded, selfId],
  );

  const archiveTerminalDragPolicy = useMemo(
    () =>
      createCrudKanbanArchiveTerminalDragPolicy<T, WorkOrderKanbanPhase>({
        showArchived,
        transitionModel: workOrderKanbanTransitionModel,
        getItemStatus: (row: T) => row.status,
      }),
    [showArchived],
  );

  const handleMoveCard = useCrudKanbanMove<T, WorkOrderKanbanPhase>({
    items,
    setItems,
    transitionModel: workOrderKanbanTransitionModel,
    getItemColumnId: (row) => workOrderKanbanTransitionModel.getColumnIdForStatus(row.status),
    getItemStatus: (row) => row.status,
    setItemStatus: (row, status) => ({ ...row, status } as T),
    persistStatusChange: async (id, nextStatus) => patchMutation.mutateAsync({ id, status: nextStatus }),
    mergePersistedItem: (persisted, nextStatus) => ({ ...persisted, status: nextStatus } as T),
    reload,
    setError,
  });

  const handleBoardMoveCard = useCallback(
    (id: string, targetColumnId: string, overItemId?: string) => {
      handleMoveCard(id, targetColumnId as WorkOrderKanbanPhase, overItemId);
    },
    [handleMoveCard],
  );

  const handleModalSaved = useCallback(
    (wo: T) => {
      upsertInListCache(wo);
    },
    [upsertInListCache],
  );

  const handleOrderRemoved = useCallback(
    (id: string) => {
      removeFromListCache(id);
      setDetailOrderId(null);
    },
    [removeFromListCache],
  );

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
      <CrudKanbanSurface<T>
        leadSlot={headerLeadSlot}
        columns={COLUMN_ORDER}
        columnIdSet={COLUMN_IDS}
        getRowColumnId={(row) => workOrderKanbanTransitionModel.getColumnIdForStatus(row.status)}
        fallbackColumnId="wo_intake"
        items={boardItems}
        loading={loading}
        error={error ?? queryError}
        onMoveCard={handleBoardMoveCard}
        resolveDropColumnId={(overId) => resolveDropColumnId(overId, items)}
        filterRow={filterRow}
        isRowDraggable={archiveTerminalDragPolicy.isRowDraggable}
        isColumnDroppable={archiveTerminalDragPolicy.isColumnDroppable}
        onCardOpen={(row) => setDetailOrderId(row.id)}
        renderCard={({ row, onOpen, suppressOpenRef }) => (
          <KanbanCardBody row={row} onOpen={onOpen} suppressOpenRef={suppressOpenRef} />
        )}
        renderOverlayCard={(row) => <CardPreview row={row} />}
        title={title}
        subtitle={showArchived ? 'Archivadas' : undefined}
        searchPlaceholder="Buscar..."
        afterStats={showCreatorBar ? (
          <CreatedByPillsBar items={items} creatorFilter={creatorFilter} onFilterChange={setCreatorFilter} selfId={selfId} />
        ) : null}
        toolbarButtonRow={
          <>
            {renderExtraToolbar?.({
              items,
              reload,
              setError: setBoardError,
              showArchived,
            })}
            <CrudArchivedSearchParamToggle
              className="btn-secondary btn-sm"
              showActiveLabel="Ver activas"
              showArchivedLabel="Ver archivadas"
              onToggle={() => setDetailOrderId(null)}
            />
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
