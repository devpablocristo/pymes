import { useCallback, useMemo } from 'react';
import { apiRequest } from '../../lib/api';
import { useI18n } from '../../lib/i18n';
import { PymesCrudResourceShellHeader } from '../../crud/PymesCrudResourceShellHeader';
import { usePymesCrudHeaderFeatures } from '../../crud/usePymesCrudHeaderFeatures';
import { usePymesCrudConfigQuery } from '../../crud/usePymesCrudConfigQuery';
import {
  CrudCreateNavigationButton,
  CrudGallerySurface,
  CrudTableSurface,
  useCrudRemoteGalleryPage,
  type CrudTableSurfaceColumn,
} from '../crud';
import { ProductDetailModal } from './ProductDetailModal';

export type ProductRow = {
  id: string;
  name: string;
  sku?: string;
  unit?: string;
  price?: number;
  currency?: string;
  cost_price?: number;
  image_url?: string;
  image_urls?: string[];
  track_stock?: boolean;
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

function useProductsRemoteState() {
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

  return useCrudRemoteGalleryPage<ProductRow>({
    pageSize: PAGE_SIZE,
    fetchPage,
  });
}

export function ProductsGalleryWorkspace() {
  const { t, localizeText } = useI18n();
  const crudConfigQuery = usePymesCrudConfigQuery<ProductRow>('products', { preserveCsvToolbar: true });
  const crudConfig = crudConfigQuery.data ?? null;

  const {
    items,
    loading,
    error,
    setError,
    hasMore,
    loadingMore,
    loadMore,
    selectedId: detailId,
    selectItem,
    closeDetail,
    reload,
    handleArchiveToggle,
  } = useProductsRemoteState();

  const { search, setSearch, visibleItems, headerLeadSlot, searchInlineActions } = usePymesCrudHeaderFeatures<ProductRow>({
    resourceId: 'products',
    items,
    matchesSearch: (row, query) =>
      [row.name, row.sku, row.unit, String(row.price ?? ''), String(row.cost_price ?? ''), row.currency]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query),
  });

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<ProductRow>
        resourceId="products"
        preserveCsvToolbar
        items={visibleItems}
        subtitleCount={visibleItems.length}
        loading={loading}
        error={error}
        setError={setError}
        reload={reload}
        searchValue={search}
        onSearchChange={setSearch}
        onArchiveToggle={handleArchiveToggle}
        headerLeadSlot={headerLeadSlot}
        searchInlineActions={searchInlineActions}
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

export function ProductsListWorkspace() {
  const { t } = useI18n();

  const {
    items,
    loading,
    error,
    setError,
    hasMore,
    loadingMore,
    loadMore,
    selectedId,
    selectItem,
    closeDetail,
    reload,
    handleArchiveToggle,
  } = useProductsRemoteState();

  const { search, setSearch, visibleItems, headerLeadSlot, searchInlineActions } = usePymesCrudHeaderFeatures<ProductRow>({
    resourceId: 'products',
    items,
    matchesSearch: (row, query) =>
      [row.name, row.sku, row.unit, String(row.price ?? ''), String(row.cost_price ?? ''), row.currency]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query),
  });

  const columns = useMemo<CrudTableSurfaceColumn<ProductRow>[]>(
    () => [
      {
        id: 'name',
        header: 'Producto',
        className: 'cell-name',
        render: (row) => (
          <>
            <strong>{row.name}</strong>
            <div className="text-secondary">{row.sku || 'Sin SKU'} · {row.unit || 'unidad'}</div>
          </>
        ),
      },
      {
        id: 'price',
        header: 'Precio',
        render: (row) =>
          row.price == null || Number.isNaN(row.price) ? '—' : `${row.currency ?? 'ARS'} ${Number(row.price).toFixed(2)}`,
      },
      {
        id: 'cost_price',
        header: 'Costo',
        render: (row) =>
          row.cost_price == null || Number.isNaN(row.cost_price)
            ? '—'
            : `${row.currency ?? 'ARS'} ${Number(row.cost_price).toFixed(2)}`,
      },
      {
        id: 'track_stock',
        header: 'Stock',
        render: (row) => (
          <span className={`badge ${row.track_stock ? 'badge-success' : 'badge-neutral'}`}>
            {row.track_stock ? 'Controlado' : 'Sin control'}
          </span>
        ),
      },
    ],
    [],
  );

  return (
    <div className="products-crud-page">
      <PymesCrudResourceShellHeader<ProductRow>
        resourceId="products"
        preserveCsvToolbar
        items={visibleItems}
        subtitleCount={visibleItems.length}
        loading={loading}
        error={error}
        setError={setError}
        reload={reload}
        searchValue={search}
        onSearchChange={setSearch}
        onArchiveToggle={handleArchiveToggle}
        headerLeadSlot={headerLeadSlot}
        searchInlineActions={searchInlineActions}
        extraHeaderActions={<CrudCreateNavigationButton to="/modules/products/list" label="+ Nuevo producto" />}
      />

      {loading ? (
        <div className="empty-state">
          <p>{t('crud.viewMode.gallery.loading')}</p>
        </div>
      ) : visibleItems.length === 0 ? (
        <div className="empty-state">
          <p>No hay productos para mostrar.</p>
        </div>
      ) : (
        <CrudTableSurface items={visibleItems} columns={columns} onRowClick={(row) => selectItem(row.id)} selectedId={selectedId} />
      )}

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

      <ProductDetailModal productId={selectedId} onClose={closeDetail} />
    </div>
  );
}
