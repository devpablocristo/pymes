import { CrudPageShell } from '@devpablocristo/core-browser/crud';
import { search as fuzzySearch, type SearchEntry } from '@devpablocristo/core-browser/search';
import { useDeferredValue, useEffect, useMemo, useState, type ReactNode } from 'react';
import './CrudExplorerPage.css';

export type CrudExplorerViewMode = 'list' | 'table-detail' | 'gallery' | 'kanban';

export type CrudExplorerMetric<T> = {
  id: string;
  label: string;
  value: string;
  tone?: 'default' | 'success' | 'warning' | 'danger';
  helper?: string;
  isVisible?: (items: T[]) => boolean;
};

export type CrudExplorerFilter<T> = {
  id: string;
  label: string;
  predicate: (row: T) => boolean;
};

export type CrudExplorerColumn<T> = {
  id: string;
  header: string;
  className?: string;
  render: (row: T) => ReactNode;
};

export type CrudExplorerRowAction<T> = {
  id: string;
  label: string;
  kind?: 'primary' | 'secondary' | 'danger' | 'success';
  onClick: (row: T) => void | Promise<void>;
  isVisible?: (row: T) => boolean;
};

export type CrudExplorerToolbarAction<T> = {
  id: string;
  label: string;
  kind?: 'primary' | 'secondary' | 'danger' | 'success';
  onClick: (ctx: { items: T[]; selectedItem: T | null }) => void | Promise<void>;
  isVisible?: (ctx: { items: T[]; selectedItem: T | null }) => boolean;
};

export type CrudExplorerView<T> = {
  id: CrudExplorerViewMode;
  label: string;
  render?: (ctx: { items: T[]; selectedItem: T | null; onSelect: (row: T | null) => void }) => ReactNode;
};

export type CrudExplorerPageProps<T extends { id: string }> = {
  title: string;
  singularLabel: string;
  pluralLabel: string;
  items: T[];
  loading?: boolean;
  error?: ReactNode;
  searchPlaceholder?: string;
  searchValue?: string;
  onSearchValueChange?: (value: string) => void;
  searchText: (row: T) => string;
  emptyState: string;
  loadingLabel?: string;
  metrics?: CrudExplorerMetric<T>[];
  filters?: CrudExplorerFilter<T>[];
  columns?: CrudExplorerColumn<T>[];
  rowActions?: CrudExplorerRowAction<T>[];
  toolbarActions?: CrudExplorerToolbarAction<T>[];
  viewModes?: CrudExplorerView<T>[];
  initialViewMode?: CrudExplorerViewMode;
  initialSelectedId?: string | null;
  detailTitle?: string;
  detailEmptyState?: string;
  renderDetail?: (row: T) => ReactNode;
  headerLeadSlot?: ReactNode;
};

function buttonClass(kind: 'primary' | 'secondary' | 'danger' | 'success' = 'secondary'): string {
  switch (kind) {
    case 'primary':
      return 'btn-sm btn-primary';
    case 'danger':
      return 'btn-sm btn-danger';
    case 'success':
      return 'btn-sm btn-success';
    default:
      return 'btn-sm btn-secondary';
  }
}

function metricClass(tone: CrudExplorerMetric<unknown>['tone']): string {
  switch (tone) {
    case 'success':
      return 'crud-explorer-metric crud-explorer-metric--success';
    case 'warning':
      return 'crud-explorer-metric crud-explorer-metric--warning';
    case 'danger':
      return 'crud-explorer-metric crud-explorer-metric--danger';
    default:
      return 'crud-explorer-metric';
  }
}

