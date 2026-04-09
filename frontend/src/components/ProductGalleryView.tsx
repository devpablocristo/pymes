/**
 * Gallery view for the products CRUD: cards with image, name, price and SKU.
 * Alternative visualization mode to the standard table.
 */
import type { CSSProperties } from 'react';

export type ProductGalleryItem = {
  id: string;
  name: string;
  sku?: string;
  price?: number;
  currency?: string;
  image_url?: string;
  /** Varias URLs; la tarjeta usa la primera. */
  image_urls?: string[];
};

export type ProductGalleryViewProps<T extends ProductGalleryItem> = {
  items: T[];
  onSelect: (id: string) => void;
  loading?: boolean;
  emptyLabel?: string;
  loadingLabel?: string;
};

const PLACEHOLDER_IMAGE =
  'data:image/svg+xml;utf8,' +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 200">' +
      '<rect width="200" height="200" fill="#e5e7eb"/>' +
      '<path d="M40 150 L80 100 L110 130 L140 90 L170 150 Z" fill="#9ca3af"/>' +
      '<circle cx="140" cy="60" r="14" fill="#9ca3af"/>' +
      '</svg>',
  );

const gridStyle: CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
  gap: 16,
  width: '100%',
};

const cardStyle: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  background: 'var(--surface, #fff)',
  border: '1px solid var(--border, #e5e7eb)',
  borderRadius: 8,
  overflow: 'hidden',
  cursor: 'pointer',
  textAlign: 'left',
  padding: 0,
  font: 'inherit',
  color: 'inherit',
};

const imageWrapStyle: CSSProperties = {
  width: '100%',
  aspectRatio: '1 / 1',
  background: '#f3f4f6',
  display: 'block',
};

const imageStyle: CSSProperties = {
  width: '100%',
  height: '100%',
  objectFit: 'cover',
  display: 'block',
};

const bodyStyle: CSSProperties = {
  padding: 8,
  display: 'flex',
  flexDirection: 'column',
  gap: 2,
  color: '#111111',
};

const nameStyle: CSSProperties = {
  fontWeight: 600,
  fontSize: 11,
  lineHeight: 1.2,
  color: '#111111',
  whiteSpace: 'nowrap',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
};

const priceStyle: CSSProperties = {
  fontSize: 10,
  fontWeight: 500,
  color: '#111111',
};

const skuStyle: CSSProperties = {
  fontSize: 9,
  color: '#111111',
  opacity: 0.85,
};

function formatPrice(price?: number, currency?: string): string {
  if (price == null || Number.isNaN(price)) return '--';
  return `${currency ?? 'ARS'} ${Number(price).toFixed(0)}`;
}

function compactText(value: string | undefined, max: number): string {
  const text = (value ?? '').trim();
  if (text.length <= max) return text;
  return `${text.slice(0, Math.max(0, max - 1)).trimEnd()}…`;
}

function cardImageSrc(item: ProductGalleryItem): string {
  const fromList = item.image_urls?.map((u) => u.trim()).filter(Boolean) ?? [];
  if (fromList.length > 0) return fromList[0]!;
  const one = item.image_url?.trim();
  return one ?? '';
}

export function ProductGalleryView<T extends ProductGalleryItem>({
  items,
  onSelect,
  loading,
  emptyLabel = 'Sin productos para mostrar.',
  loadingLabel = 'Cargando...',
}: ProductGalleryViewProps<T>) {
  if (loading) {
    return <div className="empty-state"><p>{loadingLabel}</p></div>;
  }
  if (items.length === 0) {
    return <div className="empty-state"><p>{emptyLabel}</p></div>;
  }
  return (
    <div style={gridStyle} role="list" aria-label="Galería de productos">
      {items.map((item) => (
        <button
          type="button"
          key={item.id}
          style={cardStyle}
          onClick={() => onSelect(item.id)}
          role="listitem"
          aria-label={item.name}
        >
          <span style={imageWrapStyle}>
            <img
              src={cardImageSrc(item) || PLACEHOLDER_IMAGE}
              alt={item.name}
              style={imageStyle}
              loading="lazy"
              onError={(e) => {
                (e.currentTarget as HTMLImageElement).src = PLACEHOLDER_IMAGE;
              }}
            />
          </span>
          <span style={bodyStyle}>
            <span style={nameStyle} title={item.name}>{compactText(item.name, 26)}</span>
            <span style={priceStyle}>{formatPrice(item.price, item.currency)}</span>
            <span style={skuStyle}>{item.sku ? compactText(item.sku, 18) : 'Sin SKU'}</span>
          </span>
        </button>
      ))}
    </div>
  );
}
