import type { CrudDataSource, CrudFormValues } from '../components/CrudPage';
import { apiRequest } from '../lib/api';

type ArchiveMode = 'archive' | 'hard-only';

type RestCrudDataSourceInput = {
  basePath: string;
  toBody: (values: CrudFormValues) => Record<string, unknown>;
  /**
   * `archive` (default): DELETE soft vía `{basePath}/{id}`, con `restore` y `hardDelete` explícitos.
   * `hard-only`: DELETE duro vía `{basePath}/{id}`; sin archive/restore (para recursos que no los soportan en backend).
   */
  archiveMode?: ArchiveMode;
};

/**
 * dataSource REST canónico: POST/PATCH con `toBody` único y operaciones de archive según backend.
 * Sigue las 7 operaciones CRUD del core (Create/Update/Delete/Archive/Restore).
 */
export function buildRestCrudDataSource<T extends { id: string }>({
  basePath,
  toBody,
  archiveMode = 'archive',
}: RestCrudDataSourceInput): CrudDataSource<T> {
  const base: CrudDataSource<T> = {
    create: async (values) => {
      await apiRequest(basePath, { method: 'POST', body: toBody(values) });
    },
    update: async (row, values) => {
      await apiRequest(`${basePath}/${row.id}`, { method: 'PATCH', body: toBody(values) });
    },
  };

  if (archiveMode === 'archive') {
    return {
      ...base,
      deleteItem: async (row) => {
        await apiRequest(`${basePath}/${row.id}`, { method: 'DELETE' });
      },
      restore: async (row) => {
        await apiRequest(`${basePath}/${row.id}/restore`, { method: 'POST', body: {} });
      },
      hardDelete: async (row) => {
        await apiRequest(`${basePath}/${row.id}/hard`, { method: 'DELETE' });
      },
    };
  }

  return {
    ...base,
    deleteItem: async (row) => {
      await apiRequest(`${basePath}/${row.id}`, { method: 'DELETE' });
    },
  };
}
