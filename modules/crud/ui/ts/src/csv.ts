export type CSVColumn = {
  key: string;
  label: string;
};

export type CSVFieldValue = string | boolean;

function escapeCell(value: string): string {
  if (/[",\n\r]/.test(value)) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

export function buildCSV(columns: CSVColumn[], rows: Array<Record<string, string>>): string {
  const lines = [
    columns.map((column) => escapeCell(column.key)).join(','),
    ...rows.map((row) => columns.map((column) => escapeCell(String(row[column.key] ?? ''))).join(',')),
  ];
  return `\uFEFF${lines.join('\n')}`;
}

export function downloadCSVFile(filename: string, content: string): void {
  const blob = new Blob([content], { type: 'text/csv;charset=utf-8' });
  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.URL.revokeObjectURL(url);
}

export async function pickCSVFile(): Promise<File | null> {
  return new Promise((resolve) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.csv,text/csv';
    input.onchange = () => resolve(input.files?.[0] ?? null);
    input.click();
  });
}

export function parseCSV(content: string): Array<Record<string, string>> {
  const normalized = content.replace(/^\uFEFF/, '');
  const rows: string[][] = [];
  let cell = '';
  let current: string[] = [];
  let inQuotes = false;

  for (let index = 0; index < normalized.length; index += 1) {
    const char = normalized[index];
    const next = normalized[index + 1];

    if (char === '"') {
      if (inQuotes && next === '"') {
        cell += '"';
        index += 1;
      } else {
        inQuotes = !inQuotes;
      }
      continue;
    }

    if (!inQuotes && char === ',') {
      current.push(cell);
      cell = '';
      continue;
    }

    if (!inQuotes && (char === '\n' || char === '\r')) {
      if (char === '\r' && next === '\n') {
        index += 1;
      }
      current.push(cell);
      rows.push(current);
      current = [];
      cell = '';
      continue;
    }

    cell += char;
  }

  if (cell.length > 0 || current.length > 0) {
    current.push(cell);
    rows.push(current);
  }

  const [headers = [], ...dataRows] = rows.filter((row) => row.some((value) => value.trim() !== ''));
  return dataRows.map((row) =>
    headers.reduce<Record<string, string>>((accumulator, header, index) => {
      accumulator[String(header).trim()] = String(row[index] ?? '').trim();
      return accumulator;
    }, {}),
  );
}

export function normalizeCSVFieldValue(value: string, type?: string): CSVFieldValue {
  if (type === 'checkbox') {
    const normalized = value.trim().toLowerCase();
    return ['true', '1', 'si', 'sí', 'yes'].includes(normalized);
  }
  return value;
}
