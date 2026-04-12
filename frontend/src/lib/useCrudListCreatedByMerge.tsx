import { useUser } from '@clerk/react';
import { useMemo, useState, type ReactNode } from 'react';
import { CreatedByPillsBar } from '../components/CreatedByPillsBar';
import { clerkEnabled } from './auth';
import { applyWorkOrderCreatorFilter, type CreatorFilterState } from './workOrderCreatorFilter';

type ListCtx = { items: Array<{ id: string; created_by?: string }> };

/**
 * Props extra para `CrudPage` / `ConfiguredCrudPage`: filtro y píldoras de responsable vía `created_by` (Clerk).
 * Sin Clerk no devuelve nada. Con Clerk, aplica a listados que no apaguen la franja con
 * `featureFlags.headerQuickFilterStrip: false` o `creatorFilter: false` (p. ej. inventario).
 */
export function useCrudListCreatedByMerge(): {
  preSearchFilter?: <T extends { id: string; created_by?: string }>(items: T[]) => T[];
  listHeaderInlineSlot?: (ctx: ListCtx) => ReactNode;
} {
  const { user, isLoaded: clerkUserLoaded } = useUser();
  const selfId = user?.id;
  // Por defecto "Todos": semillas/API usan created_by distinto al user id de Clerk (ej. "seed");
  // modo pick vacío se interpretaba como "solo yo" y ocultaba esas filas.
  const [creatorFilter, setCreatorFilter] = useState<CreatorFilterState>(() => ({ mode: 'all' }));

  return useMemo(() => {
    if (!clerkEnabled) {
      return {};
    }
    const opts = {
      authEnabled: clerkEnabled,
      authUserLoaded: clerkUserLoaded,
      selfId,
      creatorFilter,
    };
    const preSearchFilter = <T extends { id: string; created_by?: string }>(rows: T[]) =>
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
