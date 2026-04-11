import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Link } from 'react-router-dom';
import { ImageFullscreenViewer } from '../../components/ImageFullscreenViewer';
import { formatProductImageURLsToForm, parseImageURLList } from '../../crud/resourceConfigs.shared';
import {
  defaultCrudResourceInventoryDetailFeatureFlags,
  type CrudInventoryLevelSnapshot,
  type CrudInventoryMovementSnapshot,
  type CrudLinkedEntityPatch,
  type CrudLinkedEntitySnapshot,
  type CrudResourceInventoryDetailModalProps,
} from './crudResourceInventoryDetailContract';
import {
  CrudLinkedEntityEditBodyFields,
  CrudLinkedEntityEditHeaderFields,
  CrudLinkedEntityImageGalleryStrip,
  crudLinkedEntityHasDisplayName,
} from './crudLinkedEntityInventoryFormBlock';
import { CrudInventoryQuantitiesNotesBlock } from './crudInventoryQuantitiesNotesBlock';
import './CrudResourceInventoryDetailModal.css';

function collectLinkedImageUrls(p: CrudLinkedEntitySnapshot | null): string[] {
  if (!p) return [];
  const raw = p.imageUrls?.length ? p.imageUrls : p.legacyImageUrl?.trim() ? [p.legacyImageUrl.trim()] : [];
  const out: string[] = [];
  const seen = new Set<string>();
  for (const u of raw) {
    const t = (u ?? '').trim();
    if (!t || seen.has(t)) continue;
    seen.add(t);
    out.push(t);
  }
  return out;
}

function imageUrlListsEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i += 1) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

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
  const [imageUrlsInput, setImageUrlsInput] = useState('');
  const [trackStockInput, setTrackStockInput] = useState(true);
  const imageUrlsTouchedRef = useRef(false);
  const portsRef = useRef(ports);
  portsRef.current = ports;

  const serverImageUrls = useMemo(() => collectLinkedImageUrls(linked), [linked]);
  const draftImageUrls = useMemo(() => parseImageURLList(imageUrlsInput), [imageUrlsInput]);

  const syncFormFromServer = useCallback((lvl: CrudInventoryLevelSnapshot, lnk: CrudLinkedEntitySnapshot | null) => {
    setNameInput(String(lnk?.name ?? lvl.displayTitle ?? ''));
    setSkuInput(String(lnk?.sku ?? lvl.displaySubtitle ?? ''));
    setImageUrlsInput(formatProductImageURLsToForm(lnk?.imageUrls, lnk?.legacyImageUrl));
    setTrackStockInput(lvl.trackStock !== false);
  }, []);

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
    const next = formatProductImageURLsToForm(linked.imageUrls, linked.legacyImageUrl);
    if (!next.trim()) return;
    setImageUrlsInput(next);
  }, [editing, linked]);

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

  const productDirty = useMemo(() => {
    if (!level || !flags.linkedEntityFields) return false;
    const nameBaseline = (linked?.name ?? level.displayTitle ?? '').trim();
    const skuBaseline = (linked?.sku ?? level.displaySubtitle ?? '').trim();
    const urlsBaseline = serverImageUrls;
    const trackBaseline = level.trackStock !== false;
    const trackDirty = flags.linkedEntityTrackStock && trackStockInput !== trackBaseline;
    return (
      nameInput.trim() !== nameBaseline ||
      skuInput.trim() !== skuBaseline ||
      !imageUrlListsEqual(draftImageUrls, urlsBaseline) ||
      trackDirty
    );
  }, [
    level,
    linked,
    nameInput,
    skuInput,
    draftImageUrls,
    serverImageUrls,
    trackStockInput,
    flags.linkedEntityFields,
    flags.linkedEntityTrackStock,
  ]);

  const inventoryDirty = useMemo(() => {
    if (!level || !flags.inventoryQuantities || !trackStockInput) return false;
    const minChanged = Number.isFinite(minParsed) && minParsed !== level.minQuantity;
    const qtyChanged = Number.isFinite(absoluteQtyParsed) && absoluteQtyParsed !== level.quantity;
    return minChanged || qtyChanged;
  }, [level, minParsed, absoluteQtyParsed, flags.inventoryQuantities, trackStockInput]);

  const dirty = productDirty || inventoryDirty;

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
  };

  const handleSave = async () => {
    if (!level || !canSave) return;
    const nameTrim = nameInput.trim();
    if (!crudLinkedEntityHasDisplayName(nameInput)) {
      setFormError(strings.nameRequiredError);
      return;
    }

    const minChanged = Number.isFinite(minParsed) && minParsed !== level.minQuantity;
    const qtyChanged = Number.isFinite(absoluteQtyParsed) && absoluteQtyParsed !== level.quantity;

    const nameBaseline = (linked?.name ?? level.displayTitle ?? '').trim();
    const skuBaseline = (linked?.sku ?? level.displaySubtitle ?? '').trim();
    const urlsBaseline = serverImageUrls;
    const trackBaseline = level.trackStock !== false;

    const patch: CrudLinkedEntityPatch = {};
    if (flags.linkedEntityFields) {
      if (nameTrim !== nameBaseline) patch.name = nameTrim;
      if (skuInput.trim() !== skuBaseline) patch.sku = skuInput.trim();
      if (!imageUrlListsEqual(draftImageUrls, urlsBaseline)) patch.imageUrls = draftImageUrls;
      if (flags.linkedEntityTrackStock && trackStockInput !== trackBaseline) patch.trackStock = trackStockInput;
    }

    const hasProductPatch = flags.linkedEntityFields && Object.keys(patch).length > 0;
    const hasInventoryChange = flags.inventoryQuantities && trackStockInput && (minChanged || qtyChanged);

    if (hasInventoryChange && !notes.trim()) {
      setFormError(strings.notesRequiredError);
      return;
    }

    if (!hasProductPatch && !hasInventoryChange) return;

    setSaving(true);
    setFormError('');
    try {
      let nextLinked = linked;
      if (hasProductPatch) {
        nextLinked = await portsRef.current.patchLinkedEntity(level.linkedEntityId, patch);
        setLinked(nextLinked);
      }
      if (hasInventoryChange) {
        await portsRef.current.postInventoryAdjust(level.linkedEntityId, {
          quantityDelta: qtyChanged ? absoluteQtyParsed - level.quantity : 0,
          notes: notes.trim(),
          ...(minChanged ? { minQuantity: minParsed } : {}),
        });
      }
      const nextLevel = await portsRef.current.loadInventoryLevel(level.linkedEntityId);
      setLevel(nextLevel);
      resetInventoryFields(nextLevel);
      let refreshedLinked: CrudLinkedEntitySnapshot | null = nextLinked;
      try {
        refreshedLinked = await portsRef.current.loadLinkedEntity(level.linkedEntityId);
        setLinked(refreshedLinked);
      } catch {
        setLinked(null);
        refreshedLinked = null;
      }
      syncFormFromServer(nextLevel, refreshedLinked);
      imageUrlsTouchedRef.current = false;
      setEditing(false);
      const mv = await portsRef.current.loadMovements(level.linkedEntityId);
      setMovements(mv);
      onAfterSave?.();
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : strings.saveErrorGeneric);
    } finally {
      setSaving(false);
    }
  };

  const handleArchive = async () => {
    if (!level || !ports.archiveLinkedEntity || !strings.archiveConfirm) return;
    if (!window.confirm(strings.archiveConfirm)) return;
    setArchiving(true);
    setFormError('');
    try {
      await portsRef.current.archiveLinkedEntity!(level.linkedEntityId);
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
                            imageUrlsText={imageUrlsInput}
                            onImageUrlsTextChange={setImageUrlsInput}
                            onImageUrlsInput={() => {
                              imageUrlsTouchedRef.current = true;
                            }}
                            trackStockInputId="crud-inv-detail-track"
                            trackStock={trackStockInput}
                            onTrackStockChange={setTrackStockInput}
                            showTrackStock={flags.linkedEntityTrackStock}
                            previewUrls={draftImageUrls}
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
            {flags.archiveAction && ports.archiveLinkedEntity && strings.archiveLabel ? (
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
      <ImageFullscreenViewer
        imageUrl={lightboxUrl}
        onClose={() => setLightboxUrl(null)}
        contentLabel={nameInput.trim() || level?.displayTitle}
      />
    </>
  );
}
