// Re-export de tipos y helpers matemáticos desde sus nuevos hogares.
// `invoices` dejó de ser demo localStorage — ahora vive en pymes-core (`/v1/invoices`).
// Se conserva este archivo como shim para no romper imports externos.

export {
  INVOICE_STATUS_BADGE_CLASS,
  INVOICE_STATUS_LABELS,
  calcInvoiceSubtotal,
  calcInvoiceTotal,
  formatInvoiceMoney,
  invoiceInitials,
} from './invoiceMath';
export type { InvoiceLineItem, InvoiceRecord, InvoiceStatus } from './invoiceMath';
