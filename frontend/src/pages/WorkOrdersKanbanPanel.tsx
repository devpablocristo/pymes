import { useUser } from '@clerk/react';
import { StatusKanbanBoard, type KanbanColumnDef, type SuppressCardOpen } from '@devpablocristo/modules-crud';
import { useCallback, useEffect, useMemo, useState, type ReactElement, type RefObject } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import type { CrudHelpers } from '../components/CrudPage';
import { workOrdersCrudPageConfig } from '../crud/resourceConfigs';
import { CreatedByPillsBar } from '../components/CreatedByPillsBar';
import { WorkOrderKanbanDetailModal } from '../components/WorkOrderKanbanDetailModal';
import {
  getAllAutoRepairWorkOrders,
  getAutoRepairWorkOrdersArchived,
  patchAutoRepairWorkOrder,
} from '../lib/autoRepairApi';
import type { AutoRepairWorkOrder } from '../lib/autoRepairTypes';
import { clerkEnabled } from '../lib/auth';
import {
  applyWorkOrderCreatorFilter,
  type CreatorFilterState,
} from '../lib/workOrderCreatorFilter';
import {
  canonicalWorkOrderStatus,
  defaultCanonStatusForKanbanPhase,
  isWorkOrderKanbanTerminalStatus,
  workOrderKanbanPhaseFromStatus,
  workOrderStatusBadgeLabel,
  type WorkOrderKanbanPhase,
} from '../lib/workOrderKanban';
import { useI18n } from '../lib/i18n';
import './WorkOrdersKanbanPanel.css';

const COLUMN_ORDER: KanbanColumnDef[] = [
  { id: 'wo_intake', label: 'Ingreso' },
  { id: 'wo_quote', label: 'Presupuesto / repuestos' },
  { id: 'wo_shop', label: 'Taller' },
  { id: 'wo_exit', label: 'Salida' },
  { id: 'wo_closed', label: 'Cerradas' },
];

const COLUMN_IDS = new Set(COLUMN_ORDER.map((c) => c.id));

const LIST_SEARCH_PLACEHOLDER = 'Buscar órdenes por número, patente, cliente o trabajo…';

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

function sortInColumn(a: AutoRepairWorkOrder, b: AutoRepairWorkOrder): number {
  return (b.opened_at || '').localeCompare(a.opened_at || '');
}

function resolveDropColumnId(overId: string | undefined, items: AutoRepairWorkOrder[]): string | null {
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

function CardPreview({ row }: { row: AutoRepairWorkOrder }) {
  const overdue = isPromiseOverdue(row.promised_at);
  const soon = !overdue && isPromiseSoon(row.promised_at);
  const cls = ['m-kanban__card', 'm-kanban__card--overlay'];
  if (overdue) cls.push('m-kanban__card--overdue');
  else if (soon) cls.push('m-kanban__card--soon');
  return (
    <div className={cls.join(' ')}>
      <strong>{row.number}</strong>
      <div className="wo-kanban__badges" aria-hidden="true">
        <WorkOrderStatusBadge status={row.status} />
      </div>
      <div className="m-kanban__card-meta">
        {row.vehicle_plate || '—'} · {row.customer_name || 'Sin cliente'}
      </div>
    </div>
  );
}

function KanbanCardBody({
  row,
  onOpen,
  suppressOpenRef,
}: {
  row: AutoRepairWorkOrder;
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
      title="Clic para editar · arrastrar para mover de fase (detalle para el estado fino)"
      draggable={false}
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          handleClick();
        }
      }}
      role="button"
      tabIndex={0}
    >
      <strong>{row.number}</strong>
      <div className="wo-kanban__badges">
        <WorkOrderStatusBadge status={row.status} />
      </div>
      <div className="m-kanban__card-meta">
        {row.vehicle_plate || '—'} · {row.customer_name || 'Sin cliente'}
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

const listPath = '/modules/workOrders/list';

