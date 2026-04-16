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

function buildSimpleStatusStateMachine<T extends { id: string; status: string }>(
  states: Array<{
    value: string;
    label: string;
    badgeVariant?: NonNullable<
      NonNullable<CrudPageConfig<T>['stateMachine']>['states'][number]['badgeVariant']
    >;
  }>,
): CrudStateMachineConfig<T> {
  return {
    field: 'status',
    states: states.map((state) => ({
      value: state.value,
      label: state.label,
      columnId: state.value,
      badgeVariant: state.badgeVariant,
    })),
    columns: states.map((state) => ({
      id: state.value,
      label: state.label,
      defaultState: state.value,
    })),
  };
}

export function buildFullyConnectedStatusStateMachine<T extends { id: string; status: string }>(
  states: Array<{
    value: string;
    label: string;
    badgeVariant?: NonNullable<
      NonNullable<CrudPageConfig<T>['stateMachine']>['states'][number]['badgeVariant']
    >;
  }>,
): CrudStateMachineConfig<T> {
  return {
    ...buildSimpleStatusStateMachine(states),
    transitions: states.map((state) => ({
      from: state.value,
      to: states.filter((candidate) => candidate.value !== state.value).map((candidate) => candidate.value),
    })),
  };
}

/**
 * Agrupa múltiples estados bajo la misma columna. Cada columna tiene label propio y un defaultState.
 * Cubre kanbans con estados finos (p. ej. órdenes de trabajo con varias fases por columna).
 */
export function buildGroupedStatusStateMachine<T extends { id: string }, V extends string = string>(
  field: keyof T & string,
  columns: Array<{
    id: string;
    label: string;
    defaultState: V;
    states: Array<{
      value: V;
      label: string;
      badgeVariant?: CrudStateMachineStateConfig['badgeVariant'];
      terminal?: boolean;
    }>;
  }>,
  transitions?: Array<{ from: V; to: readonly V[] }>,
): CrudStateMachineConfig<T> {
  return {
    field,
    states: columns.flatMap((column) =>
      column.states.map((state) => ({
        value: state.value,
        label: state.label,
        columnId: column.id,
        badgeVariant: state.badgeVariant,
        terminal: state.terminal,
      })),
    ),
    columns: columns.map((column) => ({
      id: column.id,
      label: column.label,
      defaultState: column.defaultState,
    })),
    transitions: transitions?.map((t) => ({ from: t.from, to: [...t.to] })),
  };
}

/**
 * Estado libre: todas las columnas son intercambiables, sin terminales, sin restricciones.
 * Ideal para probar kanban sin configurar transiciones reales.
 */
export function buildFreeMovementStateMachine<T extends { id: string }>(
  field: keyof T & string,
  states: Array<{ value: string; label: string; badgeVariant?: CrudStateMachineStateConfig['badgeVariant'] }>,
): CrudStateMachineConfig<T> {
  return {
    field,
    states: states.map((s) => ({ value: s.value, label: s.label, columnId: s.value, badgeVariant: s.badgeVariant })),
    columns: states.map((s) => ({ id: s.value, label: s.label, defaultState: s.value })),
    transitions: states.map((s) => ({
      from: s.value,
      to: states.filter((o) => o.value !== s.value).map((o) => o.value),
    })),
  };
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
  return [];
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
