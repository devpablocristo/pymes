import { useUser } from '@clerk/react';
import { useMemo, useState, type ReactNode } from 'react';
import { CreatedByPillsBar } from '../components/CreatedByPillsBar';
import { clerkEnabled } from './auth';
import { applyWorkOrderCreatorFilter, type CreatorFilterState } from './workOrderCreatorFilter';

type ListCtx = { items: Array<{ id: string; created_by?: string }> };

/**
 * Props extra para `CrudPage` / `ConfiguredCrudPage`: filtro y píldoras de responsable vía `created_by` (Clerk).
 * Sin Clerk no devuelve nada. La franja rápida de cabecera queda gobernada por el mismo toggle
 * que el filtro de responsable (`creatorFilter`), para no duplicar controles en preferencias.
 */
export function useCrudListCreatedByMerge(): {
  preSearchFilter?: <T extends { id: string; created_by?: string }>(items: T[]) => T[];
  listHeaderInlineSlot?: (ctx: ListCtx) => ReactNode;
} {
  return useCrudListCreatedByMergeImpl();
}

// `clerkEnabled` se resuelve al cargar el módulo, así que el árbol de hooks queda
// estable para toda la vida del bundle y evitamos invocar hooks condicionalmente.
const useCrudListCreatedByMergeImpl = clerkEnabled
  ? useCrudListCreatedByMergeWithClerk
  : useCrudListCreatedByMergeWithoutClerk;

function useCrudListCreatedByMergeWithoutClerk(): {
  preSearchFilter?: <T extends { id: string; created_by?: string }>(items: T[]) => T[];
  listHeaderInlineSlot?: (ctx: ListCtx) => ReactNode;
} {
  const [creatorFilter, setCreatorFilter] = useState<CreatorFilterState>(() => ({ mode: 'all' }));

  return useMemo(() => {
    const preSearchFilter = <T extends { id: string; created_by?: string }>(rows: T[]) =>
      applyWorkOrderCreatorFilter(rows, {
        authEnabled: false,
        authUserLoaded: true,
        selfId: undefined,
        creatorFilter,
      });
    const listHeaderInlineSlot = ({ items }: ListCtx) => (
      <CreatedByPillsBar
        items={items}
        creatorFilter={creatorFilter}
        onFilterChange={setCreatorFilter}
        selfId={undefined}
      />
    );
    return { preSearchFilter, listHeaderInlineSlot };
  }, [creatorFilter]);
}

function useCrudListCreatedByMergeWithClerk(): {
  preSearchFilter?: <T extends { id: string; created_by?: string }>(items: T[]) => T[];
  listHeaderInlineSlot?: (ctx: ListCtx) => ReactNode;
} {
  const { user, isLoaded: clerkUserLoaded } = useUser();
  const selfId = user?.id;
  // Por defecto "Todos": semillas/API usan created_by distinto al user id de Clerk (ej. "seed");
  // modo pick vacío se interpretaba como "solo yo" y ocultaba esas filas.
  const [creatorFilter, setCreatorFilter] = useState<CreatorFilterState>(() => ({ mode: 'all' }));

  return useMemo(() => {
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
