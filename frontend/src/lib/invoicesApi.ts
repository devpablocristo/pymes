import { apiRequest } from './api';
import type { CrudFieldValue } from '../components/CrudPage';
import { asOptionalNumber, asOptionalString, asString, parseJSONArray } from '../crud/resourceConfigs.shared';
import { parseTagCsv } from '../modules/crud';

export type InvoiceStatus = 'paid' | 'pending' | 'overdue';

export type InvoiceLineItem = {
  id: string;
  description: string;
  qty: number;
  unit: string;
  unitPrice: number;
};

// Forma frontend (preservada por compatibilidad con el demo).
export type InvoiceRecord = {
  id: string;
  number: string;
  customer: string;
  initials: string;
  issuedDate: string;
  dueDate: string;
  status: InvoiceStatus;
  items: InvoiceLineItem[];
  discount: number;
  tax: number;
  is_favorite?: boolean;
  tags?: string[];
  archived_at?: string | null;
};

type BackendLineItem = {
  id: string;
  invoice_id: string;
  description: string;
  qty: number;
  unit: string;
  unit_price: number;
  line_total: number;
  sort_order: number;
};

type BackendInvoice = {
  id: string;
  org_id: string;
  number: string;
  party_id?: string;
  customer_name: string;
  issued_date: string;
  due_date: string;
  status: InvoiceStatus;
  subtotal: number;
  discount_percent: number;
  tax_percent: number;
  total: number;
  notes: string;
  is_favorite: boolean;
  tags: string[];
  created_by: string;
  created_at: string;
  updated_at: string;
  archived_at?: string;
  items: BackendLineItem[];
};

type ListResponse = {
  items: BackendInvoice[];
  total: number;
  has_more: boolean;
  next_cursor?: string;
};

function initialsFor(name: string): string {
  return name
    .split(' ')
    .map((word) => word[0])
    .filter(Boolean)
    .join('')
    .toUpperCase()
    .slice(0, 2);
}

function fromBackend(row: BackendInvoice): InvoiceRecord {
  return {
    id: row.id,
    number: row.number,
    customer: row.customer_name,
    initials: initialsFor(row.customer_name),
    issuedDate: row.issued_date,
    dueDate: row.due_date,
    status: row.status,
    items: (row.items ?? []).map((it) => ({
      id: it.id,
      description: it.description,
      qty: it.qty,
      unit: it.unit,
      unitPrice: it.unit_price,
    })),
    discount: row.discount_percent,
    tax: row.tax_percent,
    is_favorite: row.is_favorite,
    tags: row.tags ?? [],
    archived_at: row.archived_at ?? null,
  };
}

export async function fetchInvoices(opts?: { archived?: boolean }): Promise<InvoiceRecord[]> {
  const path = opts?.archived ? '/v1/invoices/archived' : '/v1/invoices';
  const data = await apiRequest<ListResponse>(path);
  return (data.items ?? []).map(fromBackend);
}

export function parseInvoiceItemsFromCrud(value: CrudFieldValue | undefined): Array<{
  description: string;
  qty: number;
  unit: string;
  unit_price: number;
  sort_order: number;
}> {
  return parseJSONArray<{
    id?: string;
    description?: string;
    qty?: number;
    quantity?: number;
    unit?: string;
    unitPrice?: number;
    unit_price?: number;
    unit_cost?: number;
    sort_order?: number;
  }>(value, 'Los items deben ser un arreglo JSON')
    .map((item, index) => ({
      description: String(item.description ?? '').trim(),
      qty: Number(item.qty ?? item.quantity ?? 1),
      unit: String(item.unit ?? 'unidad'),
      unit_price: Number(item.unitPrice ?? item.unit_price ?? item.unit_cost ?? 0),
      sort_order: Number(item.sort_order ?? index + 1),
    }))
    .filter((item) => item.description.length > 0);
}

function parseInvoiceStatus(value: CrudFieldValue | undefined): InvoiceStatus {
  const raw = asOptionalString(value);
  if (raw === 'paid' || raw === 'pending' || raw === 'overdue') return raw;
  return 'pending';
}

export function invoiceCreateBodyFromCrud(values: Record<string, CrudFieldValue | undefined>): Record<string, unknown> {
  return {
    number: asOptionalString(values.number) ?? '',
    customer_name: asString(values.customer),
    issued_date: asOptionalString(values.issuedDate) ?? new Date().toISOString().slice(0, 10),
    due_date: asOptionalString(values.dueDate) ?? asOptionalString(values.issuedDate) ?? new Date().toISOString().slice(0, 10),
    status: parseInvoiceStatus(values.status),
    discount_percent: asOptionalNumber(values.discount) ?? 0,
    tax_percent: asOptionalNumber(values.tax) ?? 21,
    is_favorite: Boolean(values.is_favorite),
    tags: parseTagCsv(values.tags),
    items: parseInvoiceItemsFromCrud(values.items),
  };
}

export function invoiceUpdateBodyFromCrud(values: Record<string, CrudFieldValue | undefined>): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  if (values.status !== undefined) body.status = parseInvoiceStatus(values.status);
  const discount = asOptionalNumber(values.discount);
  if (discount !== undefined) body.discount_percent = discount;
  const tax = asOptionalNumber(values.tax);
  if (tax !== undefined) body.tax_percent = tax;
  const issuedDate = asOptionalString(values.issuedDate);
  if (issuedDate) body.issued_date = issuedDate;
  const dueDate = asOptionalString(values.dueDate);
  if (dueDate) body.due_date = dueDate;
  if (values.is_favorite !== undefined) body.is_favorite = Boolean(values.is_favorite);
  if (values.tags !== undefined) body.tags = parseTagCsv(values.tags);
  return body;
}

export async function createInvoiceFromCrudValues(values: Record<string, CrudFieldValue | undefined>): Promise<void> {
  await apiRequest('/v1/invoices', { method: 'POST', body: invoiceCreateBodyFromCrud(values) });
}

export async function updateInvoiceStatus(id: string, status: InvoiceStatus): Promise<void> {
  await apiRequest(`/v1/invoices/${id}`, { method: 'PATCH', body: { status } });
}
