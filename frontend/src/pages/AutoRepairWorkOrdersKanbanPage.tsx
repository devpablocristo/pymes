import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  pointerWithin,
  rectIntersection,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type CollisionDetection,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode, type RefObject } from 'react';
import { Link } from 'react-router-dom';
import { WorkOrderKanbanDetailModal } from '../components/WorkOrderKanbanDetailModal';
import { getAllAutoRepairWorkOrders, patchAutoRepairWorkOrder } from '../lib/autoRepairApi';
import type { AutoRepairWorkOrder } from '../lib/autoRepairTypes';
import './AutoRepairWorkOrdersKanbanPage.css';

type SuppressCardOpen = { id: string | null; until: number };

const COLUMN_ORDER = [
  { status: 'received', label: 'Recibido' },
  { status: 'diagnosing', label: 'Diagnóstico' },
  { status: 'quote_pending', label: 'Presupuesto' },
  { status: 'awaiting_parts', label: 'Repuestos' },
  { status: 'in_progress', label: 'En taller' },
  { status: 'quality_check', label: 'Control' },
  { status: 'ready_for_pickup', label: 'Listo retiro' },
  { status: 'delivered', label: 'Entregado' },
  { status: 'invoiced', label: 'Facturado' },
  { status: 'on_hold', label: 'En pausa' },
  { status: 'cancelled', label: 'Cancelado' },
] as const;

const COLUMN_IDS = new Set(COLUMN_ORDER.map((c) => c.status));

/** Prioriza columna bajo el puntero; si no, intersección (huecos vacíos); último recurso esquinas (estilo Trello / dnd-kit). */
const kanbanCollisionDetection: CollisionDetection = (args) => {
  const pointer = pointerWithin(args);
  if (pointer.length > 0) {
    return pointer;
  }
  const rect = rectIntersection(args);
  if (rect.length > 0) {
    return rect;
  }
  return closestCorners(args);
};

export function canonicalWorkOrderStatus(raw: string): string {
  const s = (raw || '').toLowerCase().trim();
  if (s === 'diagnosis') return 'diagnosing';
  if (s === 'ready') return 'ready_for_pickup';
  return s;
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

function resolveDropStatus(
  overId: string | undefined,
  items: AutoRepairWorkOrder[],
): string | null {
  if (!overId) return null;
  if (overId.startsWith('col-')) {
    const s = overId.slice(4);
    return COLUMN_IDS.has(s as (typeof COLUMN_ORDER)[number]['status']) ? s : null;
  }
  const overCard = items.find((x) => x.id === overId);
  if (overCard) {
    const c = canonicalWorkOrderStatus(overCard.status);
    return COLUMN_IDS.has(c as (typeof COLUMN_ORDER)[number]['status']) ? c : 'received';
  }
  return null;
}

function CardPreview({ row }: { row: AutoRepairWorkOrder }) {
  const overdue = isPromiseOverdue(row.promised_at);
  const soon = !overdue && isPromiseSoon(row.promised_at);
  const cls = ['wo-kanban__card', 'wo-kanban__card--overlay'];
  if (overdue) cls.push('wo-kanban__card--overdue');
  else if (soon) cls.push('wo-kanban__card--soon');
  return (
    <div className={cls.join(' ')}>
      <strong>{row.number}</strong>
      <div className="wo-kanban__card-meta">
        {row.vehicle_plate || '—'} · {row.customer_name || 'Sin cliente'}
      </div>
    </div>
  );
}

function KanbanCard({
  row,
  onOpenClick,
  suppressOpenRef,
}: {
  row: AutoRepairWorkOrder;
  onOpenClick: (row: AutoRepairWorkOrder) => void;
  suppressOpenRef: RefObject<SuppressCardOpen>;
}) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: row.id,
  });
  const style = {
    transform: CSS.Translate.toString(transform),
    opacity: isDragging ? 0.35 : 1,
  };
  const overdue = isPromiseOverdue(row.promised_at);
  const soon = !overdue && isPromiseSoon(row.promised_at);
  const cls = ['wo-kanban__card'];
  if (overdue) cls.push('wo-kanban__card--overdue');
  else if (soon) cls.push('wo-kanban__card--soon');

  const handleClick = () => {
    const s = suppressOpenRef.current;
    if (s != null && s.id === row.id && Date.now() < s.until) return;
    onOpenClick(row);
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cls.join(' ')}
      title="Clic para editar · arrastrar para mover"
      {...listeners}
      {...attributes}
      draggable={false}
      onClick={handleClick}
    >
      <strong>{row.number}</strong>
      <div className="wo-kanban__card-meta">
        {row.vehicle_plate || '—'} · {row.customer_name || 'Sin cliente'}
      </div>
      {row.promised_at ? (
        <div className="wo-kanban__card-meta">
          Promesa: {new Date(row.promised_at).toLocaleString()}
          {overdue ? ' · vencida' : null}
          {soon ? ' · próxima' : null}
        </div>
      ) : null}
    </div>
  );
}

