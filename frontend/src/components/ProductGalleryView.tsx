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
  padding: 12,
  display: 'flex',
  flexDirection: 'column',
  gap: 4,
};

const nameStyle: CSSProperties = {
  fontWeight: 600,
  fontSize: 14,
  lineHeight: 1.3,
  whiteSpace: 'nowrap',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
};

const priceStyle: CSSProperties = {
  fontSize: 14,
  fontWeight: 500,
  color: 'var(--text-primary, #111827)',
};

const skuStyle: CSSProperties = {
  fontSize: 11,
  color: 'var(--text-secondary, #6b7280)',
};

function formatPrice(price?: number, currency?: string): string {
  if (price == null || Number.isNaN(price)) return '--';
  return `${currency ?? 'ARS'} ${Number(price).toFixed(2)}`;
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
              src={item.image_url || PLACEHOLDER_IMAGE}
              alt={item.name}
              style={imageStyle}
              loading="lazy"
              onError={(e) => {
                (e.currentTarget as HTMLImageElement).src = PLACEHOLDER_IMAGE;
              }}
            />
          </span>
          <span style={bodyStyle}>
            <span style={nameStyle} title={item.name}>{item.name}</span>
            <span style={priceStyle}>{formatPrice(item.price, item.currency)}</span>
            <span style={skuStyle}>{item.sku ? `SKU: ${item.sku}` : 'Sin SKU'}</span>
          </span>
        </button>
      ))}
    </div>
  );
}
