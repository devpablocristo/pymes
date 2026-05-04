import { confirmAction } from '@devpablocristo/core-browser';
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { flushSync } from 'react-dom';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { StandardCrudImageUrlsEditor } from '../../crud/standardCrudMedia';
import { parseCrudLinkedEntityImageUrlList } from './crudLinkedEntityImageUrls';
import { CrudEntityModalShell } from './CrudEntityModalShell';
import { CrudEntityMediaCarousel } from './CrudEntityMediaCarousel';
import { CrudLineItemsEditor } from './CrudLineItemsEditor';
import './CrudEntityEditorModal.css';

function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ');
}

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

type ResolvedSection = CrudEntityEditorModalSection & {
  fields: CrudEntityEditorModalField[];
  blocks: CrudEntityEditorModalBlock[];
};

/** Orden global del modal: favorito, etiquetas, primer campo de imágenes conocido, resto en orden declarado. */
const CRUD_MODAL_PINNED_FIELD_IDS = ['metadata_favorite', 'tags'] as const;
const CRUD_MODAL_IMAGE_FIELD_IDS = ['image_urls', 'image_url', 'images'] as const;
const CRUD_MODAL_IMAGE_FIELD_ID_SET = new Set<string>(CRUD_MODAL_IMAGE_FIELD_IDS);

/** Nunca `<textarea>` con base64: cualquier recurso que olvide `editControl` sigue usando el editor estándar. */
const defaultModalImageUrlsEditControl: NonNullable<CrudEntityEditorModalField['editControl']> = ({ value, setValue }) => (
  <StandardCrudImageUrlsEditor value={value} setValue={setValue} />
);

function resolveModalImageFieldEditControl(field: CrudEntityEditorModalField): CrudEntityEditorModalField['editControl'] | undefined {
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
  for (const id of CRUD_MODAL_PINNED_FIELD_IDS) {
    const f = byId.get(id);
    if (f) head.push(f);
  }
  const imageId = CRUD_MODAL_IMAGE_FIELD_IDS.find((id) => byId.has(id));
  if (imageId) head.push(byId.get(imageId)!);
  const headIds = new Set(head.map((f) => f.id));
  const tail = fields.filter((f) => !headIds.has(f.id));
  return [...head, ...tail];
}

/** Un solo panel de formulario: sin fraccionar por secciones con marcos (convención Pymes). */
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

