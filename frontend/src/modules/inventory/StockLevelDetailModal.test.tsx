import { render, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CrudResourceInventoryDetailModalProps } from '../crud';

const shell = vi.hoisted(() => ({
  lastProps: null as CrudResourceInventoryDetailModalProps | null,
}));

vi.mock('../crud/CrudResourceInventoryDetailModal', () => ({
  CrudResourceInventoryDetailModal: (props: CrudResourceInventoryDetailModalProps) => {
    shell.lastProps = props;
    return <div data-testid="crud-inv-shell-stub" />;
  },
}));

import { StockLevelDetailModal } from './StockLevelDetailModal';

describe('StockLevelDetailModal (cableado)', () => {
  beforeEach(() => {
    shell.lastProps = null;
  });

  it('propaga inventoryHandlers y catalogHref=null al shell', async () => {
    const fetchLevel = vi.fn(async () => ({
      listRecordId: 'l',
      linkedEntityId: 'p1',
      displayTitle: 'X',
      displaySubtitle: '',
      quantity: 1,
      minQuantity: 0,
      trackStock: true,
      isLowStock: false,
      updatedAt: '2026-01-01T00:00:00Z',
    }));
    render(<StockLevelDetailModal productId="p1" onClose={() => {}} catalogHref={null} inventoryHandlers={{ fetchLevel }} />);
    await waitFor(() => {
      expect(shell.lastProps).not.toBeNull();
    });
    expect(shell.lastProps!.advancedSettingsHref).toBeUndefined();
    expect(shell.lastProps!.ports.loadInventoryLevel).toBe(fetchLevel);
  });

  it('pasa advancedSettingsHref custom cuando catalogHref es string', async () => {
    render(<StockLevelDetailModal productId="p1" onClose={() => {}} catalogHref="/modules/custom" />);
    await waitFor(() => {
      expect(shell.lastProps).not.toBeNull();
    });
    expect(shell.lastProps!.advancedSettingsHref).toBe('/modules/custom');
  });

  it('propaga uploadImages al puerto generico del shell', async () => {
    const uploadImages = vi.fn(async () => ['https://cdn.example/a.png']);
    render(<StockLevelDetailModal productId="p1" onClose={() => {}} inventoryHandlers={{ uploadImages }} />);
    await waitFor(() => {
      expect(shell.lastProps).not.toBeNull();
    });
    expect(shell.lastProps!.ports.uploadLinkedEntityImages).toBe(uploadImages);
  });
});
