import { useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';

export function useCrudArchivedSearchParam({
  paramName = 'archived',
  archivedValue = '1',
}: {
  paramName?: string;
  archivedValue?: string;
} = {}) {
  const [searchParams, setSearchParams] = useSearchParams();

  const archived = useMemo(
    () => searchParams.get(paramName) === archivedValue,
    [archivedValue, paramName, searchParams],
  );

  function setArchived(nextArchived: boolean) {
    setSearchParams((prev) => {
      const params = new URLSearchParams(prev);
      if (nextArchived) {
        params.set(paramName, archivedValue);
      } else {
        params.delete(paramName);
      }
      return params;
    });
  }

  function toggleArchived() {
    setArchived(!archived);
  }

  return { archived, setArchived, toggleArchived };
}
