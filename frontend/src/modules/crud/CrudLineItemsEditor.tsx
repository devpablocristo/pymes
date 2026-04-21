import { useMemo } from 'react';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { asString, parseJSONArray } from '../../crud/resourceConfigs.shared';
import './CrudLineItemsEditor.css';

type CrudLineItemDraft = {
  description: string;
  quantity: number;
  unit_amount: number;
  product_id?: string;
  service_id?: string;
  tax_rate?: number;
};

function normalizeLineItemSource(value: CrudFieldValue | undefined): Record<string, unknown>[] {
  if (Array.isArray(value)) {
    return value.filter((item): item is Record<string, unknown> => typeof item === 'object' && item !== null);
  }
  try {
    return parseJSONArray<Record<string, unknown>>(value, 'Los items deben ser un arreglo JSON');
  } catch {
    return [];
  }
}

function normalizeItems(value: CrudFieldValue | undefined): CrudLineItemDraft[] {
  return normalizeLineItemSource(value).map((item) => ({
    description: String(item.description ?? '').trim(),
    quantity: Number(item.quantity ?? item.qty ?? 1),
    unit_amount: Number(item.unit_price ?? item.unit_cost ?? item.unitPrice ?? 0),
    product_id: typeof item.product_id === 'string' ? item.product_id : undefined,
    service_id: typeof item.service_id === 'string' ? item.service_id : undefined,
    tax_rate: typeof item.tax_rate === 'number' ? item.tax_rate : undefined,
  }));
}

function toStoredValue(items: CrudLineItemDraft[]) {
  return JSON.stringify(
    items.map((item) => ({
      description: item.description.trim(),
      quantity: Number.isFinite(item.quantity) && item.quantity > 0 ? item.quantity : 1,
      unit_cost: Number.isFinite(item.unit_amount) && item.unit_amount >= 0 ? item.unit_amount : 0,
      unit_price: Number.isFinite(item.unit_amount) && item.unit_amount >= 0 ? item.unit_amount : 0,
      product_id: item.product_id || undefined,
      service_id: item.service_id || undefined,
      tax_rate: item.tax_rate,
    })),
  );
}

export function CrudLineItemsEditor({
  value,
  onChange,
}: {
  value: CrudFieldValue | undefined;
  onChange: (nextValue: string) => void;
}) {
  const items = useMemo(() => {
    const parsed = normalizeItems(value);
    return parsed.length > 0 ? parsed : [{ description: '', quantity: 1, unit_amount: 0 }];
  }, [value]);

  const updateItems = (nextItems: CrudLineItemDraft[]) => {
    onChange(toStoredValue(nextItems));
  };

  const setItem = (index: number, patch: Partial<CrudLineItemDraft>) => {
    updateItems(items.map((item, currentIndex) => (currentIndex === index ? { ...item, ...patch } : item)));
  };

  return (
    <div className="crud-line-items-editor">
      {items.map((item, index) => (
        <div key={index} className="crud-line-items-editor__row">
          <div className="crud-line-items-editor__field crud-entity-editor-modal__field crud-line-items-editor__field--full crud-entity-editor-modal__field--full">
            <span>Concepto</span>
            <input
              type="text"
              value={item.description}
              onChange={(event) => setItem(index, { description: event.target.value })}
              placeholder="Qué se compró"
            />
          </div>
          <div className="crud-line-items-editor__field crud-entity-editor-modal__field">
            <span>Cantidad</span>
            <input
              type="number"
              min="1"
              step="any"
              value={Number.isFinite(item.quantity) ? item.quantity : 1}
              onChange={(event) => setItem(index, { quantity: Number(asString(event.target.value)) || 1 })}
            />
          </div>
          <div className="crud-line-items-editor__field crud-entity-editor-modal__field">
            <span>Importe unitario</span>
            <input
              type="number"
              min="0"
              step="any"
              value={Number.isFinite(item.unit_amount) ? item.unit_amount : 0}
              onChange={(event) => setItem(index, { unit_amount: Number(asString(event.target.value)) || 0 })}
            />
          </div>
          <div className="crud-line-items-editor__actions">
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() =>
                updateItems(items.length === 1 ? [{ description: '', quantity: 1, unit_amount: 0 }] : items.filter((_, i) => i !== index))
              }
            >
              Quitar
            </button>
          </div>
        </div>
      ))}
      <div className="crud-line-items-editor__footer">
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => updateItems([...items, { description: '', quantity: 1, unit_amount: 0 }])}
        >
          Añadir renglón
        </button>
      </div>
    </div>
  );
}
