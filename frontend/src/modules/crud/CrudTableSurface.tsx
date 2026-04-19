import { useMemo, useState, type ReactNode } from 'react';

export type CrudTableSurfaceColumn<T> = {
  id: string;
  header: string;
  className?: string;
  render: (row: T) => ReactNode;
  sortValue?: (row: T) => string | number | boolean | null | undefined;
};

export type CrudTableSurfaceRowAction<T> = {
  id: string;
  label: string;
  kind?: 'primary' | 'secondary' | 'danger' | 'success';
  onClick: (row: T) => void | Promise<void>;
  isVisible?: (row: T) => boolean;
};

function buttonClass(kind: CrudTableSurfaceRowAction<unknown>['kind'] = 'secondary'): string {
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

export function CrudTableSurface<T>({
  items,
  columns,
  rowActions = [],
  onRowClick,
  selectedId,
  sortable = true,
}: {
  items: T[];
  columns: CrudTableSurfaceColumn<T>[];
  rowActions?: CrudTableSurfaceRowAction<T>[];
  onRowClick?: (row: T) => void;
  selectedId?: string | null;
  sortable?: boolean;
}) {
  const [sortState, setSortState] = useState<{ columnId: string; direction: 'asc' | 'desc' } | null>(null);
  const hasRowActions = rowActions.length > 0;
  const sortedItems = useMemo(() => {
    if (!sortable || !sortState) return items;
    const column = columns.find((entry) => entry.id === sortState.columnId);
    if (!column?.sortValue) return items;
    const normalize = (value: string | number | boolean | null | undefined) => {
      if (typeof value === 'number') return { rank: 0, value };
      if (typeof value === 'boolean') return { rank: 0, value: value ? 1 : 0 };
      return { rank: 1, value: String(value ?? '').trim().toLocaleLowerCase() };
    };
    return [...items].sort((left, right) => {
      const a = normalize(column.sortValue?.(left));
      const b = normalize(column.sortValue?.(right));
      const base =
        a.rank !== b.rank
          ? a.rank - b.rank
          : typeof a.value === 'number' && typeof b.value === 'number'
            ? a.value - b.value
            : String(a.value).localeCompare(String(b.value), undefined, { numeric: true, sensitivity: 'base' });
      return sortState.direction === 'asc' ? base : -base;
    });
  }, [columns, items, sortState, sortable]);

  return (
    <div className="table-wrap">
      <table className="crud-table crud-explorer-table">
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column.id} className={column.className}>
                {sortable && column.sortValue ? (
                  <div className="crud-table__sort-btn">
                    <span className="crud-table__sort-label">{column.header}</span>
                    <span className="crud-table__sort-icons" aria-hidden>
                      <button
                        type="button"
                        className={`crud-table__sort-icon${
                          sortState?.columnId === column.id && sortState.direction === 'asc'
                            ? ' crud-table__sort-icon--active'
                            : ''
                        }`}
                        onClick={() => setSortState({ columnId: column.id, direction: 'asc' })}
                      >
                        ▲
                      </button>
                      <button
                        type="button"
                        className={`crud-table__sort-icon${
                          sortState?.columnId === column.id && sortState.direction === 'desc'
                            ? ' crud-table__sort-icon--active'
                            : ''
                        }`}
                        onClick={() => setSortState({ columnId: column.id, direction: 'desc' })}
                      >
                        ▼
                      </button>
                    </span>
                  </div>
                ) : (
                  column.header
                )}
              </th>
            ))}
            {hasRowActions ? <th>Acciones</th> : null}
          </tr>
        </thead>
        <tbody>
          {sortedItems.map((row) => {
            const rowId =
              typeof row === 'object' && row !== null && 'id' in row ? String((row as { id: string }).id) : undefined;
            const visibleRowActions = rowActions.filter((action) => action.isVisible?.(row) ?? true);
            return (
              <tr
                key={rowId ?? JSON.stringify(row)}
                className={selectedId && rowId === selectedId ? 'crud-explorer-row-active' : undefined}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
              >
                {columns.map((column) => (
                  <td key={column.id} className={column.className}>
                    {column.render(row)}
                  </td>
                ))}
                {hasRowActions ? (
                  <td className="cell-actions">
                    <div className="crud-row-actions">
                      {visibleRowActions.map((action) => (
                        <button
                          key={action.id}
                          type="button"
                          className={buttonClass(action.kind)}
                          onClick={(event) => {
                            event.stopPropagation();
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
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
