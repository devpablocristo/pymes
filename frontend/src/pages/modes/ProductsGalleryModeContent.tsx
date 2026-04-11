import { useCallback, useMemo } from 'react';
import { ProductDetailModal } from '../../components/ProductDetailModal';
import { apiRequest } from '../../lib/api';
import { useI18n } from '../../lib/i18n';
import { useCrudListCreatedByMerge } from '../../lib/useCrudListCreatedByMerge';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import { usePymesCrudConfigQuery } from '../../crud/usePymesCrudConfigQuery';
import {
  CrudCreateNavigationButton,
  CrudGallerySurface,
  useCrudRemoteGalleryPage,
} from '../../modules/crud';
import '../ProductsGalleryPage.css';

type ProductRow = {
  id: string;
  name: string;
  sku?: string;
  price?: number;
  currency?: string;
  image_url?: string;
  image_urls?: string[];
  created_by?: string;
  deleted_at?: string | null;
};

type ProductsListResponse = {
  items: ProductRow[];
  total?: number;
  has_more?: boolean;
  next_cursor?: string;
};

const PAGE_SIZE = 100;

export function ProductsGalleryModeContent() {
  const { t, sentenceCase, localizeText } = useI18n();
  const crudConfigQuery = usePymesCrudConfigQuery<ProductRow>('products', { preserveCsvToolbar: true });
  const crudConfig = crudConfigQuery.data ?? null;
  const { preSearchFilter, listHeaderInlineSlot } = useCrudListCreatedByMerge();

  const fetchPage = useCallback(
    async ({
      limit,
      search: q,
      archived,
      after,
      signal: _signal,
    }: {
      limit: number;
      search: string;
      archived: boolean;
      after: string | null;
      signal: AbortSignal;
    }) => {
      void _signal;
      const query = new URLSearchParams({ limit: String(limit) });
      if (q) query.set('search', q);
      if (archived) query.set('archived', 'true');
      if (after) query.set('after', after);
      return apiRequest<ProductsListResponse>(`/v1/products?${query.toString()}`);
    },
    [],
  );

  const {
    items,
    loading,
    error,
    setError,
    total,
    hasMore,
    loadingMore,
    loadMore,
    search,
    setSearch,
    selectedId: detailId,
    selectItem,
    closeDetail,
    reload,
    handleArchiveToggle,
  } = useCrudRemoteGalleryPage<ProductRow>({
    pageSize: PAGE_SIZE,
    fetchPage,
  });

  const visibleItems = useMemo(() => {
    if (!preSearchFilter) return items;
    return preSearchFilter(items);
  }, [items, preSearchFilter]);
  const subtitle = loading
    ? t('crud.viewMode.gallery.loading')
    : `${total} ${total === 1 ? crudConfig?.label ?? 'producto' : crudConfig?.labelPlural ?? 'productos'}`;

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<ProductRow>
        resourceId="products"
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
        extraHeaderActions={
          <CrudCreateNavigationButton
            to="/modules/products/list"
            label={crudConfig?.createLabel ? localizeText(crudConfig.createLabel) : '+ Nuevo producto'}
          />
        }
      />
      <CrudGallerySurface
        items={visibleItems}
        onSelect={selectItem}
        loading={loading}
        emptyLabel={t('crud.viewMode.gallery.empty')}
        loadingLabel={t('crud.viewMode.gallery.loading')}
        ariaLabel="Galería de productos"
        card={{
          title: (item) => item.name,
          subtitle: (item) => (item.price == null || Number.isNaN(item.price) ? '--' : `${item.currency ?? 'ARS'} ${Number(item.price).toFixed(0)}`),
          meta: (item) => item.sku || 'Sin SKU',
          imageSrc: (item) => item.image_urls?.map((url) => url.trim()).find(Boolean) ?? item.image_url,
          imageAlt: (item) => item.name,
        }}
      />
      {!loading && hasMore ? (
        <div className="crud-load-more">
          <button
            type="button"
            className="btn-secondary"
            disabled={loadingMore}
            onClick={() => {
              void loadMore();
            }}
          >
            {loadingMore ? t('crud.viewMode.gallery.loading') : t('crud.loadMore')}
          </button>
        </div>
      ) : null}
      <ProductDetailModal productId={detailId} onClose={closeDetail} />
    </div>
  );
}
