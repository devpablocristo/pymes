import type { QueryClient, QueryKey } from '@tanstack/react-query';
import { useCallback, type Dispatch, type SetStateAction } from 'react';

/**
 * Sincroniza ítems de un listado con react-query y con estado local (p. ej. tablero filtrado),
 * típico tras guardar en modal o borrar registro.
 */
export function useCrudQueryListCacheSync<T extends { id: string }>({
  queryClient,
  queryKey,
  setItems,
}: {
  queryClient: QueryClient;
  queryKey: QueryKey;
  setItems: Dispatch<SetStateAction<T[]>>;
}) {
  const upsertInListCache = useCallback(
    (item: T) => {
      queryClient.setQueryData<T[]>(queryKey, (current) => (current ?? []).map((row) => (row.id === item.id ? item : row)));
      setItems((prev) => prev.map((x) => (x.id === item.id ? item : x)));
    },
    [queryClient, queryKey, setItems],
  );

  const removeFromListCache = useCallback(
    (id: string) => {
      queryClient.setQueryData<T[]>(queryKey, (current) => (current ?? []).filter((row) => row.id !== id));
      setItems((prev) => prev.filter((x) => x.id !== id));
    },
    [queryClient, queryKey, setItems],
  );

  return { upsertInListCache, removeFromListCache };
}
