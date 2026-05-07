import { describe, expect, it } from 'vitest';
import { buildCSV, normalizeCSVFieldValue, parseCSV } from '@devpablocristo/modules-crud-ui/csv';
import type { CrudPageConfig } from '../components/CrudPage';
import { withCSVToolbar } from './csvToolbar';

describe('csv helpers', () => {
  it('builds CSV with BOM and escaped cells', () => {
    const csv = buildCSV(
      [
        { key: 'name', label: 'Nombre' },
        { key: 'notes', label: 'Notas' },
      ],
      [{ name: 'Juan', notes: 'Cliente "VIP", revisar frenos' }],
    );

    expect(csv.startsWith('\uFEFFname,notes\n')).toBe(true);
    expect(csv).toContain('"Cliente ""VIP"", revisar frenos"');
  });

  it('parses quoted CSV rows and normalizes checkbox values', () => {
    const rows = parseCSV('name,active,notes\n"Juan, Perez",si,"Revisar ""frenos"""');

    expect(rows).toEqual([
      {
        name: 'Juan, Perez',
        active: 'si',
        notes: 'Revisar "frenos"',
      },
    ]);
    expect(normalizeCSVFieldValue(rows[0].active, 'checkbox')).toBe(true);
  });

  it('keeps CSV toolbar injection idempotent', () => {
    const config: CrudPageConfig<{ id: string; name: string }> = {
      label: 'item',
      labelPlural: 'items',
      labelPluralCap: 'Items',
      basePath: '/v1/items',
      columns: [{ key: 'name', header: 'Nombre' }],
      formFields: [{ key: 'name', label: 'Nombre' }],
      searchText: (row) => row.name,
      toFormValues: (row) => ({ name: row.name }),
      isValid: () => true,
    };

    const wrappedTwice = withCSVToolbar('items', withCSVToolbar('items', config));
    const actionIds = (wrappedTwice.toolbarActions ?? []).map((action) => action.id);

    expect(actionIds.filter((id) => id === 'csv-import')).toHaveLength(1);
    expect(actionIds.filter((id) => id === 'csv-export')).toHaveLength(1);
  });
});
