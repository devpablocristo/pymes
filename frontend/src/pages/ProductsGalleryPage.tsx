import { useCallback, useEffect, useState } from 'react';
import { ProductDetailModal } from '../components/ProductDetailModal';
import { ProductGalleryView, type ProductGalleryItem } from '../components/ProductGalleryView';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';
import './ProductsGalleryPage.css';

type ProductRow = ProductGalleryItem & { deleted_at?: string | null };

type ProductsListResponse = { items: ProductRow[] };

/**
 * Vista galería: misma data que el listado; clic en tarjeta abre detalle (modal) con imágenes y datos.
 */
export function ProductsGalleryPage() {
  const { t } = useI18n();
  const [items, setItems] = useState<ProductRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [detailId, setDetailId] = useState<string | null>(null);

  const closeDetail = useCallback(() => setDetailId(null), []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    apiRequest<ProductsListResponse>('/v1/products')
      .then((data) => {
        if (cancelled) return;
        const list = (data.items ?? []).filter((p) => !p.deleted_at);
        setItems(list);
        setError(null);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : 'Error');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) {
    return (
      <div className="products-crud-page">
        <div className="empty-state">
          <p>{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="products-crud-page">
      <ProductGalleryView
        items={items}
        onSelect={(id) => setDetailId(id)}
        loading={loading}
        emptyLabel={t('crud.viewMode.gallery.empty')}
        loadingLabel={t('crud.viewMode.gallery.loading')}
      />
      <ProductDetailModal productId={detailId} onClose={closeDetail} />
    </div>
  );
}
