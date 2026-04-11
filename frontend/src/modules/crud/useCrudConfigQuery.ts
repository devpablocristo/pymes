import { useQuery } from '@tanstack/react-query';

export type CrudConfigQueryLoader<TConfig> = (resourceId: string, options?: unknown) => Promise<TConfig | null>;

export function useCrudConfigQuery<TConfig = unknown>(
  resourceId: string,
  loadConfig: CrudConfigQueryLoader<TConfig>,
  options?: unknown,
) {
  return useQuery<TConfig | null>({
    queryKey: ['crud-config', resourceId, JSON.stringify(options ?? null)],
    queryFn: () => loadConfig(resourceId, options),
  });
}
