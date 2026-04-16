import { describe, expect, it, vi } from 'vitest';
import type { CrudInventoryLevelSnapshot, CrudLinkedEntitySnapshot } from './crudResourceInventoryDetailContract';
import {
  buildCrudInventoryAdjustPayload,
  buildCrudInventoryDetailSavePatch,
  computeCrudInventoryDetailDirty,
  persistCrudInventoryDetailSave,
  validateCrudInventoryDetailSave,
} from './crudResourceInventoryDetailSaveOrchestration';

const levelBase: CrudInventoryLevelSnapshot = {
  listRecordId: 'l1',
  linkedEntityId: 'p1',
  displayTitle: 'Producto',
  displaySubtitle: 'SKU-1',
  quantity: 10,
  minQuantity: 2,
  trackStock: true,
  isLowStock: false,
  updatedAt: '2026-01-01T00:00:00Z',
};

const linkedBase: CrudLinkedEntitySnapshot = {
  id: 'p1',
  name: 'Producto',
  sku: 'SKU-1',
  imageUrls: [],
  trackStock: true,
};

const flags = {
  linkedEntityFields: true,
  inventoryQuantities: true,
  linkedEntityTrackStock: true,
};

describe('computeCrudInventoryDetailDirty', () => {
  it('detecta solo inventario cuando cambia cantidad', () => {
    const r = computeCrudInventoryDetailDirty(
      levelBase,
      linkedBase,
      [],
      [],
      'Producto',
      'SKU-1',
      true,
      { minParsed: 2, absoluteQtyParsed: 11 },
      flags,
    );
    expect(r.productDirty).toBe(false);
    expect(r.inventoryDirty).toBe(true);
    expect(r.dirty).toBe(true);
  });

  it('no marca inventario sucio si trackStockInput es false', () => {
    const r = computeCrudInventoryDetailDirty(
      levelBase,
      linkedBase,
      [],
      [],
      'Producto',
      'SKU-1',
      false,
      { minParsed: 99, absoluteQtyParsed: 99 },
      flags,
    );
    expect(r.inventoryDirty).toBe(false);
  });

  it('no marca producto sucio por imágenes si servidor y borrador coinciden (vacías)', () => {
    const r = computeCrudInventoryDetailDirty(
      levelBase,
      linkedBase,
      [],
      [],
      'Producto',
      'SKU-1',
      true,
      { minParsed: 2, absoluteQtyParsed: 10 },
      flags,
    );
    expect(r.productDirty).toBe(false);
  });

  it('marca producto sucio si cambian las URLs de imagen respecto al servidor', () => {
    const r = computeCrudInventoryDetailDirty(
      levelBase,
      linkedBase,
      ['https://cdn.example/a.png'],
      ['https://cdn.example/b.png'],
      'Producto',
      'SKU-1',
      true,
      { minParsed: 2, absoluteQtyParsed: 10 },
      flags,
    );
    expect(r.productDirty).toBe(true);
    expect(r.inventoryDirty).toBe(false);
  });
});

describe('validateCrudInventoryDetailSave', () => {
  it('exige notas si hay cambio de inventario', () => {
    expect(validateCrudInventoryDetailSave(true, true, true, '').ok).toBe(false);
    expect(validateCrudInventoryDetailSave(true, true, true, 'x').ok).toBe(true);
  });

  it('permite guardar solo PATCH sin notas cuando no hay cambio de inventario', () => {
    expect(validateCrudInventoryDetailSave(true, false, true, '').ok).toBe(true);
  });

  it('rechaza solo ajuste sin notas', () => {
    expect(validateCrudInventoryDetailSave(false, true, true, '').ok).toBe(false);
    expect(validateCrudInventoryDetailSave(false, true, true, 'motivo').ok).toBe(true);
  });

  it('rechaza noop', () => {
    expect(validateCrudInventoryDetailSave(false, false, true, '').ok).toBe(false);
  });
});

