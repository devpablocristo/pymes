import { type CrudColumn, type CrudFieldValue, type CrudFormField, type CrudFormValues, type CrudPageConfig } from '../../components/CrudPage';
import type { CrudToolbarAction } from '@devpablocristo/modules-crud-ui';
import { CrudEntityImageField, buildStandardCrudViewModes, formatCrudLinkedEntityImageUrlsToForm, parseCrudLinkedEntityImageUrlList } from '../../modules/crud';
import {
  asBoolean,
  asNumber,
  asOptionalString,
  asString,
} from '../../crud/resourceConfigs.shared';
import { buildStandardInternalFields, formatTagCsv, parseTagCsv } from '../crud';
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
  is_favorite?: boolean;
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

function formatProductImagesForEditor(values: string[] | undefined, legacySingle?: string): string {
  return formatCrudLinkedEntityImageUrlsToForm(values, legacySingle);
}

function parseProductImagesFromEditor(value: CrudFieldValue | undefined): string[] {
  return parseCrudLinkedEntityImageUrlList(asCrudString(value));
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
    ...buildStandardInternalFields({ tagsPlaceholder: 'nuevo, combo, premium' }),
  ];
}

export function buildProductSearchText(row: ProductRecord): string {
  return [
    row.name,
    row.sku,
    row.description,
    row.unit,
    row.currency,
    formatTagCsv(row.tags),
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
    is_favorite: row.is_favorite ?? false,
    tags: formatTagCsv(row.tags),
    image_urls: formatProductImagesForEditor(row.image_urls, row.image_url),
    notes: row.description ?? '',
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
    is_favorite: asBoolean(values.is_favorite),
    tags: parseTagCsv(values.tags),
    image_urls: parseProductImagesFromEditor(values.image_urls),
    description: asOptionalString(values.notes),
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
            <CrudEntityImageField value={value} setValue={() => {}} readOnly />
          ),
          editControl: ({ value, setValue }) => (
            <CrudEntityImageField value={value} setValue={(next) => setValue(next)} />
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
