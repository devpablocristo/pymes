/**
 * Filtro por `created_by` (creador del registro). Listas sin el campo siguen viéndose al filtrar por actor
 * (filas sin `created_by` no se excluyen).
 */

export type CreatorFilterState = { mode: 'all' } | { mode: 'pick'; actors: Set<string> };

export function formatWorkOrderActorLabel(actor: string, selfId: string | undefined): string {
  if (selfId && actor === selfId) return 'Yo';
  if (actor.includes('@')) return actor.split('@')[0] ?? actor;
  if (actor.length > 14) return `${actor.slice(0, 12)}…`;
  return actor || '—';
}

export function applyWorkOrderCreatorFilter<T extends { id: string; created_by?: string }>(
  rows: T[],
  opts: {
    clerkEnabled: boolean;
    clerkUserLoaded: boolean;
    selfId: string | undefined;
    creatorFilter: CreatorFilterState;
  },
): T[] {
  const { clerkEnabled, clerkUserLoaded, selfId, creatorFilter } = opts;
  if (!clerkEnabled || !clerkUserLoaded) return rows;
  if (creatorFilter.mode === 'all') return rows;
  let actors = creatorFilter.actors;
  if (actors.size === 0 && selfId) {
    actors = new Set([selfId]);
  }
  if (actors.size === 0) return rows;
  return rows.filter((row) => {
    const cb = (row.created_by ?? '').trim();
    if (!cb) return true;
    return actors.has(cb);
  });
}

export function isYoCreatorFilterActive(creatorFilter: CreatorFilterState, selfId: string | undefined): boolean {
  return (
    creatorFilter.mode === 'pick' &&
    selfId != null &&
    (creatorFilter.actors.size === 0 || (creatorFilter.actors.size === 1 && creatorFilter.actors.has(selfId)))
  );
}
