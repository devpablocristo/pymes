import { describe, it, expect } from 'vitest';
import {
  applyWorkOrderCreatorFilter,
  formatWorkOrderActorLabel,
  isYoCreatorFilterActive,
  type CreatorFilterState,
} from './workOrderCreatorFilter';

type Row = { id: string; created_by?: string };

const ROWS: Row[] = [
  { id: '1', created_by: 'user_alice' },
  { id: '2', created_by: 'user_bob' },
  { id: '3', created_by: '' },
  { id: '4' }, // no created_by
];

describe('formatWorkOrderActorLabel', () => {
  it('returns "Yo" when actor matches selfId', () => {
    expect(formatWorkOrderActorLabel('user_me', 'user_me')).toBe('Yo');
  });

  it('extracts name before @ for email actors', () => {
    expect(formatWorkOrderActorLabel('john@example.com', undefined)).toBe('john');
  });

  it('truncates long actor names', () => {
    expect(formatWorkOrderActorLabel('a_very_long_actor_name', undefined)).toBe('a_very_long_\u2026');
  });

  it('returns short names as-is', () => {
    expect(formatWorkOrderActorLabel('alice', undefined)).toBe('alice');
  });

  it('returns dash for empty actor', () => {
    expect(formatWorkOrderActorLabel('', undefined)).toBe('\u2014');
  });
});

describe('applyWorkOrderCreatorFilter', () => {
  const baseOpts = { clerkEnabled: true, clerkUserLoaded: true, selfId: 'user_alice' };

  it('returns all rows when clerk is disabled', () => {
    const result = applyWorkOrderCreatorFilter(ROWS, {
      ...baseOpts,
      clerkEnabled: false,
      creatorFilter: { mode: 'pick', actors: new Set(['user_bob']) },
    });
    expect(result).toHaveLength(4);
  });

  it('returns all rows when clerk user not loaded', () => {
    const result = applyWorkOrderCreatorFilter(ROWS, {
      ...baseOpts,
      clerkUserLoaded: false,
      creatorFilter: { mode: 'pick', actors: new Set(['user_bob']) },
    });
    expect(result).toHaveLength(4);
  });

  it('returns all rows when mode is all', () => {
    const result = applyWorkOrderCreatorFilter(ROWS, {
      ...baseOpts,
      creatorFilter: { mode: 'all' },
    });
    expect(result).toHaveLength(4);
  });

  it('filters by picked actors, keeping rows without created_by', () => {
    const result = applyWorkOrderCreatorFilter(ROWS, {
      ...baseOpts,
      creatorFilter: { mode: 'pick', actors: new Set(['user_bob']) },
    });
    // user_bob + empty created_by + missing created_by
    expect(result.map((r) => r.id)).toEqual(['2', '3', '4']);
  });

  it('defaults to selfId when actors set is empty', () => {
    const result = applyWorkOrderCreatorFilter(ROWS, {
      ...baseOpts,
      creatorFilter: { mode: 'pick', actors: new Set() },
    });
    // user_alice + empty + missing
    expect(result.map((r) => r.id)).toEqual(['1', '3', '4']);
  });

  it('returns all when actors empty and no selfId', () => {
    const result = applyWorkOrderCreatorFilter(ROWS, {
      ...baseOpts,
      selfId: undefined,
      creatorFilter: { mode: 'pick', actors: new Set() },
    });
    expect(result).toHaveLength(4);
  });
});

describe('isYoCreatorFilterActive', () => {
  it('returns true when pick mode with empty actors and selfId', () => {
    const filter: CreatorFilterState = { mode: 'pick', actors: new Set() };
    expect(isYoCreatorFilterActive(filter, 'user_me')).toBe(true);
  });

  it('returns true when pick mode with only selfId in actors', () => {
    const filter: CreatorFilterState = { mode: 'pick', actors: new Set(['user_me']) };
    expect(isYoCreatorFilterActive(filter, 'user_me')).toBe(true);
  });

  it('returns false when pick mode with other actors', () => {
    const filter: CreatorFilterState = { mode: 'pick', actors: new Set(['user_other']) };
    expect(isYoCreatorFilterActive(filter, 'user_me')).toBe(false);
  });

  it('returns false when mode is all', () => {
    expect(isYoCreatorFilterActive({ mode: 'all' }, 'user_me')).toBe(false);
  });

  it('returns false when selfId is undefined', () => {
    const filter: CreatorFilterState = { mode: 'pick', actors: new Set() };
    expect(isYoCreatorFilterActive(filter, undefined)).toBe(false);
  });
});
