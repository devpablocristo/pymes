import { Builder, type StringMachine } from '@devpablocristo/core-fsm';

export type CrudKanbanTransitionModel<Status extends string = string, ColumnId extends string = string> = {
  canonicalizeStatus: (raw: string) => Status;
  getColumnIdForStatus: (raw: string) => ColumnId;
  getDefaultStatusForColumn: (columnId: ColumnId) => Status | null;
  isTerminalStatus: (raw: string) => boolean;
  canTransitionToStatus: (fromStatus: string, toStatus: string) => boolean;
  canMoveToColumn: (fromStatus: string, targetColumnId: ColumnId) => boolean;
};

type ColumnStatusMapping<Status extends string, ColumnId extends string> = {
  columnId: ColumnId;
  statuses: readonly Status[];
  defaultStatus: Status | null;
};

type Options<Status extends string, ColumnId extends string> = {
  normalizeStatus: (raw: string) => Status;
  columns: readonly ColumnStatusMapping<Status, ColumnId>[];
  terminalStatuses?: readonly Status[];
  transitions?: readonly { from: Status; to: readonly Status[] }[];
};

export function createCrudKanbanTransitionModel<Status extends string, ColumnId extends string>({
  normalizeStatus,
  columns,
  terminalStatuses = [],
  transitions = [],
}: Options<Status, ColumnId>): CrudKanbanTransitionModel<Status, ColumnId> {
  const statusToColumn = new Map<Status, ColumnId>();
  const defaultStatusByColumn = new Map<ColumnId, Status | null>();

  for (const column of columns) {
    defaultStatusByColumn.set(column.columnId, column.defaultStatus);
    for (const status of column.statuses) {
      statusToColumn.set(status, column.columnId);
    }
  }

  const firstColumnId = columns[0]?.columnId;
  const transitionMachine = buildTransitionMachine(columns, terminalStatuses, transitions);

  const canonicalizeStatus = (raw: string): Status => normalizeStatus(raw);

  const getColumnIdForStatus = (raw: string): ColumnId => {
    const canonical = canonicalizeStatus(raw);
    return statusToColumn.get(canonical) ?? firstColumnId!;
  };

  const getDefaultStatusForColumn = (columnId: ColumnId): Status | null => defaultStatusByColumn.get(columnId) ?? null;

  const isTerminalStatus = (raw: string): boolean => transitionMachine.isTerminal(canonicalizeStatus(raw));

  const canTransitionToStatus = (fromStatus: string, toStatus: string): boolean =>
    transitionMachine.canTransition(canonicalizeStatus(fromStatus), canonicalizeStatus(toStatus));

  const canMoveToColumn = (fromStatus: string, targetColumnId: ColumnId): boolean => {
    const nextStatus = getDefaultStatusForColumn(targetColumnId);
    if (nextStatus == null) return false;
    return canTransitionToStatus(fromStatus, nextStatus);
  };

  return {
    canonicalizeStatus,
    getColumnIdForStatus,
    getDefaultStatusForColumn,
    isTerminalStatus,
    canTransitionToStatus,
    canMoveToColumn,
  };
}

function buildTransitionMachine<Status extends string, ColumnId extends string>(
  columns: readonly ColumnStatusMapping<Status, ColumnId>[],
  terminalStatuses: readonly Status[],
  transitions: readonly { from: Status; to: readonly Status[] }[],
): StringMachine {
  const builder = new Builder();
  if (terminalStatuses.length > 0) {
    builder.terminal(...terminalStatuses);
  }
  if (transitions.length > 0) {
    for (const transition of transitions) {
      for (const target of transition.to) {
        builder.allow(transition.from, target);
      }
    }
  } else {
    const defaultStatuses = columns
      .map((column) => column.defaultStatus)
      .filter((status): status is Status => status != null);
    for (const status of defaultStatuses) {
      builder.allowAnyTo(status);
    }
  }
  return builder.build();
}
