import type { CrudFieldValue } from '../components/CrudPage';
import {
  formatCrudMoney,
  formatCrudPercent,
  renderCrudActiveBadge,
  renderCrudBooleanBadge,
} from '../modules/crud';
import { asOptionalString, parseJSONArray } from './resourceConfigs.shared';

export type CrudAddress = {
  street?: string;
  city?: string;
  state?: string;
  zip_code?: string;
  country?: string;
};

export function parseTagCsv(value: CrudFieldValue | undefined): string[] {
  return String(value ?? '')
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

export function formatTagList(tags?: string[]): string {
  return (tags ?? []).join(', ');
}

export function formatCrudAddress(address?: CrudAddress): string {
  return [address?.street, address?.city, address?.state, address?.country].filter(Boolean).join(', ') || '---';
}

export { formatCrudMoney, formatCrudPercent, renderCrudActiveBadge, renderCrudBooleanBadge };

export function parsePricedCrudLineItems(value: CrudFieldValue | undefined): Array<{
  product_id?: string;
  service_id?: string;
  description: string;
  quantity: number;
  unit_price: number;
  tax_rate?: number;
  sort_order: number;
}> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON');
  return parsed
    .map((item, index) => ({
      product_id: asOptionalString(item.product_id as CrudFieldValue),
      service_id: asOptionalString(item.service_id as CrudFieldValue),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_price: Number(item.unit_price ?? 0),
      tax_rate: item.tax_rate === undefined || item.tax_rate === null ? undefined : Number(item.tax_rate),
      sort_order: Number(item.sort_order ?? index),
    }))
    .filter((item) => item.description && item.quantity > 0);
}

export function parseCostCrudLineItems(value: CrudFieldValue | undefined): Array<{
  product_id?: string;
  service_id?: string;
  description: string;
  quantity: number;
  unit_cost: number;
  tax_rate?: number;
}> {
  const parsed = parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON');
  return parsed
    .map((item) => ({
      product_id: asOptionalString(item.product_id as CrudFieldValue),
      service_id: asOptionalString(item.service_id as CrudFieldValue),
      description: String(item.description ?? '').trim(),
      quantity: Number(item.quantity ?? 0),
      unit_cost: Number(item.unit_cost ?? 0),
      tax_rate: item.tax_rate === undefined || item.tax_rate === null ? undefined : Number(item.tax_rate),
    }))
    .filter((item) => item.description && item.quantity > 0);
}
