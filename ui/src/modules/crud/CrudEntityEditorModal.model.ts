import { createElement, type ReactNode } from 'react';
import type { CrudFieldValue } from '@devpablocristo/platform-crud-ui';
import { StandardCrudImageUrlsEditor } from '../../crud/standardCrudMedia';

export type CrudEntityEditorModalField = {
  id: string;
  label: string;
  type?: 'text' | 'email' | 'tel' | 'number' | 'textarea' | 'datetime-local' | 'select' | 'checkbox';
  placeholder?: string;
  required?: boolean;
  defaultValue?: CrudFieldValue;
  min?: number;
  step?: number | 'any';
  rows?: number;
  options?: Array<{ label: string; value: string }>;
  helperText?: ReactNode;
  sectionId?: string;
  fullWidth?: boolean;
  readOnly?: boolean;
  editControl?: (ctx: {
    value: CrudFieldValue | undefined;
    values: Record<string, CrudFieldValue>;
    setValue: (nextValue: CrudFieldValue) => void;
  }) => ReactNode;
  visible?: (ctx: {
    value: CrudFieldValue | undefined;
    values: Record<string, CrudFieldValue>;
    editing: boolean;
  }) => boolean;
  readValue?:
    | ReactNode
    | ((ctx: { value: CrudFieldValue | undefined; values: Record<string, CrudFieldValue> }) => ReactNode);
};

export type CrudEntityEditorModalSection = {
  id: string;
  title?: ReactNode;
  description?: ReactNode;
};

export type CrudEntityEditorModalStat = {
  id: string;
  label: ReactNode;
  value: ReactNode | ((values: Record<string, CrudFieldValue>) => ReactNode);
  tone?: 'default' | 'info' | 'warning' | 'success' | 'danger';
};

export type CrudEntityEditorModalBlock = {
  id: string;
  kind: 'lineItems';
  field: string;
  sectionId: string;
  label?: ReactNode;
  required?: boolean;
  visible?: (ctx: {
    values: Record<string, CrudFieldValue>;
    editing: boolean;
    row?: unknown;
  }) => boolean;
};

export type CrudEntityEditorModalProps = {
  open: boolean;
  title: ReactNode;
  subtitle?: ReactNode;
  eyebrow?: ReactNode;
  variant?: 'modal' | 'page';
  editBehavior?: 'read-edit' | 'edit-only';
  allowEdit?: boolean;
  mediaUrls?: string[];
  mediaFieldId?: string;
  mode?: 'create' | 'update';
  cancelLabel?: string;
  submitLabel?: string;
  editLabel?: string;
  cancelEditLabel?: string;
  closeLabel?: string;
  fields: CrudEntityEditorModalField[];
  blocks?: CrudEntityEditorModalBlock[];
  sections?: CrudEntityEditorModalSection[];
  stats?: CrudEntityEditorModalStat[];
  initialValues?: Record<string, CrudFieldValue>;
  row?: unknown;
  error?: ReactNode;
  loading?: boolean;
  loadingLabel?: ReactNode;
  disableSubmit?: boolean;
  disableSubmitWhenPristine?: boolean;
  headerActions?:
    | ReactNode
    | ((ctx: {
        dirty: boolean;
        requestCancel: () => void;
      }) => ReactNode);
  editingStartActions?: ReactNode | ((ctx: { dirty: boolean }) => ReactNode);
  rootClassName?: string;
  panelClassName?: string;
  pageToolbar?: ReactNode;
  pageToolbarClassName?: string;
  confirmDiscard?: {
    title: string;
    description: string;
    confirmLabel?: string;
    cancelLabel?: string;
  };
  archiveAction?: {
    label?: string;
    busyLabel?: string;
    confirm?: {
      title: string;
      description: string;
      confirmLabel?: string;
      cancelLabel?: string;
    };
    onArchive: () => Promise<void> | void;
  };
  restoreAction?: {
    label?: string;
    busyLabel?: string;
    confirm?: {
      title: string;
      description: string;
      confirmLabel?: string;
      cancelLabel?: string;
    };
    onRestore: () => Promise<void> | void;
  };
  deleteAction?: {
    label?: string;
    busyLabel?: string;
    confirm?: {
      title: string;
      description: string;
      confirmLabel?: string;
      cancelLabel?: string;
    };
    onDelete: () => Promise<void> | void;
  };
  onCancel: () => void;
  onSubmit: (values: Record<string, CrudFieldValue>) => void;
};

export type ResolvedSection = CrudEntityEditorModalSection & {
  fields: CrudEntityEditorModalField[];
  blocks: CrudEntityEditorModalBlock[];
};

export const CRUD_MODAL_FAVORITE_FIELD_IDS = ['metadata_favorite', 'is_favorite'] as const;
export const CRUD_MODAL_IMAGE_FIELD_IDS = ['image_urls', 'image_url', 'images'] as const;
export const CRUD_MODAL_IMAGE_FIELD_ID_SET = new Set<string>(CRUD_MODAL_IMAGE_FIELD_IDS);

