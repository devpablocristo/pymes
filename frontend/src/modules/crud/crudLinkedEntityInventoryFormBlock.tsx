import type { CrudResourceInventoryDetailStrings } from './crudResourceInventoryDetailContract';
import './crudLinkedEntityInventoryFormBlock.css';

export type CrudLinkedEntityFormBlockStrings = Pick<
  CrudResourceInventoryDetailStrings,
  | 'fieldDisplayNameLabel'
  | 'fieldSkuLabel'
  | 'fieldImageUrlsLabel'
  | 'fieldImageUrlsHint'
  | 'fieldImageUploadActionLabel'
  | 'fieldImageUploadingLabel'
  | 'fieldImageRemoveLabel'
  | 'fieldTrackStockLabel'
  | 'galleryAriaLabel'
  | 'openImageFullscreenLabel'
>;

/** Nombre visible no vacío (trim). */
export function crudLinkedEntityHasDisplayName(name: string): boolean {
  return name.trim().length > 0;
}

export type CrudLinkedEntityImageGalleryStripProps = {
  urls: string[];
  ariaLabel: string;
  openImageLabel: string;
  onOpenImage: (url: string) => void;
  /** Clases del contenedor shell (p. ej. `crud-inv-detail-modal__gallery`). */
  rootClassName: string;
  itemClassName: string;
  zoomButtonClassName: string;
};

/**
 * Galería horizontal de URLs (lectura o preview en edición); sin fetch ni dominio.
 */
export function CrudLinkedEntityImageGalleryStrip({
  urls,
  ariaLabel,
  openImageLabel,
  onOpenImage,
  rootClassName,
  itemClassName,
  zoomButtonClassName,
}: CrudLinkedEntityImageGalleryStripProps) {
  if (!urls.length) return null;
  return (
    <section className={rootClassName} aria-label={ariaLabel}>
      {urls.map((url) => (
        <figure key={url} className={itemClassName}>
          <button type="button" className={zoomButtonClassName} onClick={() => onOpenImage(url)} aria-label={openImageLabel}>
            <img
              src={url}
              alt=""
              loading="lazy"
              onError={(e) => {
                const fig = (e.currentTarget as HTMLImageElement).closest('figure');
                if (fig) fig.hidden = true;
              }}
            />
          </button>
        </figure>
      ))}
    </section>
  );
}

export type CrudLinkedEntityEditHeaderFieldsProps = {
  strings: Pick<CrudResourceInventoryDetailStrings, 'fieldDisplayNameLabel' | 'fieldSkuLabel'>;
  /** Id del título (p. ej. `aria-labelledby` del diálogo). */
  titleInputId: string;
  skuInputId: string;
  name: string;
  onNameChange: (value: string) => void;
  sku: string;
  onSkuChange: (value: string) => void;
  titleInputClassName: string;
  subtitleInputClassName: string;
  /** Tras intento de guardar con nombre vacío u otra validación del padre. */
  nameFieldError?: string | null;
};

/** Nombre + SKU en cabecera (estilos de título inyectados por clase). */
export function CrudLinkedEntityEditHeaderFields({
  strings,
  titleInputId,
  skuInputId,
  name,
  onNameChange,
  sku,
  onSkuChange,
  titleInputClassName,
  subtitleInputClassName,
  nameFieldError,
}: CrudLinkedEntityEditHeaderFieldsProps) {
  return (
    <div className="crud-linked-entity-form__header">
      <label htmlFor={titleInputId} className="sr-only">
        {strings.fieldDisplayNameLabel}
      </label>
      <input
        id={titleInputId}
        className={titleInputClassName}
        value={name}
        onChange={(e) => onNameChange(e.target.value)}
        autoComplete="off"
      />
      {nameFieldError ? <p className="crud-linked-entity-form__error crud-linked-entity-form__name-error">{nameFieldError}</p> : null}
      <label htmlFor={skuInputId} className="sr-only">
        {strings.fieldSkuLabel}
      </label>
      <input
        id={skuInputId}
        className={subtitleInputClassName}
        value={sku}
        onChange={(e) => onSkuChange(e.target.value)}
        placeholder={strings.fieldSkuLabel}
        autoComplete="off"
      />
    </div>
  );
}

