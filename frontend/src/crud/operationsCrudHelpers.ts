import { type CrudFormValues } from '../components/CrudPage';
import { formatCrudMoney, renderCrudActiveBadge } from '../modules/crud';
import { asString } from './resourceConfigs.shared';

export { formatCrudMoney as formatOperationsMoney, renderCrudActiveBadge as renderOperationsActiveBadge };

export function parseReturnSaleItemsJson(raw: string): Array<{ sale_item_id: string; quantity: number }> {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch {
    throw new Error('El campo «Ítems» debe ser JSON válido.');
  }
  if (!Array.isArray(parsed) || parsed.length === 0) {
    throw new Error('Ítems: se requiere un array con al menos un elemento.');
  }
  return parsed.map((entry) => {
    if (!entry || typeof entry !== 'object') {
      throw new Error('Cada ítem debe ser un objeto con sale_item_id y quantity.');
    }
    const record = entry as Record<string, unknown>;
    const sale_item_id = String(record.sale_item_id ?? '').trim();
    const quantity = Number(record.quantity);
    if (!sale_item_id || Number.isNaN(quantity) || quantity <= 0) {
      throw new Error('Cada ítem necesita sale_item_id (UUID) y quantity > 0.');
    }
    return { sale_item_id, quantity };
  });
}

export function isValidReturnRefundMethod(value: string): boolean {
  return ['cash', 'credit_note', 'original_method'].includes(value.trim().toLowerCase());
}

export function validateReturnForm(values: CrudFormValues): boolean {
  return (
    asString(values.sale_id).trim().length >= 32 &&
    isValidReturnRefundMethod(asString(values.refund_method)) &&
    asString(values.items_json).trim().length >= 2
  );
}
