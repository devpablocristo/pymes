import { describe, expect, it } from 'vitest';
import { productFormToBody } from './inventoryHelpers';

describe('inventoryHelpers', () => {
  it('includes is_favorite in product payloads', () => {
    expect(
      productFormToBody({
        name: 'Producto demo',
        price: '10',
        track_stock: 'true',
        is_active: 'true',
        is_favorite: true,
        tags: 'nuevo, promo',
        notes: 'detalle',
        image_urls: '',
      }),
    ).toMatchObject({
      name: 'Producto demo',
      is_favorite: true,
      tags: ['nuevo', 'promo'],
      description: 'detalle',
    });
  });
});
