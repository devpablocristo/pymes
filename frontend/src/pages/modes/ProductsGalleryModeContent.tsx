import { CrudPageShell } from '@devpablocristo/core-browser/crud';
import { useCallback, useDeferredValue, useMemo, useState } from 'react';
import { ProductDetailModal } from '../../components/ProductDetailModal';
import { apiRequest } from '../../lib/api';
import { useI18n } from '../../lib/i18n';
import { useCrudListCreatedByMerge } from '../../lib/useCrudListCreatedByMerge';
import {
  CrudCreateNavigationButton,
  CrudGallerySurface,
  CrudToolbarActionButtons,
  useCrudConfigQuery,
  useCrudRemotePaginatedList,
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
  const crudConfigQuery = useCrudConfigQuery<ProductRow>('products', { preserveCsvToolbar: true });
  const crudConfig = crudConfigQuery.data ?? null;
  const { preSearchFilter, listHeaderInlineSlot } = useCrudListCreatedByMerge();
  const [detailId, setDetailId] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [showArchived, setShowArchived] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const deferredSearch = useDeferredValue(search.trim());

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
  } = useCrudRemotePaginatedList<ProductRow>({
    pageSize: PAGE_SIZE,
    deferredSearch,
    archived: showArchived,
    reloadKey,
    fetchPage,
  });

  const closeDetail = useCallback(() => setDetailId(null), []);

  const visibleItems = useMemo(() => {
    if (!preSearchFilter) return items;
    return preSearchFilter(items);
  }, [items, preSearchFilter]);
  const title = useMemo(() => {
    if (!crudConfig) return t('crud.viewMode.gallery');
    if (!showArchived) return sentenceCase(crudConfig.labelPluralCap);
    return sentenceCase(
      t('crud.title.archived', {
        labelPluralCap: crudConfig.labelPluralCap,
        labelPlural: crudConfig.labelPlural,
        label: crudConfig.label,
      }),
    );
  }, [crudConfig, sentenceCase, showArchived, t]);
  const subtitle = loading
    ? t('crud.viewMode.gallery.loading')
    : `${total} ${total === 1 ? crudConfig?.label ?? 'producto' : crudConfig?.labelPlural ?? 'productos'}`;
  const searchPlaceholderResolved = crudConfig?.searchPlaceholder
    ? localizeText(crudConfig.searchPlaceholder)
    : t('crud.search.placeholder');

  const reloadItems = useCallback(async () => {
    setReloadKey((value) => value + 1);
  }, []);

  return (
    <div className="products-crud-page">
      <CrudPageShell
        title={title}
        subtitle={subtitle}
        headerLeadSlot={
          listHeaderInlineSlot &&
          crudConfig?.featureFlags?.headerQuickFilterStrip !== false &&
          crudConfig?.featureFlags?.creatorFilter !== false ? (
            <div className="crud-list-header-lead">{listHeaderInlineSlot({ items })}</div>
          ) : undefined
        }
        search={{
          value: search,
          onChange: setSearch,
          placeholder: searchPlaceholderResolved,
          inputClassName: 'm-kanban__search',
        }}
        headerActions={
          <>
            <CrudToolbarActionButtons
              actions={crudConfig?.toolbarActions}
              items={items}
              archived={showArchived}
              reload={reloadItems}
              setError={setError}
              formatLabel={localizeText}
            />
            <CrudCreateNavigationButton
              to="/modules/products/list"
              label={crudConfig?.createLabel ? localizeText(crudConfig.createLabel) : '+ Nuevo producto'}
            />
            <button
              type="button"
              className={`btn-sm ${showArchived ? 'btn-primary' : 'btn-secondary'}`}
              onClick={() => {
                setShowArchived((value) => !value);
                setDetailId(null);
              }}
            >
              {showArchived ? t('crud.toggle.showActive') : t('crud.toggle.showArchived')}
            </button>
          </>
        }
        error={error ? <div className="alert alert-error">{error}</div> : undefined}
      >
        {null}
      </CrudPageShell>
      <CrudGallerySurface
        items={visibleItems}
        onSelect={(item) => setDetailId(item.id)}
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
