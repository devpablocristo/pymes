import { useCallback, useMemo, type Dispatch, type SetStateAction } from 'react';
import {
  type CreatorFilterState,
  formatWorkOrderActorLabel,
  isYoCreatorFilterActive,
} from '../lib/workOrderCreatorFilter';
import '../pages/WorkOrdersKanbanPanel.css';

type Props = {
  items: Array<{ created_by?: string }>;
  creatorFilter: CreatorFilterState;
  onFilterChange: Dispatch<SetStateAction<CreatorFilterState>>;
  selfId: string | undefined;
};

/**
 * Píldoras Todos / Yo / otros creadores (`created_by`). Mismo aspecto que el tablero Kanban de OT.
 */
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

  const toggleCreator = useCallback(
    (actor: string) => {
      onFilterChange((prev) => {
        if (prev.mode === 'all') {
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
    onFilterChange({ mode: 'pick', actors: new Set([selfId]) });
  }, [onFilterChange, selfId]);

  const setFilterAll = useCallback(() => {
    onFilterChange({ mode: 'all' });
  }, [onFilterChange]);

  return (
    <div className="wo-kanban__creators" role="group" aria-label="Filtrar por creador del registro">
      <button
        type="button"
        className={`wo-kanban__pill${creatorFilter.mode === 'all' ? ' wo-kanban__pill--active' : ''}`}
        aria-pressed={creatorFilter.mode === 'all'}
        onClick={setFilterAll}
      >
        Todos
      </button>
      {selfId ? (
        <button
          type="button"
          className={`wo-kanban__pill${isYoActive ? ' wo-kanban__pill--active' : ''}`}
          aria-pressed={isYoActive}
          onClick={selectOnlySelf}
        >
          Yo
        </button>
      ) : null}
      {peerCreators.map((actor) => {
        const active = creatorFilter.mode === 'pick' && creatorFilter.actors.has(actor);
        return (
          <button
            key={actor}
            type="button"
            className={`wo-kanban__pill${active ? ' wo-kanban__pill--active' : ''}`}
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
