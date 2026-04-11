import { useQuery } from '@tanstack/react-query';
import type { CrudPageConfig } from '../../components/CrudPage';
import { loadLazyCrudPageConfig, type LoadLazyCrudPageConfigOptions } from '../../crud/lazyCrudPage';

export function useCrudConfigQuery<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
  options?: LoadLazyCrudPageConfigOptions,
) {
  return useQuery<CrudPageConfig<TRecord> | null>({
    queryKey: ['crud-config', resourceId, options?.preserveCsvToolbar ?? false],
    queryFn: () => loadLazyCrudPageConfig<TRecord>(resourceId, options),
  });
}
