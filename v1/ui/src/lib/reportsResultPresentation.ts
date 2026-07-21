import type { LanguageCode } from './i18n';
import { formatDashboardMoney, localeForLanguage } from '../dashboard/utils/format';

export function isReportDatasetPath(path: string): boolean {
  return path.includes('/v1/reports');
}

function isScalarCell(value: unknown): boolean {
  if (value === null || value === undefined) {
    return true;
  }
  const t = typeof value;
  return t === 'string' || t === 'number' || t === 'boolean';
}

/** Misma lógica que el explorador de módulo: filas para tabla o null si no aplica. */
export function extractTabularRows(data: unknown): Array<Record<string, unknown>> | null {
  if (Array.isArray(data) && data.every((item) => item && typeof item === 'object')) {
    return data as Array<Record<string, unknown>>;
  }
  if (data && typeof data === 'object' && 'items' in data) {
    const raw = (data as { items: unknown }).items;
    if (raw == null) {
      return [];
    }
    if (!Array.isArray(raw)) {
      return null;
    }
    if (raw.length === 0) {
      return [];
    }
    if (raw.every((item) => item && typeof item === 'object')) {
      return raw as Array<Record<string, unknown>>;
    }
  }
  if (data && typeof data === 'object' && 'data' in data) {
    const inner = (data as { data: unknown }).data;
    if (Array.isArray(inner)) {
      if (inner.length === 0) {
        return [];
      }
      if (inner.every((item) => item && typeof item === 'object')) {
        return inner as Array<Record<string, unknown>>;
      }
      return null;
    }
    if (inner && typeof inner === 'object' && !Array.isArray(inner)) {
      const row = inner as Record<string, unknown>;
      const scalarRow = Object.fromEntries(Object.entries(row).filter(([, v]) => isScalarCell(v)));
      if (Object.keys(scalarRow).length > 0) {
        return [scalarRow];
      }
    }
  }
  return null;
}

const MONEY_KEYS = new Set([
  'revenue',
  'total',
  'total_sales',
  'average_ticket',
  'valuation',
  'cost_price',
  'total_income',
  'total_expense',
  'balance',
  'gross_profit',
  'cost',
]);

const PCT_KEYS = new Set(['margin_pct']);

const INT_KEYS = new Set(['count', 'count_sales']);

const LABELS_ES: Record<string, string> = {
  product_id: 'ID producto',
  product_name: 'Producto',
  service_id: 'ID servicio',
  service_name: 'Servicio',
  customer_id: 'ID cliente',
  customer_name: 'Cliente',
  payment_method: 'Medio de pago',
  quantity: 'Cantidad',
  revenue: 'Ingresos',
  total: 'Total',
  count: 'Operaciones',
  count_sales: 'Ventas',
  total_sales: 'Ventas totales',
  average_ticket: 'Ticket promedio',
  valuation: 'Valuación',
  cost_price: 'Costo unitario',
  min_quantity: 'Stock mínimo',
  sku: 'SKU',
  total_income: 'Ingresos (caja)',
  total_expense: 'Egresos (caja)',
  balance: 'Saldo',
  gross_profit: 'Margen bruto',
  margin_pct: 'Margen %',
  cost: 'Costo',
};

const LABELS_EN: Record<string, string> = {
  product_id: 'Product ID',
  product_name: 'Product',
  service_id: 'Service ID',
  service_name: 'Service',
  customer_id: 'Customer ID',
  customer_name: 'Customer',
  payment_method: 'Payment method',
  quantity: 'Qty',
  revenue: 'Revenue',
  total: 'Total',
  count: 'Count',
  count_sales: 'Sales count',
  total_sales: 'Total sales',
  average_ticket: 'Avg. ticket',
  valuation: 'Valuation',
  cost_price: 'Unit cost',
  min_quantity: 'Min stock',
  sku: 'SKU',
  total_income: 'Cash in',
  total_expense: 'Cash out',
  balance: 'Balance',
  gross_profit: 'Gross profit',
  margin_pct: 'Margin %',
  cost: 'Cost',
};

export function reportColumnLabel(key: string, language: LanguageCode): string {
  const map = language === 'en' ? LABELS_EN : LABELS_ES;
  return map[key] ?? key.replace(/_/g, ' ');
}

