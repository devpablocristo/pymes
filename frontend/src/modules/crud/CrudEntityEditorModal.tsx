import { confirmAction } from '@devpablocristo/core-browser';
import { useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
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
  onSubmit: (values: Record<string, CrudFieldValue>) => Promise<void> | void;
};

type ResolvedSection = CrudEntityEditorModalSection & {
  fields: CrudEntityEditorModalField[];
  blocks: CrudEntityEditorModalBlock[];
};

type PendingConfirmDialog = {
  title: string;
  description: string;
  confirmLabel: string;
  cancelLabel: string;
  tone?: 'default' | 'danger';
  onConfirm: () => Promise<void>;
  onCancel?: () => void;
};

const PRIORITY_FIELDS_IN_EDITOR_MODAL = new Set(['tags', 'is_favorite']);

function prioritizeEditorFieldsInFirstSection(sections: ResolvedSection[]): ResolvedSection[] {
  if (!sections.length) {
    return sections;
  }

  const promoted: CrudEntityEditorModalField[] = [];
  const nextSections = sections.map((section) => {
    const remainingFields = section.fields.filter((field) => {
      if (PRIORITY_FIELDS_IN_EDITOR_MODAL.has(field.id)) {
        promoted.push(field);
        return false;
      }
      return true;
    });

    return {
      ...section,
      fields: remainingFields,
    };
  });

  if (promoted.length === 0) {
    return nextSections.filter((section) => section.fields.length > 0 || section.blocks.length > 0);
  }

  const normalizedSections = nextSections.filter((section) => section.fields.length > 0 || section.blocks.length > 0);
  const firstSection = normalizedSections[0];
  if (!firstSection) {
    return [
      {
        id: 'general',
        fields: promoted,
        blocks: [],
      },
    ];
  }

  return [
    {
      ...firstSection,
      fields: [...promoted, ...firstSection.fields],
    },
    ...normalizedSections.slice(1),
  ];
}

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
    return resolved;
  }

  return [
    {
      id: 'general',
      fields,
      blocks,
    },
  ];
}

function renderStatValue(
  stat: CrudEntityEditorModalStat,
  values: Record<string, CrudFieldValue>,
): ReactNode {
  return typeof stat.value === 'function' ? stat.value(values) : stat.value;
}

function normalizeReadString(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return '';
  if (/^[-\u2010-\u2015\s]+$/.test(trimmed)) return '';
  return trimmed;
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
  cancelLabel = 'Cerrar',
  submitLabel = 'Guardar',
  editLabel = 'Editar',
  cancelEditLabel = 'Cerrar',
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
  const formRef = useRef<HTMLFormElement | null>(null);
  const resolvedEditCloseLabel = cancelEditLabel || closeLabel;
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

  useEffect(() => {
    const shouldStartEditing = mode === 'create' || editBehavior === 'edit-only';
    setValues(initialValues);
    setIsEditing((current) => (shouldStartEditing ? true : current));
  }, [editBehavior, initialValues, mode]);

  useEffect(() => {
    if (!open) {
      setPendingConfirm(null);
      setConfirming(false);
    }
  }, [open]);

  const dirty = useMemo(
    () => !areCrudFieldValuesEqual(values, initialValues),
    [initialValues, values],
  );

  const resolvedSections = useMemo(() => {
    const resolved = resolveSections(fields, blocks, sections);
    return prioritizeEditorFieldsInFirstSection(resolved);
  }, [blocks, fields, sections]);
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
    if (!dirty) {
      onCancel();
      return;
    }
    setPendingConfirm({
      title: confirmDiscard?.title ?? 'Desea guardar los cambios?',
      description: confirmDiscard?.description ?? 'Hay cambios sin guardar.',
      confirmLabel: 'Guardar',
      cancelLabel: 'Cerrar',
      onConfirm: async () => {
        const saved = await submitCurrentValues(formRef.current);
        if (!saved) return;
        onCancel();
      },
      onCancel: () => {
        setPendingConfirm(null);
        setConfirming(false);
        setValues(initialValues);
        if (mode === 'update' && editBehavior !== 'edit-only') {
          setIsEditing(false);
        }
        onCancel();
      },
    });
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

  const submitCurrentValues = async (form: HTMLFormElement | null) => {
    if (!form?.reportValidity()) return false;
    await onSubmit(values);
    return true;
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submitCurrentValues(event.currentTarget);
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

  const startEditing = () => {
    if (typeof window === 'undefined') {
      setIsEditing(true);
      return;
    }
    window.setTimeout(() => setIsEditing(true), 0);
  };

  const handleSecondaryClose = () => {
    void requestCancel();
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
            className="btn btn-primary"
            onClick={startEditing}
            disabled={!allowEdit}
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
          <button type="button" className="btn btn-secondary" onClick={handleSecondaryClose}>
            {mode === 'update' && editBehavior !== 'edit-only' ? resolvedEditCloseLabel : cancelLabel}
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
    const value = values[field.id];
    if (field.type === 'checkbox') return value ? 'Sí' : 'No';
    if (field.type === 'select') {
      const match = field.options?.find((option) => option.value === String(value ?? ''));
      return normalizeReadString(String(match?.label ?? value ?? ''));
    }
    return normalizeReadString(String(value ?? ''));
  };

  const renderBlock = (block: CrudEntityEditorModalBlock) => {
    if (!isEditing) return null;
    if (block.kind === 'lineItems') {
      const setValue = (nextValue: CrudFieldValue) =>
        setValues((current) => ({ ...current, [block.field]: nextValue }));
      return (
        <div key={block.id} className="crud-entity-editor-modal__field crud-entity-editor-modal__field--full">
          {block.label ? <span>{block.label}</span> : null}
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
        <form ref={formRef} id={formId} className="crud-entity-editor-modal__form" onSubmit={handleSubmit}>
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

        {!loading && resolvedMediaUrls?.length ? (
          <div className="crud-entity-editor-modal__media">
            <CrudEntityMediaCarousel urls={resolvedMediaUrls} variant={isEditing ? 'edit' : 'read'} />
          </div>
        ) : null}

        {!loading
          ? resolvedSections.map((section) => {
              const visibleFields = section.fields.filter((field) =>
                field.visible ? field.visible({ value: values[field.id], values, editing: isEditing }) : true,
              );
              const visibleBlocks = section.blocks.filter((block) =>
                block.visible ? block.visible({ values, editing: isEditing, row }) : isEditing,
              );
              if (visibleFields.length === 0 && visibleBlocks.length === 0) return null;
              return (
                <section key={section.id} className="crud-entity-editor-modal__section">
                  <div className="crud-entity-editor-modal__fields">
                    {visibleFields.map((field) => (
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
                            {field.type === 'checkbox' ? (
                              <div className="crud-entity-editor-modal__checkbox-row crud-entity-editor-modal__checkbox-row--read">
                                <input type="checkbox" checked={Boolean(values[field.id])} readOnly disabled />
                                <span>{field.label}</span>
                              </div>
                            ) : (
                              <>
                                <span>{field.label}</span>
                                <div className="crud-entity-editor-modal__read-value">{renderFieldValue(field)}</div>
                              </>
                            )}
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
                            {field.editControl ? (
                              field.editControl({
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
                                rows={field.rows ?? 2}
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
                        {isEditing && field.helperText ? <small>{field.helperText}</small> : null}
                      </label>
                    ))}
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
              <button
                type="button"
                className="btn btn-secondary"
                disabled={confirming}
                onClick={() => {
                  if (pendingConfirm.onCancel) {
                    pendingConfirm.onCancel();
                    return;
                  }
                  setPendingConfirm(null);
                }}
              >
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
