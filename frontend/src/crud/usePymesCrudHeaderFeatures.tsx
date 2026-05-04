import { useDeferredValue, useMemo, useState } from 'react';
import type { CrudPageConfig } from '../components/CrudPage';
import { TagPillsBar } from '../components/TagPillsBar';
import { CrudValueFilterSelector, resolveCrudValueFilterOptions } from '../modules/crud';
import { useCrudListCreatedByMerge } from '../lib/useCrudListCreatedByMerge';
import { usePymesCrudConfigQuery } from './usePymesCrudConfigQuery';

type Options<T extends { id: string }> = {
  resourceId: string;
  items: T[];
  matchesSearch: (row: T, query: string) => boolean;
  enableCreatorFilter?: boolean;
  crudConfigOverride?: CrudPageConfig<T> | null;
  search?: string;
  setSearch?: (value: string) => void;
};

const WORK_ORDER_STATUS_LABELS: Record<string, string> = {
  received: 'Recibido',
  diagnosing: 'Diagnóstico',
  quote_pending: 'Presupuesto',
  awaiting_parts: 'Repuestos',
  in_progress: 'En taller',
  quality_check: 'Control',
  ready_for_pickup: 'Listo retiro',
  delivered: 'Entregado',
  invoiced: 'Facturado',
  on_hold: 'En pausa',
  cancelled: 'Cancelado',
};