function KanbanColumnBody({
  status,
  label,
  count,
  boardDragging,
  children,
}: {
  status: string;
  label: string;
  count: number;
  boardDragging: boolean;
  children: ReactNode;
}) {
  const id = `col-${status}`;
  const { setNodeRef, isOver } = useDroppable({ id });
  return (
    <div
      ref={setNodeRef}
      className={`wo-kanban__column-body ${isOver ? 'wo-kanban__column-body--over' : ''} ${boardDragging ? 'wo-kanban__column-body--dragging' : ''}`}
      data-column={status}
    >
      <div className="wo-kanban__column-head">
        <span className="wo-kanban__column-label">{label}</span>
        <div className="wo-kanban__column-head-actions">
          <span className="wo-kanban__column-count">{count}</span>
          <span className="wo-kanban__column-menu" aria-hidden="true">
            ···
          </span>
        </div>
      </div>
      <div className="wo-kanban__column-scroll">{children}</div>
      {boardDragging && count === 0 ? <div className="wo-kanban__drop-hint">Soltar aquí</div> : null}
      <Link to="/workshops/auto-repair/orders" className="wo-kanban__column-add" draggable={false}>
        <span className="wo-kanban__column-add-icon" aria-hidden="true">
          +
        </span>
        Añadir orden
      </Link>
    </div>
  );
}