export type CrudLinkedEntityEditBodyFieldsProps = {
  strings: CrudLinkedEntityFormBlockStrings;
  imageUrlsInputId: string;
  imageUrls: string[];
  onImageUrlsChange: (value: string[]) => void;
  onImageUrlsInput?: () => void;
  onUploadImages?: (files: File[]) => Promise<void>;
  imageUploadDisabled?: boolean;
  trackStockInputId: string;
  trackStock: boolean;
  onTrackStockChange: (value: boolean) => void;
  showTrackStock: boolean;
  onOpenPreviewImage: (url: string) => void;
  galleryRootClassName: string;
  galleryItemClassName: string;
  galleryZoomClassName: string;
};

/**
 * Textarea de URLs, preview opcional, checkbox de control de stock.
 * Va dentro del `form-grid` del padre (no envuelve en grid propio).
 */
export function CrudLinkedEntityEditBodyFields({
  strings,
  imageUrlsInputId,
  imageUrls,
  onImageUrlsChange,
  onImageUrlsInput,
  onUploadImages,
  imageUploadDisabled = false,
  trackStockInputId,
  trackStock,
  onTrackStockChange,
  showTrackStock,
  onOpenPreviewImage,
  galleryRootClassName,
  galleryItemClassName,
  galleryZoomClassName,
}: CrudLinkedEntityEditBodyFieldsProps) {
  const handleFileChange = async (files: FileList | null) => {
    if (!onUploadImages || !files) return;
    const selected = Array.from(files).filter((file) => file.size > 0);
    if (!selected.length) return;
    onImageUrlsInput?.();
    await onUploadImages(selected);
  };

  return (
    <>
      <div className="crud-linked-entity-form__field" style={{ gridColumn: '1 / -1' }}>
        <label htmlFor={imageUrlsInputId}>{strings.fieldImageUrlsLabel}</label>
        <div className="crud-linked-entity-form__upload-row">
          <label className="btn-sm btn-secondary crud-linked-entity-form__upload-button" htmlFor={imageUrlsInputId}>
            {imageUploadDisabled
              ? (strings.fieldImageUploadingLabel ?? 'Subiendo…')
              : (strings.fieldImageUploadActionLabel ?? 'Subir imágenes')}
          </label>
          <input
            id={imageUrlsInputId}
            className="crud-linked-entity-form__file-input"
            type="file"
            accept="image/*"
            multiple
            disabled={imageUploadDisabled || !onUploadImages}
            onChange={(e) => {
              void handleFileChange(e.currentTarget.files);
              e.currentTarget.value = '';
            }}
          />
          <p className="crud-linked-entity-form__hint">{strings.fieldImageUrlsHint}</p>
        </div>
      </div>
      {imageUrls.length > 0 ? (
        <div style={{ gridColumn: '1 / -1' }}>
          <div className="crud-linked-entity-form__image-list">
            {imageUrls.map((url) => (
              <div key={url} className="crud-linked-entity-form__image-item">
                <CrudLinkedEntityImageGalleryStrip
                  urls={[url]}
                  ariaLabel={strings.galleryAriaLabel}
                  openImageLabel={strings.openImageFullscreenLabel}
                  onOpenImage={onOpenPreviewImage}
                  rootClassName={galleryRootClassName}
                  itemClassName={galleryItemClassName}
                  zoomButtonClassName={galleryZoomClassName}
                />
                <button
                  type="button"
                  className="btn-sm btn-secondary crud-linked-entity-form__image-remove"
                  onClick={() => {
                    onImageUrlsInput?.();
                    onImageUrlsChange(imageUrls.filter((current) => current !== url));
                  }}
                >
                  {strings.fieldImageRemoveLabel ?? 'Quitar'}
                </button>
              </div>
            ))}
          </div>
        </div>
      ) : null}
      {showTrackStock ? (
        <div className="crud-linked-entity-form__checkbox-row">
          <input
            id={trackStockInputId}
            type="checkbox"
            checked={trackStock}
            onChange={(e) => onTrackStockChange(e.target.checked)}
          />
          <label htmlFor={trackStockInputId}>{strings.fieldTrackStockLabel}</label>
        </div>
      ) : null}
    </>
  );
}
