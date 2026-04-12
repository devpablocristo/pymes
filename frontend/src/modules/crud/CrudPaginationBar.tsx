import { useI18n } from '../../lib/i18n';
import './CrudPaginationBar.css';

type CrudPaginationBarProps = {
  visibleCount: number;
  totalCount?: number | null;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
  hidden?: boolean;
};

export function CrudPaginationBar({
  visibleCount,
  totalCount,
  hasMore = false,
  loadingMore = false,
  onLoadMore,
  hidden = false,
}: CrudPaginationBarProps) {
  const { t } = useI18n();

  if (hidden) return null;

  const resolvedTotal = Math.max(visibleCount, Number(totalCount ?? visibleCount) || 0);
  if (resolvedTotal <= 0 && !hasMore) return null;

  return (
    <div className="crud-pagination-bar">
      <div className="crud-pagination-bar__meta">
        Mostrando {visibleCount} de {resolvedTotal}
      </div>
      {hasMore && onLoadMore ? (
        <div className="crud-pagination-bar__actions">
          <button
            type="button"
            className="btn-secondary"
            disabled={loadingMore}
            onClick={onLoadMore}
          >
            {loadingMore ? t('crud.viewMode.gallery.loading') : t('crud.loadMore')}
          </button>
        </div>
      ) : null}
    </div>
  );
}
