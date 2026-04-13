import { useMemo, useState } from 'react';
import './CrudEntityMediaCarousel.css';

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ');
}

export type CrudEntityMediaCarouselProps = {
  urls: string[];
  variant?: 'read' | 'edit';
  ariaLabel?: string;
};

export function CrudEntityMediaCarousel({
  urls,
  variant = 'read',
  ariaLabel = 'Imágenes',
}: CrudEntityMediaCarouselProps) {
  const safeUrls = useMemo(() => urls.filter(Boolean), [urls]);
  const [index, setIndex] = useState(0);

  if (!safeUrls.length) return null;

  const activeIndex = Math.min(index, safeUrls.length - 1);
  const activeUrl = safeUrls[activeIndex];

  return (
    <section className={cx('crud-entity-media-carousel', `crud-entity-media-carousel--${variant}`)} aria-label={ariaLabel}>
      <div className="crud-entity-media-carousel__hero">
        <img src={activeUrl} alt="" loading="lazy" />
        {safeUrls.length > 1 ? (
          <>
            <button
              type="button"
              className="crud-entity-media-carousel__nav crud-entity-media-carousel__nav--prev"
              onClick={() => setIndex((current) => (current - 1 + safeUrls.length) % safeUrls.length)}
              aria-label="Imagen anterior"
            >
              ‹
            </button>
            <button
              type="button"
              className="crud-entity-media-carousel__nav crud-entity-media-carousel__nav--next"
              onClick={() => setIndex((current) => (current + 1) % safeUrls.length)}
              aria-label="Imagen siguiente"
            >
              ›
            </button>
            <span className="crud-entity-media-carousel__counter">
              {activeIndex + 1} / {safeUrls.length}
            </span>
          </>
        ) : null}
      </div>
      {safeUrls.length > 1 ? (
        <div className="crud-entity-media-carousel__thumbs" role="tablist" aria-label="Miniaturas">
          {safeUrls.map((url, thumbIndex) => (
            <button
              key={`${url}-${thumbIndex}`}
              type="button"
              className={cx(
                'crud-entity-media-carousel__thumb',
                thumbIndex === activeIndex && 'crud-entity-media-carousel__thumb--active',
              )}
              aria-selected={thumbIndex === activeIndex}
              onClick={() => setIndex(thumbIndex)}
            >
              <img src={url} alt="" loading="lazy" />
            </button>
          ))}
        </div>
      ) : null}
    </section>
  );
}
