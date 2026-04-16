import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';
import { StockLevelDetailModal } from '../modules/inventory';

const apiMocks = vi.hoisted(() => ({
  apiRequest: vi.fn(),
}));

vi.mock('../lib/api', () => ({
  apiRequest: (...args: unknown[]) => apiMocks.apiRequest(...args),
}));

function buildLevel(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    product_id: 'prod-1',
    product_name: 'Aceite 15W40',
    sku: 'ACE-15',
    quantity: 8,
    min_quantity: 3,
    track_stock: true,
    is_low_stock: false,
    updated_at: '2026-04-10T10:00:00Z',
    ...overrides,
  };
}

function buildMovement(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    id: 'mov-1',
    product_id: 'prod-1',
    product_name: 'Aceite 15W40',
    type: 'in',
    quantity: 5,
    reason: 'Compra',
    notes: 'Reposición',
    created_by: 'seed',
    created_at: '2026-04-10T11:00:00Z',
    ...overrides,
  };
}

function renderModal() {
  return render(
    <MemoryRouter>
      <LanguageProvider initialLanguage="es">
        <StockLevelDetailModal productId="prod-1" onClose={() => {}} onAfterSave={() => {}} />
      </LanguageProvider>
    </MemoryRouter>,
  );
}

describe('StockLevelDetailModal', () => {
  beforeEach(() => {
    apiMocks.apiRequest.mockReset();
    apiMocks.apiRequest.mockImplementation((path: string) => {
      if (path === '/v1/inventory/prod-1') {
        return Promise.resolve(buildLevel());
      }
      if (path === '/v1/inventory/movements?limit=50&product_id=prod-1') {
        return Promise.resolve({ items: [buildMovement()] });
      }
      if (path === '/v1/products/prod-1') {
        return Promise.resolve({
          id: 'prod-1',
          name: 'Aceite 15W40',
          image_urls: ['https://example.com/photo-a.png', 'https://example.com/photo-b.png'],
        });
      }
      throw new Error(`unexpected path ${path}`);
    });
  });

  it('muestra resumen, movimientos y acciones (editar en el modal)', async () => {
    renderModal();

    expect(await screen.findByText('Aceite 15W40')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Aceite 15W40' })).toBeInTheDocument();
    expect(await screen.findByText('Entrada')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Guardar' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Editar' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Archivar producto/i })).toBeInTheDocument();
  });

  it('habilita Guardar cuando hay cambios y notas (tras activar edición)', async () => {
    renderModal();
    await screen.findByText('Aceite 15W40');

    fireEvent.click(screen.getByRole('button', { name: 'Editar' }));
    fireEvent.change(screen.getByLabelText(/Stock mínimo/i), { target: { value: '5' } });
    fireEvent.change(screen.getByLabelText(/Notas/i), { target: { value: 'Reposición en depósito' } });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Guardar' })).not.toBeDisabled();
    });
  });
});