describe('persistCrudInventoryDetailSave', () => {
  it('ejecuta PATCH antes que POST y luego recarga', async () => {
    const order: string[] = [];
    const ports = {
      patchLinkedEntity: vi.fn(async () => {
        order.push('patch');
        return { ...linkedBase, name: 'N' };
      }),
      postInventoryAdjust: vi.fn(async () => {
        order.push('adjust');
      }),
      loadInventoryLevel: vi.fn(async () => ({ ...levelBase, quantity: 11 })),
      loadLinkedEntity: vi.fn(async () => ({ ...linkedBase, name: 'N' })),
      loadMovements: vi.fn(async () => []),
    };

    const r = await persistCrudInventoryDetailSave(ports as never, {
      linkedEntityId: 'p1',
      hasProductPatch: true,
      patch: { name: 'N' },
      hasInventoryChange: true,
      adjustPayload: { quantityDelta: 1, notes: 'n' },
    });

    expect(order).toEqual(['patch', 'adjust']);
    expect(r.level.quantity).toBe(11);
  });

  it('solo PATCH: llama patch y recarga sin postInventoryAdjust', async () => {
    const ports = {
      patchLinkedEntity: vi.fn(async () => ({ ...linkedBase, name: 'Renombrado' })),
      postInventoryAdjust: vi.fn(),
      loadInventoryLevel: vi.fn(async () => levelBase),
      loadLinkedEntity: vi.fn(async () => ({ ...linkedBase, name: 'Renombrado' })),
      loadMovements: vi.fn(async () => []),
    };

    await persistCrudInventoryDetailSave(ports as never, {
      linkedEntityId: 'p1',
      hasProductPatch: true,
      patch: { name: 'Renombrado' },
      hasInventoryChange: false,
      adjustPayload: null,
    });

    expect(ports.patchLinkedEntity).toHaveBeenCalledTimes(1);
    expect(ports.postInventoryAdjust).not.toHaveBeenCalled();
    expect(ports.loadInventoryLevel).toHaveBeenCalledWith('p1');
  });

  it('solo ajuste: llama postInventoryAdjust y no patchLinkedEntity', async () => {
    const ports = {
      patchLinkedEntity: vi.fn(),
      postInventoryAdjust: vi.fn(),
      loadInventoryLevel: vi.fn(async () => ({ ...levelBase, quantity: 9 })),
      loadLinkedEntity: vi.fn(async () => linkedBase),
      loadMovements: vi.fn(async () => []),
    };

    await persistCrudInventoryDetailSave(ports as never, {
      linkedEntityId: 'p1',
      hasProductPatch: false,
      patch: {},
      hasInventoryChange: true,
      adjustPayload: { quantityDelta: -1, notes: 'salida' },
    });

    expect(ports.patchLinkedEntity).not.toHaveBeenCalled();
    expect(ports.postInventoryAdjust).toHaveBeenCalledTimes(1);
  });

  it('conserva el resultado del PATCH si recargar la entidad enlazada falla', async () => {
    const patched = { ...linkedBase, name: 'Renombrado tras patch' };
    const ports = {
      patchLinkedEntity: vi.fn(async () => patched),
      postInventoryAdjust: vi.fn(),
      loadInventoryLevel: vi.fn(async () => levelBase),
      loadLinkedEntity: vi.fn(async () => {
        throw new Error('temporary linked reload failure');
      }),
      loadMovements: vi.fn(async () => []),
    };

    const result = await persistCrudInventoryDetailSave(ports as never, {
      linkedEntityId: 'p1',
      hasProductPatch: true,
      patch: { name: patched.name },
      hasInventoryChange: false,
      adjustPayload: null,
    });

    expect(result.linked).toEqual(patched);
  });

  it('lanza si hay cambio de inventario pero falta adjustPayload', async () => {
    const ports = {
      patchLinkedEntity: vi.fn(),
      postInventoryAdjust: vi.fn(),
      loadInventoryLevel: vi.fn(),
      loadLinkedEntity: vi.fn(),
      loadMovements: vi.fn(),
    };

    await expect(
      persistCrudInventoryDetailSave(ports as never, {
        linkedEntityId: 'p1',
        hasProductPatch: false,
        patch: {},
        hasInventoryChange: true,
        adjustPayload: null,
      }),
    ).rejects.toThrow(/adjustPayload required/);
    expect(ports.postInventoryAdjust).not.toHaveBeenCalled();
  });
});

describe('buildCrudInventoryAdjustPayload', () => {
  it('arma delta y min opcional', () => {
    const build = buildCrudInventoryDetailSavePatch(
      levelBase,
      linkedBase,
      [],
      [],
      'Producto',
      'SKU-1',
      true,
      { minParsed: 3, absoluteQtyParsed: 10 },
      flags,
    );
    const p = buildCrudInventoryAdjustPayload(levelBase, { minParsed: 3, absoluteQtyParsed: 10 }, build, 'motivo');
    expect(p).toEqual({ quantityDelta: 0, notes: 'motivo', minQuantity: 3 });
  });
});

describe('buildCrudInventoryDetailSavePatch', () => {
  it('solo cambio de cantidad: hasInventoryChange y sin patch de producto', () => {
    const b = buildCrudInventoryDetailSavePatch(
      levelBase,
      linkedBase,
      [],
      [],
      'Producto',
      'SKU-1',
      true,
      { minParsed: 2, absoluteQtyParsed: 11 },
      flags,
    );
    expect(b.hasProductPatch).toBe(false);
    expect(b.hasInventoryChange).toBe(true);
    expect(Object.keys(b.patch)).toHaveLength(0);
  });

  it('incluye imageUrls en el patch cuando el borrador difiere del servidor', () => {
    const server = ['https://cdn.example/old.png'];
    const draft = ['https://cdn.example/new.png'];
    const b = buildCrudInventoryDetailSavePatch(
      levelBase,
      linkedBase,
      server,
      draft,
      'Producto',
      'SKU-1',
      true,
      { minParsed: 2, absoluteQtyParsed: 10 },
      flags,
    );
    expect(b.patch.imageUrls).toEqual(draft);
    expect(b.hasProductPatch).toBe(true);
  });

  it('no incluye imageUrls si lista vacía coincide con servidor vacío', () => {
    const b = buildCrudInventoryDetailSavePatch(
      levelBase,
      linkedBase,
      [],
      [],
      'Producto',
      'SKU-1',
      true,
      { minParsed: 2, absoluteQtyParsed: 10 },
      flags,
    );
    expect(b.patch.imageUrls).toBeUndefined();
  });
});
