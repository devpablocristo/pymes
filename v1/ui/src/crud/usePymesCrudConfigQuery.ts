import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { CrudPageConfig } from '../components/CrudPage';
import { CRUD_UI_CHANGE_EVENT } from '../lib/crudUiConfig';
import { loadLazyCrudPageConfig } from './lazyCrudPage';
import { useCrudConfigQuery } from '../modules/crud';

export function usePymesCrudConfigQuery<TRecord extends { id: string } = { id: string }>(resourceId: string) {
  const queryClient = useQueryClient();

  useEffect(() => {
    function invalidateAndRefetch() {
      void queryClient.invalidateQueries({ queryKey: ['crud-config', resourceId] });
      void queryClient.refetchQueries({ queryKey: ['crud-config', resourceId] });
    }
    window.addEventListener(CRUD_UI_CHANGE_EVENT, invalidateAndRefetch);
    return () => {
      window.removeEventListener(CRUD_UI_CHANGE_EVENT, invalidateAndRefetch);
    };
  }, [queryClient, resourceId]);

  return useCrudConfigQuery<CrudPageConfig<TRecord> | null>(
    resourceId,
    (id) => loadLazyCrudPageConfig<TRecord>(id),
  );
}
