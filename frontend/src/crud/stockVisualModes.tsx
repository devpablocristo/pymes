import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { PageLayout } from '../components/PageLayout';
import { CrudGallerySurface } from '../modules/crud';
import { fetchStockLevels, StockLevelDetailModal, type StockLevelRow } from '../modules/stock';

export function StockGalleryView() {
  const [items, setItems] = useState<StockLevelRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [detailProductId, setDetailProductId] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await fetchStockLevels());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  return (
    <PageLayout title="Inventario" lead="Vista galería por producto (cantidad y mínimo).">
      {error ? (
        <div className="alert alert-error" role="alert">
          {error}
        </div>
      ) : null}
      <CrudGallerySurface<StockLevelRow>
        items={items}
        loading={loading}
        emptyLabel="No hay productos con stock controlado."
        ariaLabel="Productos en galería"
        card={{
          title: (row) => row.product_name,
          subtitle: (row) => row.sku?.trim() || 'sin SKU',
          meta: (row) => `Actual ${row.quantity} · mín. ${row.min_quantity}${row.is_low_stock ? ' · bajo mínimo' : ''}`,
        }}
        onSelect={(row) => setDetailProductId(row.product_id)}
      />
      <StockLevelDetailModal productId={detailProductId} onClose={() => setDetailProductId(null)} onAfterSave={() => void reload()} />
      <p className="text-secondary text-sm" style={{ marginTop: 'var(--space-3)' }}>
        Tip: la disponibilidad de esta vista y el tablero se ajusta en{' '}
        <Link to="/modules/stock/configure">Configurar vistas del inventario</Link>.
      </p>
    </PageLayout>
  );
}

export { StockInventoryKanbanBoard as StockBoardView } from '../modules/stock';
