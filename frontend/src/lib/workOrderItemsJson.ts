import type { WorkOrderLineItem as AutoRepairWorkOrderItem } from './workOrdersApi';

/** Parsea el textarea de ítems (mismo contrato que el CRUD legacy). */
export function parseWorkOrderItemsJson(text: string): AutoRepairWorkOrderItem[] {
  const t = text.trim();
  if (!t) return [];
  let raw: unknown;
  try {
    raw = JSON.parse(t) as unknown;
  } catch {
    throw new Error('Ítems: JSON inválido');
  }
  if (!Array.isArray(raw)) {
    throw new Error('Los ítems deben ser un arreglo JSON');
  }
  return raw
    .map((item, index) => {
      const rec = item as Record<string, unknown>;
      return {
        item_type: rec.item_type === 'part' ? ('part' as const) : ('service' as const),
        service_id: typeof rec.service_id === 'string' ? rec.service_id : undefined,
        product_id: typeof rec.product_id === 'string' ? rec.product_id : undefined,
        description: String(rec.description ?? '').trim(),
        quantity: Number(rec.quantity ?? 0),
        unit_price: Number(rec.unit_price ?? 0),
        tax_rate: rec.tax_rate === undefined || rec.tax_rate === null ? 21 : Number(rec.tax_rate),
        sort_order: Number(rec.sort_order ?? index),
        metadata:
          rec.metadata && typeof rec.metadata === 'object' && !Array.isArray(rec.metadata)
            ? (rec.metadata as Record<string, unknown>)
            : {},
      };
    })
    .filter((item) => item.description.length > 0 && item.quantity > 0);
}

export function stringifyWorkOrderItems(items: AutoRepairWorkOrderItem[] | undefined): string {
  if (!items?.length) return '[]';
  return JSON.stringify(items, null, 2);
}
