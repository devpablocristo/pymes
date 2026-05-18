import type { CrudFieldValue } from '../../components/CrudPage';
import { asBoolean, asOptionalNumber, asOptionalString, asString, parseJSONArray } from '../../crud/resourceConfigs.shared';
import { parsePartyTagCsv } from '../parties/partiesHelpers';
import {
  invoiceInitials,
  nextInvoiceUid,
  readDemoInvoices,
  writeDemoInvoices,
  type InvoiceLineItem,
  type InvoiceRecord,
  type InvoiceStatus,
} from './invoicesDemo';

export type { InvoiceLineItem, InvoiceRecord, InvoiceStatus };

export type CommercialDocumentStatusOption<TStatus extends string> = {
  value: TStatus;
  label: string;
  badgeClass: string;
};

export function buildCommercialDocumentStatusOptions<TStatus extends string>(
  labels: Record<TStatus, string>,
  badgeClasses: Record<TStatus, string>,
): Array<CommercialDocumentStatusOption<TStatus>> {
  return (Object.keys(labels) as TStatus[]).map((value) => ({
    value,
    label: labels[value],
    badgeClass: badgeClasses[value],
  }));
}

export type CommercialPricedLineItem = {
  product_id?: string;
  service_id?: string;
  description: string;
  quantity: number;
  unit_price: number;
  tax_rate?: number;
  sort_order: number;
};

export type CommercialCostLineItem = {
  product_id?: string;
  service_id?: string;
  description: string;
  quantity: number;
  unit_cost: number;
  tax_rate?: number;
};

export type QuoteRecord = {
  id: string;
  branch_id?: string;
  number: string;
  customer_id?: string;
  customer_name: string;
  status: string;
  total: number;
  currency?: string;
  valid_until?: string;
  notes?: string;
  tags?: string[];
  metadata?: Record<string, unknown>;
  items?: CommercialPricedLineItem[];
};

export type SaleRecord = {
  id: string;
  branch_id?: string;
  number: string;
  customer_id?: string;
  customer_name: string;
  quote_id?: string;
  status: string;
  payment_method?: string;
  total: number;
  currency?: string;
  notes?: string;
  tags?: string[];
  metadata?: Record<string, unknown>;
  items?: CommercialPricedLineItem[];
};

export type CreditNoteRecord = {
  id: string;
  number: string;
  party_id: string;
  return_id: string;
  amount: number;
  used_amount: number;
  balance: number;
  status: string;
  created_at: string;
  expires_at?: string;
};

export type PurchaseRecord = {
  id: string;
  branch_id?: string;
  number: string;
  supplier_id?: string;
  supplier_name: string;
  status: string;
  payment_status: string;
  total: number;
  currency?: string;
  notes?: string;
  received_at?: string;
  tags?: string[];
  metadata?: Record<string, unknown>;
  items?: CommercialCostLineItem[];
};

export function parseInvoiceStatus(value: CrudFieldValue | undefined): InvoiceStatus {
  const raw = asOptionalString(value);
  if (raw === 'paid' || raw === 'pending' || raw === 'overdue') return raw;
  return 'pending';
}

export function createInvoiceCrudLineItems(value: CrudFieldValue | undefined): InvoiceLineItem[] {
  return parseJSONArray<{ id?: string; description?: string; qty?: number; unit?: string; unitPrice?: number }>(
    value,
    'Los items deben ser un arreglo JSON',
  ).map((item, index) => ({
    id: String(item.id ?? index + 1),
    description: String(item.description ?? ''),
    qty: Number(item.qty ?? 1),
    unit: String(item.unit ?? 'unidad'),
    unitPrice: Number(item.unitPrice ?? 0),
  }));
}

export async function createDemoInvoiceFromCrudValues(values: Record<string, CrudFieldValue | undefined>): Promise<void> {
  const customer = asString(values.customer);
  const invoices = readDemoInvoices();
  const meta: Record<string, unknown> = {};
  if (asBoolean(values.metadata_favorite)) {
    meta.favorite = true;
  }
  writeDemoInvoices([
    {
      id: nextInvoiceUid(),
      number: asOptionalString(values.number) ?? `INV-${3500 + Math.floor(Math.random() * 100)}`,
      customer,
      initials: invoiceInitials(customer),
      issuedDate: asOptionalString(values.issuedDate) ?? new Date().toISOString().slice(0, 10),
      dueDate: asOptionalString(values.dueDate) ?? asOptionalString(values.issuedDate) ?? new Date().toISOString().slice(0, 10),
      status: parseInvoiceStatus(values.status),
      discount: asOptionalNumber(values.discount) ?? 0,
      tax: asOptionalNumber(values.tax) ?? 21,
      items: createInvoiceCrudLineItems(values.items),
      archived_at: null,
      tags: parsePartyTagCsv(values.tags),
      metadata: Object.keys(meta).length > 0 ? meta : undefined,
    },
    ...invoices,
  ]);
}

export function parseCommercialPricedLineItems(value: CrudFieldValue | undefined): CommercialPricedLineItem[] {
  return parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON')
    .map((item, index) => ({
      product_id: asOptionalString(item.product_id as CrudFieldValue),
      service_id: asOptionalString(item.service_id as CrudFieldValue),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_price: Number(item.unit_price ?? 0),
      tax_rate: item.tax_rate === undefined || item.tax_rate === null ? undefined : Number(item.tax_rate),
      sort_order: Number(item.sort_order ?? index),
    }))
    .filter((item) => item.description && item.quantity > 0);
}

export function parseCommercialCostLineItems(value: CrudFieldValue | undefined): CommercialCostLineItem[] {
  return parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON')
    .map((item) => ({
      product_id: asOptionalString(item.product_id as CrudFieldValue),
      service_id: asOptionalString(item.service_id as CrudFieldValue),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_cost: Number(item.unit_cost ?? 0),
      tax_rate: item.tax_rate === undefined || item.tax_rate === null ? undefined : Number(item.tax_rate),
    }))
    .filter((item) => item.description && item.quantity > 0);
}