export function AutoRepairWorkOrdersKanbanPage() {
  const [items, setItems] = useState<AutoRepairWorkOrder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [activeDrag, setActiveDrag] = useState<AutoRepairWorkOrder | null>(null);
  const [detailOrderId, setDetailOrderId] = useState<string | null>(null);
  const itemsRef = useRef<AutoRepairWorkOrder[]>([]);
  itemsRef.current = items;
  const suppressCardOpenRef = useRef<SuppressCardOpen>({ id: null, until: 0 });
  const activeDragIdRef = useRef<string | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 8 },
    }),
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const all = await getAllAutoRepairWorkOrders();
      setItems(all);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudieron cargar las órdenes');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((row) => {
      const hay = [row.number, row.vehicle_plate, row.customer_name, row.requested_work].join(' ').toLowerCase();
      return hay.includes(q);
    });
  }, [items, search]);

  const byColumn = useMemo(() => {
    const map = new Map<string, AutoRepairWorkOrder[]>();
    for (const col of COLUMN_ORDER) {
      map.set(col.status, []);
    }
    for (const row of filtered) {
      const c = canonicalWorkOrderStatus(row.status);
      const bucket = map.get(c);
      if (bucket) {
        bucket.push(row);
      } else {
        const recv = map.get('received');
        if (recv) recv.push(row);
      }
    }
    for (const [, list] of map) {
      list.sort(sortInColumn);
    }
    return map;
  }, [filtered]);

  const boardDragging = activeDrag != null;

  const handleDragStart = (e: DragStartEvent) => {
    const id = String(e.active.id);
    activeDragIdRef.current = id;
    setActiveDrag(itemsRef.current.find((x) => x.id === id) ?? null);
  };

  /** Sincrónico: dnd-kit debe terminar el ciclo sin await; el PATCH va aparte. */
  const handleDragEnd = (e: DragEndEvent) => {
    suppressCardOpenRef.current = { id: String(e.active.id), until: Date.now() + 260 };
    activeDragIdRef.current = null;
    const { active, over } = e;
    setActiveDrag(null);
    const id = String(active.id);
    const snapshot = itemsRef.current;
    const targetStatus = resolveDropStatus(over?.id != null ? String(over.id) : undefined, snapshot);
    if (!targetStatus) return;
    const row = snapshot.find((x) => x.id === id);
    if (!row || canonicalWorkOrderStatus(row.status) === targetStatus) return;

    setItems((prev) =>
      prev.map((x) => (x.id === id ? { ...x, status: targetStatus as AutoRepairWorkOrder['status'] } : x)),
    );

    setError(null);
    void patchAutoRepairWorkOrder(id, { status: targetStatus }).catch(async (err: unknown) => {
      setError(err instanceof Error ? err.message : 'No se pudo actualizar el estado');
      await load();
    });
  };

  const handleDragCancel = () => {
    const id = activeDragIdRef.current;
    if (id) suppressCardOpenRef.current = { id, until: Date.now() + 260 };
    activeDragIdRef.current = null;
    setActiveDrag(null);
  };

  const handleCardOpen = (row: AutoRepairWorkOrder) => {
    setDetailOrderId(row.id);
  };

  const handleModalSaved = (wo: AutoRepairWorkOrder) => {
    setItems((prev) => prev.map((x) => (x.id === wo.id ? wo : x)));
  };

  const totalVisible = filtered.length;

  return (
    <div className="wo-kanban">
      <div className="wo-kanban__toolbar">
        <div>
          <h1 className="wo-kanban__title">Tablero de órdenes</h1>
          <p className="wo-kanban__muted wo-kanban__subtitle">
            Clic en una tarjeta para editar. Arrastrá entre listas para cambiar estado (se guarda al soltar).
          </p>
        </div>
        <div className="wo-kanban__toolbar-actions">
          <input
            type="search"
            placeholder="Filtrar por OT, patente, cliente…"
            value={search}
            onChange={(ev) => setSearch(ev.target.value)}
            aria-label="Filtrar órdenes"
          />
          <button type="button" className="btn btn-secondary" onClick={() => void load()} disabled={loading}>
            Actualizar
          </button>
          <Link to="/workshops/auto-repair/orders" className="btn btn-secondary">
            Lista detalle
          </Link>
        </div>
      </div>

      <p className="wo-kanban__stats" aria-live="polite">
        {loading ? 'Cargando…' : `${totalVisible} orden${totalVisible === 1 ? '' : 'es'} en tablero`}
        {!loading && items.length === 0 ? ' · Si tu org es nueva, creá OT desde la lista o revisá la URL del API de talleres.' : null}
      </p>

      {error ? (
        <p className="wo-kanban__error" role="alert">
          {error}
        </p>
      ) : null}

      {!loading && items.length === 0 ? (
        <div className="wo-kanban__empty">
          <p>No hay órdenes de trabajo en esta organización.</p>
          <Link to="/workshops/auto-repair/orders" className="btn btn-primary">
            Ir a órdenes (alta / lista)
          </Link>
        </div>
      ) : null}

      <DndContext
        sensors={sensors}
        collisionDetection={kanbanCollisionDetection}
        autoScroll={false}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
        onDragCancel={handleDragCancel}
      >
        <div className="wo-kanban__board">
          {COLUMN_ORDER.map((col) => {
            const columnItems = byColumn.get(col.status) ?? [];
            return (
              <div key={col.status} className="wo-kanban__column-shell">
                <KanbanColumnBody
                  status={col.status}
                  label={col.label}
                  count={columnItems.length}
                  boardDragging={boardDragging}
                >
                  {columnItems.map((row) => (
                    <KanbanCard
                      key={row.id}
                      row={row}
                      onOpenClick={handleCardOpen}
                      suppressOpenRef={suppressCardOpenRef}
                    />
                  ))}
                </KanbanColumnBody>
              </div>
            );
          })}
        </div>
        <DragOverlay dropAnimation={{ duration: 160, easing: 'cubic-bezier(0.25, 1, 0.5, 1)' }}>
          {activeDrag ? <CardPreview row={activeDrag} /> : null}
        </DragOverlay>
      </DndContext>

      <WorkOrderKanbanDetailModal
        orderId={detailOrderId}
        onClose={() => setDetailOrderId(null)}
        onSaved={handleModalSaved}
      />
    </div>
  );
}
