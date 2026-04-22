import { beforeEach, describe, expect, it } from 'vitest';
import {
  archiveInvoice,
  calcInvoiceSubtotal,
  calcInvoiceTotal,
  formatInvoiceMoney,
  INITIAL_INVOICES,
  readDemoInvoices,
  restoreInvoice,
  writeDemoInvoices,
  type InvoiceRecord,
} from './invoicesDemo';

describe('invoicesDemo', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('calculates subtotal and total correctly', () => {
    const invoice: InvoiceRecord = {
      id: '1',
      number: 'INV-1',
      customer: 'Cliente Demo',
      initials: 'CD',
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

  it('persists demo invoices to localStorage', () => {
    const invoices: InvoiceRecord[] = [
      {
        id: 'x',
        number: 'INV-9999',
        customer: 'Persistido',
        initials: 'PE',
        issuedDate: '2026-01-01',
        dueDate: '2026-01-10',
        status: 'paid',
        items: [],
        discount: 0,
        tax: 21,
        is_favorite: false,
        tags: [],
        archived_at: null,
      },
    ];

    writeDemoInvoices(invoices);
    expect(readDemoInvoices()).toEqual(invoices);
  });

  it('starts empty when localStorage has no persisted invoices', () => {
    expect(INITIAL_INVOICES).toEqual([]);
    expect(readDemoInvoices()).toEqual([]);
  });

  it('archives and restores invoices', () => {
    const invoice: InvoiceRecord = {
      id: '1',
      number: 'INV-1',
      customer: 'Cliente Demo',
      initials: 'CD',
      issuedDate: '2026-01-01',
      dueDate: '2026-01-10',
      status: 'pending',
      items: [],
      discount: 0,
      tax: 21,
      archived_at: null,
    };

    const archived = archiveInvoice(invoice);
    expect(archived.archived_at).toBeTruthy();
    expect(restoreInvoice(archived).archived_at).toBeNull();
  });
});
