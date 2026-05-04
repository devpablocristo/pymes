import { type CrudColumn, type CrudFormField, type CrudFormValues, type CrudPageConfig } from '../../components/CrudPage';
import type { CrudToolbarAction } from '@devpablocristo/modules-crud-ui';
import { renderTagBadges } from '../../crud/crudTagBadges';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
  parseImageURLList,
} from '../../crud/resourceConfigs.shared';
import { buildStandardCrudImageUrlsModalFieldConfig } from '../../crud/standardCrudMedia';
import { formatCrudRecordImageUrlsToForm } from '../../modules/crud/crudLinkedEntityImageUrls';
import { buildStandardCrudViewModes, renderCrudActiveBadge } from '../../modules/crud';
import { formatPartyTagList, parsePartyTagCsv } from '../parties';
import {
  currencyOptions,
  parseMetadataStringMap,
  productCategoryOptions,
  productKindOptions,
  productUnitOptions,
  taxRateOptions,
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

export function createProductColumns<T extends ProductRecord>(): CrudColumn<T>[] {
  return [
    { key: 'name', header: 'Producto', className: 'cell-name' },
    {
      key: 'sku',
      header: 'Código',
      render: (_v, row) =>
        row.sku || (typeof row.metadata?.barcode === 'string' ? String(row.metadata.barcode) : '') || '—',
    },
    {
      key: 'metadata',
      header: '★',
      className: 'cell-narrow',
      render: (_v, row) =>
        row.metadata?.favorite === true || String(row.metadata?.favorite).toLowerCase() === 'true' ? (
          <span title="Favorito">★</span>
        ) : (
          '—'
        ),
    },
    { key: 'unit', header: 'Unidad', render: (_v, row) => row.unit || '—' },
    { key: 'price', header: 'Precio', render: (value, row) => `${row.currency ?? 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
    { key: 'cost_price', header: 'Costo', render: (value, row) => `${row.currency ?? 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
    {
      key: 'tags',
      header: 'Etiquetas Internas',
      className: 'cell-tags',
      render: (_value, row) => renderTagBadges(row.tags),
    },
    {
      key: 'track_stock',
      header: 'Stock',
      render: (value) => renderCrudActiveBadge(Boolean(value), 'Controlado', 'Sin control'),
    },
    {
      key: 'is_active',
      header: 'Estado',
      render: (value) => renderCrudActiveBadge(Boolean(value)),
    },
  ];
}

export function productFormFields(): CrudFormField[] {
  return [
    { key: 'metadata_favorite', label: 'Favorito', type: 'checkbox' },
    { key: 'tags', label: 'Etiquetas Internas', placeholder: 'nuevo, combo, premium' },
    {
      key: 'image_urls',
      label: 'Imágenes',
      type: 'textarea',
      fullWidth: true,
    },
    { key: 'name', label: 'Nombre', required: true, placeholder: 'Nombre del producto' },
    { key: 'sku', label: 'Código interno', placeholder: 'PROD-001' },
    {
      key: 'metadata_category',
      label: 'Categoría',
      type: 'select',
      options: productCategoryOptions,
    },
    {
      key: 'metadata_kind',
      label: 'Tipo de producto',
      type: 'select',
      options: productKindOptions,
    },
    { key: 'metadata_barcode', label: 'Código de barras', placeholder: '7791234567890' },
    { key: 'unit', label: 'Unidad', type: 'select', options: productUnitOptions },
    { key: 'price', label: 'Precio', type: 'number', required: true, placeholder: '0.00' },
    { key: 'currency', label: 'Moneda', type: 'select', options: currencyOptions },
    { key: 'cost_price', label: 'Costo', type: 'number', placeholder: '0.00' },
    { key: 'metadata_margin_percent', label: 'Margen (%)', type: 'number', placeholder: '35' },
    { key: 'tax_rate', label: 'IVA', type: 'select', options: taxRateOptions },
    { key: 'track_stock', label: 'Controla stock', type: 'checkbox' },
    {
      key: 'is_active',
      label: 'Estado comercial',
      type: 'select',
      options: [
        { label: 'Activo', value: 'true' },
        { label: 'Inactivo', value: 'false' },
      ],
    },
    { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
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
    image_urls: formatCrudRecordImageUrlsToForm(row as unknown as Record<string, unknown>),
    description: row.description ?? '',
    metadata_category: typeof row.metadata?.category === 'string' ? row.metadata.category : '',
    metadata_kind: typeof row.metadata?.kind === 'string' ? row.metadata.kind : 'simple',
    metadata_barcode: typeof row.metadata?.barcode === 'string' ? row.metadata.barcode : '',
    metadata_margin_percent:
      row.metadata?.margin_percent === undefined || row.metadata?.margin_percent === null ? '' : String(row.metadata.margin_percent),
    metadata_favorite:
      row.metadata?.favorite === true ||
      String(row.metadata?.favorite ?? '').toLowerCase() === 'true' ||
      row.metadata?.favorite === 1,
  };
}

export function productFormToBody(values: CrudFormValues): Record<string, unknown> {
  const price = asNumber(values.price);
  const directCost = asOptionalNumber(values.cost_price);
  const marginPercent = asOptionalNumber(values.metadata_margin_percent);
  const derivedCost =
    directCost !== undefined
      ? directCost
      : marginPercent !== undefined && Number.isFinite(marginPercent)
        ? Math.max(0, price - price * (marginPercent / 100))
        : 0;
  const metadata: Record<string, unknown> = parseMetadataStringMap(undefined, {
    category: asOptionalString(values.metadata_category),
    kind: asOptionalString(values.metadata_kind),
    barcode: asOptionalString(values.metadata_barcode),
    margin_percent: asOptionalString(values.metadata_margin_percent),
  });
  if (asBoolean(values.metadata_favorite)) {
    metadata.favorite = true;
  } else {
    delete metadata.favorite;
  }
  const imgUrls = parseImageURLList(values.image_urls);
  if (imgUrls.length > 0) {
    metadata.image_urls = imgUrls;
  } else {
    delete metadata.image_urls;
  }
  return {
    name: asString(values.name),
    sku: asOptionalString(values.sku),
    unit: asOptionalString(values.unit),
    price,
    currency: asOptionalString(values.currency) ?? 'ARS',
    cost_price: derivedCost,
    tax_rate: asOptionalNumber(values.tax_rate),
    track_stock: asBoolean(values.track_stock),
    is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
    tags: parsePartyTagCsv(values.tags),
    description: asOptionalString(values.description),
    image_urls: imgUrls,
    metadata,
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
    columns: createProductColumns<T & { id: string }>(),
    formFields: productFormFields(),
    searchText: buildProductSearchText as CrudPageConfig<T & { id: string }>['searchText'],
    toFormValues: buildProductFormValues as CrudPageConfig<T & { id: string }>['toFormValues'],
    toBody: productFormToBody,
    isValid: isValidProductForm,
    editorModal: {
      /** Carrusel superior sincronizado con el mismo campo que guardamos (`values.image_urls`). */
      mediaFieldKey: 'image_urls',
      fieldConfig: {
        image_urls: buildStandardCrudImageUrlsModalFieldConfig(),
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
    allowEdit: true,
    allowDelete: false,
    supportsArchived: true,
    archivedEmptyState: 'No hay productos archivados en inventario.',
    searchPlaceholder: 'Buscar...',
    emptyState: 'No hay productos en el inventario.',
    viewModes: buildStandardCrudViewModes(options.renderList, {
      renderGallery: options.renderGallery,
      renderKanban: options.renderBoard,
      ariaLabel: 'Vistas de inventario',
    }),
    rowActions: [],
    toolbarActions: [createStockNewProductAction() as CrudToolbarAction<T>],
    columns: createStockColumns<T>(),
    archivedColumns: createStockArchivedColumns<T>(),
  };
}
