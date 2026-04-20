import { useEffect, useMemo, useState } from 'react';
import { type CrudColumn, type CrudFieldValue, type CrudFormField, type CrudFormValues, type CrudPageConfig } from '../../components/CrudPage';
import type { CrudToolbarAction } from '@devpablocristo/modules-crud-ui';
import { buildStandardCrudViewModes } from '../../modules/crud';
import {
  asBoolean,
  asNumber,
  asOptionalString,
  asString,
} from '../../crud/resourceConfigs.shared';
import { formatPartyTagList, parsePartyTagCsv } from '../parties';
import {
  asCrudString,
  parseMetadataStringMap,
  productCategoryOptions,
} from '../../lib/formPresets';

export type ProductRecord = {
  id: string;
  sku?: string;
  name: string;
  description?: string;
  unit?: string;
  price?: number;
  currency?: string;
  cost_price?: number;
  tax_rate?: number | null;
  image_url?: string;
  image_urls?: string[];
  track_stock: boolean;
  is_active: boolean;
  deleted_at?: string | null;
  tags?: string[];
  metadata?: Record<string, unknown>;
};

export type StockRecord = {
  id: string;
  product_id: string;
  product_name: string;
  sku: string;
  quantity: number;
  min_quantity: number;
  is_low_stock: boolean;
  updated_at: string;
};

function normalizeProductImageEntries(values: string[] | undefined, legacySingle?: string): string[] {
  const source = values?.length ? values : legacySingle?.trim() ? [legacySingle.trim()] : [];
  const out: string[] = [];
  const seen = new Set<string>();
  let lastDataPrefix = '';
  for (let index = 0; index < source.length; index += 1) {
    let current = String(source[index] ?? '').trim();
    if (!current) continue;
    if (current.startsWith('data:image/') && !current.includes(',')) {
      const next = String(source[index + 1] ?? '').trim();
      if (next) {
        current = `${current},${next}`;
        index += 1;
      }
    }
    if (current.startsWith('data:image/')) {
      const commaIndex = current.indexOf(',');
      if (commaIndex > 0) {
        lastDataPrefix = current.slice(0, commaIndex + 1);
      }
    } else if (looksLikeProductImageBase64(current)) {
      const prefix = lastDataPrefix || inferProductImageDataPrefix(current);
      if (prefix) current = `${prefix}${current}`;
    }
    if (seen.has(current)) continue;
    seen.add(current);
    out.push(current);
  }
  return out;
}

function inferProductImageDataPrefix(raw: string): string {
  const trimmed = String(raw ?? '').trim();
  if (trimmed.startsWith('/9j/')) return 'data:image/jpeg;base64,';
  if (trimmed.startsWith('iVBOR')) return 'data:image/png;base64,';
  if (trimmed.startsWith('R0lGOD')) return 'data:image/gif;base64,';
  if (trimmed.startsWith('UklGR')) return 'data:image/webp;base64,';
  return '';
}

function looksLikeProductImageBase64(raw: string): boolean {
  const trimmed = String(raw ?? '').trim();
  if (trimmed.length < 8 || inferProductImageDataPrefix(trimmed) === '') return false;
  return /^[A-Za-z0-9+/=]+$/.test(trimmed);
}

function formatProductImagesForEditor(values: string[] | undefined, legacySingle?: string): string {
  return normalizeProductImageEntries(values, legacySingle).join('\n');
}

