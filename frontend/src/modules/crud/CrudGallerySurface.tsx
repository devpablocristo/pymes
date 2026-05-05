import type { CSSProperties } from 'react';

import { CrudEntityMediaCarousel } from './CrudEntityMediaCarousel';
import { isDisplayableCrudImageSrc } from './crudLinkedEntityImageUrls';

export type CrudGalleryCard<T extends { id: string }> = {
  title: (item: T) => string;
  subtitle?: (item: T) => string;
  meta?: (item: T) => string;
  /** URLs para el mismo carrusel que el modal CRUD (varias por ítem). */
  imageUrls?: (item: T) => string[] | undefined;
  /** Legacy: una sola URL; si hay `imageUrls`, tiene prioridad. */
  imageSrc?: (item: T) => string | undefined;
  imageAlt?: (item: T) => string;
};

export type CrudGallerySurfaceProps<T extends { id: string }> = {
  items: T[];
  onSelect: (item: T) => void;
  loading?: boolean;
  emptyLabel?: string;
  loadingLabel?: string;
  ariaLabel?: string;
  card: CrudGalleryCard<T>;
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
  gap: 'var(--space-4)',
  width: '100%',
};

const cardStyle: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius-sm)',
  overflow: 'hidden',
  cursor: 'pointer',
  textAlign: 'left',
  padding: 0,
  fontFamily: 'var(--crud-ui-font-family)',
  fontSize: 'var(--crud-ui-font-size)',
  color: 'inherit',
};

const imageWrapStyle: CSSProperties = {
  width: '100%',
  aspectRatio: '1 / 1',
  background: 'var(--color-border-subtle)',
  display: 'block',
};

const carouselSlotStyle: CSSProperties = {
  width: '100%',
  height: '100%',
  display: 'block',
};

const imageStyle: CSSProperties = {
  width: '100%',
  height: '100%',
  objectFit: 'cover',
  display: 'block',
};

const bodyStyle: CSSProperties = {
  padding: 'var(--space-2)',
  display: 'flex',
  flexDirection: 'column',
  gap: 'var(--space-1)',
  color: 'var(--color-text)',
};

const titleStyle: CSSProperties = {
  fontWeight: 600,
  fontSize: 'var(--crud-ui-font-size)',
  lineHeight: 1.2,
  color: 'var(--color-text)',
  whiteSpace: 'nowrap',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
};

const subtitleStyle: CSSProperties = {
  fontSize: 'var(--crud-ui-font-size)',
  fontWeight: 500,
  color: 'var(--color-text-secondary)',
};

const metaStyle: CSSProperties = {
  fontSize: 'var(--crud-ui-font-size)',
  fontWeight: 500,
  color: 'var(--color-text-secondary)',
};

function compactText(value: string | undefined, max: number): string {
  const text = (value ?? '').trim();
  if (text.length <= max) return text;
  return `${text.slice(0, Math.max(0, max - 1)).trimEnd()}…`;
}

export function CrudGallerySurface<T extends { id: string }>({
  items,
  onSelect,
  loading,
  emptyLabel = 'Sin registros para mostrar.',
  loadingLabel = 'Cargando...',
  ariaLabel = 'Galería',
  card,
}: CrudGallerySurfaceProps<T>) {
  if (loading) {
    return <div className="empty-state"><p>{loadingLabel}</p></div>;
  }
  if (items.length === 0) {
    return <div className="empty-state"><p>{emptyLabel}</p></div>;
  }
  return (
    <div style={gridStyle} role="list" aria-label={ariaLabel}>
      {items.map((item) => {
        const title = card.title(item);
        const subtitle = card.subtitle?.(item);
        const meta = card.meta?.(item);
        const fromList = card.imageUrls?.(item)?.filter(isDisplayableCrudImageSrc) ?? [];
        const legacy = card.imageSrc?.(item)?.trim();
        const displayUrls =
          fromList.length > 0 ? fromList : legacy && isDisplayableCrudImageSrc(legacy) ? [legacy] : [];
        const imageAlt = card.imageAlt?.(item) ?? title;
        return (
          <button
            type="button"
            key={item.id}
            style={cardStyle}
            onClick={() => onSelect(item)}
            role="listitem"
            aria-label={title}
          >
            <span style={imageWrapStyle}>
              {displayUrls.length > 0 ? (
                <span style={carouselSlotStyle}>
                  <CrudEntityMediaCarousel
                    urls={displayUrls}
                    variant="read"
                    compact
                    containInteractiveEvents
                    ariaLabel={imageAlt}
                  />
                </span>
              ) : (
                <img src={PLACEHOLDER_IMAGE} alt={imageAlt} style={imageStyle} loading="lazy" />
              )}
            </span>
            <span style={bodyStyle}>
              <span style={titleStyle} title={title}>{compactText(title, 26)}</span>
              {subtitle ? <span style={subtitleStyle}>{compactText(subtitle, 24)}</span> : null}
              {meta ? <span style={metaStyle}>{compactText(meta, 18)}</span> : null}
            </span>
          </button>
        );
      })}
    </div>
  );
}
