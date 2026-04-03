import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { ConfirmDialogProvider, confirmAction } from '@devpablocristo/core-browser';
import { describe, expect, it } from 'vitest';

function ConfirmDialogFixture() {
  return (
    <button
      type="button"
      onClick={() => {
        void confirmAction({
          title: 'Eliminar registro',
          description: '¿Querés continuar con esta acción?',
          confirmLabel: 'Eliminar',
          cancelLabel: 'Cancelar',
          tone: 'danger',
        });
      }}
    >
      Abrir confirmación
    </button>
  );
}

describe('ConfirmDialogProvider', () => {
  it('muestra el popup compartido y confirma la acción', async () => {
    let resolvedValue: boolean | null = null;

    render(
      <ConfirmDialogProvider>
        <button
          type="button"
          onClick={() => {
            void confirmAction({
              title: 'Eliminar registro',
              description: '¿Querés continuar con esta acción?',
              confirmLabel: 'Eliminar',
              cancelLabel: 'Cancelar',
              tone: 'danger',
            }).then((value) => {
              resolvedValue = value;
            });
          }}
        >
          Abrir confirmación
        </button>
      </ConfirmDialogProvider>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Abrir confirmación' }));

    expect(screen.getByRole('alertdialog', { name: 'Eliminar registro' })).toBeInTheDocument();
    expect(screen.getByText('¿Querés continuar con esta acción?')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Eliminar' }));

    await waitFor(() => {
      expect(resolvedValue).toBe(true);
    });
  });

  it('cancela con Escape usando el mismo popup compartido', async () => {
    let resolvedValue: boolean | null = null;

    render(
      <ConfirmDialogProvider>
        <ConfirmDialogFixture />
        <button
          type="button"
          onClick={() => {
            void confirmAction({
              title: 'Archivar turno',
              description: '¿Querés cancelar la edición?',
              confirmLabel: 'Archivar',
              cancelLabel: 'Seguir editando',
            }).then((value) => {
              resolvedValue = value;
            });
          }}
        >
          Abrir con escape
        </button>
      </ConfirmDialogProvider>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Abrir con escape' }));
    expect(screen.getByRole('alertdialog', { name: 'Archivar turno' })).toBeInTheDocument();

    fireEvent.keyDown(window, { key: 'Escape' });

    await waitFor(() => {
      expect(resolvedValue).toBe(false);
    });
  });
});
