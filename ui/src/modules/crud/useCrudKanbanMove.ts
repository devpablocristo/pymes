import { useCallback } from 'react';
import { applyCrudKanbanMove } from './kanbanBoardState';
import type { CrudKanbanTransitionModel } from './kanbanTransitionModel';

type Options<T extends { id: string }, ColumnId extends string> = {
  items: T[];
  setItems: React.Dispatch<React.SetStateAction<T[]>>;
  transitionModel: CrudKanbanTransitionModel<string, ColumnId> | null;
  getItemColumnId: (item: T) => ColumnId;
  getItemStatus: (item: T) => string;
  setItemStatus: (item: T, status: string) => T;
  persistStatusChange: (itemId: string, nextStatus: string) => Promise<T>;
  mergePersistedItem?: (persisted: T, nextStatus: string) => T;
  reload: () => Promise<void>;
  setError: (message: string | null) => void;
};

export function useCrudKanbanMove<T extends { id: string }, ColumnId extends string>({
  items,
  setItems,
  transitionModel,
  getItemColumnId,
  getItemStatus,
  setItemStatus,
  persistStatusChange,
  mergePersistedItem,
  reload,
  setError,
}: Options<T, ColumnId>) {
  return useCallback(
    (itemId: string, targetColumnId: ColumnId, overItemId?: string) => {
      if (transitionModel == null) return;
      const card = items.find((item) => item.id === itemId);
      if (!card) return;

      const currentColumnId = getItemColumnId(card);
      setItems((prev) =>
        applyCrudKanbanMove({
          items: prev,
          itemId,
          targetColumnId,
          overItemId,
          getItemColumnId,
          getItemStatus,
          setItemStatus,
          transitionModel,
        }),
      );

      if (currentColumnId === targetColumnId) return;

      const nextStatus = transitionModel.getDefaultStatusForColumn(targetColumnId);
      if (nextStatus == null) return;

      void (async () => {
        try {
          const persisted = await persistStatusChange(itemId, nextStatus);
          const resolved = mergePersistedItem?.(persisted, nextStatus) ?? persisted;
          setItems((prev) => prev.map((item) => (item.id === itemId ? resolved : item)));
          setError(null);
        } catch (error) {
          await reload();
          setError(error instanceof Error ? error.message : 'Error al guardar');
        }
      })();
    },
    [
      getItemColumnId,
      getItemStatus,
      items,
      mergePersistedItem,
      persistStatusChange,
      reload,
      setError,
      setItemStatus,
      setItems,
      transitionModel,
    ],
  );
}
