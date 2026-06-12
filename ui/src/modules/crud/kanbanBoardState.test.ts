import { describe, expect, it } from 'vitest';
import { applyCrudKanbanMove, reorderCrudKanbanItems } from './kanbanBoardState';
import { createCrudKanbanTransitionModel } from './kanbanTransitionModel';

type Row = { id: string; status: string };
type ColumnId = 'todo' | 'doing' | 'done';

const transitionModel = createCrudKanbanTransitionModel<string, ColumnId>({
  normalizeStatus: (raw) => {
    const value = raw.trim().toLowerCase();
    if (value === 'doing') return 'doing';
    if (value === 'done') return 'done';
    return 'todo';
  },
  columns: [
    { columnId: 'todo', statuses: ['todo'], defaultStatus: 'todo' },
    { columnId: 'doing', statuses: ['doing'], defaultStatus: 'doing' },
    { columnId: 'done', statuses: ['done'], defaultStatus: 'done' },
  ],
  terminalStatuses: ['done'],
});

const getItemColumnId = (row: Row): ColumnId => transitionModel.getColumnIdForStatus(row.status);

describe('reorderCrudKanbanItems', () => {
  it('reorders bottom → top (drag up)', () => {
    const rows = [{ id: '1', status: 'todo' }, { id: '2', status: 'todo' }, { id: '3', status: 'doing' }];
    expect(reorderCrudKanbanItems(rows, '2', '1').map((row) => row.id)).toEqual(['2', '1', '3']);
  });

  it('reorders top → bottom (drag down)', () => {
    const rows = [{ id: '1', status: 'todo' }, { id: '2', status: 'todo' }, { id: '3', status: 'doing' }];
    expect(reorderCrudKanbanItems(rows, '1', '2').map((row) => row.id)).toEqual(['2', '1', '3']);
  });

  it('reorders across more than two cards — drag from top to bottom of column', () => {
    const rows = [
      { id: 'a', status: 'todo' },
      { id: 'b', status: 'todo' },
      { id: 'c', status: 'todo' },
    ];
    expect(reorderCrudKanbanItems(rows, 'a', 'c').map((row) => row.id)).toEqual(['b', 'c', 'a']);
  });

  it('reorders across more than two cards — drag from bottom to top of column', () => {
    const rows = [
      { id: 'a', status: 'todo' },
      { id: 'b', status: 'todo' },
      { id: 'c', status: 'todo' },
    ];
    expect(reorderCrudKanbanItems(rows, 'c', 'a').map((row) => row.id)).toEqual(['c', 'a', 'b']);
  });
});

describe('applyCrudKanbanMove', () => {
  it('moves across columns using the transition model default status', () => {
    const rows = [{ id: '1', status: 'todo' }, { id: '2', status: 'doing' }];
    const next = applyCrudKanbanMove({
      items: rows,
      itemId: '1',
      targetColumnId: 'doing',
      getItemColumnId,
      getItemStatus: (row) => row.status,
      setItemStatus: (row, status) => ({ ...row, status }),
      transitionModel,
    });
    expect(next[1]).toEqual({ id: '1', status: 'doing' });
  });

  it('keeps items unchanged when the transition is invalid', () => {
    const rows = [{ id: '1', status: 'done' }, { id: '2', status: 'doing' }];
    const next = applyCrudKanbanMove({
      items: rows,
      itemId: '1',
      targetColumnId: 'doing',
      getItemColumnId,
      getItemStatus: (row) => row.status,
      setItemStatus: (row, status) => ({ ...row, status }),
      transitionModel,
    });
    expect(next).toEqual(rows);
  });
});
