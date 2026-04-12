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
  archived_at?: string | null;
};

const STORAGE_KEY = 'pymes.billing.demo.invoices.v1';

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

export const INITIAL_INVOICES: InvoiceRecord[] = [
  {
    id: '1',
    number: 'INV-3492',
    customer: 'María García',
    initials: 'MG',
    issuedDate: '2026-03-10',
    dueDate: '2026-04-10',
    status: 'paid',
    items: [
      { id: '1', description: 'Diseño de logo', qty: 1, unit: 'unidad', unitPrice: 15000 },
      { id: '2', description: 'Tarjetas de presentación', qty: 500, unit: 'unidades', unitPrice: 12 },
    ],
    discount: 0,
    tax: 21,
  },
  {
    id: '2',
    number: 'INV-3493',
    customer: 'Juan Pérez',
    initials: 'JP',
    issuedDate: '2026-03-12',
    dueDate: '2026-04-12',
    status: 'pending',
    items: [{ id: '3', description: 'Desarrollo web', qty: 40, unit: 'horas', unitPrice: 5000 }],
    discount: 5,
    tax: 21,
  },
  {
    id: '3',
    number: 'INV-3494',
    customer: 'Ana López',
    initials: 'AL',
    issuedDate: '2026-03-05',
    dueDate: '2026-03-20',
    status: 'overdue',
    items: [
      { id: '4', description: 'Consultoría SEO', qty: 10, unit: 'horas', unitPrice: 3500 },
      { id: '5', description: 'Auditoría técnica', qty: 1, unit: 'unidad', unitPrice: 25000 },
    ],
    discount: 10,
    tax: 21,
  },
  {
    id: '4',
    number: 'INV-3495',
    customer: 'Carlos Ruiz',
    initials: 'CR',
    issuedDate: '2026-03-15',
    dueDate: '2026-04-15',
    status: 'paid',
    items: [{ id: '6', description: 'Hosting anual', qty: 1, unit: 'año', unitPrice: 48000 }],
    discount: 0,
    tax: 21,
  },
  {
    id: '5',
    number: 'INV-3496',
    customer: 'Laura Díaz',
    initials: 'LD',
    issuedDate: '2026-03-18',
    dueDate: '2026-04-18',
    status: 'pending',
    items: [
      { id: '7', description: 'Mantenimiento mensual', qty: 3, unit: 'meses', unitPrice: 15000 },
      { id: '8', description: 'Soporte premium', qty: 3, unit: 'meses', unitPrice: 8000 },
    ],
    discount: 0,
    tax: 21,
  },
  {
    id: '6',
    number: 'INV-3497',
    customer: 'Pedro Sánchez',
    initials: 'PS',
    issuedDate: '2026-03-20',
    dueDate: '2026-04-20',
    status: 'paid',
    items: [{ id: '9', description: 'App mobile MVP', qty: 1, unit: 'proyecto', unitPrice: 350000 }],
    discount: 15,
    tax: 21,
  },
];

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
    archived_at: source.archived_at == null ? null : String(source.archived_at),
  };
}

export function readDemoInvoices(): InvoiceRecord[] {
  return readCommercialDocumentDemoRecords(STORAGE_KEY, INITIAL_INVOICES, sanitizeInvoice);
}

export function writeDemoInvoices(invoices: InvoiceRecord[]): void {
  writeCommercialDocumentDemoRecords(STORAGE_KEY, invoices);
}
