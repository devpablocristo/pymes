import { useCallback, useEffect, useState } from 'react';
import { PageLayout } from '../components/PageLayout';
import { usePageSearch } from '../components/PageSearch';
import { apiRequest } from '../lib/api';
import './StockPage.css';

type StockLevel = {
  product_id: string;
  product_name: string;
  sku: string;
  quantity: number;
  min_quantity: number;
  track_stock: boolean;
  is_low_stock: boolean;
  updated_at: string;
};

type StockMovement = {
  id: string;
  product_id: string;
  product_name: string;
  type: string;
  quantity: number;
  reason: string;
  notes: string;
  created_by: string;
  created_at: string;
};

function formatDate(raw: string): string {
  if (!raw) return '';
  try {
    return new Date(raw).toLocaleDateString('es-AR', { day: '2-digit', month: '2-digit', year: '2-digit' });
  } catch {
    return raw;
  }
}

function formatDateTime(raw: string): string {
  if (!raw) return '';
  try {
    return new Date(raw).toLocaleString('es-AR', {
      day: '2-digit', month: '2-digit', year: '2-digit',
      hour: '2-digit', minute: '2-digit',
    });
  } catch {
    return raw;
  }
}

function movementTypeLabel(type: string): string {
  switch (type) {
    case 'in': return 'Entrada';
    case 'out': return 'Salida';
    case 'adjustment': return 'Ajuste';
    default: return type;
  }
}

