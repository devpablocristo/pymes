import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import {
  CrudLinkedEntityEditBodyFields,
  crudLinkedEntityHasDisplayName,
} from './crudLinkedEntityInventoryFormBlock';

const strings = {
  fieldDisplayNameLabel: 'Nombre',
  fieldSkuLabel: 'SKU',
  fieldImageUrlsLabel: 'Imagenes',
  fieldImageUrlsHint: 'Subi una o varias imagenes',
  fieldImageUploadActionLabel: 'Subir imagenes',
  fieldImageUploadingLabel: 'Subiendo imagenes',
  fieldImageRemoveLabel: 'Quitar',
  fieldTrackStockLabel: 'Controlar stock',
  galleryAriaLabel: 'Galeria',
  openImageFullscreenLabel: 'Ver grande',
};

describe('crudLinkedEntityHasDisplayName', () => {
  it('rechaza vacío y solo espacios', () => {
    expect(crudLinkedEntityHasDisplayName('')).toBe(false);
    expect(crudLinkedEntityHasDisplayName('   ')).toBe(false);
  });

  it('acepta texto recortable', () => {
    expect(crudLinkedEntityHasDisplayName('  x  ')).toBe(true);
  });
});

describe('CrudLinkedEntityEditBodyFields', () => {
  it('sube multiples archivos usando el puerto inyectado', async () => {
    const onUploadImages = vi.fn(async () => undefined);
    render(
      <CrudLinkedEntityEditBodyFields
        strings={strings}
        imageUrlsInputId="images"
        imageUrls={[]}
        onImageUrlsChange={() => undefined}
        onImageUrlsInput={() => undefined}
        onUploadImages={onUploadImages}
        trackStockInputId="track"
        trackStock
        onTrackStockChange={() => undefined}
        showTrackStock
        onOpenPreviewImage={() => undefined}
        galleryRootClassName="gallery"
        galleryItemClassName="gallery-item"
        galleryZoomClassName="gallery-zoom"
      />,
    );

    const input = screen.getByLabelText('Subir imagenes') as HTMLInputElement;
    const files = [
      new File(['a'], 'a.png', { type: 'image/png' }),
      new File(['b'], 'b.jpg', { type: 'image/jpeg' }),
    ];

    fireEvent.change(input, { target: { files } });

    await waitFor(() => {
      expect(onUploadImages).toHaveBeenCalledWith(files);
    });
  });

  it('permite quitar una imagen del arreglo actual', () => {
    const onImageUrlsChange = vi.fn();
    render(
      <CrudLinkedEntityEditBodyFields
        strings={strings}
        imageUrlsInputId="images"
        imageUrls={['https://cdn.example/a.png', 'https://cdn.example/b.png']}
        onImageUrlsChange={onImageUrlsChange}
        onImageUrlsInput={() => undefined}
        trackStockInputId="track"
        trackStock
        onTrackStockChange={() => undefined}
        showTrackStock
        onOpenPreviewImage={() => undefined}
        galleryRootClassName="gallery"
        galleryItemClassName="gallery-item"
        galleryZoomClassName="gallery-zoom"
      />,
    );

    fireEvent.click(screen.getAllByRole('button', { name: 'Quitar' })[0]);

    expect(onImageUrlsChange).toHaveBeenCalledWith(['https://cdn.example/b.png']);
  });
});