type PendingConfirmDialog = {
  title: string;
  description: string;
  confirmLabel: string;
  cancelLabel: string;
  tone?: 'default' | 'danger';
  onConfirm: () => Promise<void>;
};

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function areCrudFieldValuesEqual(left: unknown, right: unknown): boolean {
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

function resolveSections(
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

function renderStatValue(
  stat: CrudEntityEditorModalStat,
  values: Record<string, CrudFieldValue>,
): ReactNode {
  return typeof stat.value === 'function' ? stat.value(values) : stat.value;
}

export function CrudEntityEditorModal({
  open,
  title,
  subtitle,
  eyebrow,
  variant = 'modal',
  editBehavior = 'read-edit',
  allowEdit = true,
  mediaUrls,
  mediaFieldId,
  mode = 'create',
  cancelLabel = 'Cancelar',
  submitLabel = 'Guardar',
  editLabel = 'Editar',
  cancelEditLabel = 'Cancelar',
  closeLabel = 'Cerrar',
  fields,
  blocks = [],
  sections,
  stats,
  initialValues: initialValuesProp,
  row,
  error,
  loading = false,
  loadingLabel = 'Cargando…',
  disableSubmit = false,
  disableSubmitWhenPristine = false,
  headerActions,
  editingStartActions,
  rootClassName,
  panelClassName,
  pageToolbar,
  pageToolbarClassName,
  confirmDiscard,
  archiveAction,
  restoreAction,
  deleteAction,
  onCancel,
  onSubmit,
}: CrudEntityEditorModalProps) {
  const titleId = 'crud-entity-editor-modal-title';
  const formId = 'crud-entity-editor-modal-form';
  const initialValues = useMemo(
    () => ({
      ...Object.fromEntries(fields.map((field) => [field.id, field.defaultValue ?? (field.type === 'checkbox' ? false : '')])),
      ...(initialValuesProp ?? {}),
    }),
    [fields, initialValuesProp],
  );
  const [values, setValues] = useState<Record<string, CrudFieldValue>>(initialValues);
  const [isEditing, setIsEditing] = useState(mode === 'create' || editBehavior === 'edit-only');
  const [archiving, setArchiving] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [pendingConfirm, setPendingConfirm] = useState<PendingConfirmDialog | null>(null);
  const [confirming, setConfirming] = useState(false);
  /** Evita cierre por backdrop justo después de activar edición (race con pointer). */
  const suppressBackdropCloseRef = useRef(false);
  /** Ratón/táctil: la activación se hace en pointerup tras capture; el click siguiente se ignora. */
  const pointerActivatedEditRef = useRef(false);
  /** Libera suppressBackdrop tras una ventana (el click fantasma puede ir muy después del pointerup). */
  const suppressBackdropTimerRef = useRef<number | null>(null);

  const armBackdropSuppress = () => {
    suppressBackdropCloseRef.current = true;
    if (suppressBackdropTimerRef.current !== null) {
      window.clearTimeout(suppressBackdropTimerRef.current);
    }
    suppressBackdropTimerRef.current = window.setTimeout(() => {
      suppressBackdropCloseRef.current = false;
      suppressBackdropTimerRef.current = null;
    }, 1200);
  };

  // Sincronizar valores cuando cambian datos de formulario; no reiniciar modo lectura/edición
  // aquí: `initialValues` suele ser un objeto nuevo en cada render del padre y eso devolvía
  // al usuario al visor justo después de pulsar "Editar".
  useLayoutEffect(() => {
    setValues(initialValues);
  }, [initialValues]);

  useEffect(() => {
    setIsEditing(mode === 'create' || editBehavior === 'edit-only');
  }, [editBehavior, mode]);

  useEffect(() => {
    if (!open) {
      setPendingConfirm(null);
      setConfirming(false);
      if (suppressBackdropTimerRef.current !== null) {
        window.clearTimeout(suppressBackdropTimerRef.current);
        suppressBackdropTimerRef.current = null;
      }
      suppressBackdropCloseRef.current = false;
    }
  }, [open]);

  const dirty = useMemo(
    () => !areCrudFieldValuesEqual(values, initialValues),
    [initialValues, values],
  );

  const resolvedSections = useMemo(() => resolveSections(fields, blocks, sections), [blocks, fields, sections]);
  const mediaPreviewOwnedByImageEditControl = useMemo(() => {
    if (!mediaFieldId) return false;
    return resolvedSections.some((section) =>
      section.fields.some((field) => {
        if (field.id !== mediaFieldId) return false;
        if (field.editControl) return true;
        return CRUD_MODAL_IMAGE_FIELD_ID_SET.has(field.id);
      }),
    );
  }, [mediaFieldId, resolvedSections]);
  const hasHeaderContent = Boolean(eyebrow || title || subtitle);
  const resolvedMediaUrls = useMemo(() => {
    if (mediaFieldId) {
      const currentValue = values[mediaFieldId];
      if (Array.isArray(currentValue)) {
        return currentValue.filter((value): value is string => typeof value === 'string' && value.trim().length > 0);
      }
      if (typeof currentValue === 'string') {
        return parseCrudLinkedEntityImageUrlList(currentValue);
      }
    }
    return mediaUrls;
  }, [mediaFieldId, mediaUrls, values]);

  const requestCancel = async () => {
    if (mode === 'update' && editBehavior !== 'edit-only' && isEditing) {
      if (!dirty) {
        setValues(initialValues);
        setIsEditing(false);
        return;
      }
      if (!confirmDiscard) {
        setValues(initialValues);
        setIsEditing(false);
        return;
      }
      const confirmed = await confirmAction({
        title: confirmDiscard.title,
        description: confirmDiscard.description,
        confirmLabel: confirmDiscard.confirmLabel ?? 'Descartar cambios',
        cancelLabel: confirmDiscard.cancelLabel ?? 'Seguir editando',
        tone: 'danger',
      });
      if (confirmed) {
        setValues(initialValues);
        setIsEditing(false);
      }
      return;
    }
    if (!dirty || !confirmDiscard) {
      onCancel();
      return;
    }
    const confirmed = await confirmAction({
      title: confirmDiscard.title,
      description: confirmDiscard.description,
      confirmLabel: confirmDiscard.confirmLabel ?? 'Descartar cambios',
      cancelLabel: confirmDiscard.cancelLabel ?? 'Seguir editando',
      tone: 'danger',
    });
    if (confirmed) onCancel();
  };

  const resolvedEditingStartActions =
    typeof editingStartActions === 'function' ? editingStartActions({ dirty }) : editingStartActions;
  const resolvedHeaderActions =
    typeof headerActions === 'function'
      ? headerActions({
          dirty,
          requestCancel: () => {
            void requestCancel();
          },
        })
      : headerActions;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    if (!form.reportValidity()) return;
    onSubmit(values);
  };

  const handleArchive = async () => {
    if (!archiveAction || archiving) return;
    if (archiveAction.confirm) {
      setPendingConfirm({
        title: archiveAction.confirm.title,
        description: archiveAction.confirm.description,
        confirmLabel: archiveAction.confirm.confirmLabel ?? archiveAction.label ?? 'Archivar',
        cancelLabel: archiveAction.confirm.cancelLabel ?? 'Cancelar',
        tone: 'danger',
        onConfirm: async () => {
          setArchiving(true);
          try {
            await archiveAction.onArchive();
          } finally {
            setArchiving(false);
          }
        },
      });
      return;
    }
    setArchiving(true);
    try {
      await archiveAction.onArchive();
    } finally {
      setArchiving(false);
    }
  };

  const handleRestore = async () => {
    if (!restoreAction || restoring) return;
    if (restoreAction.confirm) {
      const confirmed = await confirmAction({
        title: restoreAction.confirm.title,
        description: restoreAction.confirm.description,
        confirmLabel: restoreAction.confirm.confirmLabel ?? restoreAction.label ?? 'Restaurar',
        cancelLabel: restoreAction.confirm.cancelLabel ?? 'Cancelar',
      });
      if (!confirmed) return;
    }
    setRestoring(true);
    try {
      await restoreAction.onRestore();
    } finally {
      setRestoring(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteAction || deleting) return;
    if (deleteAction.confirm) {
      setPendingConfirm({
        title: deleteAction.confirm.title,
        description: deleteAction.confirm.description,
        confirmLabel: deleteAction.confirm.confirmLabel ?? deleteAction.label ?? 'Eliminar',
        cancelLabel: deleteAction.confirm.cancelLabel ?? 'Cancelar',
        tone: 'danger',
        onConfirm: async () => {
          setDeleting(true);
          try {
            await deleteAction.onDelete();
          } finally {
            setDeleting(false);
          }
        },
      });
      return;
    }
    setDeleting(true);
    try {
      await deleteAction.onDelete();
    } finally {
      setDeleting(false);
    }
  };

  const handlePendingConfirm = async () => {
    if (!pendingConfirm || confirming) return;
    setConfirming(true);
    try {
      await pendingConfirm.onConfirm();
      setPendingConfirm(null);
    } finally {
      setConfirming(false);
    }
  };

  const scheduleActivateEditing = () => {
    if (!allowEdit || isEditing) return;
    armBackdropSuppress();
    // Macrotask: el «click» del mismo gesto ocurre antes que setTimeout(0); así el botón sigue en el DOM
    // durante el click y no “cae” en Cerrar/backdrop al desmontarse en flushSync.
    window.setTimeout(() => {
      flushSync(() => {
        setIsEditing(true);
      });
    }, 0);
  };

  const footer =
    mode === 'update' && editBehavior !== 'edit-only' && !isEditing && (restoreAction || deleteAction) ? (
      <div className="crud-entity-editor-modal__footer-layout">
        <div className="crud-entity-editor-modal__footer-group crud-entity-editor-modal__footer-group--start">
          {deleteAction ? (
            <button type="button" className="btn btn-danger" onClick={() => void handleDelete()} disabled={deleting || restoring}>
              {deleting ? deleteAction.busyLabel ?? 'Eliminando…' : deleteAction.label ?? 'Eliminar'}
            </button>
          ) : null}
        </div>
        <div className="crud-entity-editor-modal__footer-group crud-entity-editor-modal__footer-group--end">
          <button type="button" className="btn btn-secondary" onClick={() => void requestCancel()} disabled={deleting || restoring}>
            {closeLabel}
          </button>
          {restoreAction ? (
            <button type="button" className="btn btn-primary" onClick={() => void handleRestore()} disabled={restoring || deleting}>
              {restoring ? restoreAction.busyLabel ?? 'Restaurando…' : restoreAction.label ?? 'Restaurar'}
            </button>
          ) : null}
        </div>
      </div>
    ) : mode === 'update' && editBehavior !== 'edit-only' && !isEditing ? (
      <div className="crud-entity-editor-modal__footer-layout">
        <div className="crud-entity-editor-modal__footer-group crud-entity-editor-modal__footer-group--start">
          {archiveAction ? (
            <button type="button" className="btn btn-danger" onClick={() => void handleArchive()} disabled={archiving}>
              {archiving ? archiveAction.busyLabel ?? 'Archivando…' : archiveAction.label ?? 'Archivar'}
            </button>
          ) : null}
        </div>
        <div className="crud-entity-editor-modal__footer-group crud-entity-editor-modal__footer-group--end">
          <button type="button" className="btn btn-secondary" onClick={() => void requestCancel()}>
            {closeLabel}
          </button>
          <button
            type="button"
            className={cx('btn btn-primary', !allowEdit && 'crud-entity-editor-modal__edit-btn--blocked')}
            onPointerDown={(event) => {
              if (!allowEdit || isEditing) return;
              if (event.pointerType === 'mouse' && event.button !== 0) return;
              // Antes del pointerup: si el layout cambia al pasar a edición, el click fantasma puede caer en el backdrop.
              armBackdropSuppress();
              try {
                (event.currentTarget as HTMLButtonElement).setPointerCapture(event.pointerId);
              } catch {
                /* capture opcional según plataforma */
              }
            }}
            onPointerCancel={(event) => {
              const el = event.currentTarget as HTMLButtonElement;
              try {
                if (typeof el.hasPointerCapture === 'function' && el.hasPointerCapture(event.pointerId)) {
                  el.releasePointerCapture(event.pointerId);
                }
              } catch {
                /* noop */
              }
            }}
            onPointerUp={(event) => {
              if (!allowEdit || isEditing) return;
              if (event.pointerType === 'mouse' && event.button !== 0) return;
              const el = event.currentTarget as HTMLButtonElement;
              try {
                if (typeof el.hasPointerCapture === 'function' && el.hasPointerCapture(event.pointerId)) {
                  el.releasePointerCapture(event.pointerId);
                }
              } catch {
                /* noop */
              }
              pointerActivatedEditRef.current = true;
              scheduleActivateEditing();
            }}
            onClick={() => {
              if (pointerActivatedEditRef.current) {
                pointerActivatedEditRef.current = false;
                return;
              }
              scheduleActivateEditing();
            }}
            aria-disabled={!allowEdit}
            title={!allowEdit ? 'Este registro no admite edición' : undefined}
          >
            {editLabel}
          </button>
        </div>
      </div>
    ) : (
      <div className="crud-entity-editor-modal__footer-layout crud-entity-editor-modal__footer-layout--compact">
        <div className="crud-entity-editor-modal__footer-group crud-entity-editor-modal__footer-group--start">
          {resolvedEditingStartActions}
        </div>
        <div className="crud-entity-editor-modal__footer-group crud-entity-editor-modal__footer-group--end">
          <button type="button" className="btn btn-secondary" onClick={() => void requestCancel()}>
            {mode === 'update' && editBehavior !== 'edit-only' ? cancelEditLabel : cancelLabel}
          </button>
          <button
            type="submit"
            form={formId}
            className="btn btn-primary"
            disabled={disableSubmit || loading || (disableSubmitWhenPristine && !dirty)}
          >
            {submitLabel}
          </button>
        </div>
      </div>
    );

  const renderFieldValue = (field: CrudEntityEditorModalField): ReactNode => {
    if (field.readValue) {
      return typeof field.readValue === 'function'
        ? field.readValue({ value: values[field.id], values })
        : field.readValue;
    }
    if (CRUD_MODAL_IMAGE_FIELD_ID_SET.has(field.id)) {
      const urls = parseCrudLinkedEntityImageUrlList(String(values[field.id] ?? ''));
      if (!urls.length) return '—';
      return <CrudEntityMediaCarousel urls={urls} variant="read" ariaLabel={field.label} />;
    }
    const value = values[field.id];
    if (field.type === 'checkbox') return value ? 'Sí' : 'No';
    if (field.type === 'select') {
      const match = field.options?.find((option) => option.value === String(value ?? ''));
      return match?.label ?? String(value ?? '');
    }
    const stringValue = String(value ?? '').trim();
    return stringValue;
  };

  const renderBlock = (block: CrudEntityEditorModalBlock) => {
    if (!isEditing) return null;
    if (block.kind === 'lineItems') {
      const setValue = (nextValue: CrudFieldValue) =>
        setValues((current) => ({ ...current, [block.field]: nextValue }));
      return (
        <div key={block.id} className="crud-entity-editor-modal__field crud-entity-editor-modal__field--full">
          {block.label ? (
            <div className="crud-entity-editor-modal__section-head">
              <h3>{block.label}</h3>
            </div>
          ) : null}
          <CrudLineItemsEditor value={values[block.field]} onChange={setValue} />
        </div>
      );
    }
    return null;
  };

  return (
    <>
      <CrudEntityModalShell
        open={open}
        variant={variant}
        titleId={titleId}
        ariaLabel={typeof title === 'string' && title.trim().length > 0 ? title : 'Detalle'}
        onRequestClose={() => void requestCancel()}
        onBackdropRequestClose={() => {
          if (suppressBackdropCloseRef.current) return;
          void requestCancel();
        }}
        rootClassName={cx('crud-entity-editor-modal-root', rootClassName)}
        backdropClassName="crud-entity-editor-modal__backdrop"
        panelClassName={cx('crud-entity-editor-modal', panelClassName)}
        headerClassName="crud-entity-editor-modal__header"
        bodyClassName="crud-entity-editor-modal__body"
        footerClassName="crud-entity-editor-modal__footer"
        pageToolbar={pageToolbar}
        pageToolbarClassName={pageToolbarClassName}
        header={
          hasHeaderContent ? (
            <div className="crud-entity-editor-modal__header-layout">
              <div className="crud-entity-editor-modal__title-block">
                {eyebrow ? <p className="crud-entity-editor-modal__eyebrow">{eyebrow}</p> : null}
                <h2 className="crud-entity-editor-modal__title" id={titleId}>
                  {title}
                </h2>
                {subtitle ? <p className="crud-entity-editor-modal__subtitle">{subtitle}</p> : null}
              </div>
              {resolvedHeaderActions ? (
                <div className="crud-entity-editor-modal__header-actions">{resolvedHeaderActions}</div>
              ) : null}
            </div>
          ) : undefined
        }
        footer={footer}
      >
        <form id={formId} className="crud-entity-editor-modal__form" onSubmit={handleSubmit}>
        {loading ? <p className="crud-entity-editor-modal__loading">{loadingLabel}</p> : null}
        {error ? <p className="crud-entity-editor-modal__error">{error}</p> : null}

        {!loading && stats?.length ? (
          <div className="crud-entity-editor-modal__stats">
            {stats.map((stat) => (
              <div
                key={stat.id}
                className={cx('crud-entity-editor-modal__stat', stat.tone && `crud-entity-editor-modal__stat--${stat.tone}`)}
              >
                <span>{stat.label}</span>
                <strong>{renderStatValue(stat, values)}</strong>
              </div>
            ))}
          </div>
        ) : null}

        {!loading && resolvedMediaUrls?.length && !(isEditing && mediaPreviewOwnedByImageEditControl) ? (
          <div className="crud-entity-editor-modal__media">
            <CrudEntityMediaCarousel urls={resolvedMediaUrls} variant={isEditing ? 'edit' : 'read'} />
          </div>
        ) : null}

        {!loading
          ? resolvedSections.map((section) => {
              const visibleFields = section.fields.filter((field) => {
                if (field.visible && !field.visible({ value: values[field.id], values, editing: isEditing })) {
                  return false;
                }
                if (
                  !isEditing &&
                  mediaFieldId &&
                  field.id === mediaFieldId &&
                  (resolvedMediaUrls?.length ?? 0) > 0
                ) {
                  return false;
                }
                return true;
              });
              const visibleBlocks = section.blocks.filter((block) =>
                block.visible ? block.visible({ values, editing: isEditing, row }) : isEditing,
              );
              if (visibleFields.length === 0 && visibleBlocks.length === 0) return null;
              return (
                <section key={section.id} className="crud-entity-editor-modal__section">
                  <div className="crud-entity-editor-modal__fields">
                    {visibleFields.map((field) => {
                      const resolvedEditControl = resolveModalImageFieldEditControl(field);
                      return (
                      <label
                        key={field.id}
                        className={cx(
                          'crud-entity-editor-modal__field',
                          field.fullWidth && 'crud-entity-editor-modal__field--full',
                          field.type === 'checkbox' && 'crud-entity-editor-modal__field--checkbox',
                        )}
                      >
                        {!isEditing ? (
                          <>
                            <span>{field.label}</span>
                            <div className="crud-entity-editor-modal__read-value">{renderFieldValue(field)}</div>
                          </>
                        ) : field.type === 'checkbox' ? (
                          <div className="crud-entity-editor-modal__checkbox-row">
                            <input
                              type="checkbox"
                              checked={Boolean(values[field.id])}
                              onChange={(event) =>
                                setValues((current) => ({ ...current, [field.id]: event.target.checked }))
                              }
                            />
                            <span>{field.label}</span>
                          </div>
                        ) : (
                          <>
                            <span>{field.label}</span>
                            {resolvedEditControl ? (
                              resolvedEditControl({
                                value: values[field.id],
                                values,
                                setValue: (nextValue) =>
                                  setValues((current) => ({ ...current, [field.id]: nextValue })),
                              })
                            ) : field.type === 'textarea' ? (
                              <textarea
                                value={String(values[field.id] ?? '')}
                                onChange={(event) =>
                                  setValues((current) => ({ ...current, [field.id]: event.target.value }))
                                }
                                placeholder={field.placeholder}
                                required={field.required}
                                rows={field.rows ?? (field.id === 'notes' ? 2 : 4)}
                                readOnly={field.readOnly}
                              />
                            ) : field.type === 'select' ? (
                              <select
                                value={String(values[field.id] ?? '')}
                                onChange={(event) =>
                                  setValues((current) => ({ ...current, [field.id]: event.target.value }))
                                }
                                required={field.required}
                                disabled={field.readOnly}
                              >
                                <option value="">{field.placeholder ?? 'Seleccionar...'}</option>
                                {(field.options ?? []).map((option) => (
                                  <option key={option.value} value={option.value}>
                                    {option.label}
                                  </option>
                                ))}
                              </select>
                            ) : (
                              <input
                                type={field.type ?? 'text'}
                                value={String(values[field.id] ?? '')}
                                onChange={(event) =>
                                  setValues((current) => ({ ...current, [field.id]: event.target.value }))
                                }
                                placeholder={field.placeholder}
                                required={field.required}
                                min={field.min}
                                step={field.step}
                                readOnly={field.readOnly}
                              />
                            )}
                          </>
                        )}
                      </label>
                      );
                    })}
                    {visibleBlocks.map((block) => renderBlock(block))}
                  </div>
                </section>
              );
            })
          : null}
        </form>
      </CrudEntityModalShell>
      <CrudEntityModalShell
        open={Boolean(pendingConfirm)}
        variant="modal"
        titleId={`${titleId}-confirm`}
        ariaLabel="Confirmación"
        onRequestClose={() => {
          if (!confirming) setPendingConfirm(null);
        }}
        disableClose={confirming}
        closeOnBackdrop={!confirming}
        rootClassName="crud-entity-editor-modal__confirm-shell"
        backdropClassName="crud-entity-editor-modal__confirm-backdrop"
        panelClassName="crud-entity-editor-modal__confirm-panel"
        headerClassName="crud-entity-editor-modal__confirm-header"
        bodyClassName="crud-entity-editor-modal__confirm-body"
        footerClassName="crud-entity-editor-modal__confirm-footer-shell"
        header={
          pendingConfirm ? (
            <h3 className="crud-entity-editor-modal__confirm-title" id={`${titleId}-confirm`}>
              {pendingConfirm.title}
            </h3>
          ) : undefined
        }
        footer={
          pendingConfirm ? (
            <div className="crud-entity-editor-modal__confirm-footer">
              <button type="button" className="btn btn-secondary" disabled={confirming} onClick={() => setPendingConfirm(null)}>
                {pendingConfirm.cancelLabel}
              </button>
              <button
                type="button"
                className={cx('btn', pendingConfirm.tone === 'danger' ? 'btn-danger' : 'btn-primary')}
                disabled={confirming}
                onClick={() => void handlePendingConfirm()}
              >
                {confirming ? 'Procesando…' : pendingConfirm.confirmLabel}
              </button>
            </div>
          ) : undefined
        }
      >
        {pendingConfirm ? (
          <p className="crud-entity-editor-modal__confirm-description">{pendingConfirm.description}</p>
        ) : null}
      </CrudEntityModalShell>
    </>
  );
}
