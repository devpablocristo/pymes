import { useCallback, useDeferredValue, useState } from 'react';
import { useCrudArchivedSearchParam } from './useCrudArchivedSearchParam';
import {
  useCrudRemotePaginatedList,
  type CrudRemotePaginatedPayload,
} from './useCrudRemotePaginatedList';

export type CrudRemoteGalleryPageFetchArgs = {
  limit: number;
  search: string;
  archived: boolean;
  after: string | null;
  signal: AbortSignal;
};

export function useCrudRemoteGalleryPage<T extends { id: string }>(options: {
  pageSize: number;
  fetchPage: (args: CrudRemoteGalleryPageFetchArgs) => Promise<CrudRemotePaginatedPayload<T>>;
}) {
  const { pageSize, fetchPage } = options;
  const { archived, setArchived, toggleArchived } = useCrudArchivedSearchParam();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [reloadKey, setReloadKey] = useState(0);
  const deferredSearch = useDeferredValue(search.trim());

  const paginated = useCrudRemotePaginatedList<T>({
    pageSize,
    deferredSearch,
    archived,
    reloadKey,
    fetchPage,
  });

  const reload = useCallback(async () => {
    setReloadKey((value) => value + 1);
  }, []);

  const selectItem = useCallback((itemOrId: T | string | null) => {
    if (itemOrId == null) {
      setSelectedId(null);
      return;
    }
    setSelectedId(typeof itemOrId === 'string' ? itemOrId : itemOrId.id);
  }, []);

  const closeDetail = useCallback(() => {
    setSelectedId(null);
  }, []);

  const handleArchiveToggle = useCallback(() => {
    closeDetail();
    toggleArchived();
  }, [closeDetail, toggleArchived]);

  return {
    ...paginated,
    archived,
    setArchived,
    toggleArchived,
    search,
    setSearch,
    deferredSearch,
    reload,
    reloadKey,
    selectedId,
    setSelectedId,
    selectItem,
    closeDetail,
    handleArchiveToggle,
  };
}
