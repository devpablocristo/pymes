import { type CrudColumn, type CrudFormField, type CrudFormValues, type CrudPageConfig } from '../../components/CrudPage';
import type { CrudToolbarAction } from '@devpablocristo/modules-crud-ui';
import { renderTagBadges } from '../../crud/crudTagBadges';
import { buildStandardCrudViewModes } from '../../modules/crud';
import {
  asBoolean,
  asNumber,
  asOptionalNumber,
  asOptionalString,
  asString,
  formatProductImageURLsToForm,
  parseImageURLList,
} from '../../crud/resourceConfigs.shared';
import { renderCrudActiveBadge } from '../../modules/crud';
import { formatPartyTagList, parsePartyTagCsv } from '../parties';

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
    {
      key: 'name',
      header: 'Producto',
      className: 'cell-name',
      render: (_value, row) => (
        <>
          <strong>{row.name}</strong>
          <div className="text-secondary">{row.sku || 'Sin SKU'} · {row.unit || 'unidad'}</div>
        </>
      ),
    },
    { key: 'price', header: 'Precio', render: (value, row) => `${row.currency ?? 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
    { key: 'cost_price', header: 'Costo', render: (value, row) => `${row.currency ?? 'ARS'} ${Number(value ?? 0).toFixed(2)}` },
    {
      key: 'tags',
      header: 'Tags',
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
    { key: 'name', label: 'Nombre', required: true, placeholder: 'Nombre del producto' },
    { key: 'sku', label: 'SKU', placeholder: 'SKU-001' },
    { key: 'unit', label: 'Unidad', placeholder: 'unidad, kg, hora' },
    { key: 'price', label: 'Precio', type: 'number', required: true, placeholder: '0.00' },
    { key: 'currency', label: 'Moneda', placeholder: 'ARS' },
    { key: 'cost_price', label: 'Costo', type: 'number', placeholder: '0.00' },
    { key: 'tax_rate', label: 'IVA %', type: 'number', placeholder: '21' },
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
    { key: 'tags', label: 'Tags', placeholder: 'nuevo, combo, premium' },
    {
      key: 'image_urls',
      label: 'Imágenes (URLs)',
      type: 'textarea',
      fullWidth: true,
      placeholder: 'Una URL por línea (hasta 20). La primera es la principal.',
    },
    { key: 'description', label: 'Descripcion', type: 'textarea', fullWidth: true },
  ];
}

export function buildProductSearchText(row: ProductRecord): string {
  return [row.name, row.sku, row.description, row.unit, row.currency, formatPartyTagList(row.tags)].filter(Boolean).join(' ');
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
    image_urls: formatProductImageURLsToForm(row.image_urls, row.image_url),
    description: row.description ?? '',
  };
}

export function productFormToBody(values: CrudFormValues): Record<string, unknown> {
  return {
    name: asString(values.name),
    sku: asOptionalString(values.sku),
    unit: asOptionalString(values.unit),
    price: asNumber(values.price),
    currency: asOptionalString(values.currency) ?? 'ARS',
    cost_price: asNumber(values.cost_price),
    tax_rate: asOptionalNumber(values.tax_rate),
    track_stock: asBoolean(values.track_stock),
    is_active: asOptionalString(values.is_active) === undefined ? true : asBoolean(values.is_active),
    tags: parsePartyTagCsv(values.tags),
    description: asOptionalString(values.description),
    image_urls: parseImageURLList(values.image_urls),
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
      render: (_value, row) => <strong>{row.product_name}</strong>,
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
      render: (_value, row) => <strong>{row.product_name}</strong>,
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
