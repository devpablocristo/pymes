import { describe, expect, it } from 'vitest';
import type { CrudStateMachineConfig } from '../../components/CrudPage';
import {
  buildCrudKanbanTransitionModelFromStateMachine,
  buildCrudSelectFieldOptionsFromStateMachine,
  buildCrudValueFilterOptionsFromStateMachine,
  findCrudStateMachineStateByValue,
  getCrudStateMachineColumnDefaultState,
} from './crudStateMachine';

type PurchaseLike = { id: string; status: string };

const purchaseStateMachine: CrudStateMachineConfig<PurchaseLike> = {
  field: 'status',
  states: [
    { value: 'draft', label: 'Borrador', columnId: 'draft', badgeVariant: 'default' },
    { value: 'partial', label: 'Parcial', columnId: 'partial', badgeVariant: 'warning' },
    { value: 'received', label: 'Recibida', columnId: 'received', badgeVariant: 'info' },
    { value: 'voided', label: 'Anulada', columnId: 'voided', badgeVariant: 'danger' },
  ],
  columns: [
    { id: 'draft', label: 'Borrador', defaultState: 'draft' },
    { id: 'partial', label: 'Parcial', defaultState: 'partial' },
    { id: 'received', label: 'Recibida', defaultState: 'received' },
    { id: 'voided', label: 'Anulada', defaultState: 'voided' },
  ],
  transitions: [
    { from: 'draft', to: ['partial', 'received', 'voided'] },
    { from: 'partial', to: ['draft', 'received', 'voided'] },
    { from: 'received', to: ['draft', 'partial', 'voided'] },
    { from: 'voided', to: ['draft', 'partial', 'received'] },
  ],
};

describe('crudStateMachine', () => {
  it('derives value filter options from the state machine', () => {
    const options = buildCrudValueFilterOptionsFromStateMachine(purchaseStateMachine);
    expect(options.map((option) => ({ value: option.value, label: option.label }))).toEqual([
      { value: 'draft', label: 'Borrador' },
      { value: 'partial', label: 'Parcial' },
      { value: 'received', label: 'Recibida' },
      { value: 'voided', label: 'Anulada' },
    ]);
    expect(options[0].matches({ id: '1', status: 'draft' })).toBe(true);
    expect(options[2].matches({ id: '1', status: 'draft' })).toBe(false);
  });

  it('derives select options from the state machine', () => {
    expect(buildCrudSelectFieldOptionsFromStateMachine(purchaseStateMachine)).toEqual([
      { value: 'draft', label: 'Borrador' },
      { value: 'partial', label: 'Parcial' },
      { value: 'received', label: 'Recibida' },
      { value: 'voided', label: 'Anulada' },
    ]);
  });

  it('builds a kanban transition model from the state machine', () => {
    const model = buildCrudKanbanTransitionModelFromStateMachine(purchaseStateMachine);

    expect(model.getColumnIdForStatus('draft')).toBe('draft');
    expect(model.getColumnIdForStatus('received')).toBe('received');
    expect(model.canMoveToColumn('draft', 'received')).toBe(true);
    expect(model.canMoveToColumn('draft', 'partial')).toBe(true);
    expect(model.canMoveToColumn('draft', 'voided')).toBe(true);
    expect(model.canMoveToColumn('received', 'draft')).toBe(true);
    expect(model.canMoveToColumn('voided', 'partial')).toBe(true);
    expect(model.isTerminalStatus('draft')).toBe(false);
    expect(model.isTerminalStatus('received')).toBe(false);
    expect(model.isTerminalStatus('voided')).toBe(false);
  });

  it('resolves badge metadata and default column state from the state machine', () => {
    expect(findCrudStateMachineStateByValue(purchaseStateMachine, 'received')).toEqual({
      value: 'received',
      label: 'Recibida',
      columnId: 'received',
      badgeVariant: 'info',
    });
    expect(getCrudStateMachineColumnDefaultState(purchaseStateMachine, 'voided')).toBe('voided');
  });
});
