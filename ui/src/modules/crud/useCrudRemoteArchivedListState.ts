import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useCrudArchivedSearchParam } from './useCrudArchivedSearchParam';
import { useCrudQueryListCacheSync } from './useCrudQueryListCacheSync';

export function useCrudRemoteArchivedListState<T extends { id: string }>(options: {
  queryKey: readonly unknown[];
  listActive: () => Promise<T[]>;
  listArchived: () => Promise<T[]>;
  loadErrorMessage?: string;
}) {
  const { queryKey, listActive, listArchived, loadErrorMessage = 'Error al cargar datos' } = options;
  const queryClient = useQueryClient();
  const { archived: showArchived } = useCrudArchivedSearchParam();
  const [items, setItems] = useState<T[]>([]);
  const [error, setError] = useState<string | null>(null);

  const scopedQueryKey = useMemo(
    () => [...queryKey, showArchived ? 'archived' : 'active'] as const,
    [queryKey, showArchived],
  );

  const query = useQuery({
    queryKey: scopedQueryKey,
    queryFn: () => (showArchived ? listArchived() : listActive()),
    refetchOnWindowFocus: false,
    staleTime: Infinity,
  });

  const { upsertInListCache, removeFromListCache } = useCrudQueryListCacheSync<T>({
    queryClient,
    queryKey: scopedQueryKey,
    setItems,
  });

  useEffect(() => {
    if (query.data) {
      setItems(query.data);
      setError(null);
    }
    if (query.error) {
      setError(query.error instanceof Error ? query.error.message : loadErrorMessage);
    }
  }, [loadErrorMessage, query.data, query.error]);

  const reload = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey });
  }, [queryClient, queryKey]);

  return {
    showArchived,
    items,
    setItems,
    error,
    setError,
    loading: query.isLoading,
    reload,
    scopedQueryKey,
    upsertInListCache,
    removeFromListCache,
  };
}
