import type { CrudPageConfig } from '../components/CrudPage';
import { loadLazyCrudPageConfig } from './lazyCrudPage';
import { useCrudConfigQuery } from '../modules/crud';

export function usePymesCrudConfigQuery<TRecord extends { id: string } = { id: string }>(resourceId: string) {
  return useCrudConfigQuery<CrudPageConfig<TRecord> | null>(
    resourceId,
    (id) => loadLazyCrudPageConfig<TRecord>(id),
  );
}
