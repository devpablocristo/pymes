import { useMemo, useState } from 'react';
import { isDisplayableCrudImageSrc } from './crudLinkedEntityImageUrls';
import './CrudEntityMediaCarousel.css';

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ');
}

export type CrudEntityMediaCarouselProps = {
  urls: string[];
  variant?: 'read' | 'edit';
  ariaLabel?: string;
  /** En edición: quitar una URL del listado (p. ej. sincronizar con el campo del formulario). */
  onRequestRemoveAt?: (index: number) => void;
};

export function CrudEntityMediaCarousel({
  urls,
  variant = 'read',
  ariaLabel = 'Imágenes',
  onRequestRemoveAt,
}: CrudEntityMediaCarouselProps) {
  const safeUrls = useMemo(() => urls.filter(Boolean), [urls]);
  const [index, setIndex] = useState(0);

  if (!safeUrls.length) return null;

  const activeIndex = Math.min(index, safeUrls.length - 1);
  const activeUrl = safeUrls[activeIndex];
  const renderHeroSlide = (url: string) =>
    isDisplayableCrudImageSrc(url) ? (
      <img src={url} alt="" loading="lazy" />
    ) : (
      <span className="crud-entity-media-carousel__invalid-src" title={url}>
        Sin vista previa (pegá una URL https://… o elegí imágenes locales).
      </span>
    );

  const renderThumbSlide = (url: string) =>
    isDisplayableCrudImageSrc(url) ? (
      <img src={url} alt="" loading="lazy" />
    ) : (
      <span className="crud-entity-media-carousel__thumb-invalid" title={url} aria-hidden>
        …
      </span>
    );

  return (
    <section className={cx('crud-entity-media-carousel', `crud-entity-media-carousel--${variant}`)} aria-label={ariaLabel}>
      <div className="crud-entity-media-carousel__hero">
        {renderHeroSlide(activeUrl)}
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
      {safeUrls.length > 0 ? (
        <div className="crud-entity-media-carousel__thumbs" role="tablist" aria-label="Miniaturas">
          {safeUrls.map((url, thumbIndex) => (
            <div key={`${url}-${thumbIndex}`} className="crud-entity-media-carousel__thumb-wrap">
              <button
                type="button"
                className={cx(
                  'crud-entity-media-carousel__thumb',
                  thumbIndex === activeIndex && 'crud-entity-media-carousel__thumb--active',
                )}
                aria-selected={thumbIndex === activeIndex}
                onClick={() => setIndex(thumbIndex)}
              >
                {renderThumbSlide(url)}
              </button>
              {onRequestRemoveAt ? (
                <button
                  type="button"
                  className="crud-entity-media-carousel__thumb-remove"
                  aria-label={`Quitar imagen ${thumbIndex + 1}`}
                  onClick={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    onRequestRemoveAt(thumbIndex);
                  }}
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
