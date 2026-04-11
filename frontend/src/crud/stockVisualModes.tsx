import { normalize } from '@devpablocristo/core-browser/search';
import { useCallback, useDeferredValue, useEffect, useMemo, useState } from 'react';
import { useI18n } from '../lib/i18n';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';
import { CrudGallerySurface, CrudResourceShellHeader, useCrudArchivedSearchParam } from '../modules/crud';
import { fetchStockLevels, StockLevelDetailModal, type StockLevelRow } from '../modules/stock';
import '../pages/StockPage.css';

export function StockGalleryView() {
  const { t } = useI18n();
  const { archived: showArchived } = useCrudArchivedSearchParam();
  const { preSearchFilter } = useCrudListCreatedByMerge();
  const [items, setItems] = useState<StockLevelRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [detailProductId, setDetailProductId] = useState<string | null>(null);
  const deferredSearch = useDeferredValue(search.trim());

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await fetchStockLevels({ archived: showArchived }));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [showArchived]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const creatorFilteredItems = useMemo(
    () => (preSearchFilter ? preSearchFilter(items) : items),
    [items, preSearchFilter],
  );

  const visibleItems = useMemo(() => {
    if (!deferredSearch) return creatorFilteredItems;
    const q = normalize(deferredSearch);
    return creatorFilteredItems.filter((row) => {
      const hay = normalize(
        [row.product_name, row.sku, String(row.quantity), String(row.min_quantity), row.is_low_stock ? 'bajo' : 'normal'].join(' '),
      );
      return hay.includes(q);
    });
  }, [creatorFilteredItems, deferredSearch]);

  return (
    <div className="stock-crud-surface-page">
      <CrudResourceShellHeader<StockLevelRow>
        resourceId="stock"
        preserveCsvToolbar
        items={items}
        subtitleCount={visibleItems.length}
        loading={loading}
        error={error}
        setError={setError}
        reload={reload}
        searchValue={search}
        onSearchChange={setSearch}
        onArchiveToggle={() => setDetailProductId(null)}
      />
      <CrudGallerySurface<StockLevelRow>
        items={visibleItems}
        loading={loading}
        emptyLabel="No hay productos con stock controlado."
        loadingLabel={t('crud.viewMode.gallery.loading')}
        ariaLabel="Productos en galería"
        card={{
          title: (row) => row.product_name,
          subtitle: (row) => row.sku?.trim() || 'sin SKU',
          meta: (row) => `Actual ${row.quantity} · mín. ${row.min_quantity}${row.is_low_stock ? ' · bajo mínimo' : ''}`,
        }}
        onSelect={(row) => setDetailProductId(row.product_id)}
      />
      <StockLevelDetailModal productId={detailProductId} onClose={() => setDetailProductId(null)} onAfterSave={() => void reload()} />
    </div>
  );
}

export { StockInventoryKanbanBoard as StockBoardView } from '../modules/stock';
