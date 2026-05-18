import { crudItemPath, type CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { createElement, isValidElement } from 'react';
import type {
  CrudColumn,
  CrudEditorModalFieldConfig,
  CrudFormField,
  CrudFormValues,
  CrudPageConfig,
} from '../components/CrudPage';
import {
  collectCrudImageUrls,
  parseCrudLinkedEntityImageUrlList,
  type CrudActionDialogField,
  type CrudEntityEditorModalBlock,
  type CrudEntityEditorModalSection,
  type CrudEntityEditorModalStat,
  type CrudTableSurfaceColumn,
} from '../modules/crud';
import { StandardCrudImageUrlsEditor } from './standardCrudMedia';

const CRUD_DIALOG_IMAGE_FIELD_IDS = new Set<string>(['image_urls', 'image_url', 'images']);

function defaultModalImageUrlsEditControl(ctx: {
  value: CrudFieldValue | undefined;
  values: Record<string, CrudFieldValue>;
  setValue: (nextValue: CrudFieldValue) => void;
}) {
  return createElement(StandardCrudImageUrlsEditor, { value: ctx.value, setValue: ctx.setValue });
}

export type CrudListResponse<T> = {
  items: T[];
  has_more?: boolean;
  next_cursor?: string | null;
};

export function emptyValueForField(field: CrudFormField): CrudFieldValue {
  return field.type === 'checkbox' ? false : '';
}

function resolveEditorFieldConfig(
  field: CrudFormField,
  overrides?: CrudEditorModalFieldConfig,
  defaultSectionId?: string,
): CrudEditorModalFieldConfig {
  return {
    sectionId: overrides?.sectionId ?? defaultSectionId,
    helperText: overrides?.helperText,
    fullWidth: overrides?.fullWidth ?? field.fullWidth,
    hidden: overrides?.hidden,
    readOnly: overrides?.readOnly,
    editControl: overrides?.editControl,
    visible: overrides?.visible,
    readValue: overrides?.readValue,
  };
}

export function toDialogField(
  field: CrudFormField,
  values: CrudFormValues,
  editorFieldConfig?: CrudEditorModalFieldConfig,
  defaultSectionId?: string,
): CrudActionDialogField {
  const resolvedEditorFieldConfig = resolveEditorFieldConfig(field, editorFieldConfig, defaultSectionId);
  const resolvedEditControl =
    resolvedEditorFieldConfig.editControl ??
    (CRUD_DIALOG_IMAGE_FIELD_IDS.has(field.key) ? defaultModalImageUrlsEditControl : undefined);
  return {
    id: field.key,
    label: field.label,
    type:
      field.type === 'email' ||
      field.type === 'tel' ||
      field.type === 'number' ||
      field.type === 'textarea' ||
      field.type === 'datetime-local' ||
      field.type === 'select' ||
      field.type === 'checkbox'
        ? field.type
        : 'text',
    placeholder: field.placeholder,
    required: field.required,
    rows: field.rows,
    defaultValue: values[field.key] ?? emptyValueForField(field),
    options: field.options,
    sectionId: resolvedEditorFieldConfig.sectionId,
    helperText: resolvedEditorFieldConfig.helperText,
    fullWidth: resolvedEditorFieldConfig.fullWidth,
    readOnly: resolvedEditorFieldConfig.readOnly,
    editControl: resolvedEditControl
      ? ({ value, values: dialogValues, setValue }) =>
          resolvedEditControl({ value, values: dialogValues, setValue })
      : undefined,
    visible: resolvedEditorFieldConfig.visible
      ? ({ value, values: dialogValues, editing }) =>
          Boolean(resolvedEditorFieldConfig.visible?.({ value, values: dialogValues, editing }))
      : undefined,
    readValue: resolvedEditorFieldConfig.readValue
      ? ({ value, values: dialogValues }) => resolvedEditorFieldConfig.readValue?.({ value, values: dialogValues })
      : undefined,
  };
}

export function buildEmptyFormValues(fields: CrudFormField[]): CrudFormValues {
  return Object.fromEntries(fields.map((field) => [field.key, emptyValueForField(field)]));
}

export function activeFields(fields: CrudFormField[], editing: boolean) {
  return fields.filter((field) => {
    if (editing && field.createOnly) return false;
    if (!editing && field.editOnly) return false;
    return true;
  });
}

export function normalizeError(error: unknown, defaultMessage: string) {
  return error instanceof Error ? error.message : defaultMessage;
}

export function buildEditorSections<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
): CrudEntityEditorModalSection[] | undefined {
  return crudConfig.editorModal?.sections?.map((section) => ({
    id: section.id,
    title: section.title,
    description: section.description,
  }));
}

export function resolveEditorSectionId<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
  fieldKey: string,
): string | undefined {
  return crudConfig.editorModal?.sections?.find((section) => section.fieldKeys?.includes(fieldKey))?.id;
}

