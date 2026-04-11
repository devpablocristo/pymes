/**
 * Compat wrapper: la implementación reusable vive en modules/crud.
 */
import { CrudGallerySurface } from '../modules/crud';

export type ProductGalleryItem = {
  id: string;
  name: string;
  sku?: string;
  price?: number;
  currency?: string;
  image_url?: string;
  image_urls?: string[];
};

export type ProductGalleryViewProps<T extends ProductGalleryItem> = {
  items: T[];
  onSelect: (id: string) => void;
  loading?: boolean;
  emptyLabel?: string;
  loadingLabel?: string;
};

function formatPrice(price?: number, currency?: string): string {
  if (price == null || Number.isNaN(price)) return '--';
  return `${currency ?? 'ARS'} ${Number(price).toFixed(0)}`;
}

function cardImageSrc(item: ProductGalleryItem): string {
  const fromList = item.image_urls?.map((u) => u.trim()).filter(Boolean) ?? [];
  if (fromList.length > 0) return fromList[0]!;
  return item.image_url?.trim() ?? '';
}

export function ProductGalleryView<T extends ProductGalleryItem>({
  items,
  onSelect,
  loading,
  emptyLabel = 'Sin productos para mostrar.',
  loadingLabel = 'Cargando...',
}: ProductGalleryViewProps<T>) {
  return (
    <CrudGallerySurface
      items={items}
      onSelect={(item) => onSelect(item.id)}
      loading={loading}
      emptyLabel={emptyLabel}
      loadingLabel={loadingLabel}
      ariaLabel="Galería de productos"
      card={{
        title: (item) => item.name,
        subtitle: (item) => formatPrice(item.price, item.currency),
        meta: (item) => item.sku || 'Sin SKU',
        imageSrc: (item) => cardImageSrc(item),
        imageAlt: (item) => item.name,
      }}
    />
  );
}
