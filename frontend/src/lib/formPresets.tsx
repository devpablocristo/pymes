import type { CrudFieldValue } from '../components/CrudPage';

export type SelectOption = { label: string; value: string };

export const countryOptions: SelectOption[] = [
  { value: 'AR', label: 'Argentina' },
  { value: 'BO', label: 'Bolivia' },
  { value: 'BR', label: 'Brasil' },
  { value: 'CL', label: 'Chile' },
  { value: 'PY', label: 'Paraguay' },
  { value: 'UY', label: 'Uruguay' },
];

export const argentinaProvinceOptions: SelectOption[] = [
  'Buenos Aires',
  'CABA',
  'Catamarca',
  'Chaco',
  'Chubut',
  'Córdoba',
  'Corrientes',
  'Entre Ríos',
  'Formosa',
  'Jujuy',
  'La Pampa',
  'La Rioja',
  'Mendoza',
  'Misiones',
  'Neuquén',
  'Río Negro',
  'Salta',
  'San Juan',
  'San Luis',
  'Santa Cruz',
  'Santa Fe',
  'Santiago del Estero',
  'Tierra del Fuego',
  'Tucumán',
].map((label) => ({ value: label, label }));

export const customerGenderOptions: SelectOption[] = [
  { value: '', label: 'Prefiero completarlo después' },
  { value: 'female', label: 'Mujer' },
  { value: 'male', label: 'Hombre' },
  { value: 'non_binary', label: 'No binario' },
  { value: 'other', label: 'Otro' },
];

export const productUnitOptions: SelectOption[] = [
  { value: 'unit', label: 'Unidad' },
  { value: 'kg', label: 'Kilogramo' },
  { value: 'g', label: 'Gramo' },
  { value: 'lt', label: 'Litro' },
  { value: 'ml', label: 'Mililitro' },
  { value: 'm', label: 'Metro' },
  { value: 'm2', label: 'Metro cuadrado' },
  { value: 'box', label: 'Caja' },
  { value: 'pack', label: 'Pack' },
];

export const currencyOptions: SelectOption[] = [
  { value: 'ARS', label: 'Pesos argentinos (ARS)' },
  { value: 'USD', label: 'Dólares estadounidenses (USD)' },
  { value: 'EUR', label: 'Euros (EUR)' },
];

export const taxRateOptions: SelectOption[] = [
  { value: '', label: 'Usar configuración general' },
  { value: '0', label: '0%' },
  { value: '10.5', label: '10,5%' },
  { value: '21', label: '21%' },
  { value: '27', label: '27%' },
];

export const productCategoryOptions: SelectOption[] = [
  'Accesorios',
  'Alimentos',
  'Bebidas',
  'Electrónica',
  'Herramientas',
  'Higiene',
  'Indumentaria',
  'Insumos',
  'Librería',
  'Repuestos',
  'Servicios asociados',
].map((label) => ({ value: label.toLowerCase().replace(/\s+/g, '_'), label }));

export const productKindOptions: SelectOption[] = [
  { value: 'simple', label: 'Simple' },
  { value: 'variable', label: 'Variable' },
  { value: 'grouped', label: 'Agrupado' },
];

export const paymentMethodOptions: SelectOption[] = [
  { value: 'cash', label: 'Efectivo' },
  { value: 'transfer', label: 'Transferencia' },
  { value: 'card', label: 'Tarjeta' },
  { value: 'mixed', label: 'Mixto' },
];

export function asCrudString(value: CrudFieldValue | undefined): string {
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  return String(value ?? '');
}

export function normalizeArgentinaPhone(raw: string): string {
  const digits = raw.replace(/\D+/g, '');
  if (!digits) return '';
  if (digits.startsWith('549')) return `+${digits}`;
  if (digits.startsWith('54')) return `+${digits}`;
  if (digits.startsWith('0')) return `+54${digits.slice(1)}`;
  return `+54${digits}`;
}

export function parseMetadataStringMap(
  existing: unknown,
  updates: Record<string, string | undefined>,
): Record<string, unknown> {
  const base =
    existing && typeof existing === 'object' && !Array.isArray(existing)
      ? { ...(existing as Record<string, unknown>) }
      : {};
  for (const [key, value] of Object.entries(updates)) {
    const trimmed = String(value ?? '').trim();
    if (trimmed) base[key] = trimmed;
    else delete base[key];
  }
  return base;
}

