import { useCallback, useEffect, useState } from 'react';

export type CrudRemotePaginatedPayload<T> = {
  items: T[];
  total?: number;
  has_more?: boolean;
  next_cursor?: string | null;
};

/**
 * Lista remota paginada con búsqueda y bandera archivados; invalidación vía `reloadKey`.
 */
export function useCrudRemotePaginatedList<T extends { id: string }>(options: {
  pageSize: number;
  deferredSearch: string;
  archived: boolean;
  reloadKey: number;
  fetchPage: (args: {
    limit: number;
    search: string;
    archived: boolean;
    after: string | null;
    signal: AbortSignal;
  }) => Promise<CrudRemotePaginatedPayload<T>>;
}) {
  const { pageSize, deferredSearch, archived, reloadKey, fetchPage } = options;
  const [items, setItems] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    setLoading(true);
    setLoadingMore(false);
    void fetchPage({
      limit: pageSize,
      search: deferredSearch,
      archived,
      after: null,
      signal: controller.signal,
    })
      .then((data) => {
        if (cancelled) return;
        setItems(data.items ?? []);
        setTotal(Number(data.total ?? 0));
        setHasMore(Boolean(data.has_more));
        setNextCursor(data.next_cursor?.trim() || null);
        setError(null);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        if (controller.signal.aborted) return;
        setError(e instanceof Error ? e.message : 'Error');
        setItems([]);
        setHasMore(false);
        setNextCursor(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [archived, deferredSearch, fetchPage, pageSize, reloadKey]);

  const loadMore = useCallback(async () => {
    const cursor = nextCursor;
    if (!cursor || loadingMore || !hasMore) return;
    setLoadingMore(true);
    setError(null);
    try {
      const data = await fetchPage({
        limit: pageSize,
        search: deferredSearch,
        archived,
        after: cursor,
        signal: new AbortController().signal,
      });
      setItems((prev) => [...prev, ...(data.items ?? [])]);
      setHasMore(Boolean(data.has_more));
      setNextCursor(data.next_cursor?.trim() || null);
      setTotal((prev) => Number(data.total ?? prev));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Error');
    } finally {
      setLoadingMore(false);
    }
  }, [archived, deferredSearch, fetchPage, hasMore, loadingMore, nextCursor, pageSize]);

  return {
    items,
    setItems,
    loading,
    error,
    setError,
    total,
    hasMore,
    nextCursor,
    loadingMore,
    loadMore,
  };
}
