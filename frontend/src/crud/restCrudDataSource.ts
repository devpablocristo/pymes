import type { CrudDataSource, CrudFormValues } from '../components/CrudPage';
import { apiRequest } from '../lib/api';

type RestCrudDataSourceInput = {
  basePath: string;
  toBody: (values: CrudFormValues) => Record<string, unknown>;
};

/**
 * dataSource REST canónico para recursos CRUD:
 * create/update, archive por DELETE, restore por POST restore y hard-delete por DELETE hard.
 */
export function buildRestCrudDataSource<T extends { id: string }>({
  basePath,
  toBody,
}: RestCrudDataSourceInput): CrudDataSource<T> {
  return {
    create: async (values) => {
      await apiRequest(basePath, { method: 'POST', body: toBody(values) });
    },
    update: async (row, values) => {
      await apiRequest(`${basePath}/${row.id}`, { method: 'PATCH', body: toBody(values) });
    },
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
