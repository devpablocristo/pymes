import { confirmAction } from '@devpablocristo/core-browser';
import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { parseCrudLinkedEntityImageUrlList } from './crudLinkedEntityImageUrls';
import { CrudEntityModalShell } from './CrudEntityModalShell';
import { CrudEntityMediaCarousel } from './CrudEntityMediaCarousel';
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

export type CrudEntityEditorModalProps = {
  open: boolean;
  title: ReactNode;
  subtitle?: ReactNode;
  eyebrow?: ReactNode;
  mediaUrls?: string[];
  mediaFieldId?: string;
  mode?: 'create' | 'update';
  cancelLabel?: string;
  submitLabel?: string;
  editLabel?: string;
  cancelEditLabel?: string;
  closeLabel?: string;
  fields: CrudEntityEditorModalField[];
  sections?: CrudEntityEditorModalSection[];
  stats?: CrudEntityEditorModalStat[];
  error?: ReactNode;
  loading?: boolean;
  loadingLabel?: ReactNode;
  disableSubmit?: boolean;
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
  onCancel: () => void;
  onSubmit: (values: Record<string, CrudFieldValue>) => void;
};

type ResolvedSection = CrudEntityEditorModalSection & {
  fields: CrudEntityEditorModalField[];
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
  sections: CrudEntityEditorModalSection[] | undefined,
): ResolvedSection[] {
  if (sections?.length) {
    const used = new Set<string>();
    const resolved = sections
      .map((section) => {
        const sectionFields = fields.filter((field) => field.sectionId === section.id);
        sectionFields.forEach((field) => used.add(field.id));
        return { ...section, fields: sectionFields };
      })
      .filter((section) => section.fields.length > 0);

    const rest = fields.filter((field) => !used.has(field.id));
    if (rest.length) {
      resolved.unshift({
        id: 'general',
        fields: rest,
      });
    }
    return resolved;
  }

  return [
    {
      id: 'general',
      fields,
    },
  ];
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
  mediaUrls,
  mediaFieldId,
  mode = 'create',
  cancelLabel = 'Cancelar',
  submitLabel = 'Guardar',
  editLabel = 'Editar',
  cancelEditLabel = 'Cancelar edición',
  closeLabel = 'Cerrar',
  fields,
  sections,
  stats,
  error,
  loading = false,
  loadingLabel = 'Cargando…',
  disableSubmit = false,
  confirmDiscard,
  archiveAction,
  onCancel,
  onSubmit,
}: CrudEntityEditorModalProps) {
  const titleId = 'crud-entity-editor-modal-title';
  const formId = 'crud-entity-editor-modal-form';
  const initialValues = useMemo(
    () =>
      Object.fromEntries(fields.map((field) => [field.id, field.defaultValue ?? (field.type === 'checkbox' ? false : '')])),
    [fields],
  );
  const [values, setValues] = useState<Record<string, CrudFieldValue>>(initialValues);
  const [isEditing, setIsEditing] = useState(mode === 'create');
  const [archiving, setArchiving] = useState(false);

  useEffect(() => {
    setValues(initialValues);
    setIsEditing(mode === 'create');
  }, [initialValues]);

  const dirty = useMemo(
    () => !areCrudFieldValuesEqual(values, initialValues),
    [initialValues, values],
  );

  const resolvedSections = useMemo(() => resolveSections(fields, sections), [fields, sections]);
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
    if (mode === 'update' && isEditing) {
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

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    if (!form.reportValidity()) return;
    onSubmit(values);
  };

  const handleArchive = async () => {
    if (!archiveAction || archiving) return;
    if (archiveAction.confirm) {
      const confirmed = await confirmAction({
        title: archiveAction.confirm.title,
        description: archiveAction.confirm.description,
        confirmLabel: archiveAction.confirm.confirmLabel ?? archiveAction.label ?? 'Archivar',
        cancelLabel: archiveAction.confirm.cancelLabel ?? 'Cancelar',
        tone: 'danger',
      });
      if (!confirmed) return;
    }
    setArchiving(true);
    try {
      await archiveAction.onArchive();
    } finally {
      setArchiving(false);
    }
  };

  const footer =
    mode === 'update' && !isEditing ? (
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
          <button type="button" className="btn btn-primary" onClick={() => setIsEditing(true)}>
            {editLabel}
          </button>
        </div>
      </div>
    ) : (
      <div className="crud-entity-editor-modal__footer-layout crud-entity-editor-modal__footer-layout--compact">
        <div className="crud-entity-editor-modal__footer-group crud-entity-editor-modal__footer-group--end">
          <button type="button" className="btn btn-secondary" onClick={() => void requestCancel()}>
            {mode === 'update' ? cancelEditLabel : cancelLabel}
          </button>
          <button type="submit" form={formId} className="btn btn-primary" disabled={disableSubmit || loading}>
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
      return match?.label ?? String(value ?? '—');
    }
    const stringValue = String(value ?? '').trim();
    return stringValue || '—';
  };

  return (
    <CrudEntityModalShell
      open={open}
      titleId={titleId}
      ariaLabel={typeof title === 'string' && title.trim().length > 0 ? title : 'Detalle'}
      onRequestClose={() => void requestCancel()}
      rootClassName="crud-entity-editor-modal-root"
      backdropClassName="crud-entity-editor-modal__backdrop"
      panelClassName="crud-entity-editor-modal"
      headerClassName="crud-entity-editor-modal__header"
      bodyClassName="crud-entity-editor-modal__body"
      footerClassName="crud-entity-editor-modal__footer"
      header={
        hasHeaderContent ? (
          <div className="crud-entity-editor-modal__title-block">
            {eyebrow ? <p className="crud-entity-editor-modal__eyebrow">{eyebrow}</p> : null}
            <h2 className="crud-entity-editor-modal__title" id={titleId}>
              {title}
            </h2>
            {subtitle ? <p className="crud-entity-editor-modal__subtitle">{subtitle}</p> : null}
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
              if (visibleFields.length === 0) return null;
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
                                rows={field.rows ?? 4}
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
                  </div>
                </section>
              );
            })
          : null}
      </form>
    </CrudEntityModalShell>
  );
}
