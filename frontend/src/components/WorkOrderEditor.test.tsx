import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AutoRepairWorkOrder } from '../lib/autoRepairTypes';
import { WorkOrderEditor } from './WorkOrderEditor';

const apiMocks = vi.hoisted(() => ({
  getAutoRepairWorkOrder: vi.fn<[], Promise<AutoRepairWorkOrder>>(),
  updateAutoRepairWorkOrder: vi.fn(),
  archiveAutoRepairWorkOrder: vi.fn(),
  restoreAutoRepairWorkOrder: vi.fn(),
  confirmAction: vi.fn(),
}));

vi.mock('../lib/autoRepairApi', () => ({
  getAutoRepairWorkOrder: () => apiMocks.getAutoRepairWorkOrder(),
  updateAutoRepairWorkOrder: (...args: unknown[]) => apiMocks.updateAutoRepairWorkOrder(...args),
  archiveAutoRepairWorkOrder: (...args: unknown[]) => apiMocks.archiveAutoRepairWorkOrder(...args),
  restoreAutoRepairWorkOrder: (...args: unknown[]) => apiMocks.restoreAutoRepairWorkOrder(...args),
}));

vi.mock('@devpablocristo/core-browser', () => ({
  confirmAction: (options: unknown) => apiMocks.confirmAction(options),
}));

function buildWorkOrder(overrides?: Partial<AutoRepairWorkOrder>): AutoRepairWorkOrder {
  return {
    id: 'wo-1',
    org_id: 'org-1',
    number: 'OT-001',
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
    apiMocks.getAutoRepairWorkOrder.mockReset();
    apiMocks.updateAutoRepairWorkOrder.mockReset();
    apiMocks.archiveAutoRepairWorkOrder.mockReset();
    apiMocks.restoreAutoRepairWorkOrder.mockReset();
    apiMocks.confirmAction.mockReset();

    apiMocks.getAutoRepairWorkOrder.mockResolvedValue(buildWorkOrder());
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
