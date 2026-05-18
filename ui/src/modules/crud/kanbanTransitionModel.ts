/**
 * Kanban transition model — as of Ola B4, the canonical implementation lives
 * in @devpablocristo/platform-fsm (createKanbanTransitionModel). This file
 * keeps the previous names (CrudKanbanTransitionModel,
 * createCrudKanbanTransitionModel) as type aliases / re-exports so consumers
 * in pymes/ui don't need to update their imports.
 */
import {
  createKanbanTransitionModel,
  type KanbanTransitionModel,
} from '@devpablocristo/platform-fsm';

export type CrudKanbanTransitionModel<
  Status extends string = string,
  ColumnId extends string = string,
> = KanbanTransitionModel<Status, ColumnId>;

export const createCrudKanbanTransitionModel = createKanbanTransitionModel;
