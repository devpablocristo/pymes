import type { ReactNode } from 'react';
import type { CrudResourceInventoryDetailStrings } from './crudResourceInventoryDetailContract';
import './crudInventoryQuantitiesNotesBlock.css';

export type CrudInventoryQuantitiesNotesBlockProps = {
  strings: CrudResourceInventoryDetailStrings;
  formatDateTime: (iso: string) => string;
  updatedAtIso: string;
  quantityInputId: string;
  quantityValue: string;
  onQuantityChange: (value: string) => void;
  quantityDisabled?: boolean;
  minInputId: string;
  minValue: string;
  onMinChange: (value: string) => void;
  minDisabled?: boolean;
  notesInputId: string;
  notesValue: string;
  onNotesChange: (value: string) => void;
  notesRequired: boolean;
};

function renderLastUpdatedLine(
  strings: CrudResourceInventoryDetailStrings,
  template: string | undefined,
  updatedAtIso: string,
  formatDateTime: (iso: string) => string,
): ReactNode {
  const formatted = formatDateTime(updatedAtIso);
  if (template?.includes('{datetime}')) {
    return (
      <span className="crud-inv-qty-notes__last-line">{template.split('{datetime}').join(formatted)}</span>
    );
  }
  return (
    <span className="crud-inv-qty-notes__last-line">
      {strings.lastUpdatedPrefix} <strong>{formatted}</strong>
    </span>
  );
}

/**
 * Bloque agnóstico: cantidad actual, mínimo, línea de última actualización (plantilla o prefijo + fecha),
 * y notas con obligatoriedad controlada por el padre (p. ej. solo si cambian cantidades).
 */
export function CrudInventoryQuantitiesNotesBlock({
  strings,
  formatDateTime,
  updatedAtIso,
  quantityInputId,
  quantityValue,
  onQuantityChange,
  quantityDisabled,
  minInputId,
  minValue,
  onMinChange,
  minDisabled,
  notesInputId,
  notesValue,
  onNotesChange,
  notesRequired,
}: CrudInventoryQuantitiesNotesBlockProps) {
  const helper = (strings.fieldNotesHelper ?? '').trim();

  return (
    <div className="crud-inv-qty-notes">
      <h5 className="crud-inv-qty-notes__title">{strings.inventoryQuantitiesSectionTitle}</h5>
      <div className="crud-inv-qty-notes__grid">
        <div className="crud-inv-detail-modal__field crud-inv-qty-notes__field">
          <label htmlFor={quantityInputId}>{strings.fieldQuantityLabel}</label>
          <input
            id={quantityInputId}
            type="number"
            step="any"
            value={quantityValue}
            disabled={quantityDisabled}
            onChange={(e) => onQuantityChange(e.target.value)}
          />
        </div>
        <div className="crud-inv-detail-modal__field crud-inv-qty-notes__field">
          <label htmlFor={minInputId}>{strings.fieldMinQuantityLabel}</label>
          <input
            id={minInputId}
            type="number"
            step="any"
            value={minValue}
            disabled={minDisabled}
            onChange={(e) => onMinChange(e.target.value)}
          />
        </div>
        <div className="crud-inv-detail-modal__field crud-inv-qty-notes__field crud-inv-qty-notes__field--full">
          <p className="crud-inv-qty-notes__hint text-secondary text-sm">
            {renderLastUpdatedLine(strings, strings.lastUpdatedEditHintTemplate, updatedAtIso, formatDateTime)}
          </p>
        </div>
        <div className="crud-inv-detail-modal__field crud-inv-qty-notes__field crud-inv-qty-notes__field--full">
          <label htmlFor={notesInputId}>
            {strings.fieldNotesLabel}
            {notesRequired ? <span className="crud-inv-qty-notes__req"> *</span> : null}
          </label>
          {helper ? <p className="crud-inv-qty-notes__helper text-secondary text-sm">{helper}</p> : null}
          <textarea
            id={notesInputId}
            value={notesValue}
            onChange={(e) => onNotesChange(e.target.value)}
            rows={3}
            aria-required={notesRequired}
            required={notesRequired}
          />
        </div>
      </div>
    </div>
  );
}
