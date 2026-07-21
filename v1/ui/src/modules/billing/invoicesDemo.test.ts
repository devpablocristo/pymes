import { describe, expect, it } from 'vitest';
import {
  INVOICE_STATUS_BADGE_CLASS,
  INVOICE_STATUS_LABELS,
  calcInvoiceSubtotal,
  calcInvoiceTotal,
  formatInvoiceMoney,
  invoiceInitials,
  type InvoiceRecord,
} from './invoiceMath';

describe('invoiceMath', () => {
  it('calculates subtotal and total correctly', () => {
    const invoice: InvoiceRecord = {
      id: '1',
      number: 'INV-1',
      customer: 'Cliente Demo',
      initials: invoiceInitials('Cliente Demo'),
      issuedDate: '2026-01-01',
      dueDate: '2026-01-10',
      status: 'pending',
      items: [{ id: 'a', description: 'Servicio', qty: 2, unit: 'hora', unitPrice: 1000 }],
      discount: 10,
      tax: 21,
    };

    expect(calcInvoiceSubtotal(invoice.items)).toBe(2000);
    expect(calcInvoiceTotal(invoice)).toBe(2178);
    expect(formatInvoiceMoney(2178)).toContain('$');
  });

  it('computes initials from customer name', () => {
    expect(invoiceInitials('Distribuidora Norte')).toBe('DN');
    expect(invoiceInitials('Café Central')).toBe('CC');
    expect(invoiceInitials('Ferretería Sur')).toBe('FS');
  });

  it('exposes status labels and badge classes for the 3 canonical states', () => {
    expect(Object.keys(INVOICE_STATUS_LABELS).sort()).toEqual(['overdue', 'paid', 'pending']);
    expect(Object.keys(INVOICE_STATUS_BADGE_CLASS).sort()).toEqual(['overdue', 'paid', 'pending']);
  });
});
