import { confirmAction } from '@devpablocristo/core-browser';
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { flushSync } from 'react-dom';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { parseCrudLinkedEntityImageUrlList } from './crudLinkedEntityImageUrls';
import {
  areCrudFieldValuesEqual,
  CRUD_MODAL_IMAGE_FIELD_ID_SET,
  cx,
  renderStatValue,
  resolveModalImageFieldEditControl,
  resolveSections,
  type CrudEntityEditorModalBlock,
  type CrudEntityEditorModalField,
  type CrudEntityEditorModalProps,
} from './CrudEntityEditorModal.model';
import { CrudEntityModalShell } from './CrudEntityModalShell';
import { CrudEntityMediaCarousel } from './CrudEntityMediaCarousel';
import { CrudLineItemsEditor } from './CrudLineItemsEditor';
import './CrudEntityEditorModal.css';

export type {
  CrudEntityEditorModalBlock,
  CrudEntityEditorModalField,
  CrudEntityEditorModalProps,
  CrudEntityEditorModalSection,
  CrudEntityEditorModalStat,
} from './CrudEntityEditorModal.model';

type PendingConfirmDialog = {
  title: string;
  description: string;
  confirmLabel: string;
  cancelLabel: string;
  tone?: 'default' | 'danger';
  onConfirm: () => Promise<void>;
};

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
