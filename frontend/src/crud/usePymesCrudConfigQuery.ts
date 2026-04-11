import type { CrudPageConfig } from '../components/CrudPage';
import { loadLazyCrudPageConfig, type LoadLazyCrudPageConfigOptions } from './lazyCrudPage';
import { useCrudConfigQuery } from '../modules/crud';

export function usePymesCrudConfigQuery<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
  options?: LoadLazyCrudPageConfigOptions,
) {
  return useCrudConfigQuery<CrudPageConfig<TRecord> | null>(
    resourceId,
    (id, opts) => loadLazyCrudPageConfig<TRecord>(id, opts as LoadLazyCrudPageConfigOptions | undefined),
    options,
  );
}
