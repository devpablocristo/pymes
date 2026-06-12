import { describe, it, expect } from 'vitest';
import { parseWorkOrderItemsJson, stringifyWorkOrderItems } from './workOrderItemsJson';

describe('parseWorkOrderItemsJson', () => {
  it('returns empty array for empty string', () => {
    expect(parseWorkOrderItemsJson('')).toEqual([]);
    expect(parseWorkOrderItemsJson('   ')).toEqual([]);
  });

  it('throws on invalid JSON', () => {
    expect(() => parseWorkOrderItemsJson('not json')).toThrow('JSON inv\u00e1lido');
  });

  it('throws when JSON is not an array', () => {
    expect(() => parseWorkOrderItemsJson('{"a":1}')).toThrow('arreglo JSON');
  });

  it('parses valid items', () => {
    const input = JSON.stringify([
      { item_type: 'part', description: 'Freno', quantity: 2, unit_price: 100 },
      { item_type: 'service', description: 'Alineaci\u00f3n', quantity: 1, unit_price: 500 },
    ]);
    const result = parseWorkOrderItemsJson(input);
    expect(result).toHaveLength(2);
    expect(result[0].item_type).toBe('part');
    expect(result[0].description).toBe('Freno');
    expect(result[0].quantity).toBe(2);
    expect(result[0].unit_price).toBe(100);
    expect(result[0].tax_rate).toBe(21); // default
    expect(result[0].sort_order).toBe(0);
    expect(result[1].sort_order).toBe(1);
  });

  it('defaults item_type to service for non-part values', () => {
    const input = JSON.stringify([{ item_type: 'unknown', description: 'Test', quantity: 1, unit_price: 10 }]);
    const result = parseWorkOrderItemsJson(input);
    expect(result[0].item_type).toBe('service');
  });

  it('filters out items with empty description or zero quantity', () => {
    const input = JSON.stringify([
      { description: '', quantity: 1, unit_price: 10 },
      { description: 'Valid', quantity: 0, unit_price: 10 },
      { description: 'Good', quantity: 1, unit_price: 10 },
    ]);
    const result = parseWorkOrderItemsJson(input);
    expect(result).toHaveLength(1);
    expect(result[0].description).toBe('Good');
  });

  it('uses explicit tax_rate when provided', () => {
    const input = JSON.stringify([{ description: 'Item', quantity: 1, unit_price: 10, tax_rate: 10.5 }]);
    const result = parseWorkOrderItemsJson(input);
    expect(result[0].tax_rate).toBe(10.5);
  });

  it('handles metadata object', () => {
    const input = JSON.stringify([{ description: 'Item', quantity: 1, unit_price: 10, metadata: { key: 'val' } }]);
    const result = parseWorkOrderItemsJson(input);
    expect(result[0].metadata).toEqual({ key: 'val' });
  });

  it('defaults metadata to empty object for non-object values', () => {
    const input = JSON.stringify([{ description: 'Item', quantity: 1, unit_price: 10, metadata: 'bad' }]);
    const result = parseWorkOrderItemsJson(input);
    expect(result[0].metadata).toEqual({});
  });
});

describe('stringifyWorkOrderItems', () => {
  it('returns [] for undefined', () => {
    expect(stringifyWorkOrderItems(undefined)).toBe('[]');
  });

  it('returns [] for empty array', () => {
    expect(stringifyWorkOrderItems([])).toBe('[]');
  });

  it('returns formatted JSON for items', () => {
    const items = [
      {
        item_type: 'part' as const,
        description: 'Freno',
        quantity: 2,
        unit_price: 100,
        tax_rate: 21,
      },
    ];
    const result = stringifyWorkOrderItems(items);
    expect(JSON.parse(result)).toEqual(items);
    expect(result).toContain('\n'); // pretty-printed
  });
});
