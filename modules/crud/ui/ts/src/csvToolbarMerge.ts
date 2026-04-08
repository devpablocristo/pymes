/**
 * Hex-friendly CSV toolbar: orquestación agnóstica del producto.
 * Puertos inyectados para import/export servidor; cliente usa pickCSV + create por fila.
 */
import type { CrudFormField, CrudFormValues, CrudPageConfig, CrudToolbarAction } from "./types";
import { buildCSV, downloadCSVFile, normalizeCSVFieldValue, parseCSV, pickCSVFile, type CSVColumn } from "./csv";

export type CsvServerImportPreview = {
  preview_id: string;
  total_rows: number;
  valid_rows: number;
  error_rows: number;
  errors: Array<{ row: number; message: string; column?: string }>;
};

export type CsvServerImportResult = {
  created: number;
  updated: number;
  skipped: number;
};

/** Importación masiva vía backend (p. ej. dataio en core). */
export type CrudCsvServerImportPort = {
  preview(entity: string, file: File): Promise<CsvServerImportPreview>;
  confirm(entity: string, previewId: string, mode: "create_only" | "upsert"): Promise<CsvServerImportResult>;
};

/** Exportación masiva vía backend. */
export type CrudCsvServerExportPort = {
  download(entity: string): Promise<void>;
};

/** Diálogos / notificaciones (browser o app shell). */
export type CrudCsvToolbarUiPort = {
  confirmClientImport(fileName: string, rowCount: number): Promise<boolean>;
  confirmServerImport(description: string): Promise<boolean>;
  notify(message: string): void;
};

export type CsvToolbarMergeMode = "client" | "server";

export type CsvToolbarMessages = {
  importClientDone: (r: { created: number; failed: number }) => string;
  importServerDone: (r: CsvServerImportResult) => string;
};

export type MergeCsvToolbarParams<T extends { id: string }> = {
  config: CrudPageConfig<T>;
  /** Nombre lógico para rutas servidor y nombre de archivo export cliente. */
  entity: string;
  mode: CsvToolbarMergeMode;
  columns?: CSVColumn[];
  allowImport?: boolean;
  allowExport?: boolean;
  importMode?: "create_only" | "upsert";
  fileName?: string;
  serverImport?: CrudCsvServerImportPort;
  serverExport?: CrudCsvServerExportPort;
  ui: CrudCsvToolbarUiPort;
  /** Modo client + import: una fila CSV → crear registro. */
  importClientRow: (values: CrudFormValues) => Promise<void>;
  /** Textos post-import; por defecto inglés neutro. */
  messages?: Partial<CsvToolbarMessages>;
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
    accumulator[column.key] = typeof rawValue === "boolean" ? (rawValue ? "true" : "false") : String(rawValue ?? "");
    return accumulator;
  }, {});
}

function fieldMap(fields: CrudFormField[]): Map<string, CrudFormField> {
  return new Map(fields.map((field) => [field.key, field]));
}

async function importClientCSV<T extends { id: string }>(
  config: CrudPageConfig<T>,
  columns: CSVColumn[],
  importClientRow: (values: CrudFormValues) => Promise<void>,
  ui: CrudCsvToolbarUiPort,
): Promise<{ created: number; failed: number }> {
  const file = await pickCSVFile();
  if (!file) {
    return { created: 0, failed: 0 };
  }
  const rows = parseCSV(await file.text());
  if (rows.length === 0) {
    throw new Error("CSV file has no rows to import");
  }

  const ok = await ui.confirmClientImport(file.name, rows.length);
  if (!ok) {
    return { created: 0, failed: 0 };
  }

  const fieldsByKey = fieldMap(config.formFields);
  let created = 0;
  let failed = 0;
  for (const row of rows) {
    const values = columns.reduce<CrudFormValues>((accumulator, column) => {
      const field = fieldsByKey.get(column.key);
      accumulator[column.key] = normalizeCSVFieldValue(row[column.key] ?? "", field?.type);
      return accumulator;
    }, {});
    try {
      await importClientRow(values);
      created += 1;
    } catch {
      failed += 1;
    }
  }
  return { created, failed };
}