export function CrudExplorerPage<T extends { id: string }>(props: CrudExplorerPageProps<T>) {
  const {
    title,
    singularLabel,
    pluralLabel,
    items,
    loading = false,
    error,
    searchPlaceholder = 'Buscar...',
    searchValue,
    onSearchValueChange,
    searchText,
    emptyState,
    loadingLabel = 'Cargando…',
    metrics = [],
    filters = [],
    columns = [],
    rowActions = [],
    toolbarActions = [],
    viewModes = [
      { id: 'table-detail', label: 'Detalle' },
      { id: 'list', label: 'Lista' },
    ],
    initialViewMode,
    initialSelectedId,
    detailTitle = 'Detalle',
    detailEmptyState = 'Seleccioná un registro para ver el detalle.',
    renderDetail,
    headerLeadSlot,
  } = props;

  const controlledSearch = searchValue != null;
  const [internalSearch, setInternalSearch] = useState(searchValue ?? '');
  const [activeFilterId, setActiveFilterId] = useState<string>('all');
  const [selectedId, setSelectedId] = useState<string | null>(initialSelectedId ?? null);
  const [activeViewMode, setActiveViewMode] = useState<CrudExplorerViewMode>(
    initialViewMode ?? viewModes[0]?.id ?? 'table-detail',
  );

  useEffect(() => {
    if (controlledSearch) setInternalSearch(searchValue ?? '');
  }, [controlledSearch, searchValue]);

  useEffect(() => {
    if (!items.length) {
      setSelectedId(null);
      return;
    }
    if (selectedId && items.some((row) => row.id === selectedId)) return;
    setSelectedId(initialSelectedId && items.some((row) => row.id === initialSelectedId) ? initialSelectedId : items[0].id);
  }, [initialSelectedId, items, selectedId]);

  const search = controlledSearch ? searchValue ?? '' : internalSearch;
  const deferredSearch = useDeferredValue(search.trim());

  const searchEntries = useMemo<SearchEntry<T>[]>(
    () => items.map((item) => ({ item, text: searchText(item) })),
    [items, searchText],
  );

  const searchedItems = useMemo(() => {
    if (!deferredSearch) return items;
    return fuzzySearch(deferredSearch, searchEntries).map((entry) => entry.item);
  }, [deferredSearch, items, searchEntries]);

  const activeFilter = filters.find((filter) => filter.id === activeFilterId) ?? null;
  const visibleItems = activeFilter ? searchedItems.filter(activeFilter.predicate) : searchedItems;
  const selectedItem = visibleItems.find((row) => row.id === selectedId) ?? items.find((row) => row.id === selectedId) ?? null;
  const activeView = viewModes.find((view) => view.id === activeViewMode) ?? viewModes[0];

  function setSearch(nextValue: string) {
    if (!controlledSearch) setInternalSearch(nextValue);
    onSearchValueChange?.(nextValue);
  }

  const subtitle = loading ? loadingLabel : `${visibleItems.length} ${visibleItems.length === 1 ? singularLabel : pluralLabel}`;

  return (
    <CrudPageShell
      title={title}
      subtitle={subtitle}
      headerLeadSlot={headerLeadSlot}
      search={{
        value: search,
        onChange: setSearch,
        placeholder: searchPlaceholder,
        inputClassName: 'm-kanban__search',
      }}
      headerActions={
        <>
          {toolbarActions
            .filter((action) => action.isVisible?.({ items, selectedItem }) ?? true)
            .map((action) => (
              <button
                key={action.id}
                type="button"
                className={buttonClass(action.kind)}
                onClick={() => {
                  void action.onClick({ items, selectedItem });
                }}
              >
                {action.label}
              </button>
            ))}
        </>
      }
      error={error}
    >
      <div className="crud-explorer">
        {metrics.length ? (
          <div className="crud-explorer-metrics" role="list" aria-label="Métricas">
            {metrics
              .filter((metric) => metric.isVisible?.(items) ?? true)
              .map((metric) => (
                <article key={metric.id} role="listitem" className={metricClass(metric.tone)}>
                  <span className="crud-explorer-metric__label">{metric.label}</span>
                  <strong className="crud-explorer-metric__value">{metric.value}</strong>
                  {metric.helper ? <span className="crud-explorer-metric__helper">{metric.helper}</span> : null}
                </article>
              ))}
          </div>
        ) : null}

        {filters.length ? (
          <div className="crud-explorer-pills" role="tablist" aria-label="Filtros">
            <button
              type="button"
              role="tab"
              aria-selected={activeFilterId === 'all'}
              className={`filter-pill ${activeFilterId === 'all' ? 'active' : ''}`}
              onClick={() => setActiveFilterId('all')}
            >
              Todos
            </button>
            {filters.map((filter) => (
              <button
                key={filter.id}
                type="button"
                role="tab"
                aria-selected={activeFilterId === filter.id}
                className={`filter-pill ${activeFilterId === filter.id ? 'active' : ''}`}
                onClick={() => setActiveFilterId(filter.id)}
              >
                {filter.label}
              </button>
            ))}
          </div>
        ) : null}

        {viewModes.length > 1 ? (
          <div className="crud-explorer-pills" role="tablist" aria-label="Modos de vista">
            {viewModes.map((view) => (
              <button
                key={view.id}
                type="button"
                role="tab"
                aria-selected={activeViewMode === view.id}
                className={`filter-pill ${activeViewMode === view.id ? 'active' : ''}`}
                onClick={() => setActiveViewMode(view.id)}
              >
                {view.label}
              </button>
            ))}
          </div>
        ) : null}

        {loading ? (
          <div className="empty-state">
            <p>{loadingLabel}</p>
          </div>
        ) : visibleItems.length === 0 ? (
          <div className="empty-state">
            <p>{emptyState}</p>
          </div>
        ) : activeView?.render ? (
          activeView.render({ items: visibleItems, selectedItem, onSelect: (row) => setSelectedId(row?.id ?? null) })
        ) : activeView?.id === 'table-detail' ? (
          <div className="crud-explorer-detail-layout">
            <div className="table-wrap">
              <table className="crud-table crud-explorer-table">
                <thead>
                  <tr>
                    {columns.map((column) => (
                      <th key={column.id} className={column.className}>
                        {column.header}
                      </th>
                    ))}
                    {rowActions.length ? <th className="col-actions">Acciones</th> : null}
                  </tr>
                </thead>
                <tbody>
                  {visibleItems.map((row) => (
                    <tr
                      key={row.id}
                      className={selectedItem?.id === row.id ? 'crud-explorer-row-active' : undefined}
                      onClick={() => setSelectedId(row.id)}
                    >
                      {columns.map((column) => (
                        <td key={column.id} className={column.className}>
                          {column.render(row)}
                        </td>
                      ))}
                      {rowActions.length ? (
                        <td className="col-actions" onClick={(event) => event.stopPropagation()}>
                          <div className="crud-row-actions">
                            {rowActions
                              .filter((action) => action.isVisible?.(row) ?? true)
                              .map((action) => (
                                <button
                                  key={action.id}
                                  type="button"
                                  className={buttonClass(action.kind)}
                                  onClick={() => {
                                    void action.onClick(row);
                                  }}
                                >
                                  {action.label}
                                </button>
                              ))}
                          </div>
                        </td>
                      ) : null}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <aside className="crud-explorer-detail-panel">
              <div className="card crud-explorer-detail-card">
                <div className="card-header">
                  <h2>{detailTitle}</h2>
                </div>
                {selectedItem && renderDetail ? (
                  renderDetail(selectedItem)
                ) : (
                  <div className="crud-explorer-detail-empty">{detailEmptyState}</div>
                )}
              </div>
            </aside>
          </div>
        ) : (
          <div className="table-wrap">
            <table className="crud-table crud-explorer-table">
              <thead>
                <tr>
                  {columns.map((column) => (
                    <th key={column.id} className={column.className}>
                      {column.header}
                    </th>
                  ))}
                  {rowActions.length ? <th className="col-actions">Acciones</th> : null}
                </tr>
              </thead>
              <tbody>
                {visibleItems.map((row) => (
                  <tr key={row.id}>
                    {columns.map((column) => (
                      <td key={column.id} className={column.className}>
                        {column.render(row)}
                      </td>
                    ))}
                    {rowActions.length ? (
                      <td className="col-actions">
                        <div className="crud-row-actions">
                          {rowActions
                            .filter((action) => action.isVisible?.(row) ?? true)
                            .map((action) => (
                              <button
                                key={action.id}
                                type="button"
                                className={buttonClass(action.kind)}
                                onClick={() => {
                                  void action.onClick(row);
                                }}
                              >
                                {action.label}
                              </button>
                            ))}
                        </div>
                      </td>
                    ) : null}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </CrudPageShell>
  );
}
