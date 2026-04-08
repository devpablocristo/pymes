import type { ReactNode } from "react";
import type { CrudColumn, CrudPageConfig } from "./types";
import { defaultCrudFeatureFlags, mergeCrudFeatureFlags } from "./crudFeatureFlags";

/**
 * Superficie CRUD canónica por defecto (archivo, restore, borrado duro, CRUD estándar).
 * Las apps fusionan sobre esto y pueden desactivar vía `featureFlags` o overrides puntuales.
 */
export const defaultCanonicalCrudSurface = {
  supportsArchived: true,
  allowCreate: true,
  allowEdit: true,
  allowDelete: true,
  allowRestore: true,
  allowHardDelete: true,
  featureFlags: { ...defaultCrudFeatureFlags },
} as const;

const CSV_TOOLBAR_IDS = new Set(["csv-export", "csv-import"]);

export type MergeCanonicalCrudDefaultsOptions = {
  /** Recursos que no deben recibir el merge (p. ej. tableros bespoke). */
  excludedResourceIds?: Iterable<string>;
};

function defaultTagsCell<T extends { id: string }>(row: T): ReactNode {
  const raw = (row as { tags?: unknown }).tags;
  const list = Array.isArray(raw)
    ? raw.map((t) => String(t).trim()).filter(Boolean)
    : typeof raw === "string"
      ? raw
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean)
      : [];
  if (list.length === 0) {
    return <span className="crud-tags-empty">---</span>;
  }
  return <span className="crud-tags-inline">{list.join(", ")}</span>;
}

function ensureTagsColumn<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  const hasTagsField = (config.formFields ?? []).some((f) => f.key === "tags");
  if (!hasTagsField) {
    return config;
  }
  const cols = config.columns ?? [];
  if (cols.some((c) => c.key === "tags")) {
    return config;
  }
  const tagsColumn = {
    key: "tags" as keyof T & string,
    header: "Tags",
    className: "cell-tags",
    render: (_value: unknown, row: T) => config.renderTagsCell?.(row) ?? defaultTagsCell(row),
  } satisfies CrudColumn<T>;
  return {
    ...config,
    columns: [...cols, tagsColumn],
  };
}

function stripTagsColumns<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  return {
    ...config,
    columns: (config.columns ?? []).filter((c) => c.key !== "tags"),
  };
}

function stripCsvToolbarActions<T extends { id: string }>(config: CrudPageConfig<T>): CrudPageConfig<T> {
  return {
    ...config,
    toolbarActions: (config.toolbarActions ?? []).filter((a) => !CSV_TOOLBAR_IDS.has(a.id)),
  };
}

/**
 * Fusiona defaults canónicos + flags resueltos (todo activo por defecto).
 */
export function mergeCanonicalCrudDefaults<T extends { id: string }>(
  resourceId: string,
  config: CrudPageConfig<T>,
  options?: MergeCanonicalCrudDefaultsOptions,
): CrudPageConfig<T> {
  const excluded = options?.excludedResourceIds
    ? new Set(options.excludedResourceIds)
    : undefined;
  if (excluded?.has(resourceId)) {
    return config;
  }

  const mergedFlags = mergeCrudFeatureFlags(config.featureFlags);
  const merged: CrudPageConfig<T> = {
    ...defaultCanonicalCrudSurface,
    ...config,
    featureFlags: mergedFlags,
  };

  let out = merged;
  if (!mergedFlags.tagsColumn) {
    out = stripTagsColumns(out);
  } else {
    out = ensureTagsColumn(out);
  }
  if (!mergedFlags.csvToolbar) {
    out = stripCsvToolbarActions(out);
  }
  return out;
}
