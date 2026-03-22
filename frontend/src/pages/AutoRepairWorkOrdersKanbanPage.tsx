import { useCallback, useEffect, useMemo, useState, type DragEvent } from 'react';
import { Link } from 'react-router-dom';
import { getAutoRepairWorkOrders, patchAutoRepairWorkOrder } from '../lib/autoRepairApi';
import type { AutoRepairWorkOrder } from '../lib/autoRepairTypes';
import './AutoRepairWorkOrdersKanbanPage.css';

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

function canonicalStatus(raw: string): string {
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

export function AutoRepairWorkOrdersKanbanPage() {
  const [items, setItems] = useState<AutoRepairWorkOrder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [movingId, setMovingId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await getAutoRepairWorkOrders({ limit: 250 });
      setItems(res.items ?? []);
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
      const c = canonicalStatus(row.status);
      const bucket = map.get(c);
      if (bucket) {
        bucket.push(row);
      } else {
        const recv = map.get('received');
        if (recv) recv.push(row);
      }
    }
    return map;
  }, [filtered]);

  const onDragStart = (e: DragEvent, id: string) => {
    e.dataTransfer.setData('application/x-work-order-id', id);
    e.dataTransfer.effectAllowed = 'move';
  };

  const onDragOver = (e: DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  };

  const onDrop = async (e: DragEvent, targetStatus: string) => {
    e.preventDefault();
    const id = e.dataTransfer.getData('application/x-work-order-id');
    if (!id) return;
    const row = items.find((x) => x.id === id);
    if (!row || canonicalStatus(row.status) === targetStatus) return;
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

  return (
    <div className="card wo-kanban">
      <div className="wo-kanban__toolbar">
        <div>
          <h1 style={{ margin: 0, fontSize: '1.35rem' }}>Tablero de órdenes</h1>
          <p className="wo-kanban__muted" style={{ margin: '0.25rem 0 0' }}>
            Arrastrá una tarjeta a otra columna para cambiar el estado (validación en servidor).
          </p>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', alignItems: 'center' }}>
          <input
            type="search"
            placeholder="Buscar OT, patente, cliente…"
            value={search}
            onChange={(ev) => setSearch(ev.target.value)}
            aria-label="Buscar órdenes"
          />
          <button type="button" className="btn btn-secondary" onClick={() => void load()} disabled={loading}>
            Actualizar
          </button>
          <Link to="/workshops/auto-repair/orders" className="btn btn-secondary">
            Lista detalle
          </Link>
        </div>
      </div>

      {error ? (
        <p style={{ color: 'var(--color-danger)' }} role="alert">
          {error}
        </p>
      ) : null}
      {loading ? <p className="wo-kanban__muted">Cargando…</p> : null}

      <div className="wo-kanban__board">
        {COLUMN_ORDER.map((col) => (
          <div
            key={col.status}
            className="wo-kanban__column"
            onDragOver={onDragOver}
            onDrop={(ev) => void onDrop(ev, col.status)}
          >
            <div className="wo-kanban__column-title">
              {col.label} · {(byColumn.get(col.status) ?? []).length}
            </div>
            {(byColumn.get(col.status) ?? []).map((row) => {
              const overdue = isPromiseOverdue(row.promised_at);
              const soon = !overdue && isPromiseSoon(row.promised_at);
              const cls = ['wo-kanban__card'];
              if (overdue) cls.push('wo-kanban__card--overdue');
              else if (soon) cls.push('wo-kanban__card--soon');
              return (
                <div
                  key={row.id}
                  className={cls.join(' ')}
                  draggable={movingId !== row.id}
                  onDragStart={(ev) => onDragStart(ev, row.id)}
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
            })}
          </div>
        ))}
      </div>
    </div>
  );
}
