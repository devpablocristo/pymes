import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Link } from 'react-router-dom';
import { CrudImageFullscreenViewer } from './CrudImageFullscreenViewer';
import {
  collectCrudImageUrls,
  formatCrudLinkedEntityImageUrlsToForm,
} from './crudLinkedEntityImageUrls';
import {
  defaultCrudResourceInventoryDetailFeatureFlags,
  type CrudInventoryLevelSnapshot,
  type CrudInventoryMovementSnapshot,
  type CrudLinkedEntitySnapshot,
  type CrudResourceInventoryDetailModalProps,
} from './crudResourceInventoryDetailContract';
import {
  buildCrudInventoryAdjustPayload,
  buildCrudInventoryDetailSavePatch,
  computeCrudInventoryDetailDirty,
  persistCrudInventoryDetailSave,
  validateCrudInventoryDetailSave,
} from './crudResourceInventoryDetailSaveOrchestration';
import {
  CrudLinkedEntityEditBodyFields,
  CrudLinkedEntityEditHeaderFields,
  CrudLinkedEntityImageGalleryStrip,
  crudLinkedEntityHasDisplayName,
} from './crudLinkedEntityInventoryFormBlock';
import { CrudInventoryQuantitiesNotesBlock } from './crudInventoryQuantitiesNotesBlock';
import './CrudResourceInventoryDetailModal.css';

/**
 * Shell + contenido estándar: portal, backdrop, header, body con scroll, footer,
 * carga/error, lectura vs edición. Sin URLs de API: todo vía `ports` y `strings`.
 */
