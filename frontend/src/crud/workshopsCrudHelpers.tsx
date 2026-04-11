import { formatCrudMoney } from '../modules/crud';

function normalizeWorkshopStatus(value: unknown): string {
  const raw = String(value ?? '');
  if (raw === 'diagnosis') return 'diagnosing';
  if (raw === 'ready') return 'ready_for_pickup';
  return raw;
}

export function renderWorkshopWorkOrderStatusBadge(value: unknown) {
  const status = normalizeWorkshopStatus(value);
  const success = status === 'ready_for_pickup' || status === 'delivered' || status === 'invoiced';
  const danger = status === 'cancelled';
  const className = success ? 'badge-success' : danger ? 'badge-danger' : 'badge-warning';
  return <span className={`badge ${className}`}>{status}</span>;
}

export function formatWorkshopMoney(value: unknown, currency?: string): string {
  return formatCrudMoney(value, currency);
}
