import { describe, expect, it } from 'vitest';
import {
  buildCrudContextEntityPath,
  getCrudSearchParam,
  getCrudContextEntityParams,
} from './crudContextEntityParams';

describe('crudContextEntityParams', () => {
  it('reads entity params from a search string', () => {
    expect(
      getCrudContextEntityParams({
        search: '?entity=sales&entity_id=123',
      }),
    ).toEqual({
      entity: 'sales',
      entityId: '123',
    });
  });

  it('reads a generic CRUD search param from the URL search string', () => {
    expect(
      getCrudSearchParam('sale_id', {
        search: '?sale_id=sale-1',
      }),
    ).toBe('sale-1');
  });

  it('builds a contextual API path when params are present', () => {
    expect(
      buildCrudContextEntityPath(
        { entity: 'quotes', entityId: 'abc-1' },
        '/timeline?limit=100',
      ),
    ).toBe('/v1/quotes/abc-1/timeline?limit=100');
  });

  it('returns null when contextual params are incomplete', () => {
    expect(buildCrudContextEntityPath({ entity: 'sales' }, '/attachments')).toBeNull();
  });
});
