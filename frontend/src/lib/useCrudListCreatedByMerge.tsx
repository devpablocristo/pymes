import { useUser } from '@clerk/react';
import { useMemo, useState, type ReactNode } from 'react';
import { CreatedByPillsBar } from '../components/CreatedByPillsBar';
import { clerkEnabled } from './auth';
import {
  applyWorkOrderCreatorFilter,
  type CreatorFilterState,
} from './workOrderCreatorFilter';

type ListCtx = { items: Array<{ id: string; created_by?: string }> };

/**
 * Props extra para `CrudPage` / `ConfiguredCrudPage`: filtro y píldoras por `created_by` (Clerk).
 * Sin Clerk no devuelve nada. Con Clerk, aplica a todos los listados CRUD.
 */
export function useCrudListCreatedByMerge(): {
  preSearchFilter?: <T extends { id: string; created_by?: string }>(items: T[]) => T[];
  listHeaderInlineSlot?: (ctx: ListCtx) => ReactNode;
} {
  const { user, isLoaded: clerkUserLoaded } = useUser();
  const selfId = user?.id;
  const [creatorFilter, setCreatorFilter] = useState<CreatorFilterState>(() =>
    clerkEnabled ? { mode: 'pick', actors: new Set() } : { mode: 'all' },
  );

  return useMemo(() => {
    if (!clerkEnabled) {
      return {};
    }
    const opts = {
      clerkEnabled,
      clerkUserLoaded,
      selfId,
      creatorFilter,
    };
    const preSearchFilter = <T extends { id: string; created_by?: string },>(rows: T[]) =>
      applyWorkOrderCreatorFilter(rows, opts);
    const listHeaderInlineSlot = ({ items }: ListCtx) =>
      clerkUserLoaded && user ? (
        <CreatedByPillsBar
          items={items}
          creatorFilter={creatorFilter}
          onFilterChange={setCreatorFilter}
          selfId={selfId}
        />
      ) : null;
    return { preSearchFilter, listHeaderInlineSlot };
  }, [clerkUserLoaded, user, creatorFilter, selfId]);
}
