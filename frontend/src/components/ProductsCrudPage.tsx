/**
 * Productos: CRUD genérico con toggle de modo de visualización (tabla / galería).
 * El toggle va en la cabecera del CRUD (`listHeaderInlineSlot` + `listHeaderSlotPlacement: 'aboveTitle'`),
 * encima del título de página.
 * El modo se controla por query string ?view=gallery|table (default: table).
 */
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';
import '../styles/viewModeSegmentedSwitch.css';
import './ProductsCrudPage.css';
import { ProductGalleryView, type ProductGalleryItem } from './ProductGalleryView';

type ProductRow = ProductGalleryItem & { deleted_at?: string | null };

type ProductsListResponse = { items: ProductRow[] };

type ViewMode = 'table' | 'gallery';

function readViewMode(value: string | null): ViewMode {
  return value === 'gallery' ? 'gallery' : 'table';
}

function ViewModeToggle({
  mode,
  onChange,
  labels,
  ariaLabel,
}: {
  mode: ViewMode;
  onChange: (next: ViewMode) => void;
  labels: { table: string; gallery: string };
  ariaLabel: string;
}) {
  const isTable = mode === 'table';
  return (
    <div className="m-seg-switch" role="group" aria-label={ariaLabel}>
      <div className="m-seg-switch__track" role="presentation">
        <button
          type="button"
          className={`m-seg-switch__label${isTable ? ' m-seg-switch__label--active' : ''}`}
          aria-pressed={isTable}
          onClick={() => onChange('table')}
        >
          {labels.table}
        </button>
        <button
          type="button"
          className={`m-seg-switch__label${!isTable ? ' m-seg-switch__label--active' : ''}`}
          aria-pressed={!isTable}
          onClick={() => onChange('gallery')}
        >
          {labels.gallery}
        </button>
        <span
          className={`m-seg-switch__thumb${isTable ? ' m-seg-switch__thumb--left' : ' m-seg-switch__thumb--right'}`}
          aria-hidden
        />
      </div>
    </div>
  );
}

function ProductsGallery({ onSelect }: { onSelect: (id: string) => void }) {
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
    return <div className="empty-state"><p>{error}</p></div>;
  }
  return (
    <ProductGalleryView
      items={items}
      onSelect={onSelect}
      loading={loading}
      emptyLabel={t('crud.viewMode.gallery.empty')}
      loadingLabel={t('crud.viewMode.gallery.loading')}
    />
  );
}

export function ProductsCrudPage() {
  const { t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();
  const mode = readViewMode(searchParams.get('view'));

  const setMode = useCallback(
    (next: ViewMode) => {
      setSearchParams(
        (prev) => {
          const p = new URLSearchParams(prev);
          if (next === 'table') p.delete('view');
          else p.set('view', next);
          return p;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const labels = useMemo(
    () => ({ table: t('crud.viewMode.table'), gallery: t('crud.viewMode.gallery') }),
    [t],
  );
  const ariaViewMode = t('crud.viewMode.aria');

  const mergeConfig = useMemo(
    () => ({
      listHeaderSlotPlacement: 'aboveTitle' as const,
      listHeaderInlineSlot: () => (
        <ViewModeToggle mode="table" onChange={setMode} labels={labels} ariaLabel={ariaViewMode} />
      ),
    }),
    [ariaViewMode, labels, setMode],
  );

  if (mode === 'gallery') {
    // En modo galería usamos un layout propio: toggle arriba + grid de cards.
    return (
      <div className="products-crud-page">
        <div className="products-crud-page__toolbar">
          <ViewModeToggle mode={mode} onChange={setMode} labels={labels} ariaLabel={ariaViewMode} />
        </div>
        <ProductsGallery onSelect={() => setMode('table')} />
      </div>
    );
  }
  return <LazyConfiguredCrudPage resourceId="products" mergeConfig={mergeConfig} />;
}