/**
 * Tablero Kanban de OT: fases macro vía `StatusKanbanBoard` (modules-crud).
 */
function workOrderKanbanToolbarBtnClass(kind?: 'primary' | 'secondary' | 'danger' | 'success'): string {
  switch (kind) {
    case 'primary':
      return 'btn-sm btn-primary';
    case 'danger':
      return 'btn-sm btn-danger';
    case 'success':
      return 'btn-sm btn-success';
    default:
      return 'btn-sm btn-secondary';
  }
}

export function WorkOrdersKanbanPanel() {
  const { user, isLoaded: clerkUserLoaded } = useUser();
  const selfId = user?.id;
  const { t, localizeText: formatFieldText } = useI18n();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const showArchived = searchParams.get('archived') === '1';

  const [items, setItems] = useState<AutoRepairWorkOrder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [detailOrderId, setDetailOrderId] = useState<string | null>(null);
  /** Por defecto (Clerk): `pick` con Set vacío = solo OT del usuario actual (`selfId`). */
  const [creatorFilter, setCreatorFilter] = useState<CreatorFilterState>(() =>
    clerkEnabled ? { mode: 'pick', actors: new Set() } : { mode: 'all' },
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const all = showArchived
        ? ((await getAutoRepairWorkOrdersArchived()).items ?? [])
        : await getAllAutoRepairWorkOrders();
      setItems(all);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudieron cargar las órdenes');
    } finally {
      setLoading(false);
    }
  }, [showArchived]);

  useEffect(() => {
    void load();
  }, [load]);

  const boardItems = useMemo(
    () =>
      applyWorkOrderCreatorFilter(items, {
        clerkEnabled,
        clerkUserLoaded,
        selfId,
        creatorFilter,
      }),
    [items, creatorFilter, clerkEnabled, clerkUserLoaded, selfId],
  );

  const handleMoveCard = useCallback(
    (id: string, targetPhase: string) => {
      const phase = targetPhase as WorkOrderKanbanPhase;
      const next = defaultCanonStatusForKanbanPhase(phase);
      if (next == null) return;
      setItems((prev) =>
        prev.map((x) => (x.id === id ? { ...x, status: next as AutoRepairWorkOrder['status'] } : x)),
      );
      void (async () => {
        try {
          const updated = await patchAutoRepairWorkOrder(id, { status: next });
          setItems((prev) => prev.map((x) => (x.id === id ? updated : x)));
          setError(null);
        } catch (e) {
          const msg =
            e instanceof Error ? e.message : 'No se pudo guardar el estado de la orden en el servidor';
          await load();
          setError(msg);
        }
      })();
    },
    [load],
  );

  const handleModalSaved = useCallback((wo: AutoRepairWorkOrder) => {
    setItems((prev) => prev.map((x) => (x.id === wo.id ? wo : x)));
  }, []);

  const handleOrderRemoved = useCallback((id: string) => {
    setItems((prev) => prev.filter((x) => x.id !== id));
    setDetailOrderId(null);
  }, []);

  const filterRow = useCallback((row: AutoRepairWorkOrder, q: string) => {
    const canon = canonicalWorkOrderStatus(row.status);
    const badge = workOrderStatusBadgeLabel(row.status).toLowerCase();
    const hay = [
      row.number,
      row.vehicle_plate,
      row.customer_name,
      row.requested_work,
      row.created_by,
      canon,
      badge,
    ]
      .join(' ')
      .toLowerCase();
    return hay.includes(q);
  }, []);

  const statsLine = useCallback(
    (visible: number, _nBoard: number) => {
      return visible === 1
        ? `${visible} orden de trabajo${showArchived ? ' archivada' : ''}`
        : `${visible} órdenes de trabajo${showArchived ? ' archivadas' : ''}`;
    },
    [items.length, showArchived],
  );

  const showCreatorBar = clerkEnabled && clerkUserLoaded && user != null;

  const afterStats = showCreatorBar ? (
    <CreatedByPillsBar
      items={items}
      creatorFilter={creatorFilter}
      onFilterChange={setCreatorFilter}
      selfId={selfId}
    />
  ) : null;

  const toolbarButtonRow = useMemo((): ReactElement => {
    const cfg = workOrdersCrudPageConfig;
    const helpers: CrudHelpers<AutoRepairWorkOrder> = {
      items,
      reload: load,
      setError: (message: string) => setError(message),
    };
    const toolbarActions = (cfg.toolbarActions ?? []).filter(
      (action) => action.isVisible?.({ archived: showArchived, items }) ?? true,
    );
    const canCreate =
      cfg.allowCreate ??
      (cfg.formFields.length > 0 && Boolean(cfg.dataSource?.create || cfg.basePath));

    return (
      <>
        {toolbarActions.map((action) => (
          <button
            key={action.id}
            type="button"
            className={workOrderKanbanToolbarBtnClass(action.kind)}
            onClick={() => {
              void action.onClick(helpers);
            }}
          >
            {formatFieldText(action.label)}
          </button>
        ))}
        {canCreate ? (
          <button type="button" className="btn-sm btn-primary" onClick={() => navigate(listPath)}>
            {cfg.createLabel ? formatFieldText(cfg.createLabel) : '+ Nueva orden'}
          </button>
        ) : null}
        {cfg.supportsArchived ? (
          <button
            type="button"
            className="btn-secondary btn-sm"
            onClick={() => {
              setSearchParams((prev) => {
                const p = new URLSearchParams(prev);
                if (p.get('archived') === '1') {
                  p.delete('archived');
                } else {
                  p.set('archived', '1');
                }
                return p;
              });
            }}
          >
            {showArchived ? t('crud.toggle.showActive') : t('crud.toggle.showArchived')}
          </button>
        ) : null}
      </>
    );
  }, [items, load, formatFieldText, t, showArchived, setSearchParams, navigate]);

  return (
    <>
      <StatusKanbanBoard<AutoRepairWorkOrder>
        columns={COLUMN_ORDER}
        columnIdSet={COLUMN_IDS}
        getRowColumnId={(row) => workOrderKanbanPhaseFromStatus(row.status)}
        fallbackColumnId="wo_intake"
        items={boardItems}
        loading={loading}
        error={error}
        onMoveCard={handleMoveCard}
        resolveDropColumnId={resolveDropColumnId}
        sortInColumn={sortInColumn}
        filterRow={filterRow}
        isRowDraggable={(row) =>
          !showArchived && !isWorkOrderKanbanTerminalStatus(row.status)
        }
        isColumnDroppable={(columnId) =>
          !showArchived && columnId !== 'wo_closed'
        }
        onCardOpen={(row) => setDetailOrderId(row.id)}
        renderCard={({ row, onOpen, suppressOpenRef }) => (
          <KanbanCardBody row={row} onOpen={onOpen} suppressOpenRef={suppressOpenRef} />
        )}
        renderOverlayCard={(row) => <CardPreview row={row} />}
        title={showArchived ? 'Tablero de órdenes (archivadas)' : 'Tablero de órdenes'}
        searchPlaceholder={LIST_SEARCH_PLACEHOLDER}
        searchInputClassName="crud-search"
        afterStats={afterStats}
        toolbarButtonRow={toolbarButtonRow}
        statsLine={statsLine}
        columnFooter={() => (
          <Link to={listPath} className="m-kanban__column-add" draggable={false}>
            <span className="m-kanban__column-add-icon" aria-hidden="true">
              +
            </span>
            Añadir orden
          </Link>
        )}
      />
      <WorkOrderKanbanDetailModal
        orderId={detailOrderId}
        onClose={() => setDetailOrderId(null)}
        onSaved={handleModalSaved}
        onRecordRemoved={handleOrderRemoved}
      />
    </>
  );
}
