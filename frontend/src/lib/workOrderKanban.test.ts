import { describe, it, expect } from 'vitest';
import {
  canonicalWorkOrderStatus,
  workOrderKanbanPhaseFromStatus,
  defaultCanonStatusForKanbanPhase,
  isWorkOrderKanbanTerminalStatus,
  workOrderStatusBadgeLabel,
  type WorkOrderKanbanPhase,
} from './workOrderKanban';

describe('canonicalWorkOrderStatus', () => {
  it.each([
    ['', 'received'],
    ['received', 'received'],
    ['diagnosis', 'diagnosing'],
    ['diagnosing', 'diagnosing'],
    ['quote_pending', 'quote_pending'],
    ['awaiting_parts', 'awaiting_parts'],
    ['in_progress', 'in_progress'],
    ['quality_check', 'quality_check'],
    ['ready', 'ready_for_pickup'],
    ['ready_for_pickup', 'ready_for_pickup'],
    ['delivered', 'delivered'],
    ['invoiced', 'invoiced'],
    ['cancelled', 'cancelled'],
    ['on_hold', 'on_hold'],
    ['RECEIVED', 'received'],
    ['  Diagnosis  ', 'diagnosing'],
    ['unknown_status', 'received'],
  ])('maps %j to %j', (input, expected) => {
    expect(canonicalWorkOrderStatus(input)).toBe(expected);
  });
});

describe('workOrderKanbanPhaseFromStatus', () => {
  it.each<[string, WorkOrderKanbanPhase]>([
    ['received', 'wo_intake'],
    ['diagnosing', 'wo_intake'],
    ['quote_pending', 'wo_quote'],
    ['awaiting_parts', 'wo_quote'],
    ['in_progress', 'wo_shop'],
    ['quality_check', 'wo_shop'],
    ['on_hold', 'wo_shop'],
    ['ready_for_pickup', 'wo_exit'],
    ['delivered', 'wo_exit'],
    ['invoiced', 'wo_closed'],
    ['cancelled', 'wo_closed'],
  ])('maps %j to phase %j', (status, phase) => {
    expect(workOrderKanbanPhaseFromStatus(status)).toBe(phase);
  });

  it('defaults unknown to wo_intake', () => {
    expect(workOrderKanbanPhaseFromStatus('xyz')).toBe('wo_intake');
  });
});

describe('defaultCanonStatusForKanbanPhase', () => {
  it.each<[WorkOrderKanbanPhase, string | null]>([
    ['wo_intake', 'received'],
    ['wo_quote', 'quote_pending'],
    ['wo_shop', 'in_progress'],
    ['wo_exit', 'ready_for_pickup'],
    ['wo_closed', null],
  ])('phase %j defaults to %j', (phase, status) => {
    expect(defaultCanonStatusForKanbanPhase(phase)).toBe(status);
  });
});

describe('isWorkOrderKanbanTerminalStatus', () => {
  it('returns true for invoiced and cancelled', () => {
    expect(isWorkOrderKanbanTerminalStatus('invoiced')).toBe(true);
    expect(isWorkOrderKanbanTerminalStatus('cancelled')).toBe(true);
  });

  it('returns false for non-terminal statuses', () => {
    expect(isWorkOrderKanbanTerminalStatus('received')).toBe(false);
    expect(isWorkOrderKanbanTerminalStatus('in_progress')).toBe(false);
    expect(isWorkOrderKanbanTerminalStatus('ready_for_pickup')).toBe(false);
  });
});

describe('workOrderStatusBadgeLabel', () => {
  it.each([
    ['received', 'Recibido'],
    ['diagnosing', 'Diagn\u00f3stico'],
    ['quote_pending', 'Presupuesto'],
    ['awaiting_parts', 'Repuestos'],
    ['in_progress', 'En taller'],
    ['quality_check', 'Control'],
    ['ready_for_pickup', 'Listo retiro'],
    ['delivered', 'Entregado'],
    ['on_hold', 'En pausa'],
    ['invoiced', 'Facturado'],
    ['cancelled', 'Cancelado'],
  ])('maps %j to label %j', (status, label) => {
    expect(workOrderStatusBadgeLabel(status)).toBe(label);
  });

  it('handles alias inputs', () => {
    expect(workOrderStatusBadgeLabel('diagnosis')).toBe('Diagn\u00f3stico');
    expect(workOrderStatusBadgeLabel('ready')).toBe('Listo retiro');
  });
});
