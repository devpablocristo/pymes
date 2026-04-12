/**
 * Política CSV por recurso: misma entidad que el módulo desplegado (`resourceId`),
 * salvo overrides declarativos en `CRUD_CSV_RESOURCE_EXTRAS`.
 *
 * Dataio (import/export servidor) solo admite ciertas entidades en el core; el resto
 * usa modo cliente (export desde filas cargadas; import solo si hay alta vía API).
 */
import type { CrudPageConfig } from '../components/CrudPage';
import type { CSVColumn } from '@devpablocristo/modules-crud-ui/csv';
import type { CSVToolbarOptions } from './csvToolbar';

/** Debe coincidir con `supportsImport` / export en `pymes-core/internal/dataio`. */
export const DATAIO_SERVER_ENTITY_IDS = new Set(['customers', 'products', 'suppliers']);

function columnsFromCrudTable<T extends { id: string }>(config: CrudPageConfig<T>): CSVColumn[] {
  return (config.columns ?? []).map((col) => ({
    key: col.key,
    label: typeof col.header === 'string' ? col.header : String(col.key),
  }));
}

/**
 * Opciones para `withCSVToolbar(resourceId, config, …)` según `resourceId`, capacidad del CRUD
 * y overrides en `CRUD_CSV_RESOURCE_EXTRAS`.
 */
export function mergeCsvOptionsForResource<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
): CSVToolbarOptions {
  const reg = CRUD_CSV_RESOURCE_EXTRAS[resourceId] ?? {};

  if (DATAIO_SERVER_ENTITY_IDS.has(resourceId)) {
    return { mode: 'server', entity: resourceId, ...reg };
  }

  const canCreateViaApi = Boolean(config.dataSource?.create || config.basePath);
  const fromTable = columnsFromCrudTable(config);
  const columns =
    reg.columns ?? (fromTable.length > 0 ? fromTable : undefined);

  return {
    mode: 'client',
    entity: resourceId,
    allowImport: 'allowImport' in reg ? Boolean(reg.allowImport) : canCreateViaApi,
    allowExport: 'allowExport' in reg ? Boolean(reg.allowExport) : true,
    fileName: reg.fileName ?? `${resourceId}.csv`,
    ...(columns && columns.length > 0 ? { columns } : {}),
    ...reg,
  };
}

/**
 * Overrides por `resourceId` (columnas mínimas, desactivar import, nombre de archivo).
 * Cualquier recurso nuevo puede declararse aquí sin tocar el `Object.entries` del mapa.
 */
export const CRUD_CSV_RESOURCE_EXTRAS: Record<string, Partial<CSVToolbarOptions> | undefined> = {
  invoices: {
    fileName: 'facturacion.csv',
    columns: [
      { key: 'number', label: 'number' },
      { key: 'customer', label: 'customer' },
      { key: 'issuedDate', label: 'issuedDate' },
      { key: 'dueDate', label: 'dueDate' },
      { key: 'status', label: 'status' },
      { key: 'discount', label: 'discount' },
      { key: 'tax', label: 'tax' },
      { key: 'items_json', label: 'items_json' },
    ],
  },
  creditNotes: {
    columns: [
      { key: 'party_id', label: 'party_id (UUID)' },
      { key: 'amount', label: 'amount' },
    ],
  },
  /** Inventario: export desde filas cargadas; import vía dataio del catálogo `products`. */
  inventory: {
    allowImport: true,
    importUsesServer: true,
    importEntity: 'products',
    fileName: 'inventario.csv',
    columns: [
      { key: 'product_id', label: 'product_id' },
      { key: 'product_name', label: 'product_name' },
      { key: 'sku', label: 'sku' },
      { key: 'quantity', label: 'quantity' },
      { key: 'min_quantity', label: 'min_quantity' },
      { key: 'is_low_stock', label: 'is_low_stock' },
      { key: 'updated_at', label: 'updated_at' },
    ],
  },
};
