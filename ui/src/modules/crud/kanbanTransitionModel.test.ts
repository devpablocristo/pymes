import { describe, expect, it } from 'vitest';
import { createCrudKanbanTransitionModel } from './kanbanTransitionModel';

type Status = 'todo' | 'doing' | 'done' | 'cancelled';
type ColumnId = 'col_todo' | 'col_doing' | 'col_done';

const model = createCrudKanbanTransitionModel<Status, ColumnId>({
  normalizeStatus: (raw) => {
    const value = raw.trim().toLowerCase();
    switch (value) {
      case 'doing':
        return 'doing';
      case 'done':
        return 'done';
      case 'cancelled':
        return 'cancelled';
      default:
        return 'todo';
    }
  },
  columns: [
    { columnId: 'col_todo', statuses: ['todo'], defaultStatus: 'todo' },
    { columnId: 'col_doing', statuses: ['doing'], defaultStatus: 'doing' },
    { columnId: 'col_done', statuses: ['done', 'cancelled'], defaultStatus: 'done' },
  ],
  terminalStatuses: ['done', 'cancelled'],
});

describe('createCrudKanbanTransitionModel', () => {
  it('canonicalizes status and resolves column', () => {
    expect(model.canonicalizeStatus(' DOING ')).toBe('doing');
    expect(model.getColumnIdForStatus('cancelled')).toBe('col_done');
  });

  it('returns default status for a column', () => {
    expect(model.getDefaultStatusForColumn('col_doing')).toBe('doing');
  });

  it('detects terminal states', () => {
    expect(model.isTerminalStatus('done')).toBe(true);
    expect(model.isTerminalStatus('todo')).toBe(false);
  });

  it('delegates status transitions to the FSM', () => {
    expect(model.canTransitionToStatus('todo', 'doing')).toBe(true);
    expect(model.canTransitionToStatus('done', 'doing')).toBe(false);
    expect(model.canTransitionToStatus('todo', 'todo')).toBe(true);
  });

  it('blocks terminal moves and unknown columns', () => {
    expect(model.canMoveToColumn('done', 'col_doing')).toBe(false);
    expect(model.canMoveToColumn('todo', 'col_doing')).toBe(true);
    expect(model.canMoveToColumn('todo', 'missing' as ColumnId)).toBe(false);
  });
});
