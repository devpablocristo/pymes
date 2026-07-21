import type { CrudFieldValue, CrudPageConfig } from '../../components/CrudPage';
import { asBoolean } from '../../crud/resourceConfigs.shared';
import { mergeCsvOptionsForResource } from '../../crud/csvEntityPolicy';
import { withCSVToolbar } from '../../crud/csvToolbar';
import { paymentMethodOptions } from '../../lib/formPresets';
import { buildStandardCrudViewModes, hasReadableCrudValue } from '../crud';
import { formatPartyTagList, parsePartyTagCsv } from '../parties/partiesHelpers';

export function buildCrudNotesField() {
  return { key: 'notes', label: 'Notas', type: 'textarea' as const, fullWidth: true };
}

export function buildCrudNameField(
  key: 'customer_name' | 'supplier_name',
  label: 'Cliente' | 'Proveedor',
  placeholder: string,
) {
  return { key, label, required: true, placeholder };
}

export function buildPaymentMethodField() {
  return {
    key: 'payment_method',
    label: 'Método de cobro',
    type: 'select' as const,
    required: true,
    options: paymentMethodOptions,
  };
}

export function formatPaymentMethodLabel(value: unknown): string {
  const raw = String(value ?? '').trim();
  return paymentMethodOptions.find((option) => option.value === raw)?.label ?? raw ?? '—';
}

export function buildCommercialLineItemsBlock(sectionId = 'items', label = 'Renglones') {
  return {
    id: 'items',
    kind: 'lineItems' as const,
    field: 'items',
    sectionId,
    label,
    visible: ({ editing }: { editing: boolean }) => editing,
  };
}

export function buildInvoiceLineItemsBlock(sectionId = 'items') {
  return {
    id: 'items',
    kind: 'lineItems' as const,
    field: 'items',
    sectionId,
    visible: ({ editing }: { editing: boolean }) => editing,
  };
}

export function buildInternalFavoriteField() {
  return { key: 'is_favorite', label: 'Agregar a favoritos', type: 'checkbox' as const };
}

export function buildInternalTagsField(placeholder: string) {
  return { key: 'tags', label: 'Etiquetas internas', placeholder };
}

export function readCommercialFavorite(row: unknown): boolean {
  const record = row as { is_favorite?: unknown; metadata?: { favorite?: unknown } };
  return Boolean(record.is_favorite ?? record.metadata?.favorite);
}

export function readCommercialTags(row: unknown): string {
  const tags = (row as { tags?: unknown }).tags;
  return Array.isArray(tags) ? formatPartyTagList(tags.map((tag) => String(tag))) : '';
}

export function commercialAnnotationsToBody(values: Record<string, CrudFieldValue | undefined>) {
  return {
    is_favorite: asBoolean(values.is_favorite),
    tags: parsePartyTagCsv(values.tags),
  };
}

export const purchasePaymentStatusOptions = [
  { value: 'pending', label: 'Pendiente' },
  { value: 'partial', label: 'Parcial' },
  { value: 'paid', label: 'Pagado' },
];

export function buildCrudSummaryReadOnlyField(sectionId = 'summary') {
  return {
    sectionId,
    readOnly: true,
    visible: ({ value }: { value: unknown }) => hasReadableCrudValue(value),
  };
}

export function buildCrudSectionField(sectionId: string, extra?: Record<string, unknown>) {
  return {
    sectionId,
    ...(extra ?? {}),
  };
}

export function createCommercialDocumentCrudConfig<
  TRecord extends { id: string },
  TSearchableKey extends keyof TRecord & string,
>(opts: {
  resourceId: string;
  renderList: NonNullable<CrudPageConfig<TRecord>['viewModes']>[number]['render'];
  label: string;
  labelPlural: string;
  labelPluralCap: string;
  createLabel: string;
  searchPlaceholder?: string;
  createFromValues?: (values: Record<string, CrudFieldValue | undefined>) => Promise<void>;
  searchKeys: TSearchableKey[];
  columns: NonNullable<CrudPageConfig<TRecord>['columns']>;
}) {
  const config: Pick<
    CrudPageConfig<TRecord>,
    | 'viewModes'
    | 'label'
    | 'labelPlural'
    | 'labelPluralCap'
    | 'createLabel'
    | 'searchPlaceholder'
    | 'featureFlags'
    | 'supportsArchived'
    | 'dataSource'
    | 'columns'
    | 'formFields'
    | 'searchText'
    | 'toFormValues'
    | 'isValid'
  > = {
    viewModes: buildStandardCrudViewModes(opts.renderList),
    label: opts.label,
    labelPlural: opts.labelPlural,
    labelPluralCap: opts.labelPluralCap,
    createLabel: opts.createLabel,
    searchPlaceholder: opts.searchPlaceholder ?? 'Buscar...',
    supportsArchived: true,
    columns: opts.columns,
    formFields: [],
    searchText: (row: TRecord) =>
      opts.searchKeys
        .map((key) => row[key])
        .filter(Boolean)
        .join(' '),
    toFormValues: () => ({}),
    isValid: () => true,
  };
  if (opts.createFromValues) {
    config.dataSource = {
      create: async (values) => {
        await opts.createFromValues?.(values);
      },
    };
  }
  const shellConfig = withCSVToolbar(
    opts.resourceId,
    config,
    mergeCsvOptionsForResource(opts.resourceId, config),
  );
  return { config, shellConfig };
}
