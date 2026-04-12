import { describe, expect, it } from 'vitest';
import {
  createCreditNotesCrudConfig,
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

describe('billingHelpers', () => {
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
    const config = createQuotesCrudConfig<QuoteRecord>({ renderList: () => <></> });

    expect(config.basePath).toBe('/v1/quotes');
    expect(config.labelPluralCap).toBe('Presupuestos');
    expect(config.rowActions?.map((action) => action.id)).toEqual(['pdf', 'send', 'accept']);
    expect(
      config.toBody?.({
        customer_id: 'c1',
        customer_name: 'Cliente',
        valid_until: '2026-04-11',
        items_json: '[{"description":"Servicio","quantity":1,"unit_price":1000}]',
        notes: 'ok',
      }),
    ).toEqual({
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

  it('builds sales config from billing domain', () => {
    const config = createSalesCrudConfig<SaleRecord>({ renderList: () => <></> });

    expect(config.basePath).toBe('/v1/sales');
    expect(config.labelPluralCap).toBe('Ventas');
    expect(config.rowActions?.map((action) => action.id)).toEqual(['receipt-pdf', 'payments', 'add-payment', 'void']);
    expect(
      config.toBody?.({
        customer_id: 'c1',
        customer_name: 'Cliente',
        quote_id: 'q1',
        payment_method: 'efectivo',
        items_json: '[{"description":"Producto","quantity":1,"unit_price":1000}]',
        notes: 'ok',
      }),
    ).toEqual({
      customer_id: 'c1',
      customer_name: 'Cliente',
      quote_id: 'q1',
      payment_method: 'efectivo',
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
    const config = createPurchasesCrudConfig<PurchaseRecord>({ renderList: () => <></> });

    expect(config.basePath).toBe('/v1/purchases');
    expect(config.labelPluralCap).toBe('Compras');
    expect(
      config.toBody?.({
        supplier_id: 's1',
        supplier_name: 'Proveedor',
        status: 'draft',
        payment_status: 'pending',
        items_json: '[{"description":"Insumo","quantity":1,"unit_cost":1000}]',
        notes: 'ok',
      }),
    ).toEqual({
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
