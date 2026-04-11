import { describe, expect, it, vi } from 'vitest';
import type { CrudInventoryLevelSnapshot, CrudLinkedEntityPatch, CrudLinkedEntitySnapshot } from '../crud/crudResourceInventoryDetailContract';
import { buildStockInventoryDetailPorts } from './stockInventoryDetailModalAdapter';

function minimalLevel(overrides: Partial<CrudInventoryLevelSnapshot> = {}): CrudInventoryLevelSnapshot {
  return {
    listRecordId: 'l1',
    linkedEntityId: 'p1',
    displayTitle: 'T',
    displaySubtitle: '',
    quantity: 1,
    minQuantity: 0,
    trackStock: true,
    isLowStock: false,
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('buildStockInventoryDetailPorts', () => {
  it('delega loadInventoryLevel al fetchLevel sobreescrito', async () => {
    const fetchLevel = vi.fn(async (): Promise<CrudInventoryLevelSnapshot> => minimalLevel({ linkedEntityId: 'p99' }));
    const ports = buildStockInventoryDetailPorts({ fetchLevel });
    const row = await ports.loadInventoryLevel('p99');
    expect(fetchLevel).toHaveBeenCalledTimes(1);
    expect(fetchLevel).toHaveBeenCalledWith('p99');
    expect(row.linkedEntityId).toBe('p99');
  });

  it('delega patchLinkedEntity al patchEntity sobreescrito', async () => {
    const linked: CrudLinkedEntitySnapshot = { id: 'i1', name: 'N', sku: '', imageUrls: [] };
    const patchEntity = vi.fn(async (_id: string, _patch: CrudLinkedEntityPatch) => linked);
    const ports = buildStockInventoryDetailPorts({ patchEntity });
    const out = await ports.patchLinkedEntity('id1', { name: 'x' });
    expect(patchEntity).toHaveBeenCalledTimes(1);
    expect(patchEntity).toHaveBeenCalledWith('id1', { name: 'x' });
    expect(out).toEqual(linked);
  });
});
