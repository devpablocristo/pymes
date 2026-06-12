import { describe, expect, it } from 'vitest';
import type { CrudFormField, CrudPageConfig } from '../components/CrudPage';
import { applyStandardCrudAnnotations, normalizeStandardCrudFormFieldOrder } from './standardCrudAnnotations';

function buildCatalogLikeConfig(formFields: CrudFormField[]): CrudPageConfig<{ id: string; name?: string }> {
  return {
    basePath: '/v1/catalog-like',
    label: 'item',
    labelPlural: 'items',
    labelPluralCap: 'Items',
    columns: [{ key: 'name', header: 'Nombre' }],
    formFields,
    searchText: (row) => row.name ?? '',
    toFormValues: (row) => ({ name: row.name ?? '' }),
    isValid: () => true,
    toBody: (values) => ({ name: values.name }),
  };
}

describe('normalizeStandardCrudFormFieldOrder', () => {
  it('coloca favorito y etiquetas en la primera fila lógica e imágenes a ancho completo antes del resto', () => {
    const fields: CrudFormField[] = [
      { key: 'party_id', label: 'Cliente' },
      { key: 'image_urls', label: 'Imágenes', type: 'textarea' },
      { key: 'tags', label: 'Etiquetas Internas' },
      { key: 'metadata_favorite', label: 'Favorito', type: 'checkbox' },
    ];
    expect(normalizeStandardCrudFormFieldOrder(fields).map((f) => f.key)).toEqual([
      'metadata_favorite',
      'tags',
      'image_urls',
      'party_id',
    ]);
  });

  it('prefiere metadata_favorite sobre is_favorite cuando ambos existieran (caso anómalo)', () => {
    const fields: CrudFormField[] = [
      { key: 'is_favorite', label: 'Favorito API', type: 'checkbox' },
      { key: 'metadata_favorite', label: 'Favorito', type: 'checkbox' },
      { key: 'name', label: 'Nombre' },
    ];
    expect(normalizeStandardCrudFormFieldOrder(fields).map((f) => f.key)).toEqual([
      'metadata_favorite',
      'is_favorite',
      'name',
    ]);
  });

  it('acepta images o image_url como bloque de medios estándar', () => {
    const withImages: CrudFormField[] = [
      { key: 'title', label: 'Título' },
      { key: 'images', label: 'Imágenes' },
    ];
    expect(normalizeStandardCrudFormFieldOrder(withImages).map((f) => f.key)).toEqual(['images', 'title']);

    const withSingle: CrudFormField[] = [
      { key: 'image_url', label: 'Imagen' },
      { key: 'code', label: 'Código' },
    ];
    expect(normalizeStandardCrudFormFieldOrder(withSingle).map((f) => f.key)).toEqual(['image_url', 'code']);
  });

  it('fuerza fullWidth en campos de imagen estándar si faltaba', () => {
    const fields: CrudFormField[] = [{ key: 'image_urls', label: 'Imágenes', type: 'textarea' }];
    const next = normalizeStandardCrudFormFieldOrder(fields);
    expect(next[0].fullWidth).toBe(true);
  });
});

describe('applyStandardCrudAnnotations', () => {
  it('asigna editorModal.mediaFieldKey cuando image_urls ya está declarado y ordena el bloque estándar', () => {
    const cfg = applyStandardCrudAnnotations(
      'quotes-like',
      buildCatalogLikeConfig([
        { key: 'party_id', label: 'Cliente' },
        { key: 'image_urls', label: 'Imágenes', type: 'textarea' },
        { key: 'tags', label: 'Etiquetas Internas' },
        { key: 'metadata_favorite', label: 'Favorito', type: 'checkbox' },
      ]),
    );

    expect(cfg.editorModal?.mediaFieldKey).toBe('image_urls');
    expect(cfg.formFields?.map((f) => f.key)).toEqual([
      'metadata_favorite',
      'tags',
      'image_urls',
      'party_id',
    ]);
  });

  it('inyecta favorito, etiquetas e imágenes en orden canónico cuando el recurso no los declara', () => {
    const cfg = applyStandardCrudAnnotations(
      'widgets',
      buildCatalogLikeConfig([{ key: 'sku', label: 'SKU' }]),
    );

    expect(cfg.formFields?.map((f) => f.key)).toEqual([
      'metadata_favorite',
      'tags',
      'image_urls',
      'sku',
    ]);
    expect(cfg.editorModal?.mediaFieldKey).toBe('image_urls');
    expect(cfg.editorModal?.fieldConfig?.image_urls).toBeDefined();
  });

  it('solo reordena formFields cuando no hay inyección ni cableado de modal (caso catálogo ya completo)', () => {
    const cfg = applyStandardCrudAnnotations('complete-like', {
      ...buildCatalogLikeConfig([
        { key: 'name', label: 'Nombre' },
        { key: 'tags', label: 'Etiquetas Internas' },
        { key: 'metadata_favorite', label: 'Favorito', type: 'checkbox' },
      ]),
      featureFlags: { standardMedia: false },
      editorModal: { mediaFieldKey: 'images' },
      formFields: [
        { key: 'name', label: 'Nombre' },
        { key: 'tags', label: 'Etiquetas Internas' },
        { key: 'metadata_favorite', label: 'Favorito', type: 'checkbox' },
      ],
    });

    expect(cfg.formFields?.map((f) => f.key)).toEqual(['metadata_favorite', 'tags', 'name']);
  });
});
