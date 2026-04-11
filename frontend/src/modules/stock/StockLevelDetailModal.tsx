import { useCallback, useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { Link } from 'react-router-dom';
import { ImageFullscreenViewer } from '../../components/ImageFullscreenViewer';
import { collectProductImageUrls, type ProductDetailResponse } from '../../components/ProductDetailModal';
import { apiRequest } from '../../lib/api';
import { fetchStockLevelByProductId, type StockLevelRow } from './stockLevels';
import '../../pages/StockPage.css';
import './StockLevelDetailModal.css';

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

function formatStockDateTime(raw: string): string {
  if (!raw) return '';
  try {
    return new Date(raw).toLocaleString('es-AR', {
      day: '2-digit',
      month: '2-digit',
      year: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return raw;
  }
}

function movementTypeLabel(type: string): string {
  switch (type) {
    case 'in':
      return 'Entrada';
    case 'out':
      return 'Salida';
    case 'adjustment':
      return 'Ajuste';
    default:
      return type;
  }
}

export type StockLevelDetailModalProps = {
  productId: string | null;
  onClose: () => void;
  /** Tras guardar ajuste o archivar: refrescar listas / tablero. */
  onAfterSave?: () => void;
};

export function StockLevelDetailModal({ productId, onClose, onAfterSave }: StockLevelDetailModalProps) {
  const [row, setRow] = useState<StockLevelRow | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [movements, setMovements] = useState<StockMovement[]>([]);
  const [movementsLoading, setMovementsLoading] = useState(false);
  const [editing, setEditing] = useState(false);
  const [minInput, setMinInput] = useState('');
  const [absoluteQtyInput, setAbsoluteQtyInput] = useState('');
  const [notes, setNotes] = useState('');
  const [formError, setFormError] = useState('');
  const [saving, setSaving] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const [productForImages, setProductForImages] = useState<ProductDetailResponse | null>(null);
  const [productImagesLoading, setProductImagesLoading] = useState(false);
  const [lightboxUrl, setLightboxUrl] = useState<string | null>(null);

  const productImageUrls = useMemo(
    () => (productForImages ? collectProductImageUrls(productForImages) : []),
    [productForImages],
  );

  const resetForm = useCallback((r: StockLevelRow) => {
    setMinInput(String(r.min_quantity ?? ''));
    setAbsoluteQtyInput(String(r.quantity ?? ''));
    setNotes('');
    setFormError('');
  }, []);

  useEffect(() => {
    setEditing(false);
  }, [productId]);

  useEffect(() => {
    if (!productId) {
      setRow(null);
      setLoadError(null);
      setMovements([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    void fetchStockLevelByProductId(productId)
      .then((r) => {
        if (cancelled) return;
        setRow(r);
        resetForm(r);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setRow(null);
        setLoadError(e instanceof Error ? e.message : 'No se pudo cargar el inventario.');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [productId, resetForm]);

  useEffect(() => {
    setLightboxUrl(null);
  }, [productId]);

  useEffect(() => {
    if (!productId) {
      setProductForImages(null);
      return;
    }
    setProductForImages(null);
    let cancelled = false;
    setProductImagesLoading(true);
    void apiRequest<ProductDetailResponse>(`/v1/products/${encodeURIComponent(productId)}`)
      .then((data) => {
        if (!cancelled) setProductForImages(data);
      })
      .catch(() => {
        if (!cancelled) setProductForImages(null);
      })
      .finally(() => {
        if (!cancelled) setProductImagesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [productId]);

  useEffect(() => {
    if (!productId || !row) return;
    let cancelled = false;
    setMovementsLoading(true);
    void apiRequest<{ items?: StockMovement[] | null }>(
      `/v1/inventory/movements?limit=50&product_id=${encodeURIComponent(productId)}`,
    )
      .then((data) => {
        if (!cancelled) setMovements(data.items ?? []);
      })
      .catch(() => {
        if (!cancelled) setMovements([]);
      })
      .finally(() => {
        if (!cancelled) setMovementsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [productId, row?.product_id]);

  useEffect(() => {
    if (!productId) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [productId, onClose]);

  const minParsed = useMemo(() => {
    const n = Number(String(minInput).replace(',', '.'));
    return Number.isFinite(n) ? n : NaN;
  }, [minInput]);

  const absoluteQtyParsed = useMemo(() => {
    const t = absoluteQtyInput.trim();
    if (!t) return NaN;
    const n = Number(t.replace(',', '.'));
    return Number.isFinite(n) ? n : NaN;
  }, [absoluteQtyInput]);

  const dirty = useMemo(() => {
    if (!row) return false;
    const minChanged = Number.isFinite(minParsed) && minParsed !== row.min_quantity;
    const qtyChanged = Number.isFinite(absoluteQtyParsed) && absoluteQtyParsed !== row.quantity;
    return minChanged || qtyChanged;
  }, [row, minParsed, absoluteQtyParsed]);

  /** Con `track_stock === false` solo se permite editar el mínimo (sin movimiento de cantidad). */
  const canSave = useMemo(() => {
    if (row == null || !dirty || notes.trim().length === 0 || saving || !editing) return false;
    const minChanged = Number.isFinite(minParsed) && minParsed !== row.min_quantity;
    const qtyChanged = Number.isFinite(absoluteQtyParsed) && absoluteQtyParsed !== row.quantity;
    if (row.track_stock === false) {
      return minChanged && !qtyChanged;
    }
    return true;
  }, [row, dirty, notes, saving, editing, minParsed, absoluteQtyParsed]);

  const cancelEditing = () => {
    if (row) resetForm(row);
    setEditing(false);
  };

  const handleSave = async () => {
    if (!row || !canSave) return;
    if (!notes.trim()) {
      setFormError('Las notas son obligatorias para guardar cambios.');
      return;
    }
    const minChanged = Number.isFinite(minParsed) && minParsed !== row.min_quantity;
    const qtyChanged = Number.isFinite(absoluteQtyParsed) && absoluteQtyParsed !== row.quantity;
    if (!minChanged && !qtyChanged) return;

    setSaving(true);
    setFormError('');
    try {
      const body: { quantity: number; notes: string; min_quantity?: number } = {
        quantity: qtyChanged ? absoluteQtyParsed - row.quantity : 0,
        notes: notes.trim(),
      };
      if (minChanged) body.min_quantity = minParsed;
      await apiRequest(`/v1/inventory/${encodeURIComponent(row.product_id)}/adjust`, {
        method: 'POST',
        body,
      });
      const next = await fetchStockLevelByProductId(row.product_id);
      setRow(next);
      resetForm(next);
      setEditing(false);
      try {
        const p = await apiRequest<ProductDetailResponse>(`/v1/products/${encodeURIComponent(row.product_id)}`);
        setProductForImages(p);
      } catch {
        /* galería opcional */
      }
      const mv = await apiRequest<{ items?: StockMovement[] | null }>(
        `/v1/inventory/movements?limit=50&product_id=${encodeURIComponent(row.product_id)}`,
      );
      setMovements(mv.items ?? []);
      onAfterSave?.();
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : 'Error al guardar.');
    } finally {
      setSaving(false);
    }
  };

  const handleArchive = async () => {
    if (!row) return;
    if (
      !window.confirm(
        '¿Archivar este producto? Dejará de mostrarse en listados activos; podés restaurarlo desde Productos → archivados.',
      )
    ) {
      return;
    }
    setArchiving(true);
    setFormError('');
    try {
      await apiRequest(`/v1/products/${encodeURIComponent(row.product_id)}/archive`, { method: 'POST', body: {} });
      onAfterSave?.();
      onClose();
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : 'No se pudo archivar.');
    } finally {
      setArchiving(false);
    }
  };

  if (!productId) return null;

  const qtyLockedNoTrack = row != null && row.track_stock === false;

  const body = (
    <div className="stock-level-modal-root">
      <button type="button" className="stock-level-modal__backdrop" aria-label="Cerrar" onClick={onClose} />
      <div
        className="stock-level-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="stock-level-modal-title"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="stock-level-modal__header">
          <div className="stock-level-modal__title-block">
            <h2 id="stock-level-modal-title" className="stock-level-modal__title">
              {loading ? 'Cargando…' : row?.product_name ?? 'Stock'}
            </h2>
            {row ? <p className="stock-level-modal__subtitle">{row.sku?.trim() || 'sin SKU'}</p> : null}
          </div>
        </header>

        <div className="stock-level-modal__body">
          {!productImagesLoading && productImageUrls.length > 0 ? (
            <section className="stock-level-modal__gallery" aria-label="Imágenes del producto">
              {productImageUrls.map((url) => (
                <figure key={url} className="stock-level-modal__gallery-item">
                  <button
                    type="button"
                    className="stock-level-modal__gallery-zoom"
                    onClick={() => setLightboxUrl(url)}
                    aria-label="Ver imagen a pantalla completa"
                  >
                    <img
                      src={url}
                      alt=""
                      loading="lazy"
                      onError={(e) => {
                        const fig = (e.currentTarget as HTMLImageElement).closest('figure');
                        if (fig) fig.hidden = true;
                      }}
                    />
                  </button>
                </figure>
              ))}
            </section>
          ) : null}

          <div className="stock-level-modal__main">
            {loadError ? <p className="stock-level-modal__error">{loadError}</p> : null}
            {row && !loading ? (
              <>
                {formError && !editing ? <p className="stock-level-modal__error">{formError}</p> : null}
                {qtyLockedNoTrack ? (
                  <p className="stock-level-modal__muted">
                    Este producto no tiene control de stock activado en la ficha de producto. Podés editar el stock
                    mínimo desde aquí; para cambiar la cantidad en depósito, activá el control de stock en Productos.
                  </p>
                ) : null}

                <div className="stock-detail stock-detail--in-modal">
                  {row.is_low_stock ? (
                    <div style={{ marginBottom: 'var(--space-3)' }}>
                      <span className="stock-badge-low">bajo mínimo</span>
                    </div>
                  ) : null}

                  {!editing ? (
                    <>
                      <div className="stock-detail__stats">
                        <div className="stock-detail__stat">
                          <span>Actual</span>
                          <strong>{row.quantity}</strong>
                        </div>
                        <div className="stock-detail__stat">
                          <span>Mínimo</span>
                          <strong>{row.min_quantity}</strong>
                        </div>
                        <div className="stock-detail__stat">
                          <span>Actualizado</span>
                          <strong>{formatStockDateTime(row.updated_at)}</strong>
                        </div>
                      </div>
                      <p className="stock-level-modal__muted stock-level-modal__edit-hint">
                        Usá <strong>Editar</strong> para modificar cantidad, mínimo o dejar notas al guardar.
                      </p>
                    </>
                  ) : (
                    <div className="stock-detail__section stock-detail__section--editing">
                      <h4>Editar stock</h4>
                      {formError ? <p className="stock-level-modal__error">{formError}</p> : null}
                      <div className="stock-level-modal__form-grid">
                        <div className="stock-level-modal__field">
                          <label htmlFor="stock-modal-qty">Cantidad actual</label>
                          <input
                            id="stock-modal-qty"
                            type="number"
                            step="any"
                            value={absoluteQtyInput}
                            readOnly={qtyLockedNoTrack}
                            onChange={(e) => setAbsoluteQtyInput(e.target.value)}
                          />
                          {qtyLockedNoTrack ? (
                            <p className="stock-level-modal__field-hint text-secondary text-sm u-m-0">
                              Cantidad fijada por el catálogo (sin control de stock).
                            </p>
                          ) : null}
                        </div>
                        <div className="stock-level-modal__field">
                          <label htmlFor="stock-modal-min">Stock mínimo</label>
                          <input
                            id="stock-modal-min"
                            type="number"
                            step="any"
                            value={minInput}
                            onChange={(e) => setMinInput(e.target.value)}
                          />
                        </div>
                        <div className="stock-level-modal__field" style={{ gridColumn: '1 / -1' }}>
                          <p className="text-secondary text-sm u-mb-2">
                            Última actualización en servidor: <strong>{formatStockDateTime(row.updated_at)}</strong>
                          </p>
                        </div>
                        <div className="stock-level-modal__field" style={{ gridColumn: '1 / -1' }}>
                          <label htmlFor="stock-modal-notes">Notas / motivo (obligatorio si guardás cambios)</label>
                          <textarea id="stock-modal-notes" value={notes} onChange={(e) => setNotes(e.target.value)} rows={3} />
                        </div>
                      </div>
                      <Link className="stock-level-modal__link" to="/modules/products/list">
                        Ir a catálogo de productos (nombre, precio, SKU…)
                      </Link>
                    </div>
                  )}

                  <div className="stock-detail__section">
                    <h4>Movimientos recientes</h4>
                    {movementsLoading ? (
                      <p className="text-secondary">Cargando movimientos…</p>
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
                          {movements.map((movement) => (
                            <tr key={movement.id}>
                              <td>{movementTypeLabel(movement.type)}</td>
                              <td className="stock-col-num">
                                <span
                                  className={
                                    movement.type === 'in' ? 'stock-qty-in' : movement.type === 'out' ? 'stock-qty-out' : ''
                                  }
                                >
                                  {movement.quantity > 0 ? `+${movement.quantity}` : movement.quantity}
                                </span>
                              </td>
                              <td>{movement.reason || movement.notes || '—'}</td>
                              <td>{movement.created_by || '—'}</td>
                              <td className="stock-col-date">{formatStockDateTime(movement.created_at)}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    )}
                  </div>
                </div>
              </>
            ) : loading ? (
              <p className="text-secondary">Cargando inventario…</p>
            ) : null}
          </div>
        </div>

        <footer className="stock-level-modal__footer">
          <div className="stock-level-modal__footer-actions">
            <button type="button" className="btn-sm btn-danger" disabled={!row || archiving} onClick={() => void handleArchive()}>
              {archiving ? 'Archivando…' : 'Archivar producto'}
            </button>
          </div>
          <div className="stock-level-modal__footer-actions">
            {!editing ? (
              <>
                <button type="button" className="btn-sm btn-secondary" onClick={onClose}>
                  Cerrar
                </button>
                <button
                  type="button"
                  className="btn-sm btn-primary"
                  disabled={!row}
                  onClick={() => {
                    setFormError('');
                    if (row) resetForm(row);
                    setEditing(true);
                  }}
                >
                  Editar
                </button>
              </>
            ) : (
              <>
                <button type="button" className="btn-sm btn-secondary" onClick={cancelEditing}>
                  Cancelar edición
                </button>
                <button type="button" className="btn-sm btn-secondary" onClick={onClose}>
                  Cerrar
                </button>
                <button type="button" className="btn-sm btn-primary" disabled={!canSave} onClick={() => void handleSave()}>
                  {saving ? 'Guardando…' : 'Guardar'}
                </button>
              </>
            )}
          </div>
        </footer>
      </div>
    </div>
  );

  return (
    <>
      {createPortal(body, document.body)}
      <ImageFullscreenViewer
        imageUrl={lightboxUrl}
        onClose={() => setLightboxUrl(null)}
        contentLabel={row?.product_name ?? undefined}
      />
    </>
  );
}
