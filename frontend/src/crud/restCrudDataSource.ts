import type { CrudDataSource, CrudFormValues } from '../components/CrudPage';
import { apiRequest } from '../lib/api';

/** Base paths donde archivar = POST `…/archive` y DELETE ítem = borrado duro (alineado al backend core). */
export const REST_ARCHIVE_VIA_POST_BASE_PATHS = new Set<string>(['/v1/products', '/v1/services']);

type ArchiveMode = 'archive' | 'hard-only';

/** Archivar: DELETE ítem (customers, suppliers, quotes…) vs POST …/archive (products, services). */
export type RestSoftArchiveHttp = 'delete_item' | 'post_archive';

/** Borrado definitivo desde vista archivados: DELETE …/hard vs DELETE ítem (solo filas ya archivadas). */
export type RestHardDeleteHttp = 'suffix_hard' | 'delete_item';

type RestCrudDataSourceInput = {
  basePath: string;
  toBody: (values: CrudFormValues) => Record<string, unknown>;
  /**
   * `archive` (default): soft-archive + restore + hard delete según `softArchiveHttp` / `hardDeleteHttp`.
   * `hard-only`: DELETE duro vía `{basePath}/{id}`; sin archive/restore (para recursos que no los soportan en backend).
   */
  archiveMode?: ArchiveMode;
  /** Por defecto `delete_item` (DELETE sobre el recurso = soft-delete/archive en esos backends). */
  softArchiveHttp?: RestSoftArchiveHttp;
  /** Por defecto `suffix_hard` (DELETE `{basePath}/{id}/hard`). */
  hardDeleteHttp?: RestHardDeleteHttp;
};

/**
 * dataSource REST canónico: POST/PATCH con `toBody` único y operaciones de archive según backend.
 * Sigue las 7 operaciones CRUD del core (Create/Update/Delete/Archive/Restore).
 */
export function buildRestCrudDataSource<T extends { id: string }>({
  basePath,
  toBody,
  archiveMode = 'archive',
  softArchiveHttp = 'delete_item',
  hardDeleteHttp = 'suffix_hard',
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
        if (softArchiveHttp === 'post_archive') {
          await apiRequest(`${basePath}/${row.id}/archive`, { method: 'POST', body: {} });
        } else {
          await apiRequest(`${basePath}/${row.id}`, { method: 'DELETE' });
        }
      },
      restore: async (row) => {
        await apiRequest(`${basePath}/${row.id}/restore`, { method: 'POST', body: {} });
      },
      hardDelete: async (row) => {
        if (hardDeleteHttp === 'delete_item') {
          await apiRequest(`${basePath}/${row.id}`, { method: 'DELETE' });
        } else {
          await apiRequest(`${basePath}/${row.id}/hard`, { method: 'DELETE' });
        }
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