export function buildEditorBlocks<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
): CrudEntityEditorModalBlock[] | undefined {
  return crudConfig.editorModal?.blocks?.map((block) => ({
    id: block.id,
    kind: block.kind,
    field: block.field,
    sectionId: block.sectionId,
    label: block.label,
    required: block.required,
    visible: block.visible
      ? ({ values, editing, row }) => Boolean(block.visible?.({ values, editing, row: row as T | undefined }))
      : undefined,
  }));
}

export function buildEditorStats<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
  row: T | undefined,
  editing: boolean,
): CrudEntityEditorModalStat[] | undefined {
  return crudConfig.editorModal?.stats?.map((stat) => ({
    id: stat.id,
    label: stat.label,
    tone: stat.tone,
    value: (values) => stat.value({ row, values: values as CrudFormValues, editing }),
  }));
}

export function pickStringValue(row: Record<string, unknown>, candidates: string[]) {
  for (const key of candidates) {
    const raw = row[key];
    if (typeof raw === 'string' && raw.trim()) return raw.trim();
    if (typeof raw === 'number' && Number.isFinite(raw)) return String(raw);
  }
  return '';
}

function toSortablePrimitive(value: unknown): string | number | boolean | null {
  if (value == null) return null;
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return value;
  if (Array.isArray(value)) return value.map((entry) => String(entry ?? '').trim()).join(', ');
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

function readCrudImageUrlCandidates(record: Record<string, unknown>): Array<string | string[] | undefined> {
  const read = (key: string) => (record[key] as string | string[] | undefined);
  return [
    read('image_urls'),
    read('imageUrls'),
    read('images'),
    read('photo_urls'),
    read('photoUrls'),
    read('photos'),
    read('media'),
    read('image'),
    read('image_url'),
    read('imageUrl'),
    read('logo_url'),
    read('logoUrl'),
    read('avatar_url'),
    read('avatarUrl'),
    read('banner_image'),
    read('bannerImage'),
  ];
}

function parseCrudImageCandidate(candidate: string | string[] | undefined): string[] {
  if (!candidate) return [];
  if (Array.isArray(candidate)) return candidate.filter((value): value is string => typeof value === 'string');
  return parseCrudLinkedEntityImageUrlList(candidate);
}

export function buildEditorMediaUrls<T extends { id: string }>(row: T | undefined) {
  if (!row) return undefined;
  const record = row as Record<string, unknown>;
  const collected = readCrudImageUrlCandidates(record).flatMap((candidate) =>
    parseCrudImageCandidate(candidate as string | string[] | undefined),
  );
  return collectCrudImageUrls({ imageUrls: collected });
}

export function buildTableColumns<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
  archived: boolean,
): CrudTableSurfaceColumn<T>[] {
  const tagsEnabled = crudConfig.featureFlags?.tagsColumn !== false;
  const sourceColumns = archived && crudConfig.archivedColumns?.length ? crudConfig.archivedColumns : crudConfig.columns;
  const seenColumnIds = new Map<string, number>();
  const mappedColumns: CrudTableSurfaceColumn<T>[] = sourceColumns
    .filter((column) => tagsEnabled || column.key !== 'tags')
    .map((column: CrudColumn<T>) => {
      const baseId = String(column.key);
      const seen = seenColumnIds.get(baseId) ?? 0;
      seenColumnIds.set(baseId, seen + 1);
      const columnId = seen === 0 ? baseId : `${baseId}:${seen}`;
      return {
        id: columnId,
        header: column.header,
        className: column.className,
        render: (row: T) => {
          const value = row[column.key];
          return column.render ? column.render(value, row) : String(value ?? '—');
        },
        sortValue: (row: T) => {
          const raw = row[column.key];
          if (!column.render) {
            return toSortablePrimitive(raw);
          }
          const rendered = column.render(raw, row);
          if (typeof rendered === 'string' || typeof rendered === 'number' || typeof rendered === 'boolean') {
            return rendered;
          }
          if (isValidElement(rendered)) {
            const child = (rendered.props as { children?: unknown } | null)?.children;
            return toSortablePrimitive(child);
          }
          return toSortablePrimitive(raw);
        },
      };
    });

  if (
    tagsEnabled &&
    crudConfig.renderTagsCell &&
    !mappedColumns.some((column) => column.id === 'tags')
  ) {
    mappedColumns.push({
      id: 'tags',
      header: 'Etiquetas Internas',
      className: 'cell-tags',
      render: (row) => crudConfig.renderTagsCell?.(row) ?? '—',
    });
  }

  return mappedColumns;
}

export function buildFallbackFormValuesFromRow<T extends { id: string }>(
  crudConfig: CrudPageConfig<T>,
  row: T,
): CrudFormValues {
  const rec = row as unknown as Record<string, unknown>;
  const out: CrudFormValues = {};
  for (const column of crudConfig.columns) {
    const raw = rec[String(column.key)];
    if (column.render) {
      const rendered = column.render(raw as CrudFieldValue, row);
      out[String(column.key)] = typeof rendered === 'string' ? rendered : String(raw ?? '');
    } else {
      out[String(column.key)] = String(raw ?? '');
    }
  }
  return out;
}

export function crudRecordItemPath(basePath: string, id: string) {
  return crudItemPath(basePath, id);
}
