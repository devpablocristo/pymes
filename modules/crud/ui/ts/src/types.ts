/**
 * Tipos del módulo CRUD (TypeScript): columnas, formulario, dataSource y acciones.
 * Sin tipos de dominio de negocio; la fila solo requiere `id: string`.
 */
import type { ReactNode } from "react";
import type { CrudFeatureFlags } from "./crudFeatureFlags";

/** Contexto para slots opcionales del listado (p. ej. filtros en cabecera). */
export type CrudListHeaderSlotContext<T> = { items: T[] };

export type CrudFieldValue = string | boolean;
export type CrudFormValues = Record<string, CrudFieldValue>;

export type CrudColumn<T> = {
  key: keyof T & string;
  header: string;
  render?: (value: unknown, row: T) => ReactNode;
  className?: string;
};

export type CrudFormField = {
  key: string;
  label: string;
  type?: "text" | "email" | "tel" | "number" | "date" | "datetime-local" | "textarea" | "select" | "checkbox";
  placeholder?: string;
  required?: boolean;
  fullWidth?: boolean;
  createOnly?: boolean;
  editOnly?: boolean;
  options?: Array<{ label: string; value: string }>;
};

/**
 * Puertos de datos: la app implementa llamadas a su API.
 * deleteItem: archivo lógico o POST /archive, según el backend.
 */
export type CrudDataSource<T extends { id: string }> = {
  list?: (params: { archived: boolean }) => Promise<T[]>;
  create?: (values: CrudFormValues) => Promise<unknown>;
  update?: (row: T, values: CrudFormValues) => Promise<unknown>;
  deleteItem?: (row: T) => Promise<unknown>;
  restore?: (row: T) => Promise<unknown>;
  hardDelete?: (row: T) => Promise<unknown>;
};

export type CrudHelpers<T extends { id: string }> = {
  items: T[];
  reload: () => Promise<void>;
  setError: (message: string) => void;
};

export type CrudToolbarAction<T extends { id: string }> = {
  id: string;
  label: string;
  kind?: "primary" | "secondary" | "danger" | "success";
  isVisible?: (ctx: { archived: boolean; items: T[] }) => boolean;
  onClick: (helpers: CrudHelpers<T>) => Promise<void> | void;
};

export type CrudRowAction<T extends { id: string }> = {
  id: string;
  label: string;
  kind?: "primary" | "secondary" | "danger" | "success";
  isVisible?: (row: T, ctx: { archived: boolean }) => boolean;
  onClick: (row: T, helpers: CrudHelpers<T>) => Promise<void> | void;
};

/**
 * Cliente HTTP mínimo cuando se usa `basePath` sin `dataSource.list`.
 */
export type CrudHttpClient = {
  json<TResponse>(path: string, init?: { method?: string; body?: Record<string, unknown> }): Promise<TResponse>;
};

/**
 * Configuración de la página: etiquetas ya resueltas en el idioma de la app.
 */
export type CrudPageConfig<T extends { id: string }> = {
  basePath?: string;
  /** Query sin `?`; se concatena al GET de listado. */
  listQuery?: string;
  dataSource?: CrudDataSource<T>;
  /** Obligatorio si hay `basePath` y no hay `dataSource.list`. */
  httpClient?: CrudHttpClient;
  supportsArchived?: boolean;
  allowCreate?: boolean;
  allowEdit?: boolean;
  allowDelete?: boolean;
  allowRestore?: boolean;
  allowHardDelete?: boolean;
  label: string;
  labelPlural: string;
  labelPluralCap: string;
  columns: CrudColumn<T>[];
  formFields: CrudFormField[];
  searchText: (row: T) => string;
  toFormValues: (row: T) => CrudFormValues;
  toBody?: (values: CrudFormValues) => Record<string, unknown>;
  isValid: (values: CrudFormValues) => boolean;
  searchPlaceholder?: string;
  emptyState?: string;
  archivedEmptyState?: string;
  createLabel?: string;
  toolbarActions?: CrudToolbarAction<T>[];
  rowActions?: CrudRowAction<T>[];
  /** i18n opcional para textos de campos (por defecto identidad). */
  formatFieldText?: (raw: string) => string;
  /** Títulos (por defecto identidad). */
  sentenceCase?: (s: string) => string;
  /**
   * Si está definido, el botón Editar de la fila invoca esto en lugar de abrir el formulario inline.
   * Útil cuando el producto tiene un editor dedicado (modal o ruta).
   */
  onExternalEdit?: (row: T) => void;
  /**
   * Filtra filas en cliente después del listado y antes del input de búsqueda.
   */
  preSearchFilter?: (items: T[]) => T[];
  /**
   * Contenido extra en la columna izquierda de cabecera (p. ej. filtros tipo píldora, toggle de vista).
   * La posición vertical respecto al título la controla `listHeaderSlotPlacement`.
   */
  listHeaderInlineSlot?: (ctx: CrudListHeaderSlotContext<T>) => ReactNode;
  /**
   * Dónde renderizar `listHeaderInlineSlot` en la cabecera (el shell coloca título y subtítulo en columna).
   * @default 'belowSubtitle' — debajo del subtítulo (conteo / estado de carga).
   */
  listHeaderSlotPlacement?: "belowSubtitle" | "aboveTitle";
  /**
   * Búsqueda controlada desde afuera (p. ej. un buscador global de la página).
   * Si está definido, oculta el input de búsqueda interno y usa este valor.
   */
  externalSearch?: string;
  /**
   * Flags de superficie; CrudPage los consume (p. ej. paginación) y los omite al renderizar.
   * `creatorFilter` lo interpreta solo el shell de la app.
   */
  featureFlags?: CrudFeatureFlags;
  /**
   * Render de celda tags cuando `mergeCanonicalCrudDefaults` inyecta la columna `tags`.
   */
  renderTagsCell?: (row: T) => ReactNode;
};

export type { CrudFeatureFlags };
