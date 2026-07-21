import { describe, expect, it, vi } from 'vitest';
import { createCrudKanbanArchiveTerminalDragPolicy } from './crudKanbanDragPolicy';

function modelTerminalsOpen() {
  return {
    isTerminalStatus: (raw: string) => raw === 'done' || raw === 'cancelled',
  };
}

describe('createCrudKanbanArchiveTerminalDragPolicy', () => {
  it('archivado: nada draggable ni droppable', () => {
    const { isRowDraggable, isColumnDroppable } = createCrudKanbanArchiveTerminalDragPolicy({
      showArchived: true,
      transitionModel: modelTerminalsOpen(),
      getItemStatus: (row: { status: string }) => row.status,
    });
    expect(isRowDraggable({ status: 'open' })).toBe(false);
    expect(isRowDraggable({ status: 'done' })).toBe(false);
    expect(isColumnDroppable('col-a')).toBe(false);
  });

  it('activo: terminal no draggable; no terminal sí', () => {
    const { isRowDraggable, isColumnDroppable } = createCrudKanbanArchiveTerminalDragPolicy({
      showArchived: false,
      transitionModel: modelTerminalsOpen(),
      getItemStatus: (row: { status: string }) => row.status,
    });
    expect(isRowDraggable({ status: 'open' })).toBe(true);
    expect(isRowDraggable({ status: 'done' })).toBe(false);
    expect(isRowDraggable({ status: 'cancelled' })).toBe(false);
    expect(isColumnDroppable('any')).toBe(true);
  });

  it('delega isTerminalStatus al modelo', () => {
    const isTerminalStatus = vi.fn().mockReturnValue(true);
    const { isRowDraggable } = createCrudKanbanArchiveTerminalDragPolicy({
      showArchived: false,
      transitionModel: { isTerminalStatus },
      getItemStatus: (row: { s: string }) => row.s,
    });
    isRowDraggable({ s: 'x' });
    expect(isTerminalStatus).toHaveBeenCalledWith('x');
  });
});
