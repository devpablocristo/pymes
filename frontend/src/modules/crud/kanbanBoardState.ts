import type { CrudKanbanTransitionModel } from './kanbanTransitionModel';

type Options<T extends { id: string }, ColumnId extends string> = {
  items: T[];
  itemId: string;
  targetColumnId: ColumnId;
  overItemId?: string;
  getItemColumnId: (item: T) => ColumnId;
  getItemStatus: (item: T) => string;
  setItemStatus: (item: T, status: string) => T;
  transitionModel: CrudKanbanTransitionModel<string, ColumnId>;
};

export function applyCrudKanbanMove<T extends { id: string }, ColumnId extends string>({
  items,
  itemId,
  targetColumnId,
  overItemId,
  getItemColumnId,
  getItemStatus,
  setItemStatus,
  transitionModel,
}: Options<T, ColumnId>): T[] {
  const card = items.find((item) => item.id === itemId);
  if (!card) return items;

  const currentColumnId = getItemColumnId(card);
  if (currentColumnId === targetColumnId) {
    return reorderCrudKanbanItems(items, itemId, overItemId);
  }

  if (!transitionModel.canMoveToColumn(getItemStatus(card), targetColumnId)) {
    return items;
  }

  const nextStatus = transitionModel.getDefaultStatusForColumn(targetColumnId);
  if (nextStatus == null) return items;

  const updated = setItemStatus(card, nextStatus);
  const without = items.filter((item) => item.id !== itemId);
  if (overItemId) {
    const targetIdx = without.findIndex((item) => item.id === overItemId);
    if (targetIdx !== -1) {
      const reordered = [...without];
      reordered.splice(targetIdx, 0, updated);
      return reordered;
    }
  }

  let lastIdx = -1;
  for (let i = without.length - 1; i >= 0; i -= 1) {
    if (getItemColumnId(without[i]) === targetColumnId) {
      lastIdx = i;
      break;
    }
  }
  const reordered = [...without];
  if (lastIdx !== -1) {
    reordered.splice(lastIdx + 1, 0, updated);
  } else {
    reordered.push(updated);
  }
  return reordered;
}

export function reorderCrudKanbanItems<T extends { id: string }>(
  items: T[],
  itemId: string,
  overItemId?: string,
): T[] {
  if (!overItemId) return items;
  const idx = items.findIndex((item) => item.id === itemId);
  if (idx === -1) return items;
  const targetIdx = items.findIndex((item) => item.id === overItemId);
  if (targetIdx === -1 || idx === targetIdx) return items;

  const moved = items[idx];
  const without = [...items.slice(0, idx), ...items.slice(idx + 1)];
  const insertionIdx = without.findIndex((item) => item.id === overItemId);
  if (insertionIdx === -1) return items;
  without.splice(insertionIdx, 0, moved);
  return without;
}
