import { describe, expect, it } from 'vitest';
import type { CrudPageConfig, CrudResourceConfigMap } from '../components/CrudPage';
import { getCrudPageConfigFromMap } from './resourceConfigs.runtime';

function buildBaseConfig(): CrudPageConfig<{ id: string; category?: string }> {
  return {
    basePath: '/v1/demo',
    label: 'demo',
    labelPlural: 'demos',
    labelPluralCap: 'Demos',
    columns: [{ key: 'category', header: 'Categoría' }],
    formFields: [{ key: 'category', label: 'Categoría' }],
    searchText: (row) => row.category ?? '',
    toFormValues: (row) => ({
      category: row.category ?? '',
    }),
    isValid: () => true,
    toBody: (values) => ({
      category: values.category,
    }),
  };
}

describe('resourceConfigs.runtime', () => {
  it('preserva los formFields explícitos sin inyectar campos internos', () => {
    const map: CrudResourceConfigMap = {
      demo: buildBaseConfig(),
    };

    const config = getCrudPageConfigFromMap(map, 'demo');
    expect(config).not.toBeNull();
    if (!config) return;

    expect(config.formFields.map((field) => field.key)).toEqual(['category']);
  });

  it('mantiene el contrato explícito de toFormValues y toBody', () => {
    const map: CrudResourceConfigMap = {
      demo: buildBaseConfig(),
    };

    const config = getCrudPageConfigFromMap(map, 'demo');
    expect(config).not.toBeNull();
    if (!config || !config.toBody) return;

    expect(config.toFormValues({ id: '1', category: 'herramientas' } as { id: string })).toEqual({
      category: 'herramientas',
    });
    expect(config.toBody({ category: 'insumos' })).toEqual({
      category: 'insumos',
    });
  });

  it('normaliza la config con defaults explícitos y homogéneos', () => {
    const map: CrudResourceConfigMap = {
      demo: buildBaseConfig(),
    };

    const config = getCrudPageConfigFromMap(map, 'demo');
    expect(config).not.toBeNull();
    if (!config) return;

    expect(config.supportsArchived).toBe(false);
    expect(config.allowRestore).toBe(false);
    expect(config.allowHardDelete).toBe(false);
    expect(config.allowCreate).toBe(true);
    expect(config.allowEdit).toBe(true);
    expect(config.allowDelete).toBe(false);
    expect(config.featureFlags).toMatchObject({
      searchBar: true,
      creatorFilter: true,
      valueFilter: true,
      archivedToggle: true,
      createAction: true,
      pagination: true,
      csvToolbar: true,
      columnSort: true,
      tagsColumn: true,
    });
  });

  it('respeta overrides explícitos del recurso', () => {
    const map: CrudResourceConfigMap = {
      demo: {
        ...buildBaseConfig(),
        supportsArchived: true,
        allowCreate: false,
        allowEdit: false,
        allowDelete: true,
        allowRestore: false,
        allowHardDelete: false,
        featureFlags: {
          csvToolbar: false,
          tagsColumn: false,
        },
      },
    };

    const config = getCrudPageConfigFromMap(map, 'demo');
    expect(config).not.toBeNull();
    if (!config) return;

    expect(config.supportsArchived).toBe(true);
    expect(config.allowCreate).toBe(false);
    expect(config.allowEdit).toBe(false);
    expect(config.allowDelete).toBe(true);
    expect(config.allowRestore).toBe(false);
    expect(config.allowHardDelete).toBe(false);
    expect(config.featureFlags).toMatchObject({
      searchBar: true,
      csvToolbar: false,
      tagsColumn: false,
    });
  });
});
