import {
  StatusKanbanBoard,
  type StatusKanbanBoardProps,
} from '@devpablocristo/modules-kanban-board';
import type { ReactNode } from 'react';

type Props<T extends { id: string }> = StatusKanbanBoardProps<T> & {
  leadSlot?: ReactNode;
};

export function CrudKanbanSurface<T extends { id: string }>({ leadSlot, ...props }: Props<T>) {
  return (
    <>
      {leadSlot ? (
        <div className="generic-work-orders-board__lead crud-list-header-lead crud-list-header-lead--above-title">
          {leadSlot}
        </div>
      ) : null}
      <StatusKanbanBoard {...props} />
    </>
  );
}
