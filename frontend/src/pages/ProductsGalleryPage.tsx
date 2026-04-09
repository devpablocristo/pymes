import { CrudPageShell } from '@devpablocristo/core-browser/crud';
import { useCallback, useDeferredValue, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ProductDetailModal } from '../components/ProductDetailModal';
import { ProductGalleryView, type ProductGalleryItem } from '../components/ProductGalleryView';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';
import { getCrudPageConfig } from '../crud/resourceConfigs.commercial';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';
import './ProductsGalleryPage.css';

type ProductRow = ProductGalleryItem & { created_by?: string; deleted_at?: string | null };

type ProductsListResponse = {
  items: ProductRow[];
  total?: number;
  has_more?: boolean;
  next_cursor?: string;
};

const PAGE_SIZE = 100;

/**
 * Vista galería: misma data que el listado; clic en tarjeta abre detalle (modal) con imágenes y datos.
 */
export function ProductsGalleryPage() {
  const navigate = useNavigate();
  const { t, sentenceCase, localizeText } = useI18n();
  const crudConfig = getCrudPageConfig<ProductRow>('products');
  const { preSearchFilter, listHeaderInlineSlot } = useCrudListCreatedByMerge();
  const [items, setItems] = useState<ProductRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [detailId, setDetailId] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [search, setSearch] = useState('');
  const [showArchived, setShowArchived] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const deferredSearch = useDeferredValue(search.trim());

  const closeDetail = useCallback(() => setDetailId(null), []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setLoadingMore(false);
    const query = new URLSearchParams({ limit: String(PAGE_SIZE) });
    if (deferredSearch) query.set('search', deferredSearch);
    if (showArchived) query.set('archived', 'true');
    apiRequest<ProductsListResponse>(`/v1/products?${query.toString()}`)
      .then((data) => {
        if (cancelled) return;
        const list = data.items ?? [];
        setItems(list);
        setTotal(Number(data.total ?? 0));
        setHasMore(Boolean(data.has_more));
        setNextCursor(data.next_cursor?.trim() || null);
        setError(null);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : 'Error');
        setItems([]);
        setHasMore(false);
        setNextCursor(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [deferredSearch, showArchived, reloadKey]);

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
  const visibleToolbarActions = useMemo(
    () => (crudConfig?.toolbarActions ?? []).filter((action) => action.isVisible?.({ archived: showArchived, items }) ?? true),
    [crudConfig, items, showArchived],
  );

  const runToolbarAction = useCallback(
    async (actionId: string) => {
      if (!crudConfig) return;
      const action = (crudConfig.toolbarActions ?? []).find((candidate) => candidate.id === actionId);
      if (!action) return;
      setError(null);
      try {
        await action.onClick({
          items,
          reload: async () => {
            setReloadKey((value) => value + 1);
          },
          setError: (message: string) => setError(message),
        });
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Error');
      }
    },
    [crudConfig, items],
  );

  const loadMore = useCallback(async () => {
    if (!nextCursor || loadingMore) return;
    setLoadingMore(true);
    setError(null);
    try {
      const query = new URLSearchParams({ limit: String(PAGE_SIZE), after: nextCursor });
      if (deferredSearch) query.set('search', deferredSearch);
      if (showArchived) query.set('archived', 'true');
      const data = await apiRequest<ProductsListResponse>(`/v1/products?${query.toString()}`);
      setItems((prev) => [...prev, ...(data.items ?? [])]);
      setHasMore(Boolean(data.has_more));
      setNextCursor(data.next_cursor?.trim() || null);
      setTotal(Number(data.total ?? total));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error');
    } finally {
      setLoadingMore(false);
    }
  }, [deferredSearch, loadingMore, nextCursor, showArchived, total]);

  return (
    <div className="products-crud-page">
      {crudConfig ? (
        <CrudPageShell
          title={title}
          subtitle={subtitle}
          headerLeadSlot={listHeaderInlineSlot ? <div className="crud-list-header-lead">{listHeaderInlineSlot({ items })}</div> : undefined}
          search={{
            value: search,
            onChange: setSearch,
            placeholder: crudConfig.searchPlaceholder ? localizeText(crudConfig.searchPlaceholder) : t('crud.search.placeholder'),
            inputClassName: 'm-kanban__search',
          }}
          headerActions={
            <>
              {visibleToolbarActions.map((action) => (
                <button
                  key={action.id}
                  type="button"
                  className={`btn-sm ${action.kind === 'primary' ? 'btn-primary' : action.kind === 'danger' ? 'btn-danger' : action.kind === 'success' ? 'btn-success' : 'btn-secondary'}`}
                  onClick={() => {
                    void runToolbarAction(action.id);
                  }}
                >
                  {localizeText(action.label)}
                </button>
              ))}
              <button
                type="button"
                className="btn-sm btn-primary"
                onClick={() => navigate('/modules/products/list')}
              >
                {crudConfig.createLabel ? localizeText(crudConfig.createLabel) : '+ Nuevo producto'}
              </button>
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
      ) : null}
      <ProductGalleryView
        items={visibleItems}
        onSelect={(id) => setDetailId(id)}
        loading={loading}
        emptyLabel={t('crud.viewMode.gallery.empty')}
        loadingLabel={t('crud.viewMode.gallery.loading')}
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