async function importServerCSV(
  entity: string,
  importMode: "create_only" | "upsert",
  port: CrudCsvServerImportPort,
  ui: CrudCsvToolbarUiPort,
): Promise<CsvServerImportResult> {
  const file = await pickCSVFile();
  if (!file) {
    return { created: 0, updated: 0, skipped: 0 };
  }
  const preview = await port.preview(entity, file);
  const firstErrors = (preview.errors ?? [])
    .slice(0, 3)
    .map((error) => `row ${error.row}: ${error.message}`)
    .join("\n");
  const description = [
    `File: ${file.name}`,
    `Total: ${preview.total_rows}`,
    `Valid: ${preview.valid_rows}`,
    `Errors: ${preview.error_rows}`,
    firstErrors ? `Errors:\n${firstErrors}` : "",
    "Continue with import?",
  ]
    .filter(Boolean)
    .join("\n\n");

  const confirmed = await ui.confirmServerImport(description);
  if (!confirmed) {
    return { created: 0, updated: 0, skipped: 0 };
  }
  return port.confirm(entity, preview.preview_id, importMode);
}

/**
 * Construye acciones de toolbar Importar/Exportar CSV (orden: export, luego import).
 */
const defaultCsvToolbarMessages: CsvToolbarMessages = {
  importClientDone: (r) => `Import finished. Created: ${r.created}. Failed: ${r.failed}.`,
  importServerDone: (r) =>
    `Import finished. Created: ${r.created}. Updated: ${r.updated}. Skipped: ${r.skipped}.`,
};

export function buildCsvToolbarActions<T extends { id: string }>(
  params: MergeCsvToolbarParams<T>,
): CrudToolbarAction<T>[] {
  const {
    config,
    entity,
    mode,
    columns = defaultColumns(config),
    allowImport = Boolean(config.dataSource?.create || config.basePath),
    allowExport = true,
    importMode = "upsert",
    fileName,
    serverImport,
    serverExport,
    ui,
    importClientRow,
    messages: messagesPartial,
  } = params;
  const messages: CsvToolbarMessages = { ...defaultCsvToolbarMessages, ...messagesPartial };

  const actions: CrudToolbarAction<T>[] = [];

  const canServerImport = mode === "server" && Boolean(serverImport);
  const canServerExport = mode === "server" && Boolean(serverExport);
  const canClientImport = mode === "client" && allowImport;

  if (allowExport) {
    if (mode === "server") {
      if (canServerExport) {
        actions.push({
          id: "csv-export",
          label: "Exportar CSV",
          kind: "secondary",
          onClick: async () => {
            if (!serverExport) return;
            await serverExport.download(entity);
          },
        });
      }
    } else {
      actions.push({
        id: "csv-export",
        label: "Exportar CSV",
        kind: "secondary",
        onClick: async ({ items }) => {
          const content = buildCSV(
            columns,
            items.map((row) => valuesFromRow(config, row, columns)),
          );
          downloadCSVFile(fileName ?? `${entity}.csv`, content);
        },
      });
    }
  }

  if (allowImport && (canClientImport || canServerImport)) {
    actions.push({
      id: "csv-import",
      label: "Importar CSV",
      kind: "secondary",
      onClick: async ({ reload }) => {
        if (mode === "server") {
          if (!serverImport) return;
          const result = await importServerCSV(entity, importMode, serverImport, ui);
          await reload();
          ui.notify(messages.importServerDone(result));
          return;
        }
        const result = await importClientCSV(config, columns, importClientRow, ui);
        await reload();
        ui.notify(messages.importClientDone(result));
      },
    });
  }

  return actions;
}

/**
 * Fusiona la config CRUD con acciones CSV al inicio de `toolbarActions`.
 */
export function mergeCsvToolbarConfig<T extends { id: string }>(
  params: MergeCsvToolbarParams<T>,
): CrudPageConfig<T> {
  const { config, ...rest } = params;
  const csvActions = buildCsvToolbarActions({ config, ...rest });
  return {
    ...config,
    toolbarActions: [...csvActions, ...(config.toolbarActions ?? [])],
  };
}