export function formatReportMoneyDetail(value: number, language: LanguageCode): string {
  if (Number.isNaN(value)) {
    return '—';
  }
  const locale = localeForLanguage(language);
  return `$${value.toLocaleString(locale, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

export function formatReportCell(key: string, value: unknown, language: LanguageCode): string {
  if (value === null || value === undefined) {
    return '—';
  }
  if (typeof value === 'boolean') {
    if (language === 'en') {
      return value ? 'Yes' : 'No';
    }
    return value ? 'Sí' : 'No';
  }
  if (typeof value === 'number') {
    if (PCT_KEYS.has(key)) {
      return `${value.toLocaleString(localeForLanguage(language), { maximumFractionDigits: 2 })} %`;
    }
    if (INT_KEYS.has(key)) {
      return String(Math.round(value));
    }
    if (MONEY_KEYS.has(key)) {
      return formatReportMoneyDetail(value, language);
    }
    return value.toLocaleString(localeForLanguage(language), { maximumFractionDigits: 2 });
  }
  if (typeof value === 'string') {
    return value || '—';
  }
  return String(value);
}

/** Orden de columnas legible por tipo de reporte (resto al final alfabético). */
export function orderedReportColumns(datasetPath: string, keys: string[]): string[] {
  const orderByPath: Record<string, string[]> = {
    'sales-by-product': ['product_name', 'sku', 'quantity', 'revenue', 'product_id'],
    'sales-by-service': ['service_name', 'quantity', 'revenue', 'service_id'],
    'sales-by-customer': ['customer_name', 'total', 'count', 'customer_id'],
    'sales-by-payment': ['payment_method', 'total', 'count'],
    'inventory-valuation': ['product_name', 'sku', 'quantity', 'cost_price', 'valuation', 'product_id'],
    'low-stock': ['product_name', 'sku', 'quantity', 'min_quantity', 'product_id'],
  };
  let preferred: string[] | undefined;
  for (const [slug, cols] of Object.entries(orderByPath)) {
    if (datasetPath.includes(slug)) {
      preferred = cols;
      break;
    }
  }
  if (!preferred) {
    return [...keys].sort((a, b) => a.localeCompare(b));
  }
  const set = new Set(keys);
  const ordered: string[] = [];
  for (const c of preferred) {
    if (set.has(c)) {
      ordered.push(c);
    }
  }
  const rest = keys.filter((k) => !ordered.includes(k)).sort((a, b) => a.localeCompare(b));
  return [...ordered, ...rest];
}

export function reportBarMetricKey(datasetPath: string): string | null {
  if (datasetPath.includes('sales-by-product') || datasetPath.includes('sales-by-service')) {
    return 'revenue';
  }
  if (datasetPath.includes('sales-by-customer') || datasetPath.includes('sales-by-payment')) {
    return 'total';
  }
  if (datasetPath.includes('inventory-valuation')) {
    return 'valuation';
  }
  return null;
}

export function tableScalarColumnsForRows(rows: Array<Record<string, unknown>>, maxCols: number): string[] {
  const allKeys = Array.from(new Set(rows.flatMap((row) => Object.keys(row))));
  const scalarKeys = allKeys.filter((key) => rows.every((row) => isScalarCell(row[key])));
  const ordered = scalarKeys.length > 0 ? scalarKeys : allKeys;
  const priority = (k: string): number => {
    if (k === 'id') {
      return 0;
    }
    if (k.endsWith('_id') || k.endsWith('Id')) {
      return 1;
    }
    return 2;
  };
  ordered.sort((a, b) => {
    const pa = priority(a);
    const pb = priority(b);
    if (pa !== pb) {
      return pa - pb;
    }
    return a.localeCompare(b);
  });
  return ordered.slice(0, maxCols);
}

export function numericMetricMax(rows: Array<Record<string, unknown>>, key: string): number {
  let max = 0;
  for (const row of rows) {
    const v = row[key];
    if (typeof v === 'number' && !Number.isNaN(v) && v > max) {
      max = v;
    }
  }
  return max;
}

export function isKpiEnvelopePath(datasetPath: string): boolean {
  return (
    datasetPath.includes('sales-summary') ||
    datasetPath.includes('cashflow-summary') ||
    datasetPath.includes('profit-margin')
  );
}

export function readReportPeriod(data: unknown): { from?: string; to?: string } {
  if (!data || typeof data !== 'object') {
    return {};
  }
  const o = data as Record<string, unknown>;
  const from = typeof o.from === 'string' ? o.from : undefined;
  const to = typeof o.to === 'string' ? o.to : undefined;
  return { from, to };
}

export function formatKpiValue(key: string, value: unknown, language: LanguageCode): string {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return '—';
  }
  if (PCT_KEYS.has(key)) {
    return `${value.toLocaleString(localeForLanguage(language), { maximumFractionDigits: 2 })} %`;
  }
  if (INT_KEYS.has(key)) {
    return String(Math.round(value));
  }
  if (MONEY_KEYS.has(key)) {
    return formatDashboardMoney(value, language);
  }
  return value.toLocaleString(localeForLanguage(language));
}
