import type { PymesRoutingSource } from '../types/aiChat';

export function humanRoutedLabel(mode: string): string {
  if (mode === 'copilot') return 'Copilot';
  if (mode === 'clientes') return 'Clientes';
  if (mode === 'productos') return 'Productos';
  if (mode === 'ventas') return 'Ventas';
  if (mode === 'cobros') return 'Cobros';
  if (mode === 'compras') return 'Compras';
  if (mode === 'general') return 'General';
  if (mode === 'internal_procurement') return 'Compras internas';
  if (mode === 'internal_sales') return 'Ventas';
  return mode || 'General';
}

export function humanRoutingSourceLabel(source: PymesRoutingSource): string {
  if (source === 'copilot_agent') return 'Copilot';
  if (source === 'read_fallback') return 'Fallback lectura';
  return 'Orquestador';
}
