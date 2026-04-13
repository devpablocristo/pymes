import type {
  CrudPageConfig,
  CrudStateMachineConfig,
  CrudStateMachineStateConfig,
  CrudFormField,
  CrudValueFilterOption,
} from '../../components/CrudPage';
import { createCrudKanbanTransitionModel } from './kanbanTransitionModel';

export function normalizeCrudStateValue(raw: unknown) {
  return String(raw ?? '').trim().toLowerCase();
}

export function buildCrudValueFilterOptionsFromStateMachine<T extends { id: string }>(
  stateMachine: CrudStateMachineConfig<T>,
): CrudValueFilterOption<T>[] {
  return stateMachine.states.map((state) => ({
    value: normalizeCrudStateValue(state.value),
    label: state.label,
    matches: (row: T) =>
      normalizeCrudStateValue((row as Record<string, unknown>)[stateMachine.field]) === normalizeCrudStateValue(state.value),
  }));
}

export function buildCrudSelectFieldOptionsFromStateMachine<T extends { id: string }>(
  stateMachine: CrudStateMachineConfig<T>,
): NonNullable<CrudFormField['options']> {
  return stateMachine.states.map((state) => ({
    value: normalizeCrudStateValue(state.value),
    label: state.label,
  }));
}

export function findCrudStateMachineStateByValue<T extends { id: string }>(
  stateMachine: CrudStateMachineConfig<T>,
  rawValue: unknown,
): CrudStateMachineStateConfig | null {
  const canonical = normalizeCrudStateValue(rawValue);
  return (
    stateMachine.states.find((state) => normalizeCrudStateValue(state.value) === canonical) ?? null
  );
}

export function findCrudStateMachineStateForRow<T extends { id: string }>(
  stateMachine: CrudStateMachineConfig<T>,
  row: T,
): CrudStateMachineStateConfig | null {
  return findCrudStateMachineStateByValue(stateMachine, (row as Record<string, unknown>)[stateMachine.field]);
}

export function getCrudStateMachineColumnDefaultState<T extends { id: string }>(
  stateMachine: CrudStateMachineConfig<T>,
  columnId: string,
): string | null {
  const column = stateMachine.columns.find((entry) => entry.id === columnId);
  return column ? normalizeCrudStateValue(column.defaultState) : null;
}

export function resolveCrudValueFilterOptions<T extends { id: string }>(
  crudConfig: CrudPageConfig<T> | null,
  override?: CrudValueFilterOption<T>[],
): CrudValueFilterOption<T>[] {
  if (override?.length) return override;
  if (crudConfig?.stateMachine) {
    return buildCrudValueFilterOptionsFromStateMachine(crudConfig.stateMachine);
  }
  return crudConfig?.valueFilterOptions ?? [];
}

export function buildCrudKanbanTransitionModelFromStateMachine<T extends { id: string }>(
  stateMachine: CrudStateMachineConfig<T>,
) {
  return createCrudKanbanTransitionModel({
    normalizeStatus: normalizeCrudStateValue,
    columns: stateMachine.columns.map((column) => ({
      columnId: column.id,
      defaultStatus: normalizeCrudStateValue(column.defaultState),
      statuses: stateMachine.states
        .filter((state) => state.columnId === column.id)
        .map((state) => normalizeCrudStateValue(state.value)),
    })),
    terminalStatuses: stateMachine.states
      .filter((state) => state.terminal)
      .map((state) => normalizeCrudStateValue(state.value)),
    transitions: stateMachine.transitions?.map((transition) => ({
      from: normalizeCrudStateValue(transition.from),
      to: transition.to.map((target) => normalizeCrudStateValue(target)),
    })),
  });
}
