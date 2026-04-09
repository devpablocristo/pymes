import { useCallback, useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';
import './ProductDetailModal.css';

export type ProductDetailResponse = {
  id: string;
  name: string;
  description?: string;
  sku?: string;
  unit?: string;
  price?: number;
  currency?: string;
  cost_price?: number;
  tax_rate?: number | null;
  image_url?: string;
  image_urls?: string[];
  track_stock?: boolean;
  is_active?: boolean;
  tags?: string[];
  metadata?: Record<string, unknown>;
  created_at?: string;
  updated_at?: string;
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

function collectImageUrls(p: ProductDetailResponse): string[] {
  const raw = p.image_urls?.length ? p.image_urls : p.image_url ? [p.image_url] : [];
  const out: string[] = [];
  const seen = new Set<string>();
  for (const u of raw) {
    const t = (u ?? '').trim();
    if (!t || seen.has(t)) continue;
    seen.add(t);
    out.push(t);
  }
  return out;
}

export type ProductDetailModalProps = {
  productId: string | null;
  onClose: () => void;
};

export function ProductDetailModal({ productId, onClose }: ProductDetailModalProps) {
  const { t } = useI18n();
  const [product, setProduct] = useState<ProductDetailResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [slide, setSlide] = useState(0);

  const urls = useMemo(() => (product ? collectImageUrls(product) : []), [product]);

  useEffect(() => {
    if (!productId) {
      setProduct(null);
      setError(null);
      setSlide(0);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    setSlide(0);
    apiRequest<ProductDetailResponse>(`/v1/products/${productId}`)
      .then((data) => {
        if (cancelled) return;
        setProduct(data);
      })
      .catch(() => {
        if (cancelled) return;
        setProduct(null);
        setError(t('crud.products.detail.loadError'));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [productId, t]);

  useEffect(() => {
    if (!productId) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [productId, onClose]);

  const goPrev = useCallback(() => {
    setSlide((i) => (urls.length ? (i - 1 + urls.length) % urls.length : 0));
  }, [urls.length]);

  const goNext = useCallback(() => {
    setSlide((i) => (urls.length ? (i + 1) % urls.length : 0));
  }, [urls.length]);

  if (!productId) return null;

  const mainSrc = urls.length ? urls[Math.min(slide, urls.length - 1)]! : PLACEHOLDER_IMAGE;
  const fmtMoney = (n: number | undefined, cur: string | undefined) =>
    `${cur ?? 'ARS'} ${Number(n ?? 0).toFixed(2)}`;

  const body = (
    <div className="product-detail-modal-root">
      <button
        type="button"
        className="product-detail-modal__backdrop"
        aria-label={t('crud.products.detail.close')}
        onClick={onClose}
      />
      <div
        className="product-detail-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="product-detail-title"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="product-detail-modal__header">
          <h2 id="product-detail-title" className="product-detail-modal__title">
            {loading ? t('crud.products.detail.loading') : product?.name ?? '—'}
          </h2>
          <button type="button" className="product-detail-modal__close" onClick={onClose}>
            {t('crud.products.detail.close')}
          </button>
        </header>
        <div className="product-detail-modal__body">
          {error ? (
            <p className="product-detail-modal__error">{error}</p>
          ) : (
            <>
              <div className="product-detail-modal__media">
                <div className="product-detail-modal__main-image-wrap">
                  <img
                    src={mainSrc}
                    alt=""
                    className="product-detail-modal__main-image"
                    onError={(e) => {
                      (e.currentTarget as HTMLImageElement).src = PLACEHOLDER_IMAGE;
                    }}
                  />
                  {urls.length > 1 ? (
                    <>
                      <button
                        type="button"
                        className="product-detail-modal__nav product-detail-modal__nav--prev"
                        onClick={goPrev}
                        aria-label={t('crud.products.detail.prevImage')}
                      >
                        ‹
                      </button>
                      <button
                        type="button"
                        className="product-detail-modal__nav product-detail-modal__nav--next"
                        onClick={goNext}
                        aria-label={t('crud.products.detail.nextImage')}
                      >
                        ›
                      </button>
                      <span className="product-detail-modal__counter" aria-live="polite">
                        {slide + 1} / {urls.length}
                      </span>
                    </>
                  ) : null}
                </div>
                {urls.length > 1 ? (
                  <div className="product-detail-modal__thumbs" role="tablist" aria-label={t('crud.products.detail.thumbsAria')}>
                    {urls.map((u, idx) => (
                      <button
                        key={u + String(idx)}
                        type="button"
                        role="tab"
                        aria-selected={idx === slide}
                        className={`product-detail-modal__thumb${idx === slide ? ' product-detail-modal__thumb--active' : ''}`}
                        onClick={() => setSlide(idx)}
                      >
                        <img src={u} alt="" loading="lazy" onError={(e) => { (e.currentTarget as HTMLImageElement).src = PLACEHOLDER_IMAGE; }} />
                      </button>
                    ))}
                  </div>
                ) : null}
              </div>
              {product && !loading ? (
                <dl className="product-detail-modal__fields">
                  <div className="product-detail-modal__row">
                    <dt>{t('crud.products.detail.sku')}</dt>
                    <dd>{product.sku?.trim() || '—'}</dd>
                  </div>
                  <div className="product-detail-modal__row">
                    <dt>{t('crud.products.detail.unit')}</dt>
                    <dd>{product.unit?.trim() || '—'}</dd>
                  </div>
                  <div className="product-detail-modal__row">
                    <dt>{t('crud.products.detail.price')}</dt>
                    <dd>{fmtMoney(product.price, product.currency)}</dd>
                  </div>
                  <div className="product-detail-modal__row">
                    <dt>{t('crud.products.detail.cost')}</dt>
                    <dd>{fmtMoney(product.cost_price, product.currency)}</dd>
                  </div>
                  <div className="product-detail-modal__row">
                    <dt>{t('crud.products.detail.tax')}</dt>
                    <dd>{product.tax_rate != null ? `${product.tax_rate}%` : '—'}</dd>
                  </div>
                  <div className="product-detail-modal__row">
                    <dt>{t('crud.products.detail.stock')}</dt>
                    <dd>{product.track_stock ? t('crud.products.detail.stockOn') : t('crud.products.detail.stockOff')}</dd>
                  </div>
                  <div className="product-detail-modal__row">
                    <dt>{t('crud.products.detail.active')}</dt>
                    <dd>{product.is_active ? t('crud.products.detail.activeYes') : t('crud.products.detail.activeNo')}</dd>
                  </div>
                  <div className="product-detail-modal__row product-detail-modal__row--full">
                    <dt>{t('crud.products.detail.tags')}</dt>
                    <dd>{product.tags?.length ? product.tags.join(', ') : '—'}</dd>
                  </div>
                  <div className="product-detail-modal__row product-detail-modal__row--full">
                    <dt>{t('crud.products.detail.description')}</dt>
                    <dd className="product-detail-modal__description">{product.description?.trim() || '—'}</dd>
                  </div>
                  {product.metadata && Object.keys(product.metadata).length > 0 ? (
                    <div className="product-detail-modal__row product-detail-modal__row--full">
                      <dt>{t('crud.products.detail.metadata')}</dt>
                      <dd>
                        <pre className="product-detail-modal__metadata">{JSON.stringify(product.metadata, null, 2)}</pre>
                      </dd>
                    </div>
                  ) : null}
                </dl>
              ) : loading ? (
                <p className="product-detail-modal__loading">{t('crud.products.detail.loading')}</p>
              ) : null}
            </>
          )}
        </div>
      </div>
    </div>
  );

  return createPortal(body, document.body);
}
