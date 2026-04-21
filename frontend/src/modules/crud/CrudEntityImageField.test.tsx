import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudEntityImageField } from './CrudEntityImageField';

describe('CrudEntityImageField', () => {
  it('removes the selected image and persists the remaining list', () => {
    const setValue = vi.fn();
    render(
      <CrudEntityImageField
        value={'https://example.com/a.jpg\nhttps://example.com/b.jpg'}
        setValue={setValue}
      />,
    );

    fireEvent.click(screen.getByLabelText('Eliminar imagen 1'));

    expect(setValue).toHaveBeenCalledWith('https://example.com/b.jpg');
  });

  it('hides the upload button in read only mode', () => {
    render(
      <CrudEntityImageField
        value={'https://example.com/a.jpg'}
        setValue={() => {}}
        readOnly
      />,
    );

    expect(screen.queryByText('Cargar imágenes')).not.toBeInTheDocument();
  });
});
