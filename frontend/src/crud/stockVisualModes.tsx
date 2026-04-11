import { normalize } from '@devpablocristo/core-browser/search';
import { useCallback, useMemo } from 'react';
import { useI18n } from '../lib/i18n';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';
import { CrudGallerySurface, useCrudRemoteGalleryPage } from '../modules/crud';
import { PymesCrudResourceShellHeader } from './PymesCrudResourceShellHeader';
import { fetchStockLevels, StockLevelDetailModal, type StockLevelRow } from '../modules/stock';
import '../pages/StockPage.css';

export function StockGalleryView() {
  const { t } = useI18n();
  const { preSearchFilter } = useCrudListCreatedByMerge();

  const fetchPage = useCallback(
    async ({
      archived,
      signal: _signal,
    }: {
      limit: number;
      search: string;
      archived: boolean;
      after: string | null;
      signal: AbortSignal;
    }) => {
      void _signal;
      return {
        items: await fetchStockLevels({ archived }),
        has_more: false,
        next_cursor: null,
      };
    },
    [],
  );

  const {
    items,
    loading,
    error,
    setError,
    search,
    setSearch,
    deferredSearch,
    selectedId: detailProductId,
    selectItem,
    closeDetail,
    reload,
    handleArchiveToggle,
  } = useCrudRemoteGalleryPage<StockLevelRow>({
    pageSize: 500,
    fetchPage,
  });

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
      <PymesCrudResourceShellHeader<StockLevelRow>
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
        onArchiveToggle={handleArchiveToggle}
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
        onSelect={(row) => selectItem(row.product_id)}
      />
      <StockLevelDetailModal productId={detailProductId} onClose={closeDetail} onAfterSave={() => void reload()} />
    </div>
  );
}

export { StockInventoryKanbanBoard as StockBoardView } from '../modules/stock';
