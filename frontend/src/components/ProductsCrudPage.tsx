/**
 * Productos: CRUD genérico con toggle de modo de visualización (tabla / galería).
 * El toggle se inyecta dentro del header de la lista del CRUD via listHeaderInlineSlot,
 * para no romper el orden visual del PageLayout.
 * El modo se controla por query string ?view=gallery|table (default: table).
 */
import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';
import { apiRequest } from '../lib/api';
import { useI18n } from '../lib/i18n';
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
}: {
  mode: ViewMode;
  onChange: (next: ViewMode) => void;
  labels: { table: string; gallery: string };
}) {
  const baseStyle = {
    padding: '6px 12px',
    fontSize: 13,
    cursor: 'pointer',
    border: '1px solid var(--border, #d1d5db)',
    background: 'var(--surface, #fff)',
  } as const;
  const activeStyle = { background: 'var(--primary, #2563eb)', color: '#fff', borderColor: 'var(--primary, #2563eb)' } as const;
  return (
    <div role="group" aria-label="Modo de vista" style={{ display: 'inline-flex', gap: 0 }}>
      <button
        type="button"
        style={{ ...baseStyle, borderRadius: '6px 0 0 6px', ...(mode === 'table' ? activeStyle : null) }}
        aria-pressed={mode === 'table'}
        onClick={() => onChange('table')}
      >
        {labels.table}
      </button>
      <button
        type="button"
        style={{ ...baseStyle, borderRadius: '0 6px 6px 0', borderLeft: 'none', ...(mode === 'gallery' ? activeStyle : null) }}
        aria-pressed={mode === 'gallery'}
        onClick={() => onChange('gallery')}
      >
        {labels.gallery}
      </button>
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

  const setMode = (next: ViewMode) => {
    setSearchParams(
      (prev) => {
        const p = new URLSearchParams(prev);
        if (next === 'table') p.delete('view');
        else p.set('view', next);
        return p;
      },
      { replace: true },
    );
  };

  const labels = { table: t('crud.viewMode.table'), gallery: t('crud.viewMode.gallery') };

  const mergeConfig = useMemo(
    () => ({
      listHeaderInlineSlot: () => <ViewModeToggle mode="table" onChange={setMode} labels={labels} />,
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [labels.table, labels.gallery],
  );

  if (mode === 'gallery') {
    // En modo galería usamos un layout propio: toggle arriba + grid de cards.
    return (
      <div className="products-crud-page" style={{ padding: '24px 32px' }}>
        <div style={{ marginBottom: 16 }}>
          <ViewModeToggle mode={mode} onChange={setMode} labels={labels} />
        </div>
        <ProductsGallery onSelect={() => setMode('table')} />
      </div>
    );
  }
  return <LazyConfiguredCrudPage resourceId="products" mergeConfig={mergeConfig} />;
}
