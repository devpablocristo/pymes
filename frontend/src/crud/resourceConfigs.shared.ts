import type { CrudFieldValue } from '../components/CrudPage';
import {
  formatCrudLinkedEntityImageUrlsToForm,
  parseCrudLinkedEntityImageUrlList,
} from '../modules/crud/crudLinkedEntityImageUrls';

export function asString(value: CrudFieldValue | undefined): string {
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return String(value ?? '');
}

export function asBoolean(value: CrudFieldValue | undefined): boolean {
  return value === true || asString(value).toLowerCase() === 'true';
}

export function asOptionalString(value: CrudFieldValue | undefined): string | undefined {
  const normalized = asString(value).trim();
  return normalized || undefined;
}

export function asNumber(value: CrudFieldValue | undefined): number {
  const normalized = asString(value).trim();
  if (!normalized) return 0;
  return Number(normalized);
}

export function asOptionalNumber(value: CrudFieldValue | undefined): number | undefined {
  const normalized = asString(value).trim();
  if (!normalized) return undefined;
  return Number(normalized);
}

export function formatDate(value?: string): string {
  if (!value) return '---';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('es-AR');
}

export function toDateTimeInput(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

export function toRFC3339(value: CrudFieldValue | undefined): string | undefined {
  const normalized = asString(value).trim();
  if (!normalized) return undefined;
  return new Date(normalized).toISOString();
}

export function parseJSONArray<T>(value: CrudFieldValue | undefined, errorMessage: string): T[] {
  const normalized = asString(value).trim();
  if (!normalized) return [];
  const parsed = JSON.parse(normalized) as T[];
  if (!Array.isArray(parsed)) {
    throw new Error(errorMessage);
  }
  return parsed;
}

export function stringifyJSON(value: unknown): string {
  if (!value) return '';
  return JSON.stringify(value, null, 2);
}

/** URLs de imágenes: una por línea o separadas por coma. */
export function parseImageURLList(value: CrudFieldValue | undefined): string[] {
  return parseCrudLinkedEntityImageUrlList(asString(value));
}

export function formatProductImageURLsToForm(urls: string[] | undefined, legacySingle?: string): string {
  return formatCrudLinkedEntityImageUrlsToForm(urls, legacySingle);
}

export function openExternalURL(url?: string): void {
  if (!url) return;
  const opened = window.open(url, '_blank', 'noopener,noreferrer');
  if (!opened) {
    window.alert(`Abrir enlace manualmente:\n${url}`);
  }
}
