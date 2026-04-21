import { useMemo, useState } from 'react';
import './CrudEntityMediaCarousel.css';

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ');
}

export type CrudEntityMediaCarouselProps = {
  urls: string[];
  variant?: 'read' | 'edit';
  ariaLabel?: string;
  activeIndex?: number;
  onActiveIndexChange?: (nextIndex: number) => void;
  onRemoveAt?: (index: number) => void;
};

export function CrudEntityMediaCarousel({
  urls,
  variant = 'read',
  ariaLabel = 'Imágenes',
  activeIndex,
  onActiveIndexChange,
  onRemoveAt,
}: CrudEntityMediaCarouselProps) {
  const safeUrls = useMemo(() => urls.filter(Boolean), [urls]);
  const [internalIndex, setInternalIndex] = useState(0);

  if (!safeUrls.length) return null;

  const resolvedIndex = Math.min(activeIndex ?? internalIndex, safeUrls.length - 1);
  const activeUrl = safeUrls[resolvedIndex];
  const setIndex = (nextIndex: number) => {
    onActiveIndexChange?.(nextIndex);
    if (activeIndex === undefined) {
      setInternalIndex(nextIndex);
    }
  };

  return (
    <section className={cx('crud-entity-media-carousel', `crud-entity-media-carousel--${variant}`)} aria-label={ariaLabel}>
      <div className="crud-entity-media-carousel__hero">
        <img src={activeUrl} alt="" loading="lazy" />
        {safeUrls.length > 1 ? (
          <>
            <button
              type="button"
              className="crud-entity-media-carousel__nav crud-entity-media-carousel__nav--prev"
              onClick={() => setIndex((resolvedIndex - 1 + safeUrls.length) % safeUrls.length)}
              aria-label="Imagen anterior"
            >
              ‹
            </button>
            <button
              type="button"
              className="crud-entity-media-carousel__nav crud-entity-media-carousel__nav--next"
              onClick={() => setIndex((resolvedIndex + 1) % safeUrls.length)}
              aria-label="Imagen siguiente"
            >
              ›
            </button>
            <span className="crud-entity-media-carousel__counter">
              {resolvedIndex + 1} / {safeUrls.length}
            </span>
          </>
        ) : null}
      </div>
      {safeUrls.length > 1 ? (
        <div className="crud-entity-media-carousel__thumbs" role="tablist" aria-label="Miniaturas">
          {safeUrls.map((url, thumbIndex) => (
            <div key={`${url}-${thumbIndex}`} className="crud-entity-media-carousel__thumb-item">
              <button
                type="button"
                className={cx(
                  'crud-entity-media-carousel__thumb',
                  thumbIndex === resolvedIndex && 'crud-entity-media-carousel__thumb--active',
                )}
                aria-selected={thumbIndex === resolvedIndex}
                onClick={() => setIndex(thumbIndex)}
              >
                <img src={url} alt="" loading="lazy" />
              </button>
              {onRemoveAt ? (
                <button
                  type="button"
                  className="crud-entity-media-carousel__thumb-remove"
                  aria-label={`Eliminar imagen ${thumbIndex + 1}`}
                  onClick={() => onRemoveAt(thumbIndex)}
                >
                  ×
                </button>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}
