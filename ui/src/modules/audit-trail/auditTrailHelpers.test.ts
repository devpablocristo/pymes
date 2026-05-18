import { describe, expect, it } from 'vitest';
import { formatAuditTrailTagList, normalizeAuditTrailListItems, parseAuditTrailCsv } from './auditTrailHelpers';

describe('auditTrailHelpers', () => {
  it('parsea csv de eventos', () => {
    expect(parseAuditTrailCsv('sale.created, customer.updated , , webhook.failed')).toEqual([
      'sale.created',
      'customer.updated',
      'webhook.failed',
    ]);
  });

  it('formatea tags', () => {
    expect(formatAuditTrailTagList(['a', 'b'])).toBe('a, b');
  });

  it('normaliza ids a string', () => {
    expect(normalizeAuditTrailListItems({ items: [{ id: 7, name: 'x' }] })).toEqual([{ id: '7', name: 'x' }]);
  });
});
