import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { getAllAutoRepairWorkOrders, patchAutoRepairWorkOrder } from '../lib/autoRepairApi';
import type { AutoRepairWorkOrder } from '../lib/autoRepairTypes';
import './AutoRepairWorkOrdersKanbanPage.css';

/** Referencia OSS: tablero multi-columna con @dnd-kit (MIT) — ver MultipleContainers en clauderic/dnd-kit. */
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
  disabled,
}: {
  row: AutoRepairWorkOrder;
  disabled: boolean;
}) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: row.id,
    disabled,
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

  return (
    <div ref={setNodeRef} style={style} className={cls.join(' ')} {...listeners} {...attributes}>
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

function KanbanColumn({
  status,
  label,
  children,
}: {
  status: string;
  label: string;
  children: ReactNode;
}) {
  const id = `col-${status}`;
  const { setNodeRef, isOver } = useDroppable({ id });
  return (
    <div ref={setNodeRef} className={`wo-kanban__column ${isOver ? 'wo-kanban__column--over' : ''}`} data-column={status}>
      <div className="wo-kanban__column-title">{label}</div>
      <div className="wo-kanban__column-scroll">{children}</div>
    </div>
  );
}

export function AutoRepairWorkOrdersKanbanPage() {
  const [items, setItems] = useState<AutoRepairWorkOrder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [movingId, setMovingId] = useState<string | null>(null);
  const [activeDrag, setActiveDrag] = useState<AutoRepairWorkOrder | null>(null);

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

  const handleDragStart = (e: DragStartEvent) => {
    const id = String(e.active.id);
    setActiveDrag(items.find((x) => x.id === id) ?? null);
  };

  const handleDragEnd = async (e: DragEndEvent) => {
    const { active, over } = e;
    setActiveDrag(null);
    const id = String(active.id);
    const targetStatus = resolveDropStatus(over?.id != null ? String(over.id) : undefined, items);
    if (!targetStatus) return;
    const row = items.find((x) => x.id === id);
    if (!row || canonicalWorkOrderStatus(row.status) === targetStatus) return;
    setMovingId(id);
    setError(null);
    try {
      await patchAutoRepairWorkOrder(id, { status: targetStatus });
      setItems((prev) =>
        prev.map((x) => (x.id === id ? { ...x, status: targetStatus as AutoRepairWorkOrder['status'] } : x)),
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo actualizar el estado');
      await load();
    } finally {
      setMovingId(null);
    }
  };

  const handleDragCancel = () => {
    setActiveDrag(null);
  };

  const totalVisible = filtered.length;

  return (
    <div className="card wo-kanban">
      <div className="wo-kanban__toolbar">
        <div>
          <h1 className="wo-kanban__title">Tablero de órdenes</h1>
          <p className="wo-kanban__muted wo-kanban__subtitle">
            Estilo tablero: arrastrá tarjetas entre columnas (validación en servidor). Comportamiento similar a{' '}
            <a href="https://github.com/clauderic/dnd-kit" target="_blank" rel="noreferrer">
              dnd-kit
            </a>{' '}
            (Kanban multi-contenedor).
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
        collisionDetection={closestCorners}
        onDragStart={handleDragStart}
        onDragEnd={(ev) => void handleDragEnd(ev)}
        onDragCancel={handleDragCancel}
      >
        <div className="wo-kanban__board">
          {COLUMN_ORDER.map((col) => {
            const columnItems = byColumn.get(col.status) ?? [];
            return (
              <KanbanColumn key={col.status} status={col.status} label={`${col.label} · ${columnItems.length}`}>
                {columnItems.map((row) => (
                  <KanbanCard key={row.id} row={row} disabled={movingId === row.id} />
                ))}
              </KanbanColumn>
            );
          })}
        </div>
        <DragOverlay dropAnimation={{ duration: 180, easing: 'cubic-bezier(0.25, 1, 0.5, 1)' }}>
          {activeDrag ? <CardPreview row={activeDrag} /> : null}
        </DragOverlay>
      </DndContext>
    </div>
  );
}
