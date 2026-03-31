import type { PymesRoutingSource } from '../types/aiChat';
import type { LanguageCode } from './i18n';

function pick(language: LanguageCode, es: string, en: string): string {
  return language === 'en' ? en : es;
}

export function humanRoutedLabel(mode: string, language: LanguageCode = 'es'): string {
  if (mode === 'copilot') return 'Copilot';
  if (mode === 'clientes') return pick(language, 'Clientes', 'Customers');
  if (mode === 'productos') return pick(language, 'Productos', 'Products');
  if (mode === 'ventas') return pick(language, 'Ventas', 'Sales');
  if (mode === 'cobros') return pick(language, 'Cobros', 'Collections');
  if (mode === 'compras') return pick(language, 'Compras', 'Purchases');
  if (mode === 'general') return pick(language, 'General', 'General');
  if (mode === 'internal_procurement') return pick(language, 'Compras internas', 'Internal procurement');
  if (mode === 'internal_sales') return pick(language, 'Ventas', 'Sales');
  return mode || pick(language, 'General', 'General');
}

export function humanInsightScopeLabel(scope: string, language: LanguageCode = 'es'): string {
  if (scope === 'sales_collections') return pick(language, 'Ventas y cobranzas', 'Sales and collections');
  if (scope === 'inventory_profit') return pick(language, 'Inventario y rentabilidad', 'Inventory and profitability');
  if (scope === 'customers_retention') return pick(language, 'Clientes y retención', 'Customers and retention');
  return scope;
}

export function humanRoutingSourceLabel(source: PymesRoutingSource, language: LanguageCode = 'es'): string {
  if (source === 'copilot_agent') return 'Copilot';
  if (source === 'read_fallback') return pick(language, 'Fallback lectura', 'Read fallback');
  if (source === 'ui_hint') return pick(language, 'Selección manual', 'Manual selection');
  return pick(language, 'Orquestador', 'Orchestrator');
}
