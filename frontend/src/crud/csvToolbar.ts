import type { CrudFormField, CrudFormValues, CrudPageConfig } from '../components/CrudPage';
import { confirmAction } from '@devpablocristo/core-browser';
import {
  buildCSV,
  downloadCSVFile,
  normalizeCSVFieldValue,
  parseCSV,
  pickCSVFile,
  type CSVColumn,
} from '@devpablocristo/modules-crud-ui/csv';
import { apiRequest, downloadAPIFile } from '../lib/api';

type CSVMode = 'client' | 'server';

export type CSVToolbarOptions = {
  mode?: CSVMode;
  entity?: string;
  allowImport?: boolean;
  allowExport?: boolean;
  importMode?: 'create_only' | 'upsert';
  fileName?: string;
  columns?: CSVColumn[];
};

type ImportPreview = {
  preview_id: string;
  total_rows: number;
  valid_rows: number;
  error_rows: number;
  errors: Array<{ row: number; message: string; column?: string }>;
};

function defaultColumns<T extends { id: string }>(config: CrudPageConfig<T>): CSVColumn[] {
  return config.formFields.map((field) => ({ key: field.key, label: field.label }));
}

function valuesFromRow<T extends { id: string }>(
  config: CrudPageConfig<T>,
  row: T,
  columns: CSVColumn[],
): Record<string, string> {
  const values = config.toFormValues(row);
  return columns.reduce<Record<string, string>>((accumulator, column) => {
    const rawValue = values[column.key];
    accumulator[column.key] = typeof rawValue === 'boolean' ? (rawValue ? 'true' : 'false') : String(rawValue ?? '');
    return accumulator;
  }, {});
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

function fieldMap(fields: CrudFormField[]): Map<string, CrudFormField> {
  return new Map(fields.map((field) => [field.key, field]));
}

async function importClientCSV<T extends { id: string }>(
  config: CrudPageConfig<T>,
  columns: CSVColumn[],
): Promise<{ created: number; failed: number }> {
  const file = await pickCSVFile();
  if (!file) {
    return { created: 0, failed: 0 };
  }
  const rows = parseCSV(await file.text());
  if (rows.length === 0) {
    throw new Error('El archivo CSV no tiene filas para importar');
  }

  const fieldsByKey = fieldMap(config.formFields);
  const summary = await confirmAction({
    title: 'Importar CSV',
    description: `Se importaran ${rows.length} filas de ${file.name}. ¿Continuar?`,
    confirmLabel: 'Importar',
    cancelLabel: 'Cancelar',
  });
  if (!summary) {
    return { created: 0, failed: 0 };
  }

  let created = 0;
  let failed = 0;
  for (const row of rows) {
    const values = columns.reduce<CrudFormValues>((accumulator, column) => {
      const field = fieldsByKey.get(column.key);
      accumulator[column.key] = normalizeCSVFieldValue(row[column.key] ?? '', field?.type);
      return accumulator;
    }, {});
    try {
      await createFromValues(config, values);
      created += 1;
    } catch {
      failed += 1;
    }
  }
  return { created, failed };
}

async function importServerCSV(
  entity: string,
  importMode: 'create_only' | 'upsert',
): Promise<{ created: number; updated: number; skipped: number }> {
  const file = await pickCSVFile();
  if (!file) {
    return { created: 0, updated: 0, skipped: 0 };
  }
  const formData = new FormData();
  formData.append('file', file);
  const preview = await apiRequest<ImportPreview>(`/v1/import/${entity}/preview`, {
    method: 'POST',
    rawBody: formData,
    skipJSONContentType: true,
  });
  const firstErrors = preview.errors
    .slice(0, 3)
    .map((error) => `fila ${error.row}: ${error.message}`)
    .join('\n');
  const confirmed = await confirmAction({
    title: 'Confirmar importación',
    description: [
      `Archivo: ${file.name}`,
      `Total: ${preview.total_rows}`,
      `Validas: ${preview.valid_rows}`,
      `Con errores: ${preview.error_rows}`,
      firstErrors ? `Errores:\n${firstErrors}` : '',
      '¿Continuar con la importacion?',
    ]
      .filter(Boolean)
      .join('\n\n'),
    confirmLabel: 'Continuar',
    cancelLabel: 'Cancelar',
  });
  if (!confirmed) {
    return { created: 0, updated: 0, skipped: 0 };
  }
  const result = await apiRequest<{ created: number; updated: number; skipped: number }>(
    `/v1/import/${entity}/confirm`,
    {
      method: 'POST',
      body: { preview_id: preview.preview_id, mode: importMode },
    },
  );
  return result;
}

export function withCSVToolbar<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
  options: CSVToolbarOptions = {},
): CrudPageConfig<T> {
  const mode = options.mode ?? 'client';
  const entity = options.entity ?? resourceId;
  const columns = options.columns ?? defaultColumns(config);
  const toolbarActions = [...(config.toolbarActions ?? [])];

  if (options.allowImport ?? Boolean(config.dataSource?.create || config.basePath)) {
    toolbarActions.unshift({
      id: 'csv-import',
      label: 'Importar CSV',
      kind: 'secondary',
      onClick: async ({ reload }) => {
        if (mode === 'server') {
          const result = await importServerCSV(entity, options.importMode ?? 'upsert');
          await reload();
          window.alert(
            `Importacion completada. Creados: ${result.created}. Actualizados: ${result.updated}. Omitidos: ${result.skipped}.`,
          );
          return;
        }
        const result = await importClientCSV(config, columns);
        await reload();
        window.alert(`Importacion completada. Creados: ${result.created}. Fallidos: ${result.failed}.`);
      },
    });
  }

  if (options.allowExport ?? true) {
    toolbarActions.unshift({
      id: 'csv-export',
      label: 'Exportar CSV',
      kind: 'secondary',
      onClick: async ({ items }) => {
        if (mode === 'server') {
          await downloadAPIFile(`/v1/export/${entity}?format=csv`);
          return;
        }
        const content = buildCSV(
          columns,
          items.map((row) => valuesFromRow(config, row, columns)),
        );
        downloadCSVFile(options.fileName ?? `${entity}.csv`, content);
      },
    });
  }

  return { ...config, toolbarActions };
}