const defaultModalImageUrlsEditControl: NonNullable<CrudEntityEditorModalField['editControl']> = ({ value, setValue }) => (
  createElement(StandardCrudImageUrlsEditor, { value, setValue })
);

export function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ');
}

export function resolveModalImageFieldEditControl(
  field: CrudEntityEditorModalField,
): CrudEntityEditorModalField['editControl'] | undefined {
  return field.editControl ?? (CRUD_MODAL_IMAGE_FIELD_ID_SET.has(field.id) ? defaultModalImageUrlsEditControl : undefined);
}

function dedupeModalFieldsById(fields: CrudEntityEditorModalField[]): CrudEntityEditorModalField[] {
  const seen = new Set<string>();
  const out: CrudEntityEditorModalField[] = [];
  for (const f of fields) {
    if (seen.has(f.id)) continue;
    seen.add(f.id);
    out.push(f);
  }
  return out;
}

function dedupeModalBlocksById(blocks: CrudEntityEditorModalBlock[]): CrudEntityEditorModalBlock[] {
  const seen = new Set<string>();
  const out: CrudEntityEditorModalBlock[] = [];
  for (const b of blocks) {
    if (seen.has(b.id)) continue;
    seen.add(b.id);
    out.push(b);
  }
  return out;
}

function reorderPinnedModalFields(fields: CrudEntityEditorModalField[]): CrudEntityEditorModalField[] {
  const byId = new Map(fields.map((f) => [f.id, f]));
  const head: CrudEntityEditorModalField[] = [];

  const favoriteId = CRUD_MODAL_FAVORITE_FIELD_IDS.find((id) => byId.has(id));
  if (favoriteId) head.push(byId.get(favoriteId)!);

  const tagsField = byId.get('tags');
  if (tagsField) head.push(tagsField);

  const imageId = CRUD_MODAL_IMAGE_FIELD_IDS.find((id) => byId.has(id));
  if (imageId) head.push(byId.get(imageId)!);

  const headIds = new Set(head.map((f) => f.id));
  const tail = fields.filter((f) => !headIds.has(f.id));
  return [...head, ...tail];
}

function mergeResolvedSectionsToSingleBody(sections: ResolvedSection[]): ResolvedSection[] {
  if (sections.length === 0) return sections;
  if (sections.length === 1) {
    const only = sections[0];
    return [
      {
        ...only,
        fields: reorderPinnedModalFields(dedupeModalFieldsById(only.fields)),
        blocks: dedupeModalBlocksById(only.blocks),
      },
    ];
  }
  const mergedFields = reorderPinnedModalFields(dedupeModalFieldsById(sections.flatMap((s) => s.fields)));
  const mergedBlocks = dedupeModalBlocksById(sections.flatMap((s) => s.blocks));
  return [{ id: 'crud-form-unified', fields: mergedFields, blocks: mergedBlocks }];
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function areCrudFieldValuesEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) && Array.isArray(right)) {
    if (left.length !== right.length) return false;
    return left.every((item, index) => areCrudFieldValuesEqual(item, right[index]));
  }
  if (isPlainObject(left) && isPlainObject(right)) {
    const leftKeys = Object.keys(left);
    const rightKeys = Object.keys(right);
    if (leftKeys.length !== rightKeys.length) return false;
    return leftKeys.every((key) => areCrudFieldValuesEqual(left[key], right[key]));
  }
  return false;
}

export function resolveSections(
  fields: CrudEntityEditorModalField[],
  blocks: CrudEntityEditorModalBlock[],
  sections: CrudEntityEditorModalSection[] | undefined,
): ResolvedSection[] {
  if (sections?.length) {
    const usedFields = new Set<string>();
    const usedBlocks = new Set<string>();
    const resolved = sections
      .map((section) => {
        const sectionFields = fields.filter((field) => field.sectionId === section.id);
        const sectionBlocks = blocks.filter((block) => block.sectionId === section.id);
        sectionFields.forEach((field) => usedFields.add(field.id));
        sectionBlocks.forEach((block) => usedBlocks.add(block.id));
        return { ...section, fields: sectionFields, blocks: sectionBlocks };
      })
      .filter((section) => section.fields.length > 0 || section.blocks.length > 0);

    const restFields = fields.filter((field) => !usedFields.has(field.id));
    const restBlocks = blocks.filter((block) => !usedBlocks.has(block.id));
    if (restFields.length || restBlocks.length) {
      resolved.unshift({
        id: 'general',
        fields: restFields,
        blocks: restBlocks,
      });
    }
    return mergeResolvedSectionsToSingleBody(resolved);
  }

  return mergeResolvedSectionsToSingleBody([
    {
      id: 'general',
      fields,
      blocks,
    },
  ]);
}

export function renderStatValue(
  stat: CrudEntityEditorModalStat,
  values: Record<string, CrudFieldValue>,
): ReactNode {
  return typeof stat.value === 'function' ? stat.value(values) : stat.value;
}
