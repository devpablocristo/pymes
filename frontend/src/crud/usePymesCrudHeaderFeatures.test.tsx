import { renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { CrudPageConfig } from '../components/CrudPage';
import { usePymesCrudHeaderFeatures } from './usePymesCrudHeaderFeatures';

vi.mock('./usePymesCrudConfigQuery', () => ({
  usePymesCrudConfigQuery: () => ({
    data: {
      label: 'proveedor',
      labelPlural: 'proveedores',
      labelPluralCap: 'Proveedores',
      featureFlags: {},
      columns: [],
    } satisfies Partial<CrudPageConfig<{ id: string }>>,
  }),
}));

vi.mock('../lib/useCrudListCreatedByMerge', () => ({
  useCrudListCreatedByMerge: () => ({
    preSearchFilter: null,
    listHeaderInlineSlot: null,
  }),
}));

describe('usePymesCrudHeaderFeatures', () => {
  it('no muestra pills cuando no hay etiquetas internas reales', () => {
    const { result } = renderHook(() =>
      usePymesCrudHeaderFeatures({
        resourceId: 'suppliers',
        items: [
          { id: '1', tags: [] },
          { id: '2', tags: ['   '] },
        ],
        matchesSearch: () => true,
      }),
    );

    expect(result.current.headerLeadSlot).toBeUndefined();
  });
});
