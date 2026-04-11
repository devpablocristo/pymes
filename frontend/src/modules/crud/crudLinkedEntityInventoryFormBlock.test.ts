import { describe, expect, it } from 'vitest';
import { crudLinkedEntityHasDisplayName } from './crudLinkedEntityInventoryFormBlock';

describe('crudLinkedEntityHasDisplayName', () => {
  it('rechaza vacío y solo espacios', () => {
    expect(crudLinkedEntityHasDisplayName('')).toBe(false);
    expect(crudLinkedEntityHasDisplayName('   ')).toBe(false);
  });

  it('acepta texto recortable', () => {
    expect(crudLinkedEntityHasDisplayName('  x  ')).toBe(true);
  });
});
