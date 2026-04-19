import { beforeEach, describe, expect, it } from 'vitest';
import {
  createCreditNotesCrudConfig,
  createInvoicesCrudConfig,
  createInvoicesShellConfig,
  createPurchasesCrudConfig,
  createQuotesCrudConfig,
  createSalesCrudConfig,
  parseCommercialCostLineItems,
  parseCommercialPricedLineItems,
  type CreditNoteRecord,
  type PurchaseRecord,
  type QuoteRecord,
  type SaleRecord,
} from './billingHelpers';
import type { InvoiceRecord } from './invoicesDemo';

describe('billingHelpers', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('parses priced commercial line items', () => {
    expect(
      parseCommercialPricedLineItems(
        '[{"description":"Servicio","quantity":2,"unit_price":1000,"sort_order":3}]',
      ),
    ).toEqual([
      {
        description: 'Servicio',
        product_id: undefined,
        quantity: 2,
        service_id: undefined,
        sort_order: 3,
        tax_rate: undefined,
        unit_price: 1000,
      },
    ]);
  });

  it('parses cost commercial line items', () => {
    expect(
      parseCommercialCostLineItems(
        '[{"description":"Insumo","quantity":2,"unit_cost":500}]',
      ),
    ).toEqual([
      {
        description: 'Insumo',
        product_id: undefined,
        quantity: 2,
        service_id: undefined,
        tax_rate: undefined,
        unit_cost: 500,
      },
    ]);
  });

  it('builds quotes config from billing domain', () => {
    window.localStorage.setItem('pymes-ui:branch-selection:active', 'branch-active');
    const config = createQuotesCrudConfig<QuoteRecord>({ renderList: () => <></> });

    expect(config.basePath).toBe('/v1/quotes');
    expect(config.labelPluralCap).toBe('Presupuestos');
    expect(config.stateMachine).toMatchObject({
      field: 'status',
      states: [
        { value: 'draft', label: 'Borrador', columnId: 'draft', badgeVariant: 'default' },
        { value: 'sent', label: 'Enviado', columnId: 'sent', badgeVariant: 'info' },
        { value: 'accepted', label: 'Aceptado', columnId: 'accepted', badgeVariant: 'success' },
        { value: 'rejected', label: 'Rechazado', columnId: 'rejected', badgeVariant: 'danger' },
      ],
      columns: [
        { id: 'draft', label: 'Borrador', defaultState: 'draft' },
        { id: 'sent', label: 'Enviado', defaultState: 'sent' },
        { id: 'accepted', label: 'Aceptado', defaultState: 'accepted' },
        { id: 'rejected', label: 'Rechazado', defaultState: 'rejected' },
      ],
    });
    expect(config.editorModal).toMatchObject({
      blocks: [
        {
          id: 'items',
          kind: 'lineItems',
          field: 'items',
          sectionId: 'items',
          visible: expect.any(Function),
        },
      ],
      sections: [{ id: 'default' }, { id: 'items' }],
    });
    expect(config.rowActions?.map((action) => action.id)).toEqual(['pdf', 'send', 'accept']);
    expect(
      config.toBody?.({
        customer_id: 'c1',
        customer_name: 'Cliente',
        valid_until: '2026-04-11',
        items: '[{"description":"Servicio","quantity":1,"unit_price":1000}]',
        notes: 'ok',
      }),
    ).toEqual({
      branch_id: 'branch-active',
      customer_id: 'c1',
      customer_name: 'Cliente',
      valid_until: '2026-04-11',
      items: [
        {
          description: 'Servicio',
          product_id: undefined,
          quantity: 1,
          service_id: undefined,
          sort_order: 0,
          tax_rate: undefined,
          unit_price: 1000,
        },
      ],
      notes: 'ok',
    });
  });

  it('builds invoices state machine for list and shell configs', () => {
    const config = createInvoicesCrudConfig<InvoiceRecord>({ renderList: () => <></> });
    const shellConfig = createInvoicesShellConfig<InvoiceRecord>();

    expect(config.stateMachine).toMatchObject({
      field: 'status',
      states: [
        { value: 'paid', label: 'Pagada', columnId: 'paid', badgeVariant: 'success' },
        { value: 'pending', label: 'Pendiente', columnId: 'pending', badgeVariant: 'warning' },
        { value: 'overdue', label: 'Vencida', columnId: 'overdue', badgeVariant: 'danger' },
      ],
      columns: [
        { id: 'paid', label: 'Pagada', defaultState: 'paid' },
        { id: 'pending', label: 'Pendiente', defaultState: 'pending' },
        { id: 'overdue', label: 'Vencida', defaultState: 'overdue' },
      ],
    });
    expect(shellConfig.stateMachine).toEqual(config.stateMachine);
    expect(config.allowCreate).toBe(true);
    expect(config.allowEdit).toBe(true);
    expect(config.formFields).toEqual([
      { key: 'number', label: 'Comprobante' },
      { key: 'customer', label: 'Cliente', placeholder: 'Nombre del cliente', required: true },
      { key: 'issuedDate', label: 'Fecha de emisión', type: 'date' },
      { key: 'dueDate', label: 'Fecha de vencimiento', type: 'date' },
      {
        key: 'status',
        label: 'Estado',
        type: 'select',
        options: [
          { value: 'paid', label: 'Pagada' },
          { value: 'pending', label: 'Pendiente' },
          { value: 'overdue', label: 'Vencida' },
        ],
      },
      { key: 'discount', label: 'Descuento (%)', type: 'number' },
      { key: 'tax', label: 'Impuesto (%)', type: 'number' },
      {
        key: 'items',
        label: 'Detalle',
        type: 'textarea',
        fullWidth: true,
        required: true,
        placeholder: '[{"description":"Servicio","qty":1,"unit":"unidad","unitPrice":1000}]',
      },
    ]);
    expect(config.toFormValues?.({
      id: '1',
      number: 'INV-1',
      customer: 'Cliente Demo',
      initials: 'CD',
      issuedDate: '2026-04-14',
      dueDate: '2026-04-20',
      status: 'pending',
      items: [{ id: '1', description: 'Servicio', qty: 1, unit: 'unidad', unitPrice: 1000 }],
      discount: 0,
      tax: 21,
    })).toEqual({
      number: 'INV-1',
      customer: 'Cliente Demo',
      issuedDate: '2026-04-14',
      dueDate: '2026-04-20',
      status: 'pending',
      discount: '0',
      tax: '21',
      items: '[{"id":"1","description":"Servicio","qty":1,"unit":"unidad","unitPrice":1000}]',
    });
    expect(config.isValid?.({ customer: 'Cliente Demo', items: '[{"description":"Servicio"}]' })).toBe(true);
    expect(config.isValid?.({ customer: '', items: '' })).toBe(false);
  });

  it('builds sales config from billing domain', () => {
    window.localStorage.setItem('pymes-ui:branch-selection:active', 'branch-active');
    const config = createSalesCrudConfig<SaleRecord>({ renderList: () => <></> });

    expect(config.basePath).toBe('/v1/sales');
    expect(config.labelPluralCap).toBe('Ventas');
    expect(config.stateMachine).toMatchObject({
      field: 'status',
      states: [
        { value: 'draft', label: 'Borrador', columnId: 'draft', badgeVariant: 'default' },
        { value: 'completed', label: 'Completada', columnId: 'completed', badgeVariant: 'success' },
        { value: 'paid', label: 'Pagada', columnId: 'paid', badgeVariant: 'success' },
        { value: 'pending', label: 'Pendiente', columnId: 'pending', badgeVariant: 'warning' },
        { value: 'voided', label: 'Anulada', columnId: 'voided', badgeVariant: 'danger' },
        { value: 'cancelled', label: 'Cancelada', columnId: 'cancelled', badgeVariant: 'danger' },
      ],
      columns: [
        { id: 'draft', label: 'Borrador', defaultState: 'draft' },
        { id: 'completed', label: 'Completada', defaultState: 'completed' },
        { id: 'paid', label: 'Pagada', defaultState: 'paid' },
        { id: 'pending', label: 'Pendiente', defaultState: 'pending' },
        { id: 'voided', label: 'Anulada', defaultState: 'voided' },
        { id: 'cancelled', label: 'Cancelada', defaultState: 'cancelled' },
      ],
    });
    expect(config.editorModal).toMatchObject({
      blocks: [
        {
          id: 'items',
          kind: 'lineItems',
          field: 'items',
          sectionId: 'items',
          visible: expect.any(Function),
        },
      ],
      sections: [{ id: 'default' }, { id: 'items' }],
    });
    expect(config.rowActions?.map((action) => action.id)).toEqual(['receipt-pdf', 'payments', 'add-payment', 'void']);
    expect(
      config.toBody?.({
        customer_id: 'c1',
        customer_name: 'Cliente',
        quote_id: 'q1',
        payment_method: 'cash',
        items: '[{"description":"Producto","quantity":1,"unit_price":1000}]',
        notes: 'ok',
      }),
    ).toEqual({
      branch_id: 'branch-active',
      customer_id: 'c1',
      customer_name: 'Cliente',
      quote_id: 'q1',
      payment_method: 'cash',
      items: [
        {
          description: 'Producto',
          product_id: undefined,
          quantity: 1,
          service_id: undefined,
          sort_order: 0,
          tax_rate: undefined,
          unit_price: 1000,
        },
      ],
      notes: 'ok',
    });
  });

  it('builds credit notes config from billing domain', () => {
    const config = createCreditNotesCrudConfig<CreditNoteRecord>({ renderList: () => <></> });

    expect(config.labelPluralCap).toBe('Notas de crédito');
    expect(config.emptyState).toBe('No hay notas de crédito emitidas.');
    expect(config.allowEdit).toBe(false);
    expect(config.stateMachine).toMatchObject({
      field: 'status',
      states: [
        { value: 'active', label: 'Activa', columnId: 'active', badgeVariant: 'info' },
        { value: 'partially_used', label: 'Parcialmente usada', columnId: 'partially_used', badgeVariant: 'warning' },
        { value: 'used', label: 'Usada', columnId: 'used', badgeVariant: 'success' },
        { value: 'expired', label: 'Vencida', columnId: 'expired', badgeVariant: 'danger' },
      ],
      columns: [
        { id: 'active', label: 'Activa', defaultState: 'active' },
        { id: 'partially_used', label: 'Parcialmente usada', defaultState: 'partially_used' },
        { id: 'used', label: 'Usada', defaultState: 'used' },
        { id: 'expired', label: 'Vencida', defaultState: 'expired' },
      ],
    });
    expect(
      config.toFormValues?.({
        id: '1',
        number: 'NC-1',
        party_id: '12345678-1234-1234-1234-123456789012',
        return_id: '',
        amount: 100,
        used_amount: 0,
        balance: 100,
        status: 'active',
        created_at: '2026-04-11',
      }),
    ).toEqual({ party_id: '', amount: '' });
    expect(
      config.isValid?.({
        party_id: '12345678-1234-1234-1234-123456789012',
        amount: '100',
      }),
    ).toBe(true);
  });

  it('builds purchases config from billing domain', () => {
    window.localStorage.setItem('pymes-ui:branch-selection:active', 'branch-active');
    const config = createPurchasesCrudConfig<PurchaseRecord>({ renderList: () => <></> });

    expect(config.basePath).toBe('/v1/purchases');
    expect(config.labelPluralCap).toBe('Compras');
    expect(config.viewModes?.map((mode) => mode.id)).toEqual(['list', 'gallery', 'kanban']);
    expect(config.viewModes?.map((mode) => mode.path)).toEqual(['list', 'gallery', 'board']);
    expect(config.stateMachine).toMatchObject({
      field: 'status',
      states: [
        { value: 'draft', label: 'Borrador', columnId: 'draft', badgeVariant: 'default' },
        { value: 'partial', label: 'Parcial', columnId: 'partial', badgeVariant: 'warning' },
        { value: 'received', label: 'Recibida', columnId: 'received', badgeVariant: 'info' },
        { value: 'voided', label: 'Anulada', columnId: 'voided', badgeVariant: 'danger' },
      ],
      columns: [
        { id: 'draft', label: 'Borrador', defaultState: 'draft' },
        { id: 'partial', label: 'Parcial', defaultState: 'partial' },
        { id: 'received', label: 'Recibida', defaultState: 'received' },
        { id: 'voided', label: 'Anulada', defaultState: 'voided' },
      ],
      transitions: [
        { from: 'draft', to: ['partial', 'received', 'voided'] },
        { from: 'partial', to: ['draft', 'received', 'voided'] },
        { from: 'received', to: ['draft', 'partial', 'voided'] },
        { from: 'voided', to: ['draft', 'partial', 'received'] },
      ],
    });
    expect(config.kanban).toEqual(
      expect.objectContaining({
        createFooterLabel: 'Añadir compra',
        persistMove: expect.any(Function),
      }),
    );
    expect(config.editorModal).toEqual(
      expect.objectContaining({
        eyebrow: 'Compras',
        blocks: [
          {
            id: 'items',
            kind: 'lineItems',
            field: 'items',
            sectionId: 'items',
            visible: expect.any(Function),
          },
        ],
        sections: [
          {
            id: 'summary',
            title: 'Resumen de la compra',
            fieldKeys: ['number', 'supplier_name', 'status', 'payment_status', 'total', 'received_at'],
          },
          {
            id: 'items',
          },
          {
            id: 'notes',
            title: 'Notas',
            fieldKeys: ['notes'],
          },
        ],
      }),
    );
    expect(config.editorModal?.stats).toBeUndefined();
    expect(config.formFields.find((field) => field.key === 'number')).toEqual({
      key: 'number',
      label: 'Comprobante',
    });
    expect(config.formFields.find((field) => field.key === 'status')).toEqual({
      key: 'status',
      label: 'Estado',
      type: 'select',
      options: [
        { value: 'draft', label: 'Borrador' },
        { value: 'partial', label: 'Parcial' },
        { value: 'received', label: 'Recibida' },
        { value: 'voided', label: 'Anulada' },
      ],
    });
    expect(config.formFields.find((field) => field.key === 'supplier_id')).toBeUndefined();
    expect(config.formFields.find((field) => field.key === 'payment_status')).toEqual({
      key: 'payment_status',
      label: 'Pago',
      type: 'select',
      options: [
        { value: 'pending', label: 'Pendiente' },
        { value: 'partial', label: 'Parcial' },
        { value: 'paid', label: 'Pagado' },
      ],
    });
    expect(config.formFields.find((field) => field.key === 'total')).toEqual({
      key: 'total',
      label: 'Total',
    });
    expect(config.formFields.find((field) => field.key === 'received_at')).toEqual({
      key: 'received_at',
      label: 'Fecha de recepción',
    });
    expect(
      config.toBody?.({
        supplier_id: 's1',
        supplier_name: 'Proveedor',
        status: 'draft',
        payment_status: 'pending',
        items: '[{"description":"Insumo","quantity":1,"unit_cost":1000}]',
        notes: 'ok',
      }),
    ).toEqual({
      branch_id: 'branch-active',
      supplier_id: 's1',
      supplier_name: 'Proveedor',
      status: 'draft',
      payment_status: 'pending',
      items: [
        {
          description: 'Insumo',
          product_id: undefined,
          quantity: 1,
          service_id: undefined,
          tax_rate: undefined,
          unit_cost: 1000,
        },
      ],
      notes: 'ok',
    });
  });
});
