import type { ReactNode } from 'react';

export type CrudTableSurfaceColumn<T> = {
  id: string;
  header: string;
  className?: string;
  render: (row: T) => ReactNode;
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
}: {
  items: T[];
  columns: CrudTableSurfaceColumn<T>[];
  rowActions?: CrudTableSurfaceRowAction<T>[];
  onRowClick?: (row: T) => void;
  selectedId?: string | null;
}) {
  return (
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
          {items.map((row) => {
            const rowId =
              typeof row === 'object' && row !== null && 'id' in row ? String((row as { id: string }).id) : undefined;
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
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