export function usePymesCrudHeaderFeatures<T extends { id: string; created_by?: string }>({
  resourceId,
  items,
  matchesSearch,
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
  const [tagFilter, setTagFilter] = useState('all');
  const search = externalSearch ?? internalSearch;
  const setSearch = externalSetSearch ?? setInternalSearch;
  const deferredSearch = useDeferredValue(search.trim().toLowerCase());
  const normalizedTagValues = useMemo(
    () =>
      Array.from(
        new Set(
          items.flatMap((row) => {
            const rec = row as Record<string, unknown>;
            const tags = Array.isArray(rec.tags) ? rec.tags : [];
            return tags.map((raw) => String(raw ?? '').trim()).filter(Boolean);
          }),
        ),
      ).sort((a, b) => a.localeCompare(b)),
    [items],
  );

  const hasCreatorSignals = useMemo(
    () => items.some((row) => typeof row.created_by === 'string' && row.created_by.trim().length > 0),
    [items],
  );
  const hasTagSignals = normalizedTagValues.length > 0;
  const tagPillsFeatureOn = crudConfig?.featureFlags?.tagPills !== false;
  /**
   * Solo mostrar chips de etiquetas internas cuando hay valores que filtrar.
   * Si no, una segunda fila con solo «Todos» duplica la franja de responsable (`CreatedByPillsBar`)
   * y confunde (mismo label, distinta dimensión).
   */
  const showTagPills = tagPillsFeatureOn && hasTagSignals;

  const creatorFilterEnabled =
    enableCreatorFilter && crudConfig?.featureFlags?.creatorFilter !== false && hasCreatorSignals;
  const headerQuickFilterStripEnabled = creatorFilterEnabled && listHeaderInlineSlot != null;

  const creatorFilteredItems = useMemo(
    () => (creatorFilterEnabled && preSearchFilter ? preSearchFilter(items) : items),
    [creatorFilterEnabled, items, preSearchFilter],
  );

  const supplierCategoryFilterOptions = useMemo(
    () =>
      resourceId !== 'suppliers'
        ? []
        : Array.from(
            new Set(
              items
                .map((row) => {
                  const rec = row as Record<string, unknown>;
                  const metadata = rec.metadata as Record<string, unknown> | undefined;
                  return typeof metadata?.category === 'string' ? metadata.category.trim() : '';
                })
                .filter(Boolean),
            ),
          )
            .sort((a, b) => a.localeCompare(b))
            .map((category) => ({
              value: category,
              label: category,
              matches: (row: T) => {
                const rec = row as Record<string, unknown>;
                const metadata = rec.metadata as Record<string, unknown> | undefined;
                return (typeof metadata?.category === 'string' ? metadata.category.trim() : '') === category;
              },
            })),
    [items, resourceId],
  );

  const workOrderStatusFilterOptions = useMemo(
    () =>
      resourceId !== 'carWorkOrders' && resourceId !== 'bikeWorkOrders'
        ? []
        : Array.from(
            new Set(
              items
                .map((row) => {
                  const rec = row as Record<string, unknown>;
                  return typeof rec.status === 'string' ? rec.status.trim() : '';
                })
                .filter(Boolean),
            ),
          )
            .sort((a, b) => a.localeCompare(b))
            .map((status) => ({
              value: status,
              label: WORK_ORDER_STATUS_LABELS[status] ?? status,
              matches: (row: T) => {
                const rec = row as Record<string, unknown>;
                return (typeof rec.status === 'string' ? rec.status.trim() : '') === status;
              },
            })),
    [items, resourceId],
  );

  const tagFilteredItems = useMemo(() => {
    if (tagFilter === 'all' || !hasTagSignals) return creatorFilteredItems;
    return creatorFilteredItems.filter((row) => {
      const rec = row as Record<string, unknown>;
      const tags = Array.isArray(rec.tags) ? rec.tags : [];
      return tags.some((raw) => String(raw ?? '').trim() === tagFilter);
    });
  }, [creatorFilteredItems, hasTagSignals, tagFilter]);

  const resolvedValueFilterOptions = resolveCrudValueFilterOptions(crudConfig);
  const stateFilterEnabled = resolvedValueFilterOptions.length > 0;
  const workOrderStateFilterEnabled = workOrderStatusFilterOptions.length > 0;
  const categoryFilterEnabled = supplierCategoryFilterOptions.length > 0;
  const valueFilterEnabled =
    crudConfig?.featureFlags?.valueFilter !== false &&
    (stateFilterEnabled || workOrderStateFilterEnabled || categoryFilterEnabled);
  const valueFilterOptions = stateFilterEnabled
    ? resolvedValueFilterOptions
    : workOrderStateFilterEnabled
      ? workOrderStatusFilterOptions
      : supplierCategoryFilterOptions;

  const valueFilteredItems = useMemo(() => {
    if (!valueFilterEnabled || valueFilter === 'all') return tagFilteredItems;
    const selectedOption = valueFilterOptions.find((option) => option.value === valueFilter);
    if (!selectedOption) return tagFilteredItems;
    return tagFilteredItems.filter((row) => selectedOption.matches(row));
  }, [tagFilteredItems, valueFilter, valueFilterEnabled, valueFilterOptions]);

  const visibleItems = useMemo(() => {
    if (!deferredSearch) return valueFilteredItems;
    return valueFilteredItems.filter((row) => matchesSearch(row, deferredSearch));
  }, [deferredSearch, matchesSearch, valueFilteredItems]);

  const searchInlineActions = valueFilterEnabled ? (
    <CrudValueFilterSelector<T>
      value={valueFilter}
      onChange={setValueFilter}
      options={valueFilterOptions}
      className="crud-status-selector"
      ariaLabel={
        stateFilterEnabled || workOrderStateFilterEnabled ? 'Filtrar por estado' : 'Filtrar por categoría'
      }
    />
  ) : null;

  const creatorStripEl =
    headerQuickFilterStripEnabled && listHeaderInlineSlot ? listHeaderInlineSlot({ items }) : null;
  const tagPillsEl = showTagPills ? (
    <TagPillsBar tags={normalizedTagValues} value={tagFilter} onChange={setTagFilter} />
  ) : null;

  const headerLeadSlot =
    creatorStripEl || tagPillsEl ? (
      <div
        className={
          creatorStripEl && tagPillsEl
            ? 'crud-list-header-lead crud-list-header-lead--stacked'
            : 'crud-list-header-lead'
        }
      >
        {creatorStripEl}
        {tagPillsEl}
      </div>
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
