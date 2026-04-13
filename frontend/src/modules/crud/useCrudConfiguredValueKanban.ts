import { crudItemPath } from '@devpablocristo/modules-crud-ui';
import { useCallback, useMemo } from 'react';
import type { CrudPageConfig, CrudValueFilterOption } from '../../components/CrudPage';
import { apiRequest } from '../../lib/api';
import { buildCrudKanbanTransitionModelFromStateMachine, resolveCrudValueFilterOptions } from './crudStateMachine';
import { createCrudKanbanArchiveTerminalDragPolicy } from './crudKanbanDragPolicy';
import { createCrudKanbanTransitionModel } from './kanbanTransitionModel';
import { useCrudKanbanMove } from './useCrudKanbanMove';

const NO_KANBAN_DRAG = {
  isRowDraggable: () => false,
  isColumnDroppable: () => false,
} as const;

function normalizeKanbanValue(raw: unknown) {
  return String(raw ?? '').trim().toLowerCase();
}

type Options<T extends { id: string }> = {
  crudConfig: CrudPageConfig<T> | null;
  items: T[];
  setItems: React.Dispatch<React.SetStateAction<T[]>>;
  reload: () => Promise<void>;
  setError: (message: string | null) => void;
  archived: boolean;
  valueFilterOptions?: CrudValueFilterOption<T>[];
};

export function useCrudConfiguredValueKanban<T extends { id: string }>({
  crudConfig,
  items,
  setItems,
  reload,
  setError,
  archived,
  valueFilterOptions,
}: Options<T>) {
  const resolvedValueFilterOptions = resolveCrudValueFilterOptions(crudConfig, valueFilterOptions);
  const stateMachine = crudConfig?.stateMachine ?? null;
  const kanbanConfig = crudConfig?.kanban ?? null;
  const kanbanField = stateMachine?.field ?? kanbanConfig?.field ?? null;

  const transitionModel = useMemo(() => {
    if (stateMachine) {
      return buildCrudKanbanTransitionModelFromStateMachine(stateMachine);
    }
    if (!kanbanConfig || resolvedValueFilterOptions.length === 0) return null;
    return createCrudKanbanTransitionModel({
      normalizeStatus: normalizeKanbanValue,
      columns: resolvedValueFilterOptions.map((option) => ({
        columnId: option.value,
        statuses: [option.value],
        defaultStatus: option.value,
      })),
      terminalStatuses: (kanbanConfig.terminalValues ?? []).map(normalizeKanbanValue),
    });
  }, [kanbanConfig, resolvedValueFilterOptions, stateMachine]);

  const getItemStatus = useCallback(
    (row: T) => {
      if (!kanbanField) return '';
      return normalizeKanbanValue((row as Record<string, unknown>)[kanbanField]);
    },
    [kanbanField],
  );

  const getItemColumnId = useCallback(
    (row: T) => transitionModel?.getColumnIdForStatus(getItemStatus(row)) ?? 'all',
    [getItemStatus, transitionModel],
  );

  const dragPolicy = useMemo(() => {
    if (transitionModel == null) return NO_KANBAN_DRAG;
    return createCrudKanbanArchiveTerminalDragPolicy<T>({
      showArchived: archived,
      transitionModel,
      getItemStatus,
    });
  }, [archived, getItemStatus, transitionModel]);

  const persistStatusChange = useCallback(
    async (itemId: string, nextStatus: string) => {
      if (!crudConfig || !kanbanField) throw new Error('Recurso sin configuración de kanban.');
      const row = items.find((item) => item.id === itemId);
      if (!row) throw new Error('No se encontró el registro a mover.');

      if (kanbanConfig?.persistMove) {
        return kanbanConfig.persistMove({
          row,
          field: kanbanField,
          nextValue: nextStatus,
        });
      }

      const nextValues = {
        ...crudConfig.toFormValues(row),
        [kanbanField]: nextStatus,
      };

      if (crudConfig.dataSource?.update) {
        await crudConfig.dataSource.update(row, nextValues);
      } else if (crudConfig.basePath) {
        await apiRequest(crudItemPath(crudConfig.basePath, row.id), {
          method: 'PUT',
          body: crudConfig.toBody
            ? crudConfig.toBody(nextValues)
            : (nextValues as Record<string, unknown>),
        });
      } else {
        throw new Error('Este recurso no soporta actualización por tablero.');
      }

      return { ...row, [kanbanField]: nextStatus } as T;
    },
    [crudConfig, items, kanbanConfig, kanbanField],
  );

  const handleMoveCard = useCrudKanbanMove<T, string>({
    items,
    setItems,
    transitionModel,
    getItemColumnId,
    getItemStatus,
    setItemStatus: (row, status) =>
      kanbanField ? ({ ...row, [kanbanField]: status } as T) : row,
    persistStatusChange,
    reload,
    setError,
  });

  return {
    enabled: transitionModel != null,
    onMoveCard: handleMoveCard,
    isRowDraggable: dragPolicy.isRowDraggable,
    isColumnDroppable: dragPolicy.isColumnDroppable,
  };
}
