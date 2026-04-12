import { useDeferredValue, useMemo, useState } from 'react';
import type { CrudPageConfig, CrudValueFilterOption } from '../components/CrudPage';
import { CrudValueFilterSelector } from '../modules/crud';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';
import { usePymesCrudConfigQuery } from './usePymesCrudConfigQuery';

type Options<T extends { id: string }> = {
  resourceId: string;
  items: T[];
  matchesSearch: (row: T, query: string) => boolean;
  valueFilterOptions?: CrudValueFilterOption<T>[];
  enableCreatorFilter?: boolean;
  crudConfigOverride?: CrudPageConfig<T> | null;
  search?: string;
  setSearch?: (value: string) => void;
};

export function usePymesCrudHeaderFeatures<T extends { id: string; created_by?: string }>({
  resourceId,
  items,
  matchesSearch,
  valueFilterOptions,
  enableCreatorFilter = true,
  crudConfigOverride,
  search: externalSearch,
  setSearch: externalSetSearch,
}: Options<T>) {
  const crudConfigQuery = usePymesCrudConfigQuery<T>(resourceId);
  const crudConfig = (crudConfigOverride ?? crudConfigQuery.data ?? null) as CrudPageConfig<T> | null;
  const { preSearchFilter, listHeaderInlineSlot } = useCrudListCreatedByMerge();
  const [internalSearch, setInternalSearch] = useState('');
  const [valueFilter, setValueFilter] = useState('all');
  const search = externalSearch ?? internalSearch;
  const setSearch = externalSetSearch ?? setInternalSearch;
  const deferredSearch = useDeferredValue(search.trim().toLowerCase());

  const creatorFilterEnabled = enableCreatorFilter && crudConfig?.featureFlags?.creatorFilter !== false;
  const headerQuickFilterStripEnabled =
    creatorFilterEnabled &&
    listHeaderInlineSlot != null &&
    crudConfig?.featureFlags?.headerQuickFilterStrip !== false;

  const creatorFilteredItems = useMemo(
    () => (creatorFilterEnabled && preSearchFilter ? preSearchFilter(items) : items),
    [creatorFilterEnabled, items, preSearchFilter],
  );

  const resolvedValueFilterOptions = (valueFilterOptions ?? crudConfig?.valueFilterOptions ?? []) as CrudValueFilterOption<T>[];
  const valueFilterEnabled =
    resolvedValueFilterOptions.length > 0 && crudConfig?.featureFlags?.valueFilter !== false;

  const valueFilteredItems = useMemo(() => {
    if (!valueFilterEnabled || valueFilter === 'all') return creatorFilteredItems;
    const selectedOption = resolvedValueFilterOptions.find((option) => option.value === valueFilter);
    if (!selectedOption) return creatorFilteredItems;
    return creatorFilteredItems.filter((row) => selectedOption.matches(row));
  }, [creatorFilteredItems, resolvedValueFilterOptions, valueFilter, valueFilterEnabled]);

  const visibleItems = useMemo(() => {
    if (!deferredSearch) return valueFilteredItems;
    return valueFilteredItems.filter((row) => matchesSearch(row, deferredSearch));
  }, [deferredSearch, matchesSearch, valueFilteredItems]);

  const searchInlineActions = valueFilterEnabled ? (
    <CrudValueFilterSelector<T>
      value={valueFilter}
      onChange={setValueFilter}
      options={resolvedValueFilterOptions}
      className="crud-status-selector"
    />
  ) : null;

  const headerLeadSlot = headerQuickFilterStripEnabled ? (
    <div className="crud-list-header-lead">{listHeaderInlineSlot?.({ items })}</div>
  ) : undefined;

  return {
    crudConfig,
    search,
    setSearch,
    visibleItems,
    headerLeadSlot,
    searchInlineActions,
  };
}
