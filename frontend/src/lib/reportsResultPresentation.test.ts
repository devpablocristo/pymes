import { describe, expect, it } from 'vitest';
import { extractTabularRows, orderedReportColumns } from './reportsResultPresentation';

describe('extractTabularRows', () => {
  it('extrae items de envelope con items', () => {
    const rows = extractTabularRows({
      items: [{ a: 1 }, { a: 2 }],
    });
    expect(rows).toEqual([{ a: 1 }, { a: 2 }]);
  });

  it('extrae objeto data como una fila', () => {
    const rows = extractTabularRows({
      from: '2025-01-01',
      to: '2025-01-31',
      data: { total_sales: 100, count_sales: 3 },
    });
    expect(rows).toEqual([{ total_sales: 100, count_sales: 3 }]);
  });
});

describe('orderedReportColumns', () => {
  it('ordena columnas de ventas por producto', () => {
    const keys = ['revenue', 'product_id', 'product_name', 'quantity'];
    expect(orderedReportColumns('/v1/reports/sales-by-product', keys)).toEqual([
      'product_name',
      'quantity',
      'revenue',
      'product_id',
    ]);
  });
});
