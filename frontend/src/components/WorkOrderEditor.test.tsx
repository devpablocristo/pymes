import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { WorkOrder as AutoRepairWorkOrder } from '../lib/workOrdersApi';
import { WorkOrderEditor } from '../modules/work-orders';

const apiMocks = vi.hoisted(() => ({
  getWorkOrder: vi.fn<[], Promise<AutoRepairWorkOrder>>(),
  updateWorkOrder: vi.fn(),
  archiveWorkOrder: vi.fn(),
  restoreWorkOrder: vi.fn(),
  confirmAction: vi.fn(),
}));

vi.mock('../lib/workOrdersApi', () => ({
  getWorkOrder: () => apiMocks.getWorkOrder(),
  updateWorkOrder: (...args: unknown[]) => apiMocks.updateWorkOrder(...args),
  archiveWorkOrder: (...args: unknown[]) => apiMocks.archiveWorkOrder(...args),
  restoreWorkOrder: (...args: unknown[]) => apiMocks.restoreWorkOrder(...args),
}));

vi.mock('@devpablocristo/core-browser', () => ({
  confirmAction: (options: unknown) => apiMocks.confirmAction(options),
}));

function buildWorkOrder(overrides?: Partial<AutoRepairWorkOrder>): AutoRepairWorkOrder {
  return {
    id: 'wo-1',
    org_id: 'org-1',
    number: 'OT-001',
    target_type: 'vehicle',
    target_id: 'veh-1',
    target_label: 'AAA111',
    metadata: {},
    vehicle_id: 'veh-1',
    vehicle_plate: 'AAA111',
    customer_id: 'cust-1',
    customer_name: 'Cliente original',
    booking_id: undefined,
    quote_id: undefined,
    sale_id: undefined,
    status: 'received',
    requested_work: 'Cambio de aceite',
    diagnosis: '',
    notes: '',
    internal_notes: '',
    currency: 'ARS',
    subtotal_services: 0,
    subtotal_parts: 0,
    tax_total: 0,
    total: 0,
    opened_at: '2026-04-02T10:00:00Z',
    promised_at: undefined,
    ready_at: undefined,
    delivered_at: undefined,
    ready_pickup_notified_at: undefined,
    created_by: 'tech-1',
    archived_at: null,
    created_at: '2026-04-02T10:00:00Z',
    updated_at: '2026-04-02T10:00:00Z',
    items: [],
    ...overrides,
  };
}

describe('WorkOrderEditor', () => {
  beforeEach(() => {
    apiMocks.getWorkOrder.mockReset();
    apiMocks.updateWorkOrder.mockReset();
    apiMocks.archiveWorkOrder.mockReset();
    apiMocks.restoreWorkOrder.mockReset();
    apiMocks.confirmAction.mockReset();

    apiMocks.getWorkOrder.mockResolvedValue(buildWorkOrder());
  });

  it('cierra sin confirmación cuando no hay cambios pendientes', async () => {
    const onClose = vi.fn();

    render(<WorkOrderEditor orderId="wo-1" variant="modal" onClose={onClose} onSaved={vi.fn()} />);

    expect(await screen.findByLabelText('Cliente')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Cerrar' }));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(apiMocks.confirmAction).not.toHaveBeenCalled();
  });

  it('pide confirmación antes de cerrar con Escape si hay cambios sin guardar', async () => {
    const onClose = vi.fn();
    apiMocks.confirmAction.mockResolvedValue(false);

    render(<WorkOrderEditor orderId="wo-1" variant="modal" onClose={onClose} onSaved={vi.fn()} />);

    const customerInput = await screen.findByLabelText('Cliente');
    fireEvent.change(customerInput, { target: { value: 'Cliente editado' } });
    fireEvent.keyDown(window, { key: 'Escape' });

    await waitFor(() => {
      expect(apiMocks.confirmAction).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'Cancelar edición',
          confirmLabel: 'Sí, cancelar',
          cancelLabel: 'Seguir editando',
        }),
      );
    });
    expect(onClose).not.toHaveBeenCalled();
  });
});
