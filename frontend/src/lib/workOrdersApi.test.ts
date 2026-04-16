import { beforeEach, describe, expect, it, vi } from 'vitest';

const apiMocks = vi.hoisted(() => ({
  request: vi.fn(),
  readActiveBranchId: vi.fn(),
}));

vi.mock('./verticalApi', () => ({
  createVerticalRequest: () => apiMocks.request,
}));

vi.mock('./branchSelectionStorage', () => ({
  readActiveBranchId: () => apiMocks.readActiveBranchId(),
}));

import {
  createWorkOrder,
  createWorkshopBooking,
  getWorkOrders,
  getWorkOrdersArchived,
} from './workOrdersApi';

describe('workOrdersApi branch scoping', () => {
  beforeEach(() => {
    apiMocks.request.mockReset();
    apiMocks.request.mockResolvedValue({ items: [], has_more: false });
    apiMocks.readActiveBranchId.mockReset();
    apiMocks.readActiveBranchId.mockReturnValue('branch-active');
  });

  it('scopes active work orders to the globally selected branch by default', async () => {
    await getWorkOrders({ target_type: 'vehicle', search: 'ford' });

    expect(apiMocks.request).toHaveBeenCalledWith('/v1/work-orders?branch_id=branch-active&target_type=vehicle&search=ford');
  });

  it('uses the explicit branch filter when the caller provides one', async () => {
    await getWorkOrdersArchived({ target_type: 'vehicle', branch_id: 'branch-north' });

    expect(apiMocks.request).toHaveBeenCalledWith('/v1/work-orders/archived?branch_id=branch-north&target_type=vehicle');
  });

  it('injects the active branch into work order creation when the form omits it', async () => {
    await createWorkOrder({
      target_type: 'vehicle',
      target_id: 'vehicle-1',
      items: [],
    });

    expect(apiMocks.request).toHaveBeenCalledWith('/v1/work-orders', {
      method: 'POST',
      body: {
        target_type: 'vehicle',
        target_id: 'vehicle-1',
        items: [],
        branch_id: 'branch-active',
      },
    });
  });

  it('preserves an explicit branch when scheduling a booking from a work order', async () => {
    await createWorkshopBooking({
      branch_id: 'branch-from-row',
      customer_name: 'Juan',
      start_at: '2026-04-16T13:00:00Z',
    });

    expect(apiMocks.request).toHaveBeenCalledWith('/v1/workshop-bookings', {
      method: 'POST',
      body: {
        branch_id: 'branch-from-row',
        customer_name: 'Juan',
        start_at: '2026-04-16T13:00:00Z',
      },
    });
  });
});
