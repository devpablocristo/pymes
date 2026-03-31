import { describe, expect, it } from 'vitest';
import { buildCSV, normalizeCSVFieldValue, parseCSV } from '@devpablocristo/modules-crud-ui/csv';

describe('csv helpers', () => {
  it('builds CSV with BOM and escaped cells', () => {
    const csv = buildCSV(
      [
        { key: 'name', label: 'Nombre' },
        { key: 'notes', label: 'Notas' },
      ],
      [
        { name: 'Juan', notes: 'Cliente "VIP", revisar frenos' },
      ],
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
});
