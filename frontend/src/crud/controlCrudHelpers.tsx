import { parseListItemsFromResponse } from '@devpablocristo/core-browser/crud';
import type { CrudFieldValue } from '../components/CrudPage';
import { apiRequest } from '../lib/api';
import { renderCrudActiveBadge } from '../modules/crud';

export function parseControlCsv(value: CrudFieldValue | undefined): string[] {
  return String(value ?? '')
    .split(',')
    .map((entry) => entry.trim())
    .filter(Boolean);
}

export function formatControlTagList(tags?: string[]): string {
  return (tags ?? []).join(', ');
}

export function normalizeControlListItems<T extends { id: string | number }>(data: { items?: T[] | null }): Array<T & { id: string }> {
  return parseListItemsFromResponse<T>(data).map((row) => ({
    ...row,
    id: String(row.id),
  }));
}

export async function openControlSignedUrl(path: string): Promise<void> {
  const link = await apiRequest<{ url: string }>(path);
  if (link.url) {
    window.open(link.url, '_blank', 'noopener,noreferrer');
  }
}

export { renderCrudActiveBadge as renderControlActiveBadge };