export function StockPage() {
  const search = usePageSearch();

  const [levels, setLevels] = useState<StockLevel[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [movements, setMovements] = useState<StockMovement[]>([]);
  const [movementsLoading, setMovementsLoading] = useState(false);

  // Ajuste manual
  const [adjustingId, setAdjustingId] = useState<string | null>(null);
  const [adjustQty, setAdjustQty] = useState('');
  const [adjustNotes, setAdjustNotes] = useState('');
  const [adjustError, setAdjustError] = useState('');
  const [adjustSaving, setAdjustSaving] = useState(false);

  const fetchLevels = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const data = await apiRequest<{ items?: StockLevel[] | null }>('/v1/inventory?limit=500');
      setLevels(data.items ?? []);
    } catch {
      setError('No se pudo cargar el stock.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void fetchLevels(); }, [fetchLevels]);

  const fetchMovements = useCallback(async (productId: string) => {
    setMovementsLoading(true);
    try {
      const data = await apiRequest<{ items?: StockMovement[] | null }>(
        `/v1/inventory/movements?limit=50&product_id=${productId}`,
      );
      setMovements(data.items ?? []);
    } catch {
      setMovements([]);
    } finally {
      setMovementsLoading(false);
    }
  }, []);

  function toggleExpand(productId: string) {
    if (expandedId === productId) {
      setExpandedId(null);
      setMovements([]);
      return;
    }
    setExpandedId(productId);
    void fetchMovements(productId);
  }

  function startAdjust(productId: string) {
    setAdjustingId(productId);
    setAdjustQty('');
    setAdjustNotes('');
    setAdjustError('');
  }

  async function submitAdjust() {
    if (!adjustingId) return;
    const qty = Number(adjustQty.replace(',', '.'));
    if (!Number.isFinite(qty) || qty === 0) {
      setAdjustError('Ingresá una cantidad válida (positiva para entrada, negativa para salida).');
      return;
    }
    if (!adjustNotes.trim()) {
      setAdjustError('El motivo es obligatorio.');
      return;
    }
    setAdjustSaving(true);
    setAdjustError('');
    try {
      await apiRequest(`/v1/inventory/${adjustingId}/adjust`, {
        method: 'POST',
        body: { quantity: qty, notes: adjustNotes.trim() },
      });
      setAdjustingId(null);
      await fetchLevels();
      if (expandedId === adjustingId) {
        await fetchMovements(adjustingId);
      }
    } catch (e) {
      setAdjustError(e instanceof Error ? e.message : 'Error al ajustar.');
    } finally {
      setAdjustSaving(false);
    }
  }

  const query = (search ?? '').toLowerCase();
  const filtered = query
    ? levels.filter((l) =>
        l.product_name.toLowerCase().includes(query) ||
        (l.sku ?? '').toLowerCase().includes(query),
      )
    : levels;

  return (
    <PageLayout className="stock-page" title="Stock" lead="Niveles de stock y movimientos por producto.">
      {loading && <div className="spinner" aria-label="Cargando" />}
      {error && <p className="alert alert-error">{error}</p>}

      {!loading && !error && (
        <div className="card">
          <table className="stock-table">
            <thead>
              <tr>
                <th>Producto</th>
                <th className="stock-col-num">Actual</th>
                <th className="stock-col-num">Mínimo</th>
                <th className="stock-col-date">Actualizado</th>
                <th className="stock-col-actions" />
              </tr>
            </thead>
            <tbody>
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={5} className="stock-empty">
                    {query ? 'Sin resultados.' : 'No hay productos con stock controlado.'}
                  </td>
                </tr>
              )}
              {filtered.map((row) => {
                const isExpanded = expandedId === row.product_id;
                const isAdjusting = adjustingId === row.product_id;
                return (
                  <StockRow
                    key={row.product_id}
                    row={row}
                    isExpanded={isExpanded}
                    isAdjusting={isAdjusting}
                    onToggle={() => toggleExpand(row.product_id)}
                    onAdjust={() => startAdjust(row.product_id)}
                    adjustForm={
                      isAdjusting
                        ? {
                            qty: adjustQty,
                            notes: adjustNotes,
                            error: adjustError,
                            saving: adjustSaving,
                            onQtyChange: setAdjustQty,
                            onNotesChange: setAdjustNotes,
                            onSubmit: () => void submitAdjust(),
                            onCancel: () => setAdjustingId(null),
                          }
                        : null
                    }
                    movementsLoading={isExpanded && movementsLoading}
                    movements={isExpanded ? movements : []}
                  />
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </PageLayout>
  );
}

type AdjustForm = {
  qty: string;
  notes: string;
  error: string;
  saving: boolean;
  onQtyChange: (v: string) => void;
  onNotesChange: (v: string) => void;
  onSubmit: () => void;
  onCancel: () => void;
};

function StockRow({
  row,
  isExpanded,
  isAdjusting,
  onToggle,
  onAdjust,
  adjustForm,
  movementsLoading,
  movements,
}: {
  row: StockLevel;
  isExpanded: boolean;
  isAdjusting: boolean;
  onToggle: () => void;
  onAdjust: () => void;
  adjustForm: AdjustForm | null;
  movementsLoading: boolean;
  movements: StockMovement[];
}) {
  return (
    <>
      <tr className={row.is_low_stock ? 'stock-row-low' : ''}>
        <td>
          <button type="button" className="stock-product-btn" onClick={onToggle}>
            <span className={`stock-expand-icon ${isExpanded ? 'open' : ''}`}>▶</span>
            <span>
              <strong>{row.product_name}</strong>
              <span className="text-secondary stock-sku">{row.sku || 'sin SKU'}</span>
              {row.is_low_stock && <span className="stock-badge-low">bajo mínimo</span>}
            </span>
          </button>
        </td>
        <td className="stock-col-num">{row.quantity}</td>
        <td className="stock-col-num">{row.min_quantity}</td>
        <td className="stock-col-date">{formatDate(row.updated_at)}</td>
        <td className="stock-col-actions">
          <button type="button" className="btn-sm btn-primary" onClick={onAdjust}>
            Ajustar
          </button>
        </td>
      </tr>
      {adjustForm && (
        <tr className="stock-adjust-row">
          <td colSpan={5}>
            <div className="stock-adjust-form">
              <input
                type="number"
                step="any"
                placeholder="Cantidad (+/-)"
                value={adjustForm.qty}
                onChange={(e) => adjustForm.onQtyChange(e.target.value)}
                autoFocus
              />
              <input
                type="text"
                placeholder="Motivo (obligatorio)"
                value={adjustForm.notes}
                onChange={(e) => adjustForm.onNotesChange(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && adjustForm.onSubmit()}
              />
              <button
                type="button"
                className="btn-sm btn-primary"
                disabled={adjustForm.saving}
                onClick={adjustForm.onSubmit}
              >
                {adjustForm.saving ? 'Guardando...' : 'Confirmar'}
              </button>
              <button type="button" className="btn-sm" onClick={adjustForm.onCancel}>
                Cancelar
              </button>
              {adjustForm.error && <span className="stock-adjust-error">{adjustForm.error}</span>}
            </div>
          </td>
        </tr>
      )}
      {isExpanded && (
        <tr className="stock-movements-row">
          <td colSpan={5}>
            {movementsLoading ? (
              <div className="spinner" aria-label="Cargando movimientos" />
            ) : movements.length === 0 ? (
              <p className="stock-empty">Sin movimientos registrados.</p>
            ) : (
              <table className="stock-movements-table">
                <thead>
                  <tr>
                    <th>Tipo</th>
                    <th className="stock-col-num">Cant.</th>
                    <th>Motivo</th>
                    <th>Usuario</th>
                    <th className="stock-col-date">Fecha</th>
                  </tr>
                </thead>
                <tbody>
                  {movements.map((m) => (
                    <tr key={m.id} className={`stock-mov-${m.type}`}>
                      <td>{movementTypeLabel(m.type)}</td>
                      <td className="stock-col-num">
                        <span className={m.type === 'in' ? 'stock-qty-in' : m.type === 'out' ? 'stock-qty-out' : ''}>
                          {m.type === 'in' ? '+' : m.type === 'out' ? '-' : ''}{m.quantity}
                        </span>
                      </td>
                      <td>{m.reason || m.notes || '—'}</td>
                      <td>{m.created_by || '—'}</td>
                      <td className="stock-col-date">{formatDateTime(m.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </td>
        </tr>
      )}
    </>
  );
}

export default StockPage;
