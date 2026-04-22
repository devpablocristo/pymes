import { commercialDocumentInitials } from './commercialDocumentMath';
import {
  archiveCommercialDocument,
  readCommercialDocumentDemoRecords,
  restoreCommercialDocument,
  writeCommercialDocumentDemoRecords,
} from './commercialDocumentDemoStore';

export type InvoiceStatus = 'paid' | 'pending' | 'overdue';

export type InvoiceLineItem = {
  id: string;
  description: string;
  qty: number;
  unit: string;
  unitPrice: number;
};

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

const STORAGE_KEY = 'pymes.billing.demo.invoices.v2';

let nextLineId = 200;
let nextInvoiceId = 20;

function lineUid() {
  nextLineId += 1;
  return String(nextLineId);
}

export function nextInvoiceUid() {
  nextInvoiceId += 1;
  return String(nextInvoiceId);
}

export function invoiceInitials(name: string): string {
  return commercialDocumentInitials(name);
}

export function createEmptyInvoiceLine(): InvoiceLineItem {
  return { id: lineUid(), description: '', qty: 1, unit: 'unidad', unitPrice: 0 };
}

export function calcInvoiceSubtotal(items: InvoiceLineItem[]): number {
  return items.reduce((sum, item) => sum + item.qty * item.unitPrice, 0);
}

export function calcInvoiceTotal(invoice: Pick<InvoiceRecord, 'items' | 'discount' | 'tax'>): number {
  const subtotal = calcInvoiceSubtotal(invoice.items);
  const afterDiscount = subtotal * (1 - invoice.discount / 100);
  return afterDiscount * (1 + invoice.tax / 100);
}

export function formatInvoiceMoney(value: number): string {
  return value.toLocaleString('es-AR', {
    style: 'currency',
    currency: 'ARS',
    minimumFractionDigits: 0,
  });
}

export function archiveInvoice(invoice: InvoiceRecord): InvoiceRecord {
  return archiveCommercialDocument(invoice);
}

export function restoreInvoice(invoice: InvoiceRecord): InvoiceRecord {
  return restoreCommercialDocument(invoice);
}

export const INVOICE_STATUS_LABELS: Record<InvoiceStatus, string> = {
  paid: 'Pagada',
  pending: 'Pendiente',
  overdue: 'Vencida',
};

export const INVOICE_STATUS_BADGE_CLASS: Record<InvoiceStatus, string> = {
  paid: 'badge-success',
  pending: 'badge-warning',
  overdue: 'badge-danger',
};

export const INITIAL_INVOICES: InvoiceRecord[] = [];

function isInvoiceStatus(value: unknown): value is InvoiceStatus {
  return value === 'paid' || value === 'pending' || value === 'overdue';
}

function sanitizeInvoiceLine(raw: unknown): InvoiceLineItem | null {
  if (!raw || typeof raw !== 'object') return null;
  const source = raw as Record<string, unknown>;
  return {
    id: String(source.id ?? lineUid()),
    description: String(source.description ?? ''),
    qty: Number(source.qty ?? 1),
    unit: String(source.unit ?? 'unidad'),
    unitPrice: Number(source.unitPrice ?? 0),
  };
}

function sanitizeInvoice(raw: unknown): InvoiceRecord | null {
  if (!raw || typeof raw !== 'object') return null;
  const source = raw as Record<string, unknown>;
  const status = isInvoiceStatus(source.status) ? source.status : 'pending';
  const items = Array.isArray(source.items) ? source.items.map(sanitizeInvoiceLine).filter(Boolean) as InvoiceLineItem[] : [];
  const tagsSource = Array.isArray(source.tags) ? source.tags : [];
  return {
    id: String(source.id ?? nextInvoiceUid()),
    number: String(source.number ?? `INV-${3500 + Math.floor(Math.random() * 100)}`),
    customer: String(source.customer ?? ''),
    initials: String(source.initials ?? invoiceInitials(String(source.customer ?? ''))),
    issuedDate: String(source.issuedDate ?? new Date().toISOString().slice(0, 10)),
    dueDate: String(source.dueDate ?? source.issuedDate ?? new Date().toISOString().slice(0, 10)),
    status,
    items,
    discount: Number(source.discount ?? 0),
    tax: Number(source.tax ?? 21),
    is_favorite: Boolean(source.is_favorite),
    tags: tagsSource.map((tag) => String(tag)).filter((tag) => tag.length > 0),
    archived_at: source.archived_at == null ? null : String(source.archived_at),
  };
}

export function readDemoInvoices(): InvoiceRecord[] {
  return readCommercialDocumentDemoRecords(STORAGE_KEY, INITIAL_INVOICES, sanitizeInvoice);
}

export function writeDemoInvoices(invoices: InvoiceRecord[]): void {
  writeCommercialDocumentDemoRecords(STORAGE_KEY, invoices);
}

export function updateDemoInvoice(id: string, mutator: (invoice: InvoiceRecord) => InvoiceRecord): void {
  writeDemoInvoices(readDemoInvoices().map((r) => (r.id === id ? mutator(r) : r)));
}

export function removeDemoInvoice(id: string): void {
  writeDemoInvoices(readDemoInvoices().filter((r) => r.id !== id));
}
