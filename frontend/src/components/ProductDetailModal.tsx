import { useCallback, useEffect, useMemo, useState } from 'react';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';
import { collectCrudImageUrls, CrudEntityDetailModal, CrudImageFullscreenViewer } from '../modules/crud';
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

/** URLs de galería del producto (sin duplicados). Reutilizable en inventario y otros modales. */
export function collectProductImageUrls(
  p: Pick<ProductDetailResponse, 'image_url' | 'image_urls'>,
): string[] {
  return collectCrudImageUrls({
    imageUrls: p.image_urls,
    legacyImageUrl: p.image_url,
  });
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
  const [lightboxUrl, setLightboxUrl] = useState<string | null>(null);

  const urls = useMemo(() => (product ? collectProductImageUrls(product) : []), [product]);

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
    setLightboxUrl(null);
  }, [productId]);

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

  const fmtMoney = (n: number | undefined, cur: string | undefined) =>
    `${cur ?? 'ARS'} ${Number(n ?? 0).toFixed(2)}`;

  const detailFields = product
    ? [
        { id: 'sku', label: t('crud.products.detail.sku'), value: product.sku?.trim() || '—' },
        { id: 'unit', label: t('crud.products.detail.unit'), value: product.unit?.trim() || '—' },
        { id: 'price', label: t('crud.products.detail.price'), value: fmtMoney(product.price, product.currency) },
        { id: 'cost', label: t('crud.products.detail.cost'), value: fmtMoney(product.cost_price, product.currency) },
        { id: 'tax', label: t('crud.products.detail.tax'), value: product.tax_rate != null ? `${product.tax_rate}%` : '—' },
        {
          id: 'stock',
          label: t('crud.products.detail.stock'),
          value: product.track_stock ? t('crud.products.detail.stockOn') : t('crud.products.detail.stockOff'),
        },
        {
          id: 'active',
          label: t('crud.products.detail.active'),
          value: product.is_active ? t('crud.products.detail.activeYes') : t('crud.products.detail.activeNo'),
        },
        {
          id: 'tags',
          label: t('crud.products.detail.tags'),
          value: product.tags?.length ? product.tags.join(', ') : '—',
          fullWidth: true,
        },
        {
          id: 'description',
          label: t('crud.products.detail.description'),
          value: product.description?.trim() || '—',
          fullWidth: true,
          valueClassName: 'product-detail-modal__description',
        },
        ...(product.metadata && Object.keys(product.metadata).length > 0
          ? [
              {
                id: 'metadata',
                label: t('crud.products.detail.metadata'),
                value: <pre className="product-detail-modal__metadata">{JSON.stringify(product.metadata, null, 2)}</pre>,
                fullWidth: true,
              },
            ]
          : []),
      ]
    : [];

  const body = (
    <CrudEntityDetailModal
      open
      titleId="product-detail-title"
      title={loading ? t('crud.products.detail.loading') : product?.name ?? '—'}
      onClose={onClose}
      closeLabel={t('crud.products.detail.close')}
      panelClassName="product-detail-modal"
      media={
        urls.length > 0 ? (
          <div className="product-detail-modal__media">
            <div className="product-detail-modal__main-image-wrap">
              <img
                src={urls[Math.min(slide, urls.length - 1)]!}
                alt=""
                className="product-detail-modal__main-image product-detail-modal__main-image--zoomable"
                onClick={() => setLightboxUrl(urls[Math.min(slide, urls.length - 1)]!)}
                onError={(e) => {
                  const media = (e.currentTarget as HTMLImageElement).closest('.product-detail-modal__media');
                  if (media) (media as HTMLElement).hidden = true;
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
                    onDoubleClick={(ev) => {
                      ev.preventDefault();
                      setLightboxUrl(u);
                    }}
                    title={t('crud.products.detail.thumbDblClickZoom')}
                  >
                    <img
                      src={u}
                      alt=""
                      loading="lazy"
                      onError={(ev) => {
                        const btn = (ev.currentTarget as HTMLImageElement).closest('button');
                        if (btn) btn.hidden = true;
                      }}
                    />
                  </button>
                ))}
              </div>
            ) : null}
          </div>
        ) : null
      }
      fields={detailFields}
      error={error}
      loading={loading}
      loadingLabel={t('crud.products.detail.loading')}
    />
  );

  return (
    <>
      {body}
      <CrudImageFullscreenViewer
        imageUrl={lightboxUrl}
        onClose={() => setLightboxUrl(null)}
        contentLabel={product?.name}
      />
    </>
  );
}
