import { useCallback, useMemo, type Dispatch, type SetStateAction } from 'react';
import {
  type CreatorFilterState,
  formatWorkOrderActorLabel,
  isSeedActor,
  isYoCreatorFilterActive,
} from '../lib/workOrderCreatorFilter';

type Props = {
  items: Array<{ created_by?: string }>;
  creatorFilter: CreatorFilterState;
  onFilterChange: Dispatch<SetStateAction<CreatorFilterState>>;
  selfId: string | undefined;
};

export function CreatedByPillsBar({ items, creatorFilter, onFilterChange, selfId }: Props) {
  const uniqueCreators = useMemo(() => {
    const s = new Set<string>();
    for (const row of items) {
      const a = (row.created_by || '').trim();
      if (a) s.add(a);
    }
    return Array.from(s).sort((a, b) => a.localeCompare(b));
  }, [items]);

  const peerCreators = useMemo(
    () => (selfId ? uniqueCreators.filter((a) => a !== selfId) : uniqueCreators),
    [uniqueCreators, selfId],
  );

  const isYoActive = isYoCreatorFilterActive(creatorFilter, selfId);
  const isSeedsActive = creatorFilter.mode === 'seeds';
  const isAllActive = creatorFilter.mode === 'all';

  const toggleCreator = useCallback(
    (actor: string) => {
      onFilterChange((prev) => {
        if (prev.mode === 'all' || prev.mode === 'yo' || prev.mode === 'seeds') {
          return { mode: 'pick', actors: new Set([actor]) };
        }
        let base = prev.actors;
        if (prev.mode === 'pick' && prev.actors.size === 0 && selfId) {
          base = new Set([selfId]);
        }
        const next = new Set(base);
        if (next.has(actor)) {
          next.delete(actor);
        } else {
          next.add(actor);
        }
        if (next.size === 0) {
          return { mode: 'all' };
        }
        return { mode: 'pick', actors: next };
      });
    },
    [onFilterChange, selfId],
  );

  const selectOnlySelf = useCallback(() => {
    if (!selfId) return;
    onFilterChange({ mode: 'yo' });
  }, [onFilterChange, selfId]);

  const setFilterAll = useCallback(() => {
    onFilterChange({ mode: 'all' });
  }, [onFilterChange]);

  const setFilterSeeds = useCallback(() => {
    onFilterChange({ mode: 'seeds' });
  }, [onFilterChange]);

  const hasSeedRows = useMemo(() => items.some((row) => isSeedActor(row.created_by)), [items]);

  return (
    <div className="crud-creator-badges" role="group" aria-label="Filtrar por creador del registro">
      <button
        type="button"
        className={`badge crud-creator-badge${isAllActive ? ' crud-creator-badge--active' : ''}`}
        aria-pressed={isAllActive}
        onClick={setFilterAll}
      >
        Todos
      </button>
      {selfId ? (
        <button
          type="button"
          className={`badge crud-creator-badge${isYoActive ? ' crud-creator-badge--active' : ''}`}
          aria-pressed={isYoActive}
          onClick={selectOnlySelf}
        >
          Yo
        </button>
      ) : null}
      {hasSeedRows ? (
        <button
          type="button"
          className={`badge crud-creator-badge${isSeedsActive ? ' crud-creator-badge--active' : ''}`}
          aria-pressed={isSeedsActive}
          onClick={setFilterSeeds}
        >
          Seeds
        </button>
      ) : null}
      {peerCreators.map((actor) => {
        const active = creatorFilter.mode === 'pick' && creatorFilter.actors.has(actor);
        return (
          <button
            key={actor}
            type="button"
            className={`badge crud-creator-badge${active ? ' crud-creator-badge--active' : ''}`}
            aria-pressed={active}
            onClick={() => toggleCreator(actor)}
          >
            {formatWorkOrderActorLabel(actor, selfId)}
          </button>
        );
      })}
    </div>
  );
}
