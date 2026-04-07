import type { CrudFormValues, CrudPageConfig } from '../components/CrudPage';
import { confirmAction } from '@devpablocristo/core-browser';
import type { CSVColumn } from '@devpablocristo/modules-crud-ui/csv';
import {
  mergeCsvToolbarConfig,
  type CrudCsvServerExportPort,
  type CrudCsvServerImportPort,
  type CrudCsvToolbarUiPort,
  type CsvToolbarMergeMode,
} from '@devpablocristo/modules-crud-ui';
import { apiRequest, downloadAPIFile } from '../lib/api';

export type CSVToolbarOptions = {
  mode?: CsvToolbarMergeMode;
  entity?: string;
  allowImport?: boolean;
  allowExport?: boolean;
  importMode?: 'create_only' | 'upsert';
  fileName?: string;
  columns?: CSVColumn[];
  /** Sustituye el export dataio del core (p. ej. auditoría). */
  serverExport?: CrudCsvServerExportPort;
  /** Sustituye import dataio del core. */
  serverImport?: CrudCsvServerImportPort;
};

const pymesCsvUi: CrudCsvToolbarUiPort = {
  confirmClientImport: (fileName, rowCount) =>
    confirmAction({
      title: 'Importar CSV',
      description: `Se importaran ${rowCount} filas de ${fileName}. ¿Continuar?`,
      confirmLabel: 'Importar',
      cancelLabel: 'Cancelar',
    }).then(Boolean),
  confirmServerImport: (description) =>
    confirmAction({
      title: 'Confirmar importación',
      description,
      confirmLabel: 'Continuar',
      cancelLabel: 'Cancelar',
    }).then(Boolean),
  notify: (message) => {
    window.alert(message);
  },
};

const spanishCsvMessages = {
  importClientDone: (r: { created: number; failed: number }) =>
    `Importacion completada. Creados: ${r.created}. Fallidos: ${r.failed}.`,
  importServerDone: (r: { created: number; updated: number; skipped: number }) =>
    `Importacion completada. Creados: ${r.created}. Actualizados: ${r.updated}. Omitidos: ${r.skipped}.`,
};

function createCoreDataioServerImportPort(): CrudCsvServerImportPort {
  return {
    preview: (entity, file) => {
      const formData = new FormData();
      formData.append('file', file);
      return apiRequest(`/v1/import/${entity}/preview`, {
        method: 'POST',
        rawBody: formData,
        skipJSONContentType: true,
      });
    },
    confirm: (entity, previewId, mode) =>
      apiRequest(`/v1/import/${entity}/confirm`, {
        method: 'POST',
        body: { preview_id: previewId, mode },
      }),
  };
}

function createCoreDataioServerExportPort(): CrudCsvServerExportPort {
  return {
    download: async (entity) => {
      await downloadAPIFile(`/v1/export/${entity}?format=csv`);
    },
  };
}

async function createFromValues<T extends { id: string }>(
  config: CrudPageConfig<T>,
  values: CrudFormValues,
): Promise<void> {
  if (config.dataSource?.create) {
    await config.dataSource.create(values);
    return;
  }
  if (!config.basePath) {
    throw new Error('El recurso no tiene endpoint de creación configurado');
  }
  await apiRequest(config.basePath, {
    method: 'POST',
    body: config.toBody ? config.toBody(values) : values,
  });
}

export function withCSVToolbar<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
  options: CSVToolbarOptions = {},
): CrudPageConfig<T> {
  const mode = options.mode ?? 'client';
  const entity = options.entity ?? resourceId;
  const defaultAllowImport = Boolean(config.dataSource?.create || config.basePath);
  const allowImport = options.allowImport ?? defaultAllowImport;

  const serverImport =
    options.serverImport ??
    (mode === 'server' && allowImport ? createCoreDataioServerImportPort() : undefined);
  const serverExport =
    options.serverExport ?? (mode === 'server' ? createCoreDataioServerExportPort() : undefined);

  return mergeCsvToolbarConfig({
    config,
    entity,
    mode,
    columns: options.columns,
    allowImport: options.allowImport,
    allowExport: options.allowExport,
    importMode: options.importMode ?? 'upsert',
    fileName: options.fileName,
    serverImport,
    serverExport,
    ui: pymesCsvUi,
    importClientRow: (values) => createFromValues(config, values),
    messages: spanishCsvMessages,
  });
}
