import { normalize } from '@devpablocristo/core-browser/search';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useState } from 'react';
import { LazyConfiguredCrudPage } from '../../crud/lazyCrudPage';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import { useI18n } from '../../lib/i18n';
import { useCrudListCreatedByMerge } from '../../lib/useCrudListCreatedByMerge';
import { CrudGallerySurface, useCrudRemoteGalleryPage } from '../crud';
import { fetchStockLevels, type StockLevelRow } from './stockData';
import { StockInventoryKanbanBoard } from './StockInventoryKanbanBoard';
import { StockLevelDetailModal } from './StockLevelDetailModal';
import '../../pages/InventoryPage.css';

function useStockRemoteState() {
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

  return useCrudRemoteGalleryPage<StockLevelRow>({
    pageSize: 500,
    fetchPage,
  });
}

export function StockGalleryWorkspace() {
  const { t } = useI18n();
  const { preSearchFilter } = useCrudListCreatedByMerge();
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
  } = useStockRemoteState();

  const creatorFilteredItems = useMemo(() => (preSearchFilter ? preSearchFilter(items) : items), [items, preSearchFilter]);

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
        resourceId="inventory"
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

export function StockListWorkspace() {
  const queryClient = useQueryClient();
  const [detailProductId, setDetailProductId] = useState<string | null>(null);
  const [listKey, setListKey] = useState(0);

  const bumpList = useCallback(() => {
    setListKey((current) => current + 1);
    void queryClient.invalidateQueries({ queryKey: ['inventory'] });
  }, [queryClient]);

  return (
    <>
      <div className="stock-inventory-list-crud">
        <LazyConfiguredCrudPage
          key={listKey}
          resourceId="inventory"
          mergeConfig={{
            onRowClick: (row: { id: string }) => setDetailProductId(row.id),
          }}
        />
      </div>
      <StockLevelDetailModal productId={detailProductId} onClose={() => setDetailProductId(null)} onAfterSave={bumpList} />
    </>
  );
}

export { StockInventoryKanbanBoard as StockBoardWorkspace };
