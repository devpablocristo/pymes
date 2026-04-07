import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ProductGalleryView, type ProductGalleryItem } from '../components/ProductGalleryView';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';
import './ProductsGalleryPage.css';

type ProductRow = ProductGalleryItem & { deleted_at?: string | null };

type ProductsListResponse = { items: ProductRow[] };

const LIST_PATH = '/modules/products/list';

/**
 * Vista galería: misma data que el listado; al elegir una tarjeta volvemos a la tabla (como antes con el toggle).
 */
export function ProductsGalleryPage() {
  const navigate = useNavigate();
  const { t } = useI18n();
  const [items, setItems] = useState<ProductRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
        onSelect={() => navigate(LIST_PATH)}
        loading={loading}
        emptyLabel={t('crud.viewMode.gallery.empty')}
        loadingLabel={t('crud.viewMode.gallery.loading')}
      />
    </div>
  );
}
