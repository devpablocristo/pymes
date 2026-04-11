import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudInventoryQuantitiesNotesBlock } from './crudInventoryQuantitiesNotesBlock';
import type { CrudResourceInventoryDetailStrings } from './crudResourceInventoryDetailContract';

const baseStrings: CrudResourceInventoryDetailStrings = {
  dialogLoadingTitle: 'L',
  dialogFallbackTitle: 'F',
  loadErrorGeneric: 'E',
  sectionEditHeading: 'Editar',
  fieldDisplayNameLabel: 'Nombre',
  fieldSkuLabel: 'SKU',
  fieldImageUrlsLabel: 'Imgs',
  fieldImageUrlsHint: 'H',
  fieldTrackStockLabel: 'Track',
  fieldQuantityLabel: 'Cantidad',
  fieldMinQuantityLabel: 'Mínimo',
  fieldNotesLabel: 'Notas',
  fieldNotesHelper: 'Ayuda notas',
  inventoryQuantitiesSectionTitle: 'Cantidades y notas',
  lastUpdatedPrefix: 'Actualizado:',
  lastUpdatedEditHintTemplate: 'Servidor {datetime}',
  movementsHeading: 'Mov',
  movementsEmpty: '—',
  movementsLoading: '…',
  movementColumns: { kind: 'K', quantity: 'Q', reason: 'R', user: 'U', date: 'D' },
  badgeLowStock: 'bajo',
  readHintEdit: 'hint',
  statCurrentLabel: 'A',
  statMinLabel: 'M',
  statUpdatedLabel: 'U',
  loadingBodyLabel: 'load',
  galleryAriaLabel: 'g',
  openImageFullscreenLabel: 'o',
  closeLabel: 'c',
  editLabel: 'e',
  cancelEditLabel: 'x',
  saveLabel: 's',
  savingLabel: '…',
  notesRequiredError: 'n',
  nameRequiredError: 'nm',
  saveErrorGeneric: 'se',
};

describe('CrudInventoryQuantitiesNotesBlock', () => {
  it('renderiza título, cantidades y plantilla de última actualización', () => {
    const fmt = vi.fn(() => '10/04/26 12:00');
    render(
      <CrudInventoryQuantitiesNotesBlock
        strings={baseStrings}
        formatDateTime={fmt}
        updatedAtIso="2026-04-10T12:00:00Z"
        quantityInputId="q"
        quantityValue="5"
        onQuantityChange={() => undefined}
        minInputId="m"
        minValue="1"
        onMinChange={() => undefined}
        notesInputId="n"
        notesValue=""
        onNotesChange={() => undefined}
        notesRequired={false}
      />,
    );
    expect(screen.getByText('Cantidades y notas')).toBeInTheDocument();
    expect(screen.getByLabelText('Cantidad')).toHaveValue(5);
    expect(screen.getByLabelText('Mínimo')).toHaveValue(1);
    expect(screen.getByText('Servidor 10/04/26 12:00')).toBeInTheDocument();
    expect(screen.getByText('Ayuda notas')).toBeInTheDocument();
    expect(fmt).toHaveBeenCalledWith('2026-04-10T12:00:00Z');
  });
});