export function CrudResourceInventoryDetailModal<TMove extends CrudInventoryMovementSnapshot = CrudInventoryMovementSnapshot>({
  linkedEntityId,
  onClose,
  onAfterSave,
  strings,
  flags: flagsProp,
  ports,
  formatMovementKind,
  formatDateTime,
  advancedSettingsHref,
  onArchive,
  onCancelEdit,
  permissions,
}: CrudResourceInventoryDetailModalProps<TMove>) {
  const flags = useMemo(
    () => ({ ...defaultCrudResourceInventoryDetailFeatureFlags, ...flagsProp }),
    [flagsProp],
  );

  const [level, setLevel] = useState<CrudInventoryLevelSnapshot | null>(null);
  const [linked, setLinked] = useState<CrudLinkedEntitySnapshot | null>(null);
  const [movements, setMovements] = useState<TMove[]>([]);
  const [movementsLoading, setMovementsLoading] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [minInput, setMinInput] = useState('');
  const [absoluteQtyInput, setAbsoluteQtyInput] = useState('');
  const [notes, setNotes] = useState('');
  const [formError, setFormError] = useState('');
  const [saving, setSaving] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const [lightboxUrl, setLightboxUrl] = useState<string | null>(null);
  const [nameInput, setNameInput] = useState('');
  const [skuInput, setSkuInput] = useState('');
  const [imageUrlsInput, setImageUrlsInput] = useState<string[]>([]);
  const [trackStockInput, setTrackStockInput] = useState(true);
  const [uploadingImages, setUploadingImages] = useState(false);
  const imageUrlsTouchedRef = useRef(false);
  const portsRef = useRef(ports);
  portsRef.current = ports;
  const onArchiveRef = useRef(onArchive);
  onArchiveRef.current = onArchive;

  const canArchive = permissions?.canArchive !== false;
  const archiveLinkedEntityFn = onArchive ?? ports.archiveLinkedEntity;

  const serverImageUrls = useMemo(
    () => collectCrudImageUrls({ imageUrls: linked?.imageUrls, legacyImageUrl: linked?.legacyImageUrl }),
    [linked],
  );
  const draftImageUrls = imageUrlsInput;

  const toImageUrlArray = useCallback((lnk: CrudLinkedEntitySnapshot | null): string[] => {
    return formatCrudLinkedEntityImageUrlsToForm(lnk?.imageUrls, lnk?.legacyImageUrl)
      .split('\n')
      .map((url) => url.trim())
      .filter(Boolean);
  }, []);

  const syncFormFromServer = useCallback((lvl: CrudInventoryLevelSnapshot, lnk: CrudLinkedEntitySnapshot | null) => {
    setNameInput(String(lnk?.name ?? lvl.displayTitle ?? ''));
    setSkuInput(String(lnk?.sku ?? lvl.displaySubtitle ?? ''));
    setImageUrlsInput(toImageUrlArray(lnk));
    setTrackStockInput(lvl.trackStock !== false);
  }, [toImageUrlArray]);

  const resetInventoryFields = useCallback((lvl: CrudInventoryLevelSnapshot) => {
    setMinInput(String(lvl.minQuantity ?? ''));
    setAbsoluteQtyInput(String(lvl.quantity ?? ''));
    setNotes('');
    setFormError('');
  }, []);

  useEffect(() => {
    setEditing(false);
  }, [linkedEntityId]);

  useEffect(() => {
    setLightboxUrl(null);
  }, [linkedEntityId]);

  useEffect(() => {
    if (!linkedEntityId) {
      setLevel(null);
      setLinked(null);
      setLoadError(null);
      setMovements([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setLoadError(null);
    void Promise.all([
      portsRef.current.loadInventoryLevel(linkedEntityId),
      portsRef.current.loadLinkedEntity(linkedEntityId).catch(() => null),
    ])
      .then(([lvl, lnk]) => {
        if (cancelled) return;
        setLevel(lvl);
        setLinked(lnk);
        resetInventoryFields(lvl);
        syncFormFromServer(lvl, lnk);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setLevel(null);
        setLinked(null);
        setLoadError(e instanceof Error ? e.message : strings.loadErrorGeneric);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [linkedEntityId, resetInventoryFields, strings.loadErrorGeneric, syncFormFromServer]);

  useEffect(() => {
    if (!linkedEntityId || !level) return;
    let cancelled = false;
    setMovementsLoading(true);
    void portsRef.current
      .loadMovements(linkedEntityId)
      .then((items) => {
        if (!cancelled) setMovements(items);
      })
      .catch(() => {
        if (!cancelled) setMovements([]);
      })
      .finally(() => {
        if (!cancelled) setMovementsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [linkedEntityId, level?.linkedEntityId]);

  useEffect(() => {
    if (!editing || !linked || imageUrlsTouchedRef.current) return;
    const next = toImageUrlArray(linked);
    if (!next.length) return;
    setImageUrlsInput(next);
  }, [editing, linked, toImageUrlArray]);

  useEffect(() => {
    if (!linkedEntityId) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [linkedEntityId, onClose]);

  /** Sin control de stock, las cantidades no se editan: volver a los valores del nivel. */
  useEffect(() => {
    if (!level || !editing) return;
    if (!trackStockInput) {
      setAbsoluteQtyInput(String(level.quantity ?? ''));
      setMinInput(String(level.minQuantity ?? ''));
    }
  }, [trackStockInput, level, editing]);

  const minParsed = useMemo(() => {
    const n = Number(String(minInput).replace(',', '.'));
    return Number.isFinite(n) ? n : NaN;
  }, [minInput]);

  const absoluteQtyParsed = useMemo(() => {
    const t = absoluteQtyInput.trim();
    if (!t) return NaN;
    const n = Number(t.replace(',', '.'));
    return Number.isFinite(n) ? n : NaN;
  }, [absoluteQtyInput]);

  const { inventoryDirty, dirty } = useMemo(() => {
    if (!level) {
      return { productDirty: false, inventoryDirty: false, dirty: false };
    }
    return computeCrudInventoryDetailDirty(
      level,
      linked,
      serverImageUrls,
      draftImageUrls,
      nameInput,
      skuInput,
      trackStockInput,
      { minParsed, absoluteQtyParsed },
      flags,
    );
  }, [
    level,
    linked,
    serverImageUrls,
    draftImageUrls,
    nameInput,
    skuInput,
    trackStockInput,
    minParsed,
    absoluteQtyParsed,
    flags,
  ]);

  const canSave = useMemo(() => {
    if (level == null || !dirty || saving || !editing) return false;
    if (inventoryDirty && notes.trim().length === 0) return false;
    if (!crudLinkedEntityHasDisplayName(nameInput)) return false;
    return true;
  }, [level, dirty, inventoryDirty, notes, saving, editing, nameInput]);

  const cancelEditing = () => {
    if (level) {
      resetInventoryFields(level);
      syncFormFromServer(level, linked);
    }
    imageUrlsTouchedRef.current = false;
    setEditing(false);
    onCancelEdit?.();
  };

  const handleUploadImages = useCallback(
    async (files: File[]) => {
      if (!level || !portsRef.current.uploadLinkedEntityImages || files.length === 0) return;
      setUploadingImages(true);
      setFormError('');
      try {
        const uploadedUrls = await portsRef.current.uploadLinkedEntityImages(level.linkedEntityId, files);
        setImageUrlsInput((current) => {
          const merged = [...current];
          for (const url of uploadedUrls) {
            const trimmed = url.trim();
            if (!trimmed || merged.includes(trimmed)) continue;
            merged.push(trimmed);
          }
          return merged;
        });
      } catch (e: unknown) {
        setFormError(e instanceof Error ? e.message : strings.saveErrorGeneric);
      } finally {
        setUploadingImages(false);
      }
    },
    [level, strings.saveErrorGeneric],
  );

  const handleSave = async () => {
    if (!level || !canSave) return;
    const nameTrim = nameInput.trim();
    const skuTrim = skuInput.trim();
    const notesTrim = notes.trim();
    const build = buildCrudInventoryDetailSavePatch(
      level,
      linked,
      serverImageUrls,
      draftImageUrls,
      nameTrim,
      skuTrim,
      trackStockInput,
      { minParsed, absoluteQtyParsed },
      flags,
    );
    const validation = validateCrudInventoryDetailSave(
      build.hasProductPatch,
      build.hasInventoryChange,
      crudLinkedEntityHasDisplayName(nameInput),
      notesTrim,
    );
    if (!validation.ok) {
      if (validation.kind === 'noop') return;
      if (validation.kind === 'name') setFormError(strings.nameRequiredError);
      else setFormError(strings.notesRequiredError);
      return;
    }

    setSaving(true);
    setFormError('');
    try {
      const adjustPayload = buildCrudInventoryAdjustPayload(level, { minParsed, absoluteQtyParsed }, build, notesTrim);
      const { level: nextLevel, linked: refreshedLinked, movements: mv } = await persistCrudInventoryDetailSave(
        portsRef.current,
        {
          linkedEntityId: level.linkedEntityId,
          hasProductPatch: build.hasProductPatch,
          patch: build.patch,
          hasInventoryChange: build.hasInventoryChange,
          adjustPayload,
        },
      );
      setLevel(nextLevel);
      setLinked(refreshedLinked);
      resetInventoryFields(nextLevel);
      syncFormFromServer(nextLevel, refreshedLinked);
      imageUrlsTouchedRef.current = false;
      setEditing(false);
      setMovements(mv as TMove[]);
      onAfterSave?.();
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : strings.saveErrorGeneric);
    } finally {
      setSaving(false);
    }
  };

  const handleArchive = async () => {
    const runArchive = onArchiveRef.current ?? portsRef.current.archiveLinkedEntity;
    if (!level || !runArchive || !strings.archiveConfirm) return;
    if (!window.confirm(strings.archiveConfirm)) return;
    setArchiving(true);
    setFormError('');
    try {
      await runArchive(level.linkedEntityId);
      onAfterSave?.();
      onClose();
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : strings.archiveError ?? strings.saveErrorGeneric);
    } finally {
      setArchiving(false);
    }
  };

  if (!linkedEntityId) return null;

  const dialogTitleId = 'crud-inv-detail-modal-title';
  const readGalleryUrls = !editing ? serverImageUrls : [];
  const nameFieldError =
    editing && flags.linkedEntityFields && formError === strings.nameRequiredError ? formError : undefined;

  const body = (
    <div className="crud-inv-detail-modal-root">
      <button type="button" className="crud-inv-detail-modal__backdrop" aria-label={strings.closeLabel} onClick={onClose} />
      <div
        className="crud-inv-detail-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby={dialogTitleId}
        onClick={(e) => e.stopPropagation()}
      >
        <header className="crud-inv-detail-modal__header">
          <div className="crud-inv-detail-modal__title-block">
            {loading ? (
              <h2 id={dialogTitleId} className="crud-inv-detail-modal__title">
                {strings.dialogLoadingTitle}
              </h2>
            ) : editing && level && flags.linkedEntityFields ? (
              <CrudLinkedEntityEditHeaderFields
                strings={strings}
                titleInputId={dialogTitleId}
                skuInputId="crud-inv-detail-modal-sku"
                name={nameInput}
                onNameChange={setNameInput}
                sku={skuInput}
                onSkuChange={setSkuInput}
                titleInputClassName="crud-inv-detail-modal__title-input"
                subtitleInputClassName="crud-inv-detail-modal__subtitle-input"
                nameFieldError={nameFieldError}
              />
            ) : (
              <>
                <h2 id={dialogTitleId} className="crud-inv-detail-modal__title">
                  {level?.displayTitle ?? strings.dialogFallbackTitle}
                </h2>
                {level ? (
                  <p className="crud-inv-detail-modal__subtitle">{level.displaySubtitle.trim() || '—'}</p>
                ) : null}
              </>
            )}
          </div>
        </header>

        <div className="crud-inv-detail-modal__body">
          {!editing && readGalleryUrls.length > 0 ? (
            <CrudLinkedEntityImageGalleryStrip
              urls={readGalleryUrls}
              ariaLabel={strings.galleryAriaLabel}
              openImageLabel={strings.openImageFullscreenLabel}
              onOpenImage={setLightboxUrl}
              rootClassName="crud-inv-detail-modal__gallery"
              itemClassName="crud-inv-detail-modal__gallery-item"
              zoomButtonClassName="crud-inv-detail-modal__gallery-zoom"
            />
          ) : null}

          <div className="crud-inv-detail-modal__main">
            {loadError ? <p className="crud-inv-detail-modal__error">{loadError}</p> : null}
            {level && !loading ? (
              <>
                {formError && !editing ? <p className="crud-inv-detail-modal__error">{formError}</p> : null}

                <div className="crud-inv-detail-modal__detail">
                  {flags.inventoryQuantities && level.isLowStock ? (
                    <div style={{ marginBottom: 'var(--space-3)' }}>
                      <span className="crud-inv-detail-modal__badge-low">{strings.badgeLowStock}</span>
                    </div>
                  ) : null}

                  {!editing ? (
                    flags.inventoryQuantities ? (
                      <>
                        <div className="crud-inv-detail-modal__stats">
                          <div className="crud-inv-detail-modal__stat">
                            <span>{strings.statCurrentLabel}</span>
                            <strong>{level.quantity}</strong>
                          </div>
                          <div className="crud-inv-detail-modal__stat">
                            <span>{strings.statMinLabel}</span>
                            <strong>{level.minQuantity}</strong>
                          </div>
                          <div className="crud-inv-detail-modal__stat">
                            <span>{strings.statUpdatedLabel}</span>
                            <strong>{formatDateTime(level.updatedAt)}</strong>
                          </div>
                        </div>
                        <p className="crud-inv-detail-modal__muted">{strings.readHintEdit}</p>
                      </>
                    ) : null
                  ) : (
                    <div className="crud-inv-detail-modal__section">
                      <h4>{strings.sectionEditHeading}</h4>
                      {formError ? <p className="crud-inv-detail-modal__error">{formError}</p> : null}
                      <div className="crud-inv-detail-modal__form-grid">
                        {flags.linkedEntityFields ? (
                          <CrudLinkedEntityEditBodyFields
                            strings={strings}
                            imageUrlsInputId="crud-inv-detail-image-urls"
                            imageUrls={imageUrlsInput}
                            onImageUrlsChange={setImageUrlsInput}
                            onImageUrlsInput={() => {
                              imageUrlsTouchedRef.current = true;
                            }}
                            onUploadImages={ports.uploadLinkedEntityImages ? handleUploadImages : undefined}
                            imageUploadDisabled={uploadingImages || saving}
                            trackStockInputId="crud-inv-detail-track"
                            trackStock={trackStockInput}
                            onTrackStockChange={setTrackStockInput}
                            showTrackStock={flags.linkedEntityTrackStock}
                            onOpenPreviewImage={setLightboxUrl}
                            galleryRootClassName="crud-inv-detail-modal__gallery"
                            galleryItemClassName="crud-inv-detail-modal__gallery-item"
                            galleryZoomClassName="crud-inv-detail-modal__gallery-zoom"
                          />
                        ) : null}
                        {flags.inventoryQuantities ? (
                          <CrudInventoryQuantitiesNotesBlock
                            strings={strings}
                            formatDateTime={formatDateTime}
                            updatedAtIso={level.updatedAt}
                            quantityInputId="crud-inv-detail-qty"
                            quantityValue={absoluteQtyInput}
                            onQuantityChange={setAbsoluteQtyInput}
                            quantityDisabled={!trackStockInput}
                            minInputId="crud-inv-detail-min"
                            minValue={minInput}
                            onMinChange={setMinInput}
                            minDisabled={!trackStockInput}
                            notesInputId="crud-inv-detail-notes"
                            notesValue={notes}
                            onNotesChange={setNotes}
                            notesRequired={inventoryDirty}
                          />
                        ) : null}
                      </div>
                      {strings.linkToAdvancedSettings && advancedSettingsHref ? (
                        <Link className="crud-inv-detail-modal__link" to={advancedSettingsHref}>
                          {strings.linkToAdvancedSettings}
                        </Link>
                      ) : null}
                    </div>
                  )}

                  {flags.movementsTable ? (
                    <div className="crud-inv-detail-modal__section">
                      <h4>{strings.movementsHeading}</h4>
                      {movementsLoading ? (
                        <p className="text-secondary">{strings.movementsLoading}</p>
                      ) : movements.length === 0 ? (
                        <p className="crud-inv-detail-modal__empty">{strings.movementsEmpty}</p>
                      ) : (
                        <table className="crud-inv-detail-modal__movements-table">
                          <thead>
                            <tr>
                              <th>{strings.movementColumns.kind}</th>
                              <th className="crud-inv-detail-modal__col-num">{strings.movementColumns.quantity}</th>
                              <th>{strings.movementColumns.reason}</th>
                              <th>{strings.movementColumns.user}</th>
                              <th className="crud-inv-detail-modal__col-date">{strings.movementColumns.date}</th>
                            </tr>
                          </thead>
                          <tbody>
                            {movements.map((movement) => (
                              <tr key={movement.id}>
                                <td>{formatMovementKind(movement.kind)}</td>
                                <td className="crud-inv-detail-modal__col-num">
                                  <span
                                    className={
                                      movement.kind === 'in'
                                        ? 'crud-inv-detail-modal__qty--in'
                                        : movement.kind === 'out'
                                          ? 'crud-inv-detail-modal__qty--out'
                                          : ''
                                    }
                                  >
                                    {movement.quantity > 0 ? `+${movement.quantity}` : movement.quantity}
                                  </span>
                                </td>
                                <td>{movement.reason || movement.notes || '—'}</td>
                                <td>{movement.actorLabel || '—'}</td>
                                <td className="crud-inv-detail-modal__col-date">{formatDateTime(movement.createdAt)}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      )}
                    </div>
                  ) : null}
                </div>
              </>
            ) : loading ? (
              <p className="text-secondary">{strings.loadingBodyLabel}</p>
            ) : null}
          </div>
        </div>

        <footer className="crud-inv-detail-modal__footer">
          <div className="crud-inv-detail-modal__footer-actions">
            {flags.archiveAction && canArchive && archiveLinkedEntityFn && strings.archiveLabel ? (
              <button
                type="button"
                className="btn-sm btn-danger"
                disabled={!level || archiving}
                onClick={() => void handleArchive()}
              >
                {archiving ? strings.archivingLabel : strings.archiveLabel}
              </button>
            ) : null}
          </div>
          <div className="crud-inv-detail-modal__footer-actions">
            {!editing ? (
              <>
                <button type="button" className="btn-sm btn-secondary" onClick={onClose}>
                  {strings.closeLabel}
                </button>
                <button
                  type="button"
                  className="btn-sm btn-primary"
                  disabled={!level}
                  onClick={() => {
                    imageUrlsTouchedRef.current = false;
                    setFormError('');
                    if (level) {
                      resetInventoryFields(level);
                      syncFormFromServer(level, linked);
                    }
                    setEditing(true);
                  }}
                >
                  {strings.editLabel}
                </button>
              </>
            ) : (
              <>
                <button type="button" className="btn-sm btn-secondary" onClick={cancelEditing}>
                  {strings.cancelEditLabel}
                </button>
                <button type="button" className="btn-sm btn-secondary" onClick={onClose}>
                  {strings.closeLabel}
                </button>
                <button type="button" className="btn-sm btn-primary" disabled={!canSave} onClick={() => void handleSave()}>
                  {saving ? strings.savingLabel : strings.saveLabel}
                </button>
              </>
            )}
          </div>
        </footer>
      </div>
    </div>
  );

  return (
    <>
      {createPortal(body, document.body)}
      <CrudImageFullscreenViewer
        imageUrl={lightboxUrl}
        onClose={() => setLightboxUrl(null)}
        contentLabel={nameInput.trim() || level?.displayTitle}
      />
    </>
  );
}