function parseProductImagesFromEditor(value: CrudFieldValue | undefined): string[] {
  return asCrudString(value)
    .split('\n')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function ProductImagesField({
  value,
  setValue,
  readOnly = false,
}: {
  value: CrudFieldValue | undefined;
  setValue: (nextValue: string) => void;
  readOnly?: boolean;
}) {
  const images = useMemo(
    () =>
      asCrudString(value)
        .split('\n')
        .map((entry) => entry.trim())
        .filter(Boolean),
    [value],
  );
  const [selectedIndex, setSelectedIndex] = useState(0);

  useEffect(() => {
    if (!images.length) {
      setSelectedIndex(0);
      return;
    }
    if (selectedIndex > images.length - 1) {
      setSelectedIndex(images.length - 1);
    }
  }, [images, selectedIndex]);

  const selectedImage = images[selectedIndex] ?? '';
  const removeImage = (indexToRemove: number) => {
    const nextImages = images.filter((_, index) => index !== indexToRemove);
    setValue(nextImages.join('\n'));
    setSelectedIndex((current) => {
      if (!nextImages.length) return 0;
      if (current > indexToRemove) return current - 1;
      return Math.min(current, nextImages.length - 1);
    });
  };

  return (
    <div className="crud-inline-upload">
      <input
        id="product-images-upload-input"
        className="crud-inline-upload__input"
        type="file"
        accept="image/*"
        multiple
        disabled={readOnly}
        onChange={async (event) => {
          const files = Array.from(event.target.files ?? []);
          if (!files.length) return;
          try {
            const encoded = await Promise.all(
              files.map(
                (file) =>
                  new Promise<string>((resolve, reject) => {
                    const reader = new FileReader();
                    reader.onload = () => resolve(String(reader.result ?? ''));
                    reader.onerror = () => reject(reader.error ?? new Error('upload_failed'));
                    reader.readAsDataURL(file);
                  }),
              ),
            );
            const nextImages = [...images, ...encoded];
            setValue(nextImages.join('\n'));
            setSelectedIndex(images.length);
          } finally {
            event.currentTarget.value = '';
          }
        }}
      />
      {!readOnly ? (
        <label htmlFor="product-images-upload-input" className="crud-inline-upload__button">
          Subir imágenes desde la computadora
        </label>
      ) : null}
      {selectedImage ? (
        <div className="crud-inline-upload__preview">
          <div className="crud-inline-upload__hero">
            {images.length > 1 ? (
              <button
                type="button"
                className="crud-inline-upload__nav crud-inline-upload__nav--prev"
                onClick={() => setSelectedIndex((current) => (current - 1 + images.length) % images.length)}
                aria-label="Imagen anterior"
              >
                ←
              </button>
            ) : null}
            <img src={selectedImage} alt={`Imagen ${selectedIndex + 1}`} />
            {images.length > 1 ? (
              <button
                type="button"
                className="crud-inline-upload__nav crud-inline-upload__nav--next"
                onClick={() => setSelectedIndex((current) => (current + 1) % images.length)}
                aria-label="Imagen siguiente"
              >
                →
              </button>
            ) : null}
          </div>
          <div className="crud-inline-upload__thumbs">
            {images.map((image, index) => (
              <div
                key={`${image}-${index}`}
                className={`crud-inline-upload__thumb-wrap${index === selectedIndex ? ' crud-inline-upload__thumb-wrap--active' : ''}`}
              >
                <button
                  type="button"
                  className={`crud-inline-upload__thumb${index === selectedIndex ? ' crud-inline-upload__thumb--active' : ''}`}
                  onClick={() => setSelectedIndex(index)}
                >
                  <img src={image} alt={`Miniatura ${index + 1}`} />
                </button>
                {!readOnly ? (
                  <button
                    type="button"
                    className="crud-inline-upload__remove"
                    onClick={() => removeImage(index)}
                    aria-label={`Eliminar imagen ${index + 1}`}
                  >
                    ×
                  </button>
                ) : null}
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}

export function createProductColumns<T extends ProductRecord>(): CrudColumn<T>[] {
  return [
    { key: 'name', header: 'Producto', className: 'cell-name' },
    {
      key: 'sku',
      header: 'Código',
      render: (_v, row) =>
        row.sku || (typeof row.metadata?.barcode === 'string' ? String(row.metadata.barcode) : '') || '—',
    },
    { key: 'unit', header: 'Unidad', render: (_v, row) => row.unit || '—' },
    { key: 'price', header: 'Precio', render: (value, row) => `${row.currency ?? 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
    { key: 'cost_price', header: 'Costo', render: (value, row) => `${row.currency ?? 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
  ];
}

export function productFormFields(): CrudFormField[] {
  return [
    { key: 'image_urls', label: 'Imágenes', type: 'textarea', rows: 3, fullWidth: true },
    { key: 'name', label: 'Nombre', required: true, placeholder: 'Nombre del producto' },
    { key: 'sku', label: 'Código interno', placeholder: 'PROD-001' },
    {
      key: 'metadata_category',
      label: 'Categoría',
      type: 'select',
      options: productCategoryOptions,
    },
    { key: 'metadata_barcode', label: 'Código de barras', placeholder: '7791234567890' },
    { key: 'price', label: 'Precio', type: 'number', required: true, placeholder: '0.00' },
    {
      key: 'track_stock',
      label: 'Controla stock',
      type: 'select',
      options: [
        { label: 'Sí', value: 'true' },
        { label: 'No', value: 'false' },
      ],
    },
    {
      key: 'is_active',
      label: 'Estado comercial',
      type: 'select',
      options: [
        { label: 'Activo', value: 'true' },
        { label: 'Inactivo', value: 'false' },
      ],
    },
    { key: 'tags', label: 'Etiquetas internas', placeholder: 'nuevo, combo, premium' },
    { key: 'description', label: 'Descripcion', type: 'textarea', rows: 3, fullWidth: true },
  ];
}

export function buildProductSearchText(row: ProductRecord): string {
  return [
    row.name,
    row.sku,
    row.description,
    row.unit,
    row.currency,
    formatPartyTagList(row.tags),
    typeof row.metadata?.barcode === 'string' ? row.metadata.barcode : '',
    typeof row.metadata?.category === 'string' ? row.metadata.category : '',
  ]
    .filter(Boolean)
    .join(' ');
}

export function buildProductFormValues(row: ProductRecord) {
  return {
    name: row.name ?? '',
    sku: row.sku ?? '',
    unit: row.unit ?? '',
    price: row.price?.toString() ?? '0',
    currency: row.currency ?? 'ARS',
    cost_price: row.cost_price?.toString() ?? '',
    tax_rate: row.tax_rate?.toString() ?? '',
    track_stock: row.track_stock ?? true,
    is_active: row.is_active ? 'true' : 'false',
    tags: formatPartyTagList(row.tags),
    image_urls: formatProductImagesForEditor(row.image_urls, row.image_url),
    description: row.description ?? '',
    metadata_category: typeof row.metadata?.category === 'string' ? row.metadata.category : '',
    metadata_kind: typeof row.metadata?.kind === 'string' ? row.metadata.kind : 'simple',
    metadata_barcode: typeof row.metadata?.barcode === 'string' ? row.metadata.barcode : '',
    metadata_margin_percent:
      row.metadata?.margin_percent === undefined || row.metadata?.margin_percent === null ? '' : String(row.metadata.margin_percent),
  };
}

export function productFormToBody(values: CrudFormValues): Record<string, unknown> {
  const price = asNumber(values.price);
  return {
    name: asString(values.name),
    sku: asOptionalString(values.sku),
    unit: 'unit',
    price,
    currency: 'ARS',
    cost_price: 0,
    track_stock: asBoolean(values.track_stock),
    is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
    tags: parsePartyTagCsv(values.tags),
    image_urls: parseProductImagesFromEditor(values.image_urls),
    description: asOptionalString(values.description),
    metadata: parseMetadataStringMap(undefined, {
      category: asOptionalString(values.metadata_category),
      barcode: asOptionalString(values.metadata_barcode),
    }),
  };
}

export function isValidProductForm(values: CrudFormValues): boolean {
  return asString(values.name).trim().length >= 2 && Number(asString(values.price) || '0') >= 0;
}

export function createProductCrudConfig<T extends ProductRecord>(options: {
  renderGallery: () => JSX.Element;
  renderList: () => JSX.Element;
}): Pick<
  CrudPageConfig<T & { id: string }>,
  | 'supportsArchived'
  | 'viewModes'
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'allowEdit'
  | 'columns'
  | 'formFields'
  | 'searchText'
  | 'toFormValues'
  | 'toBody'
  | 'isValid'
  | 'editorModal'
> {
  return {
    supportsArchived: true,
    viewModes: buildStandardCrudViewModes(options.renderList, {
      defaultModeId: 'gallery',
      renderGallery: options.renderGallery,
      ariaLabel: 'Vista galería, lista o tablero',
    }),
    label: 'producto',
    labelPlural: 'productos',
    labelPluralCap: 'Productos',
    allowEdit: true,
    columns: createProductColumns<T & { id: string }>(),
    formFields: productFormFields(),
    searchText: buildProductSearchText as CrudPageConfig<T & { id: string }>['searchText'],
    toFormValues: buildProductFormValues as CrudPageConfig<T & { id: string }>['toFormValues'],
    toBody: productFormToBody,
    isValid: isValidProductForm,
    editorModal: {
      disableBuiltInMedia: true,
      fieldConfig: {
        image_urls: {
          readValue: ({ value }) => (
            <ProductImagesField
              value={normalizeProductImageEntries(
                asCrudString(value)
                  .split('\n')
                  .map((entry) => entry.trim())
                  .filter(Boolean),
              ).join('\n')}
              setValue={() => {}}
              readOnly
            />
          ),
          editControl: ({ value, setValue }) => (
            <ProductImagesField
              value={normalizeProductImageEntries(
                asCrudString(value)
                  .split('\n')
                  .map((entry) => entry.trim())
                  .filter(Boolean),
              ).join('\n')}
              setValue={(next) => setValue(next)}
            />
          ),
        },
      },
    },
  };
}

export function formatInventoryUpdatedAt(raw: string): JSX.Element | string {
  const t = String(raw ?? '').trim();
  if (!t) return '—';
  const d = new Date(t);
  if (Number.isNaN(d.getTime())) return t;
  return (
    <div className="stock-datetime-cell">
      <span className="stock-datetime-cell__date">
        {d.toLocaleDateString('es-AR', { weekday: 'short', day: '2-digit', month: 'short', year: 'numeric' })}
      </span>
      <span className="stock-datetime-cell__sep" aria-hidden>
        {' · '}
      </span>
      <span className="stock-datetime-cell__time">{d.toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' })}</span>
    </div>
  );
}

export function createStockColumns<T extends StockRecord>(): CrudColumn<T>[] {
  return [
    {
      key: 'product_name',
      header: 'Nombre',
      className: 'cell-name stock-col-product-name',
      render: (_value, row) => row.product_name,
    },
    {
      key: 'sku',
      header: 'Sku',
      className: 'stock-col-sku',
      render: (_value, row) => <span className="stock-sku-inline">{row.sku?.trim() || '—'}</span>,
    },
    { key: 'quantity', header: 'Actual', className: 'stock-col-num stock-col-qty' },
    { key: 'min_quantity', header: 'Mínimo', className: 'stock-col-num stock-col-min' },
    {
      key: 'is_low_stock',
      header: 'Estado',
      className: 'stock-col-estado',
      render: (value) => (
        <div className="stock-status-cell">
          <span className={value ? 'stock-status stock-status--warning' : 'stock-status'}>
            {value ? 'Bajo mínimo' : 'Normal'}
          </span>
        </div>
      ),
    },
    {
      key: 'updated_at',
      header: 'Actualizado',
      className: 'stock-col-date',
      render: (value) => formatInventoryUpdatedAt(String(value ?? '')),
    },
  ];
}

export function createStockArchivedColumns<T extends StockRecord>(): CrudColumn<T>[] {
  return [
    {
      key: 'product_name',
      header: 'Nombre',
      className: 'cell-name stock-col-product-name',
      render: (_value, row) => row.product_name,
    },
    {
      key: 'sku',
      header: 'Sku',
      className: 'stock-col-sku',
      render: (_value, row) => <span className="stock-sku-inline">{row.sku?.trim() || '—'}</span>,
    },
  ];
}

export function createStockNewProductAction(): CrudToolbarAction<StockRecord> {
  return {
    id: 'stock-new-product',
    label: '+ Nuevo producto',
    kind: 'primary',
    isVisible: ({ archived }) => !archived,
    onClick: async () => {
      window.location.assign('/modules/products/list');
    },
  };
}

export function createStockCrudConfig<T extends StockRecord>(options: {
  renderList: () => JSX.Element;
  renderGallery: () => JSX.Element;
  renderBoard: () => JSX.Element;
}): Pick<
  CrudPageConfig<T>,
  | 'label'
  | 'labelPlural'
  | 'labelPluralCap'
  | 'allowCreate'
  | 'allowEdit'
  | 'allowDelete'
  | 'supportsArchived'
  | 'archivedEmptyState'
  | 'searchPlaceholder'
  | 'emptyState'
  | 'viewModes'
  | 'rowActions'
  | 'toolbarActions'
  | 'columns'
  | 'archivedColumns'
> {
  return {
    label: 'producto en el inventario',
    labelPlural: 'productos en el inventario',
    labelPluralCap: 'Inventario',
    allowCreate: false,
    allowEdit: false,
    allowDelete: false,
    supportsArchived: true,
    archivedEmptyState: 'No hay productos archivados en inventario.',
    searchPlaceholder: 'Buscar...',
    emptyState: 'No hay productos en el inventario.',
    viewModes: [
      { id: 'list', label: 'Lista', path: 'list', ariaLabel: 'Vistas de inventario', isDefault: true, render: options.renderList },
      { id: 'gallery', label: 'Galería', path: 'gallery', ariaLabel: 'Vistas de inventario', render: options.renderGallery },
      { id: 'kanban', label: 'Tablero', path: 'board', ariaLabel: 'Vistas de inventario', render: options.renderBoard },
    ],
    rowActions: [],
    toolbarActions: [createStockNewProductAction() as CrudToolbarAction<T>],
    columns: createStockColumns<T>(),
    archivedColumns: createStockArchivedColumns<T>(),
  };
}
