import { commercialDocumentInitials } from './commercialDocumentMath';
import type { InvoiceLineItem, InvoiceRecord, InvoiceStatus } from '../../lib/invoicesApi';

export type { InvoiceLineItem, InvoiceRecord, InvoiceStatus };

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

export function invoiceInitials(name: string): string {
  return commercialDocumentInitials(name);
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
