import type { CrudKanbanTransitionModel } from './kanbanTransitionModel';

export type CrudKanbanArchiveTerminalDragPolicy<T> = {
  isRowDraggable: (row: T) => boolean;
  isColumnDroppable: (columnId: string) => boolean;
};

/**
 * Política de drag & drop para tableros CRUD:
 * - Vista **archivada**: sin arrastre ni drop en columnas (solo lectura operativa).
 * - Vista **activa**: filas en **estado terminal** no son arrastrables (siguen en su columna).
 *
 * Agnóstico de dominio: el criterio de terminalidad lo da `transitionModel`; el status por fila
 * viene de `getItemStatus`.
 */
export function createCrudKanbanArchiveTerminalDragPolicy<
  T,
  ColumnId extends string = string,
>(options: {
  showArchived: boolean;
  transitionModel: Pick<CrudKanbanTransitionModel<string, ColumnId>, 'isTerminalStatus'>;
  getItemStatus: (row: T) => string;
}): CrudKanbanArchiveTerminalDragPolicy<T> {
  const { showArchived, transitionModel, getItemStatus } = options;

  return {
    isRowDraggable: (row) => !showArchived && !transitionModel.isTerminalStatus(getItemStatus(row)),
    isColumnDroppable: () => !showArchived,
  };
}
