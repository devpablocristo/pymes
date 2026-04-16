import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';

export function useCrudRemoteListState<T extends { id: string }>(options: {
  queryKey: readonly unknown[];
  list: () => Promise<T[]>;
  loadErrorMessage?: string;
}) {
  const { queryKey, list, loadErrorMessage = 'Error al cargar datos' } = options;
  const queryClient = useQueryClient();
  const [items, setItems] = useState<T[]>([]);
  const [error, setError] = useState<string | null>(null);

  const query = useQuery({
    queryKey,
    queryFn: list,
    refetchOnWindowFocus: false,
    staleTime: Infinity,
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

  return { items, setItems, error, setError, loading: query.isLoading, reload };
}
